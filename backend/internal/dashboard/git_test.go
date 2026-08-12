package dashboard

import (
	"context"
	"encoding/base64"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestGitEnvironmentUsesEphemeralHeaderAndDisablesTracing(t *testing.T) {
	const token = "ghp_secret"
	environment := gitEnvironment(
		[]string{"PATH=/usr/bin", "GIT_TRACE=1", "GIT_CURL_VERBOSE=1", "GIT_CONFIG_COUNT=9"},
		token,
		"https://github.com/org/repo.git",
	)
	joined := strings.Join(environment, "\n")
	if strings.Contains(joined, "GIT_TRACE=1") || strings.Contains(joined, "GIT_CURL_VERBOSE=1") {
		t.Fatalf("Git tracing should be disabled around credentials: %s", joined)
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
	if err := runner.Sync(context.Background(), "unused-token", repository, destination); err != nil {
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
	if err := runner.Sync(context.Background(), "unused-token", repository, destination); err != nil {
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
	if err := runner.Sync(context.Background(), "unused-token", Repository{CloneURL: remote, DefaultBranch: "main"}, destination); err != nil {
		t.Fatalf("sync empty repository: %v", err)
	}
	if !runner.emptyRepository(context.Background(), gitBinary, destination) {
		t.Fatal("expected repository without HEAD to be treated as empty")
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
