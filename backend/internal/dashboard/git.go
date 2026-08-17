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
	"sort"
	"strconv"
	"strings"
	"time"
)

type RepositoryRunner interface {
	Sync(context.Context, string, Repository, string) (RepositorySyncResult, error)
	Analyze(context.Context, string, string) (CommitStats, error)
}

type RepositoryCacheInspector interface {
	CachedHead(context.Context, string) (string, bool)
}

type RepositorySyncResult struct {
	Head string
}

type ExecRepositoryRunner struct {
	GitBinary      string
	AnalyzerBinary string
}

func NewExecRepositoryRunner() *ExecRepositoryRunner {
	return &ExecRepositoryRunner{GitBinary: "git", AnalyzerBinary: "git-changes-by-day"}
}

func (runner *ExecRepositoryRunner) CachedHead(ctx context.Context, repositoryPath string) (string, bool) {
	gitBinary := runner.GitBinary
	if gitBinary == "" {
		gitBinary = "git"
	}
	if _, err := runner.runGit(ctx, gitBinary, repositoryPath, "", "", "rev-parse", "--is-inside-work-tree"); err != nil {
		return "", false
	}
	head, err := runner.runGit(ctx, gitBinary, repositoryPath, "", "", "rev-parse", "HEAD")
	if err == nil {
		return strings.TrimSpace(string(head)), true
	}
	return "", runner.emptyRepository(ctx, gitBinary, repositoryPath)
}

func (runner *ExecRepositoryRunner) Sync(ctx context.Context, token string, repository Repository, destination string) (RepositorySyncResult, error) {
	gitBinary := runner.GitBinary
	if gitBinary == "" {
		gitBinary = "git"
	}
	if repository.CloneURL == "" {
		return RepositorySyncResult{}, errors.New("repository has no HTTPS clone URL")
	}
	_, statErr := os.Stat(destination)
	exists := statErr == nil
	if statErr != nil && !errors.Is(statErr, os.ErrNotExist) {
		return RepositorySyncResult{}, fmt.Errorf("inspect repository workspace: %w", statErr)
	}
	if exists {
		if _, err := runner.runGit(ctx, gitBinary, destination, "", "", "rev-parse", "--is-inside-work-tree"); err != nil {
			if removeErr := os.RemoveAll(destination); removeErr != nil {
				return RepositorySyncResult{}, fmt.Errorf("replace invalid cached repository: %w", removeErr)
			}
			exists = false
		}
	}
	if !exists {
		if err := os.MkdirAll(filepath.Dir(destination), 0o700); err != nil {
			return RepositorySyncResult{}, fmt.Errorf("create repository workspace: %w", err)
		}
		if _, err := runner.runGit(ctx, gitBinary, "", token, repository.CloneURL,
			"clone", "--no-tags", "--single-branch", repository.CloneURL, destination); err != nil {
			_ = os.RemoveAll(destination)
			return RepositorySyncResult{}, fmt.Errorf("clone repository: %w", err)
		}
	} else {
		if _, err := runner.runGit(ctx, gitBinary, destination, "", "", "remote", "set-url", "origin", repository.CloneURL); err != nil {
			return RepositorySyncResult{}, fmt.Errorf("update repository remote: %w", err)
		}
		refspec := "+refs/heads/" + repository.DefaultBranch + ":refs/remotes/origin/" + repository.DefaultBranch
		if _, err := runner.runGit(ctx, gitBinary, destination, token, repository.CloneURL, "fetch", "--prune", "--no-tags", "origin", refspec); err != nil {
			output, remoteErr := runner.runGit(ctx, gitBinary, destination, token, repository.CloneURL,
				"ls-remote", "--heads", "origin", "refs/heads/"+repository.DefaultBranch)
			if remoteErr != nil || strings.TrimSpace(string(output)) != "" {
				return RepositorySyncResult{}, fmt.Errorf("fetch default branch: %w", err)
			}
			// A previously populated repository can become empty. Re-cloning is
			// safer than analyzing the stale local branch that Git otherwise keeps.
			if removeErr := os.RemoveAll(destination); removeErr != nil {
				return RepositorySyncResult{}, fmt.Errorf("replace repository after default branch removal: %w", removeErr)
			}
			if _, cloneErr := runner.runGit(ctx, gitBinary, "", token, repository.CloneURL,
				"clone", "--no-tags", "--single-branch", repository.CloneURL, destination); cloneErr != nil {
				return RepositorySyncResult{}, fmt.Errorf("re-clone repository without a default branch: %w", cloneErr)
			}
		}
	}

	remoteRef := "refs/remotes/origin/" + repository.DefaultBranch
	if _, err := runner.runGit(ctx, gitBinary, destination, "", "", "show-ref", "--verify", remoteRef); err != nil {
		if runner.emptyRepository(ctx, gitBinary, destination) {
			return RepositorySyncResult{}, nil
		}
		return RepositorySyncResult{}, fmt.Errorf("locate default branch %q: %w", repository.DefaultBranch, err)
	}
	if _, err := runner.runGit(ctx, gitBinary, destination, "", "", "checkout", "--force", "-B", repository.DefaultBranch, "origin/"+repository.DefaultBranch); err != nil {
		return RepositorySyncResult{}, fmt.Errorf("check out default branch: %w", err)
	}
	if _, err := runner.runGit(ctx, gitBinary, destination, "", "", "reset", "--hard", "origin/"+repository.DefaultBranch); err != nil {
		return RepositorySyncResult{}, fmt.Errorf("reset default branch: %w", err)
	}
	head, err := runner.runGit(ctx, gitBinary, destination, "", "", "rev-parse", "HEAD")
	if err != nil {
		return RepositorySyncResult{}, fmt.Errorf("read default branch HEAD: %w", err)
	}
	return RepositorySyncResult{Head: strings.TrimSpace(string(head))}, nil
}

