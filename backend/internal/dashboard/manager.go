package dashboard

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"net/http"
	"os"
	"sort"
	"strings"
	"sync"
	"time"
)

var ErrSyncActive = errors.New("a sync is already active")

type GitHubFactory func(string, RateLimitCallbacks) (GitHubService, error)

type ManagerConfig struct {
	DataDir       string
	Concurrency   int
	GitHubToken   string
	GitHubBaseURL string
	HTTPClient    *http.Client
	Runner        RepositoryRunner
	GitHubFactory GitHubFactory
}

type Manager struct {
	store       *Store
	runner      RepositoryRunner
	github      GitHubFactory
	concurrency int

	ctx    context.Context
	cancel context.CancelFunc
	wg     sync.WaitGroup

	mu                sync.RWMutex
	operationMu       sync.Mutex
	status            SyncStatus
	githubToken       string
	selectedTarget    *OwnerIdentity
	availableTargets  []OwnerIdentity
	snapshot          *Snapshot
	reports           map[int64]RepositoryReport
	identityOverrides map[string]IdentityOverride

	eventMu     sync.Mutex
	actionsMu   sync.Mutex
	subscribers map[chan DashboardEvent]struct{}
	revision    uint64
}

func NewManager(config ManagerConfig) (*Manager, error) {
	store, err := NewStore(config.DataDir)
	if err != nil {
		return nil, err
	}
	concurrency := config.Concurrency
	if concurrency <= 0 {
		concurrency = 4
	}
	if concurrency > 16 {
		concurrency = 16
	}
	runner := config.Runner
	if runner == nil {
		runner = NewExecRepositoryRunner()
	}
	baseURL := config.GitHubBaseURL
	if baseURL == "" {
		baseURL = "https://api.github.com"
	}
	githubFactory := config.GitHubFactory
	if githubFactory == nil {
		githubFactory = func(token string, callbacks RateLimitCallbacks) (GitHubService, error) {
			return NewGitHubClient(token, baseURL, config.HTTPClient, callbacks)
		}
	}
	selectedTarget, err := store.LoadConfiguration()
	if err != nil {
		return nil, err
	}
	var snapshot *Snapshot
	if selectedTarget != nil {
		snapshot, err = store.LoadTargetSnapshot(*selectedTarget)
		if err != nil {
			return nil, err
		}
	}
	reports, warnings := store.LoadReports(snapshot)
	for id, report := range reports {
		if selectedTarget == nil || !repositoryOwnedBy(report.Repository, *selectedTarget) {
			delete(reports, id)
		}
	}
	identityOverrides, identityErr := store.LoadIdentityOverrides()
	if identityErr != nil {
		return nil, identityErr
	}
	if snapshot != nil {
		repositories := make([]Repository, 0, len(snapshot.Repositories))
		repoStates := make(map[int64]string, len(snapshot.Repositories))
		for _, summary := range snapshot.Repositories {
			report, exists := reports[summary.ID]
			if !exists || selectedTarget == nil || !repositoryOwnedBy(report.Repository, *selectedTarget) {
				continue
			}
			repositories = append(repositories, report.Repository)
			repoStates[summary.ID] = report.SyncStatus
		}
		snapshot = BuildSnapshot(snapshot.Viewer, repositories, reports, repoStates)
		if err := store.SaveTargetSnapshot(*selectedTarget, snapshot); err != nil {
			warnings = append(warnings, "Dashboard liveness metadata could not be persisted")
		}
	}
	ctx, cancel := context.WithCancel(context.Background())
	manager := &Manager{
		store:             store,
		runner:            runner,
		github:            githubFactory,
		concurrency:       concurrency,
		ctx:               ctx,
		cancel:            cancel,
		status:            SyncStatus{State: SyncIdle, Warnings: warnings},
		githubToken:       strings.TrimSpace(config.GitHubToken),
		selectedTarget:    selectedTarget,
		snapshot:          snapshot,
		reports:           reports,
		identityOverrides: identityOverrides,
		subscribers:       make(map[chan DashboardEvent]struct{}),
	}
	return manager, nil
}

func (manager *Manager) Close() {
	manager.cancel()
	manager.wg.Wait()
	manager.mu.Lock()
	manager.githubToken = ""
	manager.mu.Unlock()
}

