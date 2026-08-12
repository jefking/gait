package dashboard

import (
	"context"
	"encoding/csv"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"
)

type fakeGitHubService struct {
	mu           sync.Mutex
	repositories []Repository
	viewerGate   chan struct{}
	pullGate     chan struct{}
	pullStarted  chan struct{}
	pullOnce     sync.Once
}

func (service *fakeGitHubService) Viewer(context.Context) (Viewer, error) {
	if service.viewerGate != nil {
		<-service.viewerGate
	}
	return Viewer{Login: "viewer", Name: "Viewer"}, nil
}

func (service *fakeGitHubService) Repositories(context.Context) ([]Repository, error) {
	service.mu.Lock()
	defer service.mu.Unlock()
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

	if _, err := manager.Start("memory-only-token"); err != nil {
		t.Fatalf("start sync: %v", err)
	}
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

type fakeRepositoryRunner struct {
	mu      sync.Mutex
	clones  int
	fetches int
}

func (runner *fakeRepositoryRunner) Sync(_ context.Context, _ string, _ Repository, destination string) error {
	runner.mu.Lock()
	defer runner.mu.Unlock()
	if _, err := os.Stat(destination); errors.Is(err, os.ErrNotExist) {
		runner.clones++
		return os.MkdirAll(destination, 0o700)
	}
	runner.fetches++
	return nil
}

func (runner *fakeRepositoryRunner) Analyze(_ context.Context, repositoryPath, outputPath string) (CommitStats, error) {
	file, err := os.Create(outputPath)
	if err != nil {
		return CommitStats{}, err
	}
	writer := csv.NewWriter(file)
	_ = writer.Write(commitCSVHeader)
	_ = writer.Write([]string{"2024-01-02T03:04:05Z", "2024-01-02", filepath.Base(repositoryPath), "octocat", "The Octocat", "commit", "1", "2", "1", "3"})
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
	viewerGate := make(chan struct{})
	github := &fakeGitHubService{
		viewerGate: viewerGate,
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
	if _, err := manager.Start(pat); err != nil {
		t.Fatalf("start first sync: %v", err)
	}
	if _, err := manager.Start("another-token"); !errors.Is(err, ErrSyncActive) {
		t.Fatalf("expected single active sync guard, got %v", err)
	}
	close(viewerGate)
	waitForSync(t, manager)
	if response := manager.Dashboard(); response.Snapshot == nil || len(response.Snapshot.Repositories) != 2 {
		t.Fatalf("unexpected first snapshot: %+v", response.Snapshot)
	}

	github.mu.Lock()
	github.repositories = []Repository{fakeRepository(2, "two-renamed"), fakeRepository(3, "three")}
	github.mu.Unlock()
	if _, err := manager.Start(pat); err != nil {
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
	if clones != 3 || fetches != 1 {
		t.Fatalf("expected three clones and one fetch, got clones=%d fetches=%d", clones, fetches)
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

	for _, token := range []string{"first-token", "", "replacement-token", ""} {
		if _, err := manager.Start(token); err != nil {
			t.Fatalf("start sync: %v", err)
		}
		waitForSync(t, manager)
	}
	if got, want := strings.Join(tokens, ","), "first-token,first-token,replacement-token,replacement-token"; got != want {
		t.Fatalf("unexpected token sequence: got %q want %q", got, want)
	}
}

func TestManagerRefreshRequiresPreviouslySuppliedPAT(t *testing.T) {
	manager, err := NewManager(ManagerConfig{DataDir: t.TempDir()})
	if err != nil {
		t.Fatalf("create manager: %v", err)
	}
	defer manager.Close()
	if _, err := manager.Start(""); err == nil || err.Error() != "PAT is required" {
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
