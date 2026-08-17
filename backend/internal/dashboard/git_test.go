package dashboard

import (
	"context"
	"encoding/base64"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestGitEnvironmentUsesEphemeralHeaderAndDisablesTracing(t *testing.T) {
	const token = "ghp_secret"
	environment := gitEnvironment(
		[]string{"PATH=/usr/bin", "GIT_TRACE=1", "GIT_CURL_VERBOSE=1", "GIT_CONFIG_COUNT=9", "GITHUB_TOKEN=ambient-secret", "GH_TOKEN=another-secret"},
		token,
		"https://github.com/org/repo.git",
	)
	joined := strings.Join(environment, "\n")
	if strings.Contains(joined, "GIT_TRACE=1") || strings.Contains(joined, "GIT_CURL_VERBOSE=1") {
		t.Fatalf("Git tracing should be disabled around credentials: %s", joined)
	}
	if strings.Contains(joined, "ambient-secret") || strings.Contains(joined, "another-secret") {
		t.Fatalf("ambient GitHub credentials appeared in the child environment: %s", joined)
	}
	if strings.Contains(joined, token) {
		t.Fatalf("raw PAT appeared in child environment: %s", joined)
	}
	expected := base64.StdEncoding.EncodeToString([]byte("x-access-token:" + token))
	if !strings.Contains(joined, "GIT_CONFIG_KEY_0=http.https://github.com/.extraheader") ||
		!strings.Contains(joined, "Authorization: Basic "+expected) {
		t.Fatalf("missing scoped ephemeral authorization header: %s", joined)
	}
}

func TestRepositoryRunnerClonesFetchesAndFollowsDefaultBranch(t *testing.T) {
	gitBinary, err := exec.LookPath("git")
	if err != nil {
		t.Skip("git is not installed")
	}
	root := t.TempDir()
	remote := filepath.Join(root, "remote.git")
	working := filepath.Join(root, "working")
	destination := filepath.Join(root, "cache")
	runTestGit(t, gitBinary, "", "init", "--bare", "--initial-branch=main", remote)
	runTestGit(t, gitBinary, "", "init", "--initial-branch=main", working)
	runTestGit(t, gitBinary, working, "config", "user.name", "Test User")
	runTestGit(t, gitBinary, working, "config", "user.email", "test@example.com")
	if err := os.WriteFile(filepath.Join(working, "history.txt"), []byte("first\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	runTestGit(t, gitBinary, working, "add", "history.txt")
	runTestGit(t, gitBinary, working, "commit", "-m", "first")
	runTestGit(t, gitBinary, working, "remote", "add", "origin", remote)
	runTestGit(t, gitBinary, working, "push", "-u", "origin", "main")

	runner := &ExecRepositoryRunner{GitBinary: gitBinary}
	repository := Repository{ID: 1, CloneURL: remote, DefaultBranch: "main"}
	if _, err := runner.Sync(context.Background(), "unused-token", repository, destination); err != nil {
		t.Fatalf("clone repository: %v", err)
	}
	firstHead := strings.TrimSpace(runTestGit(t, gitBinary, destination, "rev-parse", "HEAD"))

	runTestGit(t, gitBinary, working, "checkout", "-b", "next")
	if err := os.WriteFile(filepath.Join(working, "history.txt"), []byte("first\nsecond\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	runTestGit(t, gitBinary, working, "add", "history.txt")
	runTestGit(t, gitBinary, working, "commit", "-m", "second")
	runTestGit(t, gitBinary, working, "push", "-u", "origin", "next")
	repository.DefaultBranch = "next"
	if _, err := runner.Sync(context.Background(), "unused-token", repository, destination); err != nil {
		t.Fatalf("fetch renamed default branch: %v", err)
	}
	secondHead := strings.TrimSpace(runTestGit(t, gitBinary, destination, "rev-parse", "HEAD"))
	branch := strings.TrimSpace(runTestGit(t, gitBinary, destination, "branch", "--show-current"))
	if firstHead == secondHead || branch != "next" {
		t.Fatalf("expected latest next branch, first=%s second=%s branch=%s", firstHead, secondHead, branch)
	}
}

func TestRepositoryRunnerAcceptsEmptyRepository(t *testing.T) {
	gitBinary, err := exec.LookPath("git")
	if err != nil {
		t.Skip("git is not installed")
	}
	root := t.TempDir()
	remote := filepath.Join(root, "empty.git")
	destination := filepath.Join(root, "cache")
	runTestGit(t, gitBinary, "", "init", "--bare", "--initial-branch=main", remote)
	runner := &ExecRepositoryRunner{GitBinary: gitBinary}
	if _, err := runner.Sync(context.Background(), "unused-token", Repository{CloneURL: remote, DefaultBranch: "main"}, destination); err != nil {
		t.Fatalf("sync empty repository: %v", err)
	}
	if !runner.emptyRepository(context.Background(), gitBinary, destination) {
		t.Fatal("expected repository without HEAD to be treated as empty")
	}
}

func TestRepositoryRunnerClearsStaleHistoryWhenRemoteBecomesEmpty(t *testing.T) {
	gitBinary, err := exec.LookPath("git")
	if err != nil {
		t.Skip("git is not installed")
	}
	root := t.TempDir()
	remote := filepath.Join(root, "remote.git")
	working := filepath.Join(root, "working")
	destination := filepath.Join(root, "cache")
	runTestGit(t, gitBinary, "", "init", "--bare", "--initial-branch=main", remote)
	runTestGit(t, gitBinary, "", "init", "--initial-branch=main", working)
	runTestGit(t, gitBinary, working, "config", "user.name", "Test User")
	runTestGit(t, gitBinary, working, "config", "user.email", "test@example.com")
	if err := os.WriteFile(filepath.Join(working, "history.txt"), []byte("history\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	runTestGit(t, gitBinary, working, "add", "history.txt")
	runTestGit(t, gitBinary, working, "commit", "-m", "history")
	runTestGit(t, gitBinary, working, "remote", "add", "origin", remote)
	runTestGit(t, gitBinary, working, "push", "origin", "main")

	runner := &ExecRepositoryRunner{GitBinary: gitBinary}
	repository := Repository{CloneURL: remote, DefaultBranch: "main"}
	if _, err := runner.Sync(context.Background(), "unused-token", repository, destination); err != nil {
		t.Fatal(err)
	}
	runTestGit(t, gitBinary, remote, "config", "receive.denyDeleteCurrent", "ignore")
	runTestGit(t, gitBinary, working, "push", "origin", "--delete", "main")
	if _, err := runner.Sync(context.Background(), "unused-token", repository, destination); err != nil {
		t.Fatalf("sync repository after branch removal: %v", err)
	}
	if !runner.emptyRepository(context.Background(), gitBinary, destination) {
		t.Fatal("stale local history survived an empty remote")
	}
}

func TestRetainedLinesUsesRepositoryStateAtThirtyDayHorizon(t *testing.T) {
	gitBinary, err := exec.LookPath("git")
	if err != nil {
		t.Skip("git is unavailable")
	}
	repository := t.TempDir()
	runTestGit(t, gitBinary, repository, "init")
	runTestGit(t, gitBinary, repository, "config", "user.name", "Test User")
	runTestGit(t, gitBinary, repository, "config", "user.email", "test@example.com")
	path := filepath.Join(repository, "code.txt")
	if err := os.WriteFile(path, []byte("one\ntwo\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	runTestGit(t, gitBinary, repository, "add", "code.txt")
	runTestGit(t, gitBinary, repository, "commit", "-m", "first")
	first := strings.TrimSpace(runTestGit(t, gitBinary, repository, "rev-parse", "HEAD"))
	if err := os.WriteFile(path, []byte("one\nchanged\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	runTestGit(t, gitBinary, repository, "add", "code.txt")
	runTestGit(t, gitBinary, repository, "commit", "-m", "second")
	second := strings.TrimSpace(runTestGit(t, gitBinary, repository, "rev-parse", "HEAD"))
	if err := os.WriteFile(filepath.Join(repository, "other.txt"), []byte("later\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	runTestGit(t, gitBinary, repository, "add", "other.txt")
	runTestGit(t, gitBinary, repository, "commit", "-m", "third")
	third := strings.TrimSpace(runTestGit(t, gitBinary, repository, "rev-parse", "HEAD"))
	start := time.Date(2025, time.January, 1, 0, 0, 0, 0, time.UTC)
	events := []CommitEvent{
		{Hash: first, CommittedAt: start, Paths: []string{"code.txt"}, Parents: []string{"parent"}, LinesAdded: 2},
		{Hash: second, CommittedAt: start.AddDate(0, 0, 15), Paths: []string{"code.txt"}, Parents: []string{first}, LinesAdded: 1},
		{Hash: third, CommittedAt: start.AddDate(0, 0, 31), Paths: []string{"other.txt"}, Parents: []string{second}, LinesAdded: 1},
	}
	NewExecRepositoryRunner().measureRetainedLines(context.Background(), gitBinary, repository, events, 30, 50)
	if !events[0].RetentionMeasured || events[0].RetainedLines != 1 {
		t.Fatalf("expected one of two lines retained at day 30: %+v", events[0])
	}
	if !events[1].RetentionMeasured || events[1].RetainedLines != 1 {
		t.Fatalf("an inactive repository should still mature against its stable latest state: %+v", events[1])
	}
}

func runTestGit(t *testing.T, gitBinary, directory string, args ...string) string {
	t.Helper()
	command := exec.Command(gitBinary, args...)
	command.Dir = directory
	output, err := command.CombinedOutput()
	if err != nil {
		t.Fatalf("git %s: %v: %s", strings.Join(args, " "), err, output)
	}
	return string(output)
}