func (manager *Manager) Dashboard() DashboardResponse {
	manager.mu.RLock()
	defer manager.mu.RUnlock()
	configuration := GitHubConfiguration{
		AvailableTargets: append([]OwnerIdentity{}, manager.availableTargets...),
		TokenAvailable:   manager.githubToken != "",
	}
	if manager.selectedTarget != nil {
		target := *manager.selectedTarget
		configuration.SelectedTarget = &target
	}
	return DashboardResponse{Snapshot: manager.snapshot, Sync: cloneSyncStatus(manager.status), Configuration: configuration}
}

// Subscribe streams coalesced dashboard invalidation events until either the
// request or the manager is closed. Dashboard data itself remains available
// through the regular APIs, which also makes reconnects lossless.
func (manager *Manager) Subscribe(ctx context.Context) <-chan DashboardEvent {
	events := make(chan DashboardEvent, 64)
	manager.eventMu.Lock()
	manager.subscribers[events] = struct{}{}
	events <- DashboardEvent{Type: "dashboard", Revision: manager.revision}
	manager.eventMu.Unlock()

	go func() {
		select {
		case <-ctx.Done():
		case <-manager.ctx.Done():
		}
		manager.eventMu.Lock()
		if _, exists := manager.subscribers[events]; exists {
			delete(manager.subscribers, events)
			close(events)
		}
		manager.eventMu.Unlock()
	}()
	return events
}

func (manager *Manager) notify(eventType string) {
	manager.notifyRepository(eventType, nil)
}

func (manager *Manager) notifyRepository(eventType string, repository *RepositoryEventMetadata) {
	manager.eventMu.Lock()
	defer manager.eventMu.Unlock()
	manager.revision++
	event := DashboardEvent{Type: eventType, Revision: manager.revision, Repository: repository}
	for subscriber := range manager.subscribers {
		select {
		case subscriber <- event:
		default:
		}
	}
}

func (manager *Manager) Activity(query ActivityQuery) (ActivityResponse, error) {
	if query.Group != ActivityByOwner && query.Group != ActivityByContributor {
		return ActivityResponse{}, errors.New("group_by must be owner or contributor")
	}
	if query.Metric != ActivityCommits && query.Metric != ActivityPullRequests {
		return ActivityResponse{}, errors.New("metric must be commits or pull_requests")
	}
	if query.From != nil && query.To != nil && query.From.After(*query.To) {
		return ActivityResponse{}, errors.New("from must be on or before to")
	}
	manager.mu.RLock()
	reports := make(map[int64]RepositoryReport, len(manager.reports))
	for id, report := range manager.reports {
		reports[id] = report
	}
	evaluatedAt := time.Now().UTC()
	if manager.snapshot != nil {
		evaluatedAt = manager.snapshot.GeneratedAt
	}
	manager.mu.RUnlock()
	return BuildActivity(reports, query, evaluatedAt), nil
}

func (manager *Manager) DiscoverTargets(token string) (TargetDiscovery, error) {
	manager.operationMu.Lock()
	defer manager.operationMu.Unlock()
	token = strings.TrimSpace(token)
	manager.mu.RLock()
	if token == "" {
		token = manager.githubToken
	}
	active := manager.status.Active()
	manager.mu.RUnlock()
	if active {
		return TargetDiscovery{}, ErrSyncActive
	}
	if token == "" {
		return TargetDiscovery{}, errors.New("PAT is required")
	}
	github, err := manager.github(token, RateLimitCallbacks{})
	if err != nil {
		return TargetDiscovery{}, errors.New(sanitizeError(err, token))
	}
	viewer, err := github.Viewer(manager.ctx)
	if err != nil {
		return TargetDiscovery{}, errors.New(sanitizeError(err, token))
	}
	if viewer.ID <= 0 || strings.TrimSpace(viewer.Login) == "" {
		return TargetDiscovery{}, errors.New("GitHub returned an invalid authenticated account")
	}
	if viewer.Type == "" {
		viewer.Type = "User"
	}
	organizations, err := github.Organizations(manager.ctx)
	if err != nil {
		return TargetDiscovery{}, errors.New(sanitizeError(err, token))
	}
	targets := githubTargetChoices(viewer, organizations)
	manager.mu.Lock()
	manager.githubToken = token
	manager.availableTargets = append([]OwnerIdentity(nil), targets...)
	manager.mu.Unlock()
	manager.notify("configuration")
	return TargetDiscovery{Viewer: viewer, Targets: targets}, nil
}

