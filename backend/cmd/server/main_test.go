package main

import (
	"os"
	"testing"
)

func TestGitHubTokenFromEnvironmentIsTrimmedAndRemoved(t *testing.T) {
	t.Setenv("GITHUB_TOKEN", "  github_pat_test  ")

	if got := githubTokenFromEnvironment(); got != "github_pat_test" {
		t.Fatalf("unexpected token: %q", got)
	}
	if _, exists := os.LookupEnv("GITHUB_TOKEN"); exists {
		t.Fatal("GITHUB_TOKEN must be removed from the child-process environment")
	}
}

func TestMissingGitHubTokenReturnsEmpty(t *testing.T) {
	_ = os.Unsetenv("GITHUB_TOKEN")

	if got := githubTokenFromEnvironment(); got != "" {
		t.Fatalf("expected no token, got %q", got)
	}
}
