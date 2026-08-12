package dashboard

import (
	"context"
	"encoding/base64"
	"encoding/csv"
	"errors"
	"fmt"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

type RepositoryRunner interface {
	Sync(context.Context, string, Repository, string) error
	Analyze(context.Context, string, string) (CommitStats, error)
}

type ExecRepositoryRunner struct {
	GitBinary      string
	AnalyzerBinary string
}

func NewExecRepositoryRunner() *ExecRepositoryRunner {
	return &ExecRepositoryRunner{GitBinary: "git", AnalyzerBinary: "git-changes-by-day"}
}

func (runner *ExecRepositoryRunner) Sync(ctx context.Context, token string, repository Repository, destination string) error {
	gitBinary := runner.GitBinary
	if gitBinary == "" {
		gitBinary = "git"
	}
	if repository.CloneURL == "" {
		return errors.New("repository has no HTTPS clone URL")
	}
	_, statErr := os.Stat(destination)
	exists := statErr == nil
	if statErr != nil && !errors.Is(statErr, os.ErrNotExist) {
		return fmt.Errorf("inspect repository workspace: %w", statErr)
	}
	if exists {
		if _, err := runner.runGit(ctx, gitBinary, destination, "", "", "rev-parse", "--is-inside-work-tree"); err != nil {
			if removeErr := os.RemoveAll(destination); removeErr != nil {
				return fmt.Errorf("replace invalid cached repository: %w", removeErr)
			}
			exists = false
		}
	}
	if !exists {
		if err := os.MkdirAll(filepath.Dir(destination), 0o700); err != nil {
			return fmt.Errorf("create repository workspace: %w", err)
		}
		if _, err := runner.runGit(ctx, gitBinary, "", token, repository.CloneURL,
			"clone", "--no-tags", "--single-branch", repository.CloneURL, destination); err != nil {
			_ = os.RemoveAll(destination)
			return fmt.Errorf("clone repository: %w", err)
		}
	} else {
		if _, err := runner.runGit(ctx, gitBinary, destination, "", "", "remote", "set-url", "origin", repository.CloneURL); err != nil {
			return fmt.Errorf("update repository remote: %w", err)
		}
		refspec := "+refs/heads/" + repository.DefaultBranch + ":refs/remotes/origin/" + repository.DefaultBranch
		if _, err := runner.runGit(ctx, gitBinary, destination, token, repository.CloneURL, "fetch", "--prune", "--no-tags", "origin", refspec); err != nil {
			output, remoteErr := runner.runGit(ctx, gitBinary, destination, token, repository.CloneURL,
				"ls-remote", "--heads", "origin", "refs/heads/"+repository.DefaultBranch)
			if remoteErr != nil || strings.TrimSpace(string(output)) != "" {
				return fmt.Errorf("fetch default branch: %w", err)
			}
			return nil
		}
	}

	remoteRef := "refs/remotes/origin/" + repository.DefaultBranch
	if _, err := runner.runGit(ctx, gitBinary, destination, "", "", "show-ref", "--verify", remoteRef); err != nil {
		if runner.emptyRepository(ctx, gitBinary, destination) {
			return nil
		}
		return fmt.Errorf("locate default branch %q: %w", repository.DefaultBranch, err)
	}
	if _, err := runner.runGit(ctx, gitBinary, destination, "", "", "checkout", "--force", "-B", repository.DefaultBranch, "origin/"+repository.DefaultBranch); err != nil {
		return fmt.Errorf("check out default branch: %w", err)
	}
	if _, err := runner.runGit(ctx, gitBinary, destination, "", "", "reset", "--hard", "origin/"+repository.DefaultBranch); err != nil {
		return fmt.Errorf("reset default branch: %w", err)
	}
	return nil
}

func (runner *ExecRepositoryRunner) Analyze(ctx context.Context, repositoryPath, outputPath string) (CommitStats, error) {
	analyzer := runner.AnalyzerBinary
	if analyzer == "" {
		analyzer = "git-changes-by-day"
	}
	command := exec.CommandContext(ctx, analyzer, "-repo", repositoryPath, "-text-out", outputPath)
	output, err := command.CombinedOutput()
	if err != nil {
		gitBinary := runner.GitBinary
		if gitBinary == "" {
			gitBinary = "git"
		}
		if !runner.emptyRepository(ctx, gitBinary, repositoryPath) {
			return CommitStats{}, fmt.Errorf("run git-changes-by-day: %w: %s", err, truncateOutput(output))
		}
		if writeErr := writeEmptyCommitCSV(outputPath); writeErr != nil {
			return CommitStats{}, writeErr
		}
	}
	file, err := os.Open(outputPath)
	if err != nil {
		return CommitStats{}, fmt.Errorf("open generated commit report: %w", err)
	}
	defer file.Close()
	stats, err := ParseCommitCSV(file)
	if err != nil {
		return CommitStats{}, fmt.Errorf("validate generated commit report: %w", err)
	}
	return stats, nil
}

func (runner *ExecRepositoryRunner) emptyRepository(ctx context.Context, gitBinary, repositoryPath string) bool {
	_, err := runner.runGit(ctx, gitBinary, repositoryPath, "", "", "rev-parse", "--verify", "HEAD")
	return err != nil
}

func (runner *ExecRepositoryRunner) runGit(ctx context.Context, gitBinary, directory, token, cloneURL string, args ...string) ([]byte, error) {
	command := exec.CommandContext(ctx, gitBinary, args...)
	command.Dir = directory
	command.Env = gitEnvironment(os.Environ(), token, cloneURL)
	output, err := command.CombinedOutput()
	if err != nil {
		return output, fmt.Errorf("git %s: %w: %s", strings.Join(args, " "), err, truncateOutput(output))
	}
	return output, nil
}

func gitEnvironment(environment []string, token, cloneURL string) []string {
	filtered := make([]string, 0, len(environment)+4)
	for _, value := range environment {
		if strings.HasPrefix(value, "GIT_CONFIG_COUNT=") || strings.HasPrefix(value, "GIT_CONFIG_KEY_0=") ||
			strings.HasPrefix(value, "GIT_CONFIG_VALUE_0=") || strings.HasPrefix(value, "GIT_TERMINAL_PROMPT=") ||
			strings.HasPrefix(value, "GIT_TRACE=") || strings.HasPrefix(value, "GIT_TRACE_CURL=") ||
			strings.HasPrefix(value, "GIT_CURL_VERBOSE=") {
			continue
		}
		filtered = append(filtered, value)
	}
	filtered = append(filtered, "GIT_TERMINAL_PROMPT=0")
	if token == "" || cloneURL == "" {
		return filtered
	}
	parsed, err := url.Parse(cloneURL)
	if err != nil || parsed.Scheme == "" || parsed.Host == "" {
		return filtered
	}
	credential := base64.StdEncoding.EncodeToString([]byte("x-access-token:" + token))
	filtered = append(filtered,
		"GIT_CONFIG_COUNT=1",
		"GIT_CONFIG_KEY_0=http."+parsed.Scheme+"://"+parsed.Host+"/.extraheader",
		"GIT_CONFIG_VALUE_0=Authorization: Basic "+credential,
	)
	return filtered
}

func truncateOutput(output []byte) string {
	const maximum = 2 << 10
	trimmed := strings.TrimSpace(string(output))
	if len(trimmed) > maximum {
		return trimmed[:maximum] + "…"
	}
	return trimmed
}

func writeEmptyCommitCSV(path string) error {
	file, err := os.Create(path)
	if err != nil {
		return fmt.Errorf("create empty commit report: %w", err)
	}
	writer := csv.NewWriter(file)
	writeErr := writer.Write(commitCSVHeader)
	writer.Flush()
	if writeErr == nil {
		writeErr = writer.Error()
	}
	closeErr := file.Close()
	if writeErr != nil {
		return fmt.Errorf("write empty commit report: %w", writeErr)
	}
	if closeErr != nil {
		return fmt.Errorf("close empty commit report: %w", closeErr)
	}
	return nil
}