func (manager *Manager) SelectTarget(targetID int64) (DashboardResponse, error) {
	manager.operationMu.Lock()
	defer manager.operationMu.Unlock()
	manager.mu.RLock()
	if manager.status.Active() {
		response := manager.dashboardLocked()
		manager.mu.RUnlock()
		return response, ErrSyncActive
	}
	var selected *OwnerIdentity
	for _, target := range manager.availableTargets {
		if target.ID == targetID {
			copyTarget := target
			selected = &copyTarget
			break
		}
	}
	manager.mu.RUnlock()
	if selected == nil {
		return DashboardResponse{}, errors.New("GitHub target must be selected from the discovered account or organizations")
	}
	snapshot, err := manager.store.LoadTargetSnapshot(*selected)
	if err != nil {
		return DashboardResponse{}, err
	}
	reports, warnings := manager.store.LoadReports(snapshot)
	for id, report := range reports {
		if !repositoryOwnedBy(report.Repository, *selected) {
			delete(reports, id)
		}
	}
	if err := manager.store.SaveConfiguration(*selected); err != nil {
		return DashboardResponse{}, err
	}
	manager.mu.Lock()
	manager.selectedTarget = selected
	manager.snapshot = snapshot
	manager.reports = reports
	manager.status = SyncStatus{State: SyncIdle, Warnings: warnings}
	manager.mu.Unlock()
	manager.notify("configuration")
	if _, err := manager.start(); err != nil {
		return manager.Dashboard(), err
	}
	return manager.Dashboard(), nil
}

func (manager *Manager) Start() (SyncStatus, error) {
	manager.operationMu.Lock()
	defer manager.operationMu.Unlock()
	return manager.start()
}

func (manager *Manager) start() (SyncStatus, error) {
	manager.mu.Lock()
	if manager.status.Active() {
		status := cloneSyncStatus(manager.status)
		manager.mu.Unlock()
		return status, ErrSyncActive
	}
	token := manager.githubToken
	if token == "" {
		manager.mu.Unlock()
		return SyncStatus{}, errors.New("PAT is required")
	}
	if manager.selectedTarget == nil {
		manager.mu.Unlock()
		return SyncStatus{}, errors.New("GitHub target is required")
	}
	target := *manager.selectedTarget
	now := time.Now().UTC()
	status := SyncStatus{
		ID:        newSyncID(),
		State:     SyncDiscovering,
		StartedAt: &now,
		Message:   "Connecting to GitHub",
	}
	manager.status = status
	manager.mu.Unlock()
	manager.notify("sync")

	manager.wg.Add(1)
	go func(syncID, pat string, selected OwnerIdentity) {
		defer manager.wg.Done()
		defer func() { pat = "" }()
		manager.runSync(manager.ctx, syncID, pat, selected)
	}(status.ID, token, target)
	return cloneSyncStatus(status), nil
}

func (manager *Manager) dashboardLocked() DashboardResponse {
	configuration := GitHubConfiguration{AvailableTargets: append([]OwnerIdentity{}, manager.availableTargets...), TokenAvailable: manager.githubToken != ""}
	if manager.selectedTarget != nil {
		target := *manager.selectedTarget
		configuration.SelectedTarget = &target
	}
	return DashboardResponse{Snapshot: manager.snapshot, Sync: cloneSyncStatus(manager.status), Configuration: configuration}
}

