package dashboard

import (
	"context"
	"encoding/csv"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"
)

type fakeGitHubService struct {
	mu             sync.Mutex
	repositories   []Repository
	repositoryErr  error
	repositoryGate chan struct{}
	pullGate       chan struct{}
	pullStarted    chan struct{}
	pullOnce       sync.Once
}

func (service *fakeGitHubService) Viewer(context.Context) (Viewer, error) {
	return Viewer{ID: 99, Login: "viewer", Name: "Viewer", Type: "User"}, nil
}

func (service *fakeGitHubService) Organizations(context.Context) ([]OwnerIdentity, error) {
	service.mu.Lock()
	defer service.mu.Unlock()
	seen := make(map[int64]struct{})
	organizations := make([]OwnerIdentity, 0)
	for _, repository := range service.repositories {
		if !strings.EqualFold(repository.Owner.Type, "Organization") {
			continue
		}
		if _, exists := seen[repository.Owner.ID]; exists {
			continue
		}
		seen[repository.Owner.ID] = struct{}{}
		organizations = append(organizations, repository.Owner)
	}
	return organizations, nil
}

func (service *fakeGitHubService) Repositories(ctx context.Context, _ OwnerIdentity) ([]Repository, error) {
	if service.repositoryGate != nil {
		select {
		case <-service.repositoryGate:
		case <-ctx.Done():
			return nil, ctx.Err()
		}
	}
	service.mu.Lock()
	defer service.mu.Unlock()
	if service.repositoryErr != nil {
		return nil, service.repositoryErr
	}
	return append([]Repository(nil), service.repositories...), nil
}

func (service *fakeGitHubService) PullRequests(ctx context.Context, repository Repository, previous PullCache) (PullCache, error) {
	if service.pullStarted != nil {
		service.pullOnce.Do(func() { close(service.pullStarted) })
	}
	if service.pullGate != nil {
		select {
		case <-service.pullGate:
		case <-ctx.Done():
			return PullCache{}, ctx.Err()
		}
	}
	return PullCache{
		Checkpoint: time.Now().UTC(),
		PullRequests: []PullRequest{{
			Number: 1, State: "open", CreatedAt: time.Date(2024, time.January, int(repository.ID), 0, 0, 0, 0, time.UTC),
			UpdatedAt: time.Now().UTC(), Author: Person{Login: "octocat"},
		}},
	}, nil
}

func TestManagerPublishesCommitDataBeforePullRequestStepCompletes(t *testing.T) {
	pullGate := make(chan struct{})
	pullStarted := make(chan struct{})
	github := &fakeGitHubService{
		repositories: []Repository{fakeRepository(1, "one")},
		pullGate:     pullGate,
		pullStarted:  pullStarted,
	}
	manager, err := NewManager(ManagerConfig{
		DataDir:     t.TempDir(),
		Concurrency: 1,
		Runner:      &fakeRepositoryRunner{},
		GitHubFactory: func(_ string, _ RateLimitCallbacks) (GitHubService, error) {
			return github, nil
		},
	})
	if err != nil {
		t.Fatalf("create manager: %v", err)
	}
	defer manager.Close()

	discoverAndSelect(t, manager, "memory-only-token", 10)
	select {
	case <-pullStarted:
	case <-time.After(5 * time.Second):
		t.Fatal("pull request workflow did not start")
	}

	deadline := time.Now().Add(5 * time.Second)
	for {
		response := manager.Dashboard()
		if response.Snapshot != nil && len(response.Snapshot.Repositories) == 1 && response.Snapshot.Repositories[0].Commits == 1 {
			if response.Sync.CompletedRepos != 0 {
				t.Fatalf("repository completed before pull request step: %+v", response.Sync)
			}
			if response.Snapshot.Repositories[0].SyncStatus != "processing_pull_requests" {
				t.Fatalf("unexpected incremental repository status: %+v", response.Snapshot.Repositories[0])
			}
			break
		}
		if time.Now().After(deadline) {
			t.Fatalf("commit data was not published incrementally: %+v", response)
		}
		time.Sleep(10 * time.Millisecond)
	}

	close(pullGate)
	waitForSync(t, manager)
	response := manager.Dashboard()
	if response.Snapshot.Repositories[0].PullRequests == nil || response.Snapshot.Repositories[0].PullRequests.Opened != 1 {
		t.Fatalf("pull request data was not added by the final workflow step: %+v", response.Snapshot.Repositories[0])
	}
}