func (runner *ExecRepositoryRunner) Analyze(ctx context.Context, repositoryPath, outputPath string) (CommitStats, error) {
	gitBinary := runner.GitBinary
	if gitBinary == "" {
		gitBinary = "git"
	}
	analyzer := runner.AnalyzerBinary
	if analyzer == "" {
		analyzer = "git-changes-by-day"
	}
	command := exec.CommandContext(ctx, analyzer, "-repo", repositoryPath, "-text-out", outputPath)
	output, err := command.CombinedOutput()
	if err != nil {
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
	if err := runner.enrichCommitEvents(ctx, gitBinary, repositoryPath, stats.Events); err != nil {
		return CommitStats{}, fmt.Errorf("enrich commit events: %w", err)
	}
	return stats, nil
}

func (runner *ExecRepositoryRunner) enrichCommitEvents(ctx context.Context, gitBinary, repositoryPath string, events []CommitEvent) error {
	if len(events) == 0 {
		return nil
	}
	byHash := make(map[string]*CommitEvent, len(events))
	for index := range events {
		byHash[events[index].Hash] = &events[index]
	}
	messageCommand := exec.CommandContext(ctx, gitBinary, "-c", "log.showSignature=false", "log", "--format=%x1e%H%x1f%B%x00")
	messageCommand.Dir = repositoryPath
	messageOutput, err := messageCommand.Output()
	if err != nil {
		return fmt.Errorf("read commit messages: %w", err)
	}
	for _, raw := range strings.Split(string(messageOutput), "\x00") {
		record := strings.TrimPrefix(strings.TrimSpace(raw), "\x1e")
		parts := strings.SplitN(record, "\x1f", 2)
		if len(parts) != 2 {
			continue
		}
		if event := byHash[strings.TrimSpace(parts[0])]; event != nil {
			event.Message = strings.TrimSpace(parts[1])
		}
	}
	pathCommand := exec.CommandContext(ctx, gitBinary, "-c", "log.showSignature=false", "log", "--format=%x1e%H%x1f%P", "--name-only")
	pathCommand.Dir = repositoryPath
	pathOutput, err := pathCommand.Output()
	if err != nil {
		return fmt.Errorf("read commit paths: %w", err)
	}
	for _, raw := range strings.Split(string(pathOutput), "\x1e") {
		lines := strings.Split(strings.TrimSpace(raw), "\n")
		if len(lines) == 0 {
			continue
		}
		metadata := strings.SplitN(lines[0], "\x1f", 2)
		if len(metadata) != 2 {
			continue
		}
		event := byHash[strings.TrimSpace(metadata[0])]
		if event == nil {
			continue
		}
		event.Parents = strings.Fields(metadata[1])
		seen := make(map[string]struct{})
		for _, line := range lines[1:] {
			path := strings.TrimSpace(line)
			if path == "" {
				continue
			}
			if _, exists := seen[path]; exists {
				continue
			}
			seen[path] = struct{}{}
			event.Paths = append(event.Paths, path)
		}
	}
	for index := range events {
		// Current git-changes-by-day releases provide structured co-author
		// identities in the CSV. Older reports do not, so retain the full-message
		// trailer parser strictly as a backwards-compatible fallback.
		if len(events[index].Participants) == 0 {
			events[index].Participants = append([]ContributorMetrics{events[index].Author}, commitCoauthors(events[index].Message)...)
		}
		events[index].ExplicitRevert = isExplicitRevert(events[index].Message)
	}
	runner.measureRetainedLines(ctx, gitBinary, repositoryPath, events, 30, 500)
	return nil
}

// measureRetainedLines attributes lines in the repository state at the
// maturity horizon back to the commit that introduced them. The bounded cache
// keeps first-sync work predictable for very large histories; unmeasured
// commits remain explicitly unavailable instead of being assigned a proxy.
func (runner *ExecRepositoryRunner) measureRetainedLines(ctx context.Context, gitBinary, repositoryPath string, events []CommitEvent, survivalDays, maximumBlames int) {
	ordered := make([]*CommitEvent, 0, len(events))
	for index := range events {
		ordered = append(ordered, &events[index])
	}
	sort.Slice(ordered, func(i, j int) bool { return ordered[i].CommittedAt.Before(ordered[j].CommittedAt) })
	if len(ordered) == 0 {
		return
	}
	asOf := time.Now().UTC()
	type blameResult struct {
		counts   map[string]int
		measured bool
	}
	cache := make(map[string]blameResult)
	queries := 0
	for _, event := range ordered {
		if event.LinesAdded <= 0 || len(event.Parents) > 1 || len(event.Paths) == 0 {
			continue
		}
		maturesAt := event.CommittedAt.AddDate(0, 0, survivalDays)
		if maturesAt.After(asOf) {
			continue
		}
		horizon := ordered[0]
		for _, candidate := range ordered {
			if candidate.CommittedAt.After(maturesAt) {
				break
			}
			horizon = candidate
		}
		retained, complete := 0, true
		for _, path := range event.Paths {
			key := horizon.Hash + "\x00" + path
			result, exists := cache[key]
			if !exists {
				if queries >= maximumBlames {
					complete = false
					break
				}
				queries++
				command := exec.CommandContext(ctx, gitBinary, "-c", "blame.showEmail=false", "blame", "--line-porcelain", horizon.Hash, "--", path)
				command.Dir = repositoryPath
				output, err := command.Output()
				result = blameResult{counts: make(map[string]int), measured: err == nil}
				if err != nil {
					pathCheck := exec.CommandContext(ctx, gitBinary, "ls-tree", "--name-only", horizon.Hash, "--", path)
					pathCheck.Dir = repositoryPath
					pathOutput, pathErr := pathCheck.Output()
					result.measured = pathErr == nil && strings.TrimSpace(string(pathOutput)) == ""
				}
				if err == nil {
					for _, line := range strings.Split(string(output), "\n") {
						fields := strings.Fields(line)
						if len(fields) < 3 {
							continue
						}
						hash := strings.TrimPrefix(fields[0], "^")
						if len(hash) < 7 {
							continue
						}
						if _, parseErr := strconv.ParseUint(hash[:7], 16, 32); parseErr == nil {
							result.counts[hash]++
						}
					}
				}
				cache[key] = result
			}
			if !result.measured {
				complete = false
				break
			}
			retained += result.counts[event.Hash]
		}
		if complete {
			event.RetentionMeasured = true
			event.RetainedLines = min(retained, event.LinesAdded)
			event.RetentionDays = survivalDays
		}
	}
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
			strings.HasPrefix(value, "GIT_CURL_VERBOSE=") || strings.HasPrefix(value, "GITHUB_TOKEN=") ||
			strings.HasPrefix(value, "GH_TOKEN=") {
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