func (manager *Manager) runSync(ctx context.Context, syncID, token string, target OwnerIdentity) {
	callbacks := RateLimitCallbacks{
		Waiting: func(reset time.Time) { manager.setRateLimitWait(syncID, reset) },
		Resumed: func() { manager.resumeSync(syncID) },
	}
	github, err := manager.github(token, callbacks)
	if err != nil {
		manager.failSync(syncID, err, token)
		return
	}
	viewer, err := github.Viewer(ctx)
	if err != nil {
		manager.failSync(syncID, err, token)
		return
	}
	organizations, err := github.Organizations(ctx)
	if err != nil {
		manager.failSync(syncID, err, token)
		return
	}
	targets := githubTargetChoices(viewer, organizations)
	var refreshedTarget *OwnerIdentity
	for _, candidate := range targets {
		if candidate.ID == target.ID && strings.EqualFold(candidate.Type, target.Type) {
			candidateCopy := candidate
			refreshedTarget = &candidateCopy
			break
		}
	}
	manager.mu.Lock()
	manager.availableTargets = append([]OwnerIdentity(nil), targets...)
	if refreshedTarget != nil {
		manager.selectedTarget = refreshedTarget
	}
	manager.mu.Unlock()
	manager.notify("configuration")
	if refreshedTarget == nil {
		manager.failSync(syncID, errors.New("configured GitHub target is no longer an active or accessible account"), token)
		return
	}
	target = *refreshedTarget
	discoveryWarnings := make([]string, 0)
	if saveErr := manager.store.SaveConfiguration(target); saveErr != nil {
		discoveryWarnings = append(discoveryWarnings, "Updated GitHub target identity could not be persisted")
	}
	var repositories []Repository
	if catalogService, supported := github.(GitHubRepositoryCatalogService); supported {
		previousCatalog, loadErr := manager.store.LoadTargetRepositoryCatalog(target)
		if loadErr != nil && !errors.Is(loadErr, os.ErrNotExist) {
			discoveryWarnings = append(discoveryWarnings, "Cached repository catalog could not be read; refreshing it in full")
			previousCatalog = RepositoryCatalog{}
		}
		catalog, refreshErr := catalogService.RefreshRepositories(ctx, target, previousCatalog)
		if refreshErr != nil {
			manager.failSync(syncID, refreshErr, token)
			return
		}
		repositories = catalog.Repositories()
		if saveErr := manager.store.SaveTargetRepositoryCatalog(target, catalog); saveErr != nil {
			discoveryWarnings = append(discoveryWarnings, "Repository catalog could not be persisted")
		}
	} else {
		repositories, err = github.Repositories(ctx, target)
		if err != nil {
			manager.failSync(syncID, err, token)
			return
		}
	}
	ownedRepositories := repositories[:0]
	for _, repository := range repositories {
		if repositoryOwnedBy(repository, target) {
			ownedRepositories = append(ownedRepositories, repository)
		}
	}
	repositories = ownedRepositories
	sort.Slice(repositories, func(left, right int) bool {
		return strings.ToLower(repositories[left].FullName) < strings.ToLower(repositories[right].FullName)
	})

	manager.mu.RLock()
	previousReports := make(map[int64]RepositoryReport, len(manager.reports))
	for id, report := range manager.reports {
		previousReports[id] = report
	}
	manager.mu.RUnlock()

	reports := make(map[int64]RepositoryReport, len(repositories))
	repoStates := make(map[int64]string, len(repositories))
	cacheWarnings := make([]string, 0)
	for _, repository := range repositories {
		if previous, exists := previousReports[repository.ID]; exists {
			previous.Repository = repository
			previous.SyncStatus = "cached"
			reports[repository.ID] = previous
			repoStates[repository.ID] = "cached"
		} else {
			cached, loadedWarnings := manager.store.LoadCachedReport(repository)
			cacheWarnings = append(cacheWarnings, loadedWarnings...)
			if manager.store.CommitCacheComplete(repository.ID) || cached.Pulls != nil || cached.Actions != nil {
				reports[repository.ID] = cached
				repoStates[repository.ID] = "cached"
				previousReports[repository.ID] = cached
			} else {
				reports[repository.ID] = RepositoryReport{Repository: repository, SyncStatus: "pending"}
				repoStates[repository.ID] = "pending"
			}
		}
	}
	manager.updateStatus(syncID, func(status *SyncStatus) {
		status.State = SyncRunning
		status.TotalRepositories = len(repositories)
		status.CompletedRepos = 0
		status.FailedRepositories = 0
		status.Message = "Updating repositories"
		status.Warnings = nil
		status.RateLimitResetAt = nil
		for _, warning := range discoveryWarnings {
			appendStatusWarning(status, warning)
		}
		for _, warning := range cacheWarnings {
			appendStatusWarning(status, warning)
		}
	})

	if len(repositories) == 0 {
		snapshot := BuildSnapshot(viewer, repositories, reports, repoStates)
		manager.publish(snapshot, reports, 0)
		if err := manager.store.SaveTargetSnapshot(target, snapshot); err != nil {
			manager.appendWarning(syncID, "Dashboard snapshot could not be persisted")
		}
		manager.completeSync(syncID)
		return
	}

	type repositoryResult struct {
		repository Repository
		report     RepositoryReport
		warnings   []string
		final      bool
	}
	jobs := make(chan Repository)
	results := make(chan repositoryResult)
	workerCount := min(manager.concurrency, len(repositories))
	var workers sync.WaitGroup
	for range workerCount {
		workers.Add(1)
		go func() {
			defer workers.Done()
			for repository := range jobs {
				manager.markWorkflow(syncID, repository, "updating_git", "Checking Git history for "+repository.FullName)
				previous := previousReports[repository.ID]
				report, warnings := manager.processGitRepository(ctx, syncID, token, repository, previous)
				manager.markWorkflow(syncID, repository, "pull_requests", "Loading pull requests for "+repository.FullName)
				select {
				case results <- repositoryResult{repository: repository, report: report, warnings: warnings}:
				case <-ctx.Done():
					return
				}
				report, warnings = manager.processPullRequests(ctx, token, github, repository, report, warnings)
				manager.markWorkflow(syncID, repository, "delivery_evidence", "Loading GitHub Actions evidence for "+repository.FullName)
				report, warnings = manager.processDeliveryEvidence(ctx, token, github, repository, report, warnings)
				manager.markWorkflow(syncID, repository, "publishing", "Publishing statistics for "+repository.FullName)
				select {
				case results <- repositoryResult{repository: repository, report: report, warnings: warnings, final: true}:
				case <-ctx.Done():
					return
				}
			}
		}()
	}
	go func() {
		defer close(jobs)
		for _, repository := range repositories {
			select {
			case jobs <- repository:
			case <-ctx.Done():
				return
			}
		}
	}()
	go func() {
		workers.Wait()
		close(results)
	}()

	for result := range results {
		reports[result.repository.ID] = result.report
		repoStates[result.repository.ID] = result.report.SyncStatus
		if result.final {
			manager.updateStatus(syncID, func(status *SyncStatus) {
				status.CompletedRepos++
				if len(result.warnings) > 0 {
					status.FailedRepositories++
				}
				for _, warning := range result.warnings {
					appendStatusWarning(status, warning)
				}
			})
		}
		snapshot := BuildSnapshot(viewer, repositories, reports, repoStates)
		manager.publish(snapshot, reports, result.repository.ID)
		if err := manager.store.SaveTargetSnapshot(target, snapshot); err != nil {
			manager.appendWarning(syncID, "Dashboard snapshot could not be persisted")
		}
		if result.final {
			manager.removeWorkflow(syncID, result.repository.ID)
		}
	}
	if ctx.Err() != nil {
		manager.failSync(syncID, ctx.Err(), token)
		return
	}
	manager.completeSync(syncID)
}