func TestManagerSchedulesOnlyTheSelectedOwner(t *testing.T) {
	organization := fakeRepository(1, "organization-repo")
	personal := fakeRepository(2, "personal-repo")
	personal.FullName, personal.Owner = "person/personal-repo", OwnerIdentity{ID: 20, Login: "person", Type: "User"}
	runner := &fakeRepositoryRunner{}
	manager, err := NewManager(ManagerConfig{
		DataDir: t.TempDir(), Runner: runner,
		GitHubFactory: func(_ string, _ RateLimitCallbacks) (GitHubService, error) {
			return &fakeGitHubService{repositories: []Repository{organization, personal}}, nil
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	defer manager.Close()
	discoverAndSelect(t, manager, "token", organization.Owner.ID)
	waitForSync(t, manager)
	response := manager.Dashboard()
	if response.Snapshot == nil || len(response.Snapshot.Repositories) != 1 || response.Snapshot.Repositories[0].ID != organization.ID || response.Snapshot.Totals.Repositories != 1 {
		t.Fatalf("personal repository entered the dashboard: %+v", response.Snapshot)
	}
	runner.mu.Lock()
	clones := runner.clones
	runner.mu.Unlock()
	if clones != 1 {
		t.Fatalf("scheduled %d repositories, want only the organization repository", clones)
	}
}

func TestManagerCanSelectPersonalAccountAndRequiresExactOwner(t *testing.T) {
	personal := fakeRepository(1, "mine")
	personal.FullName = "viewer/mine"
	personal.Owner = OwnerIdentity{ID: 99, Login: "viewer", Type: "User"}
	otherUser := fakeRepository(2, "not-mine")
	otherUser.FullName = "someone/not-mine"
	otherUser.Owner = OwnerIdentity{ID: 100, Login: "someone", Type: "User"}
	manager, err := NewManager(ManagerConfig{
		DataDir: t.TempDir(), Runner: &fakeRepositoryRunner{},
		GitHubFactory: func(_ string, _ RateLimitCallbacks) (GitHubService, error) {
			return &fakeGitHubService{repositories: []Repository{personal, otherUser}}, nil
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	defer manager.Close()
	discoverAndSelect(t, manager, "token", 99)
	waitForSync(t, manager)
	response := manager.Dashboard()
	if response.Snapshot == nil || len(response.Snapshot.Repositories) != 1 || response.Snapshot.Repositories[0].ID != personal.ID {
		t.Fatalf("personal target was not isolated: %+v", response.Snapshot)
	}
}

type fakeRepositoryRunner struct {
	mu       sync.Mutex
	clones   int
	fetches  int
	analyses int
	heads    map[int64]string
}

func (runner *fakeRepositoryRunner) Sync(_ context.Context, _ string, repository Repository, destination string) (RepositorySyncResult, error) {
	runner.mu.Lock()
	defer runner.mu.Unlock()
	head := fmt.Sprintf("head-%d", repository.ID)
	if runner.heads != nil && runner.heads[repository.ID] != "" {
		head = runner.heads[repository.ID]
	}
	if _, err := os.Stat(destination); errors.Is(err, os.ErrNotExist) {
		runner.clones++
		return RepositorySyncResult{Head: head}, os.MkdirAll(destination, 0o700)
	}
	runner.fetches++
	return RepositorySyncResult{Head: head}, nil
}

func (runner *fakeRepositoryRunner) Analyze(_ context.Context, repositoryPath, outputPath string) (CommitStats, error) {
	runner.mu.Lock()
	runner.analyses++
	runner.mu.Unlock()
	file, err := os.Create(outputPath)
	if err != nil {
		return CommitStats{}, err
	}
	writer := csv.NewWriter(file)
	_ = writer.Write(commitCSVHeader)
	_ = writer.Write([]string{"2024-01-02T03:04:05Z", "2024-01-02", filepath.Base(repositoryPath), "1208574+octocat@users.noreply.github.com", "octocat", "The Octocat", "commit", "1", "2", "1", "3", "[]", "[]", "[]"})
	writer.Flush()
	if err := writer.Error(); err != nil {
		_ = file.Close()
		return CommitStats{}, err
	}
	if err := file.Close(); err != nil {
		return CommitStats{}, err
	}
	file, err = os.Open(outputPath)
	if err != nil {
		return CommitStats{}, err
	}
	defer file.Close()
	return ParseCommitCSV(file)
}

func TestManagerSyncDiffsRepositoriesPersistsCacheAndNeverPersistsPAT(t *testing.T) {
	dataDir := t.TempDir()
	github := &fakeGitHubService{
		repositories: []Repository{
			fakeRepository(1, "one"),
			fakeRepository(2, "two"),
		},
	}
	runner := &fakeRepositoryRunner{}
	manager, err := NewManager(ManagerConfig{
		DataDir: dataDir,
		Runner:  runner,
		GitHubFactory: func(_ string, _ RateLimitCallbacks) (GitHubService, error) {
			return github, nil
		},
	})
	if err != nil {
		t.Fatalf("create manager: %v", err)
	}
	defer manager.Close()

	const pat = "ghp_must-never-be-persisted"
	discoverAndSelect(t, manager, pat, 10)
	if _, err := manager.Start(); !errors.Is(err, ErrSyncActive) {
		t.Fatalf("expected single active sync guard, got %v", err)
	}
	waitForSync(t, manager)
	if response := manager.Dashboard(); response.Snapshot == nil || len(response.Snapshot.Repositories) != 2 {
		t.Fatalf("unexpected first snapshot: %+v", response.Snapshot)
	}

	github.mu.Lock()
	github.repositories = []Repository{fakeRepository(2, "two-renamed"), fakeRepository(3, "three")}
	github.mu.Unlock()
	if _, err := manager.Start(); err != nil {
		t.Fatalf("start second sync: %v", err)
	}
	waitForSync(t, manager)
	response := manager.Dashboard()
	if len(response.Snapshot.Repositories) != 2 || response.Snapshot.Repositories[0].ID != 3 && response.Snapshot.Repositories[1].ID != 3 {
		t.Fatalf("expected revoked repo 1 to be omitted and repo 3 added: %+v", response.Snapshot.Repositories)
	}
	runner.mu.Lock()
	clones, fetches := runner.clones, runner.fetches
	runner.mu.Unlock()
	if clones != 3 || fetches != 0 {
		t.Fatalf("expected three clones and unchanged repository reuse, got clones=%d fetches=%d", clones, fetches)
	}
	if _, statErr := os.Stat(filepath.Join(dataDir, "reports", "1", "commits.csv")); statErr != nil {
		t.Fatalf("removed repository cache was evicted: %v", statErr)
	}

	err = filepath.Walk(dataDir, func(path string, info os.FileInfo, walkErr error) error {
		if walkErr != nil || info.IsDir() {
			return walkErr
		}
		content, readErr := os.ReadFile(path)
		if readErr != nil {
			return readErr
		}
		if strings.Contains(string(content), pat) {
			t.Errorf("PAT was persisted in %s", path)
		}
		return nil
	})
	if err != nil {
		t.Fatalf("inspect persisted data: %v", err)
	}

	restarted, err := NewManager(ManagerConfig{DataDir: dataDir, Runner: runner, GitHubFactory: func(_ string, _ RateLimitCallbacks) (GitHubService, error) { return github, nil }})
	if err != nil {
		t.Fatalf("restart manager: %v", err)
	}
	defer restarted.Close()
	if restarted.Dashboard().Snapshot == nil || len(restarted.Dashboard().Snapshot.Repositories) != 2 {
		t.Fatalf("cached dashboard did not survive restart: %+v", restarted.Dashboard())
	}
	activity, err := restarted.Activity(ActivityQuery{Group: ActivityByOwner, Metric: ActivityCommits})
	if err != nil || len(activity.Series) == 0 {
		t.Fatalf("cached activity did not survive restart: %+v, %v", activity, err)
	}
}

func TestManagerIncrementalGitWorkUsesCatalogAndAnalyzedHead(t *testing.T) {
	dataDir := t.TempDir()
	pushedAt := time.Date(2025, time.January, 1, 0, 0, 0, 0, time.UTC)
	repository := fakeRepository(1, "one")
	repository.PushedAt = pushedAt
	github := &fakeGitHubService{repositories: []Repository{repository}}
	runner := &fakeRepositoryRunner{heads: map[int64]string{1: "head-one"}}
	manager, err := NewManager(ManagerConfig{
		DataDir: dataDir, Runner: runner,
		GitHubFactory: func(_ string, _ RateLimitCallbacks) (GitHubService, error) { return github, nil },
	})
	if err != nil {
		t.Fatal(err)
	}
	defer manager.Close()
	discoverAndSelect(t, manager, "token", 10)
	waitForSync(t, manager)

	if _, err := manager.Start(); err != nil {
		t.Fatal(err)
	}
	waitForSync(t, manager)
	runner.mu.Lock()
	clones, fetches, analyses := runner.clones, runner.fetches, runner.analyses
	runner.mu.Unlock()
	if clones != 1 || fetches != 0 || analyses != 1 {
		t.Fatalf("unchanged repository did extra Git work: clones=%d fetches=%d analyses=%d", clones, fetches, analyses)
	}

	github.mu.Lock()
	github.repositories[0].PushedAt = pushedAt.Add(time.Hour)
	github.mu.Unlock()
	if _, err := manager.Start(); err != nil {
		t.Fatal(err)
	}
	waitForSync(t, manager)
	runner.mu.Lock()
	fetches, analyses = runner.fetches, runner.analyses
	runner.heads[1] = "head-two"
	runner.mu.Unlock()
	if fetches != 1 || analyses != 1 {
		t.Fatalf("unchanged HEAD should reuse analysis: fetches=%d analyses=%d", fetches, analyses)
	}

	github.mu.Lock()
	github.repositories[0].PushedAt = pushedAt.Add(2 * time.Hour)
	github.mu.Unlock()
	if _, err := manager.Start(); err != nil {
		t.Fatal(err)
	}
	waitForSync(t, manager)
	runner.mu.Lock()
	fetches, analyses = runner.fetches, runner.analyses
	runner.mu.Unlock()
	if fetches != 2 || analyses != 2 {
		t.Fatalf("changed HEAD was not fully reanalyzed: fetches=%d analyses=%d", fetches, analyses)
	}

	github.mu.Lock()
	github.repositories[0].DefaultBranch = "trunk"
	github.mu.Unlock()
	if _, err := manager.Start(); err != nil {
		t.Fatal(err)
	}
	waitForSync(t, manager)
	runner.mu.Lock()
	fetches, analyses = runner.fetches, runner.analyses
	runner.mu.Unlock()
	if fetches != 3 || analyses != 3 {
		t.Fatalf("default-branch change did not force reanalysis: fetches=%d analyses=%d", fetches, analyses)
	}

	if err := os.RemoveAll(filepath.Join(dataDir, "repos", "1")); err != nil {
		t.Fatal(err)
	}
	if _, err := manager.Start(); err != nil {
		t.Fatal(err)
	}
	waitForSync(t, manager)
	runner.mu.Lock()
	clones, analyses = runner.clones, runner.analyses
	runner.mu.Unlock()
	if clones != 2 || analyses != 4 {
		t.Fatalf("missing clone did not force a fresh analysis: clones=%d analyses=%d", clones, analyses)
	}
}

func TestManagerSwitchesImmediatelyToTargetSnapshotAndIsolatesReports(t *testing.T) {
	first := fakeRepository(1, "first")
	second := fakeRepository(2, "second")
	second.FullName = "second-org/second"
	second.Owner = OwnerIdentity{ID: 20, Login: "second-org", Type: "Organization"}
	github := &fakeGitHubService{repositories: []Repository{first, second}}
	manager, err := NewManager(ManagerConfig{
		DataDir: t.TempDir(), Runner: &fakeRepositoryRunner{},
		GitHubFactory: func(_ string, _ RateLimitCallbacks) (GitHubService, error) { return github, nil },
	})
	if err != nil {
		t.Fatal(err)
	}
	defer manager.Close()
	if _, err := manager.DiscoverTargets("token"); err != nil {
		t.Fatal(err)
	}
	if _, err := manager.SelectTarget(10); err != nil {
		t.Fatal(err)
	}
	waitForSync(t, manager)
	if _, err := manager.SelectTarget(20); err != nil {
		t.Fatal(err)
	}
	waitForSync(t, manager)

	gate := make(chan struct{})
	github.repositoryGate = gate
	response, err := manager.SelectTarget(10)
	if err != nil {
		t.Fatal(err)
	}
	if response.Snapshot == nil || len(response.Snapshot.Repositories) != 1 || response.Snapshot.Repositories[0].Owner.ID != 10 || !response.Sync.Active() {
		t.Fatalf("cached target was not activated before refresh: %+v", response)
	}
	manager.mu.RLock()
	for _, report := range manager.reports {
		if report.Repository.Owner.ID != 10 {
			manager.mu.RUnlock()
			t.Fatalf("inactive target report leaked into active state: %+v", report.Repository)
		}
	}
	manager.mu.RUnlock()
	close(gate)
	waitForSync(t, manager)
}

func TestManagerIgnoresLegacyCombinedSnapshotAndKeepsLowLevelCaches(t *testing.T) {
	dataDir := t.TempDir()
	store, err := NewStore(dataDir)
	if err != nil {
		t.Fatal(err)
	}
	target := OwnerIdentity{ID: 10, Login: "org", Type: "Organization"}
	legacy := BuildSnapshot(Viewer{ID: 99, Login: "viewer"}, []Repository{fakeRepository(1, "legacy")}, map[int64]RepositoryReport{}, map[int64]string{})
	if err := store.SaveSnapshot(legacy); err != nil {
		t.Fatal(err)
	}
	if err := store.SaveConfiguration(target); err != nil {
		t.Fatal(err)
	}
	cloneMarker := filepath.Join(store.RepoPath(1), "marker")
	if err := os.MkdirAll(store.RepoPath(1), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(cloneMarker, []byte("keep"), 0o600); err != nil {
		t.Fatal(err)
	}
	manager, err := NewManager(ManagerConfig{DataDir: dataDir})
	if err != nil {
		t.Fatal(err)
	}
	defer manager.Close()
	if manager.Dashboard().Snapshot != nil {
		t.Fatalf("legacy combined snapshot was activated: %+v", manager.Dashboard().Snapshot)
	}
	if _, err := os.Stat(cloneMarker); err != nil {
		t.Fatalf("low-level cache was removed during upgrade: %v", err)
	}
}

func TestManagerEmptyTargetInvalidSelectionInaccessibleTargetAndPATRedaction(t *testing.T) {
	t.Run("empty target", func(t *testing.T) {
		manager, err := NewManager(ManagerConfig{
			DataDir: t.TempDir(), Runner: &fakeRepositoryRunner{},
			GitHubFactory: func(_ string, _ RateLimitCallbacks) (GitHubService, error) { return &fakeGitHubService{}, nil },
		})
		if err != nil {
			t.Fatal(err)
		}
		defer manager.Close()
		discoverAndSelect(t, manager, "token", 99)
		waitForSync(t, manager)
		response := manager.Dashboard()
		if response.Snapshot == nil || response.Snapshot.Totals.Repositories != 0 || len(response.Snapshot.Repositories) != 0 {
			t.Fatalf("zero-repository target was not published: %+v", response.Snapshot)
		}
		delivery, err := manager.InsightDelivery(InsightQuery{})
		if err != nil || delivery.Meta.Scope.OwnerID != 99 || delivery.Meta.Scope.Owner != "viewer" {
			t.Fatalf("empty target owner scope missing: %+v, %v", delivery.Meta.Scope, err)
		}
		if _, err := manager.SelectTarget(12345); err == nil {
			t.Fatal("undiscovered target selection succeeded")
		}
	})

	t.Run("inaccessible target", func(t *testing.T) {
		service := &fakeGitHubService{repositories: []Repository{fakeRepository(1, "one")}, repositoryErr: errors.New("forbidden")}
		manager, err := NewManager(ManagerConfig{
			DataDir: t.TempDir(), Runner: &fakeRepositoryRunner{},
			GitHubFactory: func(_ string, _ RateLimitCallbacks) (GitHubService, error) { return service, nil },
		})
		if err != nil {
			t.Fatal(err)
		}
		defer manager.Close()
		if _, err := manager.DiscoverTargets("token"); err != nil {
			t.Fatal(err)
		}
		if _, err := manager.SelectTarget(10); err != nil {
			t.Fatal(err)
		}
		deadline := time.Now().Add(5 * time.Second)
		for manager.Dashboard().Sync.Active() && time.Now().Before(deadline) {
			time.Sleep(10 * time.Millisecond)
		}
		if manager.Dashboard().Sync.State != SyncFailed {
			t.Fatalf("inaccessible target did not fail refresh: %+v", manager.Dashboard().Sync)
		}
	})

	t.Run("redaction", func(t *testing.T) {
		const pat = "ghp_do_not_echo"
		manager, err := NewManager(ManagerConfig{
			DataDir: t.TempDir(),
			GitHubFactory: func(token string, _ RateLimitCallbacks) (GitHubService, error) {
				return nil, fmt.Errorf("invalid credential %s", token)
			},
		})
		if err != nil {
			t.Fatal(err)
		}
		defer manager.Close()
		_, discoverErr := manager.DiscoverTargets(pat)
		if discoverErr == nil || strings.Contains(discoverErr.Error(), pat) {
			t.Fatalf("discovery error leaked PAT: %v", discoverErr)
		}
	})
}

func TestManagerReusesAndReplacesInMemoryPAT(t *testing.T) {
	var tokens []string
	manager, err := NewManager(ManagerConfig{
		DataDir: t.TempDir(),
		Runner:  &fakeRepositoryRunner{},
		GitHubFactory: func(token string, _ RateLimitCallbacks) (GitHubService, error) {
			tokens = append(tokens, token)
			return &fakeGitHubService{}, nil
		},
	})
	if err != nil {
		t.Fatalf("create manager: %v", err)
	}
	defer manager.Close()

	if _, err := manager.DiscoverTargets("first-token"); err != nil {
		t.Fatal(err)
	}
	if _, err := manager.SelectTarget(99); err != nil {
		t.Fatal(err)
	}
	waitForSync(t, manager)
	if _, err := manager.Start(); err != nil {
		t.Fatal(err)
	}
	waitForSync(t, manager)
	if _, err := manager.DiscoverTargets("replacement-token"); err != nil {
		t.Fatal(err)
	}
	if _, err := manager.Start(); err != nil {
		t.Fatal(err)
	}
	waitForSync(t, manager)
	if got, want := strings.Join(tokens, ","), "first-token,first-token,first-token,replacement-token,replacement-token"; got != want {
		t.Fatalf("unexpected token sequence: got %q want %q", got, want)
	}
}

func TestManagerUsesConfiguredGitHubTokenForRefresh(t *testing.T) {
	var received string
	manager, err := NewManager(ManagerConfig{
		DataDir:     t.TempDir(),
		GitHubToken: " configured-token ",
		Runner:      &fakeRepositoryRunner{},
		GitHubFactory: func(token string, _ RateLimitCallbacks) (GitHubService, error) {
			received = token
			return &fakeGitHubService{}, nil
		},
	})
	if err != nil {
		t.Fatalf("create manager: %v", err)
	}
	defer manager.Close()

	if _, err := manager.DiscoverTargets(""); err != nil {
		t.Fatal(err)
	}
	if _, err := manager.SelectTarget(99); err != nil {
		t.Fatalf("start sync with configured token: %v", err)
	}
	waitForSync(t, manager)
	if received != "configured-token" {
		t.Fatalf("configured token was not used: %q", received)
	}
}

func TestManagerRefreshRequiresPreviouslySuppliedPAT(t *testing.T) {
	manager, err := NewManager(ManagerConfig{DataDir: t.TempDir()})
	if err != nil {
		t.Fatalf("create manager: %v", err)
	}
	defer manager.Close()
	if _, err := manager.Start(); err == nil || err.Error() != "PAT is required" {
		t.Fatalf("expected missing PAT error, got %v", err)
	}
}

func TestManagerActivityUsesSnapshotTimeForLiveness(t *testing.T) {
	evaluatedAt := time.Date(2025, time.January, 10, 0, 0, 0, 0, time.UTC)
	manager := &Manager{
		snapshot: &Snapshot{GeneratedAt: evaluatedAt},
		reports: map[int64]RepositoryReport{1: {
			Repository: Repository{ID: 1, Owner: OwnerIdentity{ID: 10, Login: "org"}},
			Commits: CommitStats{
				FirstAt: time.Date(2024, time.January, 1, 0, 0, 0, 0, time.UTC),
				LastAt:  time.Date(2025, time.January, 1, 0, 0, 0, 0, time.UTC),
				Daily:   map[string]map[string]int{"2025-01-01": {"github:user": 1}},
			},
		}},
	}

	activity, err := manager.Activity(ActivityQuery{Group: ActivityByOwner, Metric: ActivityCommits, ExcludeDead: true})
	if err != nil || len(activity.Series) != 1 || activity.Series[0].Total != 1 {
		t.Fatalf("expected activity liveness to match snapshot evaluation time, got %+v, %v", activity, err)
	}
}

func fakeRepository(id int64, name string) Repository {
	return Repository{
		ID: id, Name: name, FullName: "org/" + name, CloneURL: "https://github.com/org/" + name + ".git",
		HTMLURL: "https://github.com/org/" + name, DefaultBranch: "main",
		Owner: OwnerIdentity{ID: 10, Login: "org", Type: "Organization"},
	}
}

func discoverAndSelect(t *testing.T, manager *Manager, token string, targetID int64) {
	t.Helper()
	if _, err := manager.DiscoverTargets(token); err != nil {
		t.Fatalf("discover GitHub targets: %v", err)
	}
	if _, err := manager.SelectTarget(targetID); err != nil {
		t.Fatalf("select GitHub target: %v", err)
	}
}

func waitForSync(t *testing.T, manager *Manager) {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		if !manager.Dashboard().Sync.Active() {
			if manager.Dashboard().Sync.State == SyncFailed {
				t.Fatalf("sync failed: %+v", manager.Dashboard().Sync)
			}
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatal("timed out waiting for sync")
}
