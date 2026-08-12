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

func (service *fakeGitHubService) PullRequests(_ context.Context, repository Repository, previous PullCache) (PullCache, error) {
	return PullCache{
		Checkpoint: time.Now().UTC(),
		PullRequests: []PullRequest{{
			Number: 1, State: "open", CreatedAt: time.Date(2024, time.January, int(repository.ID), 0, 0, 0, 0, time.UTC),
			UpdatedAt: time.Now().UTC(), Author: Person{Login: "octocat"},
		}},
	}, nil
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