func (manager *Manager) processGitRepository(ctx context.Context, syncID, token string, repository Repository, previous RepositoryReport) (RepositoryReport, []string) {
	report := previous
	report.Repository = repository
	report.SyncStatus = "processing_pull_requests"
	report.SyncMessage = ""
	warnings := make([]string, 0)

	repositoryPath := manager.store.RepoPath(repository.ID)
	_, cloneErr := os.Stat(repositoryPath)
	cloneAvailable := cloneErr == nil
	metadata, metadataErr := manager.store.LoadRepositoryCacheMetadata(repository.ID)
	cacheComplete := manager.store.CommitCacheComplete(repository.ID)
	metadataMatches := metadataErr == nil && metadata.Version == 1 && metadata.Analyzed &&
		metadata.Owner.ID == repository.Owner.ID && strings.EqualFold(metadata.Owner.Type, repository.Owner.Type) &&
		metadata.DefaultBranch == repository.DefaultBranch
	cloneCacheValid := cloneAvailable
	if inspector, supported := manager.runner.(RepositoryCacheInspector); supported && cloneAvailable && metadataMatches {
		cachedHead, valid := inspector.CachedHead(ctx, repositoryPath)
		cloneCacheValid = valid && cachedHead == metadata.AnalyzedHead
	}
	if cloneCacheValid && metadataMatches && cacheComplete && metadata.PushedAt.Equal(repository.PushedAt) {
		report.SyncMessage = "Cached commit history is current; pull requests are updating"
		return report, warnings
	}
	syncResult, syncErr := manager.runner.Sync(ctx, token, repository, repositoryPath)
	if syncErr != nil {
		warnings = append(warnings, repository.FullName+": Git update failed — "+sanitizeError(syncErr, token))
	} else if cloneCacheValid && metadataMatches && cacheComplete && metadata.AnalyzedHead == syncResult.Head {
		metadata.PushedAt = repository.PushedAt
		if err := manager.store.SaveRepositoryCacheMetadata(repository.ID, metadata); err != nil {
			warnings = append(warnings, repository.FullName+": repository cache metadata could not be persisted")
		}
	} else {
		manager.markWorkflow(syncID, repository, "analyzing", "Analyzing Git history for "+repository.FullName)
		temporary, err := manager.store.NewCommitTemp(repository.ID)
		if err != nil {
			warnings = append(warnings, repository.FullName+": commit report could not be created")
		} else {
			temporaryPath := temporary.Name()
			_ = temporary.Close()
			defer os.Remove(temporaryPath)
			commits, analyzeErr := manager.runner.Analyze(ctx, repositoryPath, temporaryPath)
			if analyzeErr != nil {
				warnings = append(warnings, repository.FullName+": commit analysis failed — "+sanitizeError(analyzeErr, token))
			} else if promoteErr := manager.store.PromoteCommitTemp(repository.ID, temporaryPath); promoteErr != nil {
				warnings = append(warnings, repository.FullName+": commit report could not be persisted")
			} else {
				report.Commits = commits
				if cacheErr := manager.store.SaveAnalysisCache(repository.ID, commits.Events); cacheErr != nil {
					warnings = append(warnings, repository.FullName+": enriched analysis cache could not be persisted")
				} else if cacheErr := manager.store.SaveRepositoryCacheMetadata(repository.ID, RepositoryCacheMetadata{
					Version: 1, Owner: repository.Owner, DefaultBranch: repository.DefaultBranch,
					PushedAt: repository.PushedAt, AnalyzedHead: syncResult.Head, Analyzed: true,
				}); cacheErr != nil {
					warnings = append(warnings, repository.FullName+": repository cache metadata could not be persisted")
				}
			}
		}
	}
	if len(warnings) > 0 {
		report.SyncMessage = strings.TrimPrefix(warnings[0], repository.FullName+": ")
	} else {
		report.SyncMessage = "Commit history updated; pull requests are updating"
	}
	return report, warnings
}

func (manager *Manager) processPullRequests(ctx context.Context, token string, github GitHubService, repository Repository, report RepositoryReport, warnings []string) (RepositoryReport, []string) {
	previousPulls, loadErr := manager.store.LoadPullCache(repository.ID)
	if loadErr != nil && !errors.Is(loadErr, os.ErrNotExist) {
		warnings = append(warnings, repository.FullName+": prior pull request cache could not be read")
		previousPulls = PullCache{}
	}
	pulls, pullErr := github.PullRequests(ctx, repository, previousPulls)
	if pullErr == nil {
		if err := manager.store.SavePullCache(repository.ID, pulls); err != nil {
			warnings = append(warnings, repository.FullName+": pull request cache could not be persisted")
			if len(previousPulls.PullRequests) > 0 || !previousPulls.Checkpoint.IsZero() {
				stats := BuildPullStats(previousPulls.PullRequests)
				report.Pulls = &stats
			}
		} else {
			stats := BuildPullStats(pulls.PullRequests)
			report.Pulls = &stats
		}
	} else if IsPullPermissionError(pullErr) {
		report.Pulls = nil
		warnings = append(warnings, repository.FullName+": PAT cannot read pull requests")
	} else {
		warnings = append(warnings, repository.FullName+": pull request update failed — "+sanitizeError(pullErr, token))
		if len(previousPulls.PullRequests) > 0 || !previousPulls.Checkpoint.IsZero() {
			stats := BuildPullStats(previousPulls.PullRequests)
			report.Pulls = &stats
		}
	}

	if len(warnings) > 0 {
		report.SyncStatus = "warning"
		report.SyncMessage = strings.TrimPrefix(warnings[0], repository.FullName+": ")
	} else {
		report.SyncStatus = "synced"
		report.SyncMessage = ""
	}
	return report, warnings
}

func (manager *Manager) processDeliveryEvidence(ctx context.Context, token string, github GitHubService, repository Repository, report RepositoryReport, warnings []string) (RepositoryReport, []string) {
	actionsService, supported := github.(GitHubActionsService)
	if !supported {
		return report, warnings
	}
	previous, loadErr := manager.store.LoadActionsCache(repository.ID)
	if loadErr != nil && !errors.Is(loadErr, os.ErrNotExist) {
		warnings = append(warnings, repository.FullName+": prior GitHub Actions cache could not be read")
		previous = ActionsCache{}
	}
	earliest := time.Time{}
	if report.Pulls != nil {
		for _, pull := range report.Pulls.PullRequests {
			if earliest.IsZero() || pull.CreatedAt.Before(earliest) {
				earliest = pull.CreatedAt
			}
		}
	}
	manager.actionsMu.Lock()
	actions, actionsErr := actionsService.WorkflowRuns(ctx, repository, previous, earliest)
	manager.actionsMu.Unlock()
	if actionsErr == nil {
		if err := manager.store.SaveActionsCache(repository.ID, actions); err != nil {
			warnings = append(warnings, repository.FullName+": GitHub Actions cache could not be persisted")
			report.Actions = &previous
		} else {
			report.Actions = &actions
		}
	} else if IsPullPermissionError(actionsErr) {
		previous.Version = 1
		previous.Checkpoint = time.Now().UTC()
		previous.PermissionDenied = true
		report.Actions = &previous
		if err := manager.store.SaveActionsCache(repository.ID, previous); err != nil {
			warnings = append(warnings, repository.FullName+": GitHub Actions permission state could not be persisted")
		}
		warnings = append(warnings, repository.FullName+": PAT cannot read GitHub Actions; delivery metrics remain available without build evidence")
	} else {
		warnings = append(warnings, repository.FullName+": GitHub Actions update failed — "+sanitizeError(actionsErr, token))
		if len(previous.Runs) > 0 || !previous.Checkpoint.IsZero() {
			report.Actions = &previous
		}
	}
	if len(warnings) > 0 {
		report.SyncStatus = "warning"
		report.SyncMessage = strings.TrimPrefix(warnings[0], repository.FullName+": ")
	}
	return report, warnings
}

func repositoryOwnedBy(repository Repository, target OwnerIdentity) bool {
	return repository.Owner.ID == target.ID && strings.EqualFold(strings.TrimSpace(repository.Owner.Type), strings.TrimSpace(target.Type))
}

func githubTargetChoices(viewer Viewer, organizations []OwnerIdentity) []OwnerIdentity {
	targets := make([]OwnerIdentity, 0, len(organizations)+1)
	targets = append(targets, OwnerIdentity{
		ID: viewer.ID, Login: viewer.Login, Type: "User", AvatarURL: viewer.AvatarURL, HTMLURL: viewer.HTMLURL,
	})
	for _, organization := range organizations {
		if organization.ID <= 0 || strings.TrimSpace(organization.Login) == "" {
			continue
		}
		organization.Type = "Organization"
		targets = append(targets, organization)
	}
	return targets
}

func (manager *Manager) publish(snapshot *Snapshot, reports map[int64]RepositoryReport, repositoryID int64) {
	copyReports := make(map[int64]RepositoryReport, len(reports))
	for id, report := range reports {
		copyReports[id] = report
	}
	manager.mu.Lock()
	manager.snapshot = snapshot
	manager.reports = copyReports
	manager.mu.Unlock()
	var metadata *RepositoryEventMetadata
	if repositoryID != 0 {
		for _, repository := range snapshot.Repositories {
			if repository.ID == repositoryID {
				metadata = &RepositoryEventMetadata{
					ID:         repository.ID,
					FullName:   repository.FullName,
					SyncStatus: repository.SyncStatus,
					Liveness:   repository.Liveness,
				}
				break
			}
		}
	}
	manager.notifyRepository("snapshot", metadata)
}

func (manager *Manager) completeSync(syncID string) {
	manager.updateStatus(syncID, func(status *SyncStatus) {
		now := time.Now().UTC()
		status.FinishedAt = &now
		status.CurrentRepos = nil
		status.CurrentWorkflows = nil
		status.RateLimitResetAt = nil
		if len(status.Warnings) > 0 {
			status.State = SyncCompleteWithWarnings
			status.Message = "Sync completed with warnings"
		} else {
			status.State = SyncComplete
			status.Message = "Sync complete"
		}
	})
}

func (manager *Manager) failSync(syncID string, err error, token string) {
	manager.updateStatus(syncID, func(status *SyncStatus) {
		now := time.Now().UTC()
		status.State = SyncFailed
		status.FinishedAt = &now
		status.CurrentRepos = nil
		status.CurrentWorkflows = nil
		status.RateLimitResetAt = nil
		status.Message = fatalSyncMessage(err, token)
	})
}

func fatalSyncMessage(err error, token string) string {
	var responseError *HTTPError
	if errors.As(err, &responseError) {
		switch responseError.StatusCode {
		case http.StatusUnauthorized:
			return "GitHub rejected the PAT. Check the token and try again."
		case http.StatusForbidden:
			return "GitHub denied repository discovery. Check token permissions and organization SSO."
		}
	}
	if errors.Is(err, context.Canceled) {
		return "Sync was cancelled because the server is shutting down."
	}
	return "Sync failed: " + sanitizeError(err, token)
}

func sanitizeError(err error, token string) string {
	message := strings.TrimSpace(err.Error())
	if token != "" {
		message = strings.ReplaceAll(message, token, "[redacted]")
	}
	message = strings.Join(strings.Fields(message), " ")
	if len(message) > 240 {
		message = message[:240] + "…"
	}
	return message
}

func (manager *Manager) setRateLimitWait(syncID string, reset time.Time) {
	manager.updateStatus(syncID, func(status *SyncStatus) {
		reset = reset.UTC()
		status.State = SyncWaitingRateLimit
		status.RateLimitResetAt = &reset
		status.Message = "Waiting for GitHub rate limit reset"
	})
}

func (manager *Manager) resumeSync(syncID string) {
	manager.updateStatus(syncID, func(status *SyncStatus) {
		status.State = SyncRunning
		status.RateLimitResetAt = nil
		status.Message = "Updating repositories"
	})
}

func (manager *Manager) markWorkflow(syncID string, repository Repository, stage, message string) {
	manager.updateStatus(syncID, func(status *SyncStatus) {
		workflow := RepositoryWorkflow{RepositoryID: repository.ID, FullName: repository.FullName, Stage: stage, Message: message}
		found := false
		for index := range status.CurrentWorkflows {
			if status.CurrentWorkflows[index].RepositoryID == repository.ID {
				status.CurrentWorkflows[index] = workflow
				found = true
				break
			}
		}
		if !found {
			status.CurrentWorkflows = append(status.CurrentWorkflows, workflow)
		}
		sort.Slice(status.CurrentWorkflows, func(left, right int) bool {
			return strings.ToLower(status.CurrentWorkflows[left].FullName) < strings.ToLower(status.CurrentWorkflows[right].FullName)
		})
		status.CurrentRepos = status.CurrentRepos[:0]
		for _, current := range status.CurrentWorkflows {
			status.CurrentRepos = append(status.CurrentRepos, current.FullName)
		}
	})
}

func (manager *Manager) removeWorkflow(syncID string, repositoryID int64) {
	manager.updateStatus(syncID, func(status *SyncStatus) {
		workflows := status.CurrentWorkflows[:0]
		for _, workflow := range status.CurrentWorkflows {
			if workflow.RepositoryID != repositoryID {
				workflows = append(workflows, workflow)
			}
		}
		status.CurrentWorkflows = workflows
		status.CurrentRepos = status.CurrentRepos[:0]
		for _, workflow := range workflows {
			status.CurrentRepos = append(status.CurrentRepos, workflow.FullName)
		}
	})
}

func (manager *Manager) appendWarning(syncID, warning string) {
	manager.updateStatus(syncID, func(status *SyncStatus) { appendStatusWarning(status, warning) })
}

func appendStatusWarning(status *SyncStatus, warning string) {
	const maximumWarnings = 100
	if len(status.Warnings) < maximumWarnings {
		status.Warnings = append(status.Warnings, warning)
		return
	}
	if len(status.Warnings) == maximumWarnings {
		status.Warnings = append(status.Warnings, "Additional warnings omitted")
	}
}

func (manager *Manager) updateStatus(syncID string, update func(*SyncStatus)) {
	manager.mu.Lock()
	if manager.status.ID != syncID {
		manager.mu.Unlock()
		return
	}
	update(&manager.status)
	manager.mu.Unlock()
	manager.notify("sync")
}

func cloneSyncStatus(status SyncStatus) SyncStatus {
	status.CurrentRepos = append([]string(nil), status.CurrentRepos...)
	status.CurrentWorkflows = append([]RepositoryWorkflow(nil), status.CurrentWorkflows...)
	status.Warnings = append([]string(nil), status.Warnings...)
	return status
}

func newSyncID() string {
	buffer := make([]byte, 8)
	if _, err := rand.Read(buffer); err == nil {
		return hex.EncodeToString(buffer)
	}
	return fmt.Sprintf("%d", time.Now().UnixNano())
}
