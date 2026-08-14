package dashboard

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"
)

const pullCacheVersion = 3

const githubAPIVersion = "2026-03-10"

type GitHubService interface {
	Viewer(context.Context) (Viewer, error)
	Repositories(context.Context) ([]Repository, error)
	PullRequests(context.Context, Repository, PullCache) (PullCache, error)
}

// GitHubActionsService is optional so test doubles and installations without
// Actions access can continue to provide repository and pull-request data.
type GitHubActionsService interface {
	WorkflowRuns(context.Context, Repository, ActionsCache, time.Time) (ActionsCache, error)
}

type HTTPError struct {
	StatusCode int
	Message    string
}

func (err *HTTPError) Error() string {
	if err.Message == "" {
		return fmt.Sprintf("GitHub returned HTTP %d", err.StatusCode)
	}
	return fmt.Sprintf("GitHub returned HTTP %d: %s", err.StatusCode, err.Message)
}

func IsPullPermissionError(err error) bool {
	var responseError *HTTPError
	return errors.As(err, &responseError) && (responseError.StatusCode == http.StatusForbidden || responseError.StatusCode == http.StatusNotFound)
}

type RateLimitCallbacks struct {
	Waiting func(time.Time)
	Resumed func()
}

type githubClient struct {
	baseURL    *url.URL
	httpClient *http.Client
	token      string
	limiter    *rateLimiter
}

type rateLimiter struct {
	mu        sync.Mutex
	waitUntil time.Time
	callbacks RateLimitCallbacks
}

func NewGitHubClient(token, baseURL string, httpClient *http.Client, callbacks RateLimitCallbacks) (GitHubService, error) {
	parsedBase, err := url.Parse(baseURL)
	if err != nil {
		return nil, fmt.Errorf("parse GitHub API URL: %w", err)
	}
	if httpClient == nil {
		httpClient = &http.Client{Timeout: 30 * time.Second}
	}
	return &githubClient{
		baseURL:    parsedBase,
		httpClient: httpClient,
		token:      token,
		limiter:    &rateLimiter{callbacks: callbacks},
	}, nil
}

func (client *githubClient) Viewer(ctx context.Context) (Viewer, error) {
	var response struct {
		Login     string `json:"login"`
		Name      string `json:"name"`
		AvatarURL string `json:"avatar_url"`
		HTMLURL   string `json:"html_url"`
	}
	if err := client.get(ctx, "/user", nil, &response); err != nil {
		return Viewer{}, err
	}
	return Viewer(response), nil
}

func (client *githubClient) Repositories(ctx context.Context) ([]Repository, error) {
	repositories := make([]Repository, 0)
	for page := 1; ; page++ {
		query := url.Values{
			"affiliation": {"owner,collaborator,organization_member"},
			"visibility":  {"all"},
			"sort":        {"full_name"},
			"direction":   {"asc"},
			"per_page":    {"100"},
			"page":        {strconv.Itoa(page)},
		}
		var response []struct {
			ID            int64     `json:"id"`
			Name          string    `json:"name"`
			FullName      string    `json:"full_name"`
			CloneURL      string    `json:"clone_url"`
			HTMLURL       string    `json:"html_url"`
			Description   string    `json:"description"`
			DefaultBranch string    `json:"default_branch"`
			Private       bool      `json:"private"`
			Archived      bool      `json:"archived"`
			Fork          bool      `json:"fork"`
			CreatedAt     time.Time `json:"created_at"`
			Owner         struct {
				ID        int64  `json:"id"`
				Login     string `json:"login"`
				Type      string `json:"type"`
				AvatarURL string `json:"avatar_url"`
				HTMLURL   string `json:"html_url"`
			} `json:"owner"`
		}
		if err := client.get(ctx, "/user/repos", query, &response); err != nil {
			return nil, err
		}
		for _, item := range response {
			if !strings.EqualFold(item.Owner.Type, "Organization") {
				continue
			}
			defaultBranch := item.DefaultBranch
			if defaultBranch == "" {
				defaultBranch = "main"
			}
			repositories = append(repositories, Repository{
				ID:            item.ID,
				Name:          item.Name,
				FullName:      item.FullName,
				CloneURL:      item.CloneURL,
				HTMLURL:       item.HTMLURL,
				Description:   item.Description,
				DefaultBranch: defaultBranch,
				Private:       item.Private,
				Archived:      item.Archived,
				Fork:          item.Fork,
				CreatedAt:     item.CreatedAt,
				Owner: OwnerIdentity{
					ID:        item.Owner.ID,
					Login:     item.Owner.Login,
					Type:      item.Owner.Type,
					AvatarURL: item.Owner.AvatarURL,
					HTMLURL:   item.Owner.HTMLURL,
				},
			})
		}
		if len(response) < 100 {
			break
		}
	}
	return repositories, nil
}

func (client *githubClient) PullRequests(ctx context.Context, repository Repository, previous PullCache) (PullCache, error) {
	startedAt := time.Now().UTC()
	incremental := pullCacheEnrichmentComplete(previous)
	byNumber := make(map[int64]PullRequest, len(previous.PullRequests))
	for _, pull := range previous.PullRequests {
		byNumber[pull.Number] = pull
	}

	stop := false
	for page := 1; !stop; page++ {
		query := url.Values{
			"state":     {"all"},
			"sort":      {"updated"},
			"direction": {"desc"},
			"per_page":  {"100"},
			"page":      {strconv.Itoa(page)},
		}
		var response []struct {
			Number    int64      `json:"number"`
			State     string     `json:"state"`
			MergedAt  *time.Time `json:"merged_at"`
			ClosedAt  *time.Time `json:"closed_at"`
			CreatedAt time.Time  `json:"created_at"`
			UpdatedAt time.Time  `json:"updated_at"`
			User      struct {
				Login     string `json:"login"`
				AvatarURL string `json:"avatar_url"`
				Type      string `json:"type"`
			} `json:"user"`
		}
		endpoint := "/repos/" + url.PathEscape(repository.Owner.Login) + "/" + url.PathEscape(repository.Name) + "/pulls"
		if err := client.get(ctx, endpoint, query, &response); err != nil {
			return previous, err
		}
		for _, item := range response {
			if incremental && !previous.Checkpoint.IsZero() && !item.UpdatedAt.After(previous.Checkpoint) {
				stop = true
				break
			}
			pull := PullRequest{
				Number:    item.Number,
				State:     item.State,
				MergedAt:  item.MergedAt,
				ClosedAt:  item.ClosedAt,
				CreatedAt: item.CreatedAt,
				UpdatedAt: item.UpdatedAt,
				Author: Person{
					Login:     item.User.Login,
					Name:      item.User.Login,
					AvatarURL: item.User.AvatarURL,
					Type:      item.User.Type,
				},
			}
			if previousPull, exists := byNumber[item.Number]; exists {
				pull = previousPull
				pull.Number, pull.State, pull.MergedAt, pull.ClosedAt = item.Number, item.State, item.MergedAt, item.ClosedAt
				pull.CreatedAt, pull.UpdatedAt = item.CreatedAt, item.UpdatedAt
				pull.Author = Person{Login: item.User.Login, Name: item.User.Login, AvatarURL: item.User.AvatarURL, Type: item.User.Type}
			}
			if detail, detailErr := client.pullRequestDetail(ctx, repository, item.Number); detailErr == nil {
				pull.Additions, pull.Deletions, pull.ChangedFiles, pull.Commits = detail.Additions, detail.Deletions, detail.ChangedFiles, detail.Commits
				pull.MergeCommitSHA, pull.DetailComplete = detail.MergeCommitSHA, true
			} else if !IsPullPermissionError(detailErr) {
				return previous, detailErr
			}
			if reviews, reviewErr := client.pullRequestReviews(ctx, repository, item.Number); reviewErr == nil {
				pull.Reviews = reviews
			} else if !IsPullPermissionError(reviewErr) {
				return previous, reviewErr
			}
			if commits, complete, commitErr := client.pullRequestCommits(ctx, repository, item.Number, pull.Commits); commitErr == nil {
				pull.CommitEvidence, pull.CommitEvidenceComplete = commits, complete
			} else if !IsPullPermissionError(commitErr) {
				return previous, commitErr
			}
			byNumber[item.Number] = pull
		}
		if len(response) < 100 {
			break
		}
	}

	cache := PullCache{Version: pullCacheVersion, Checkpoint: startedAt, PullRequests: make([]PullRequest, 0, len(byNumber))}
	for _, pull := range byNumber {
		cache.PullRequests = append(cache.PullRequests, pull)
	}
	sort.Slice(cache.PullRequests, func(left, right int) bool {
		return cache.PullRequests[left].Number < cache.PullRequests[right].Number
	})
	return cache, nil
}

func pullCacheEnrichmentComplete(cache PullCache) bool {
	if cache.Version < pullCacheVersion {
		return false
	}
	for _, pull := range cache.PullRequests {
		commitEvidenceTruncatedAtLimit := pull.Commits > 250 && len(pull.CommitEvidence) == 250
		if !pull.DetailComplete || !pull.CommitEvidenceComplete && !commitEvidenceTruncatedAtLimit {
			return false
		}
	}
	return true
}

type pullRequestDetail struct {
	Additions      int    `json:"additions"`
	Deletions      int    `json:"deletions"`
	ChangedFiles   int    `json:"changed_files"`
	Commits        int    `json:"commits"`
	MergeCommitSHA string `json:"merge_commit_sha"`
}

func (client *githubClient) pullRequestDetail(ctx context.Context, repository Repository, number int64) (pullRequestDetail, error) {
	var detail pullRequestDetail
	endpoint := "/repos/" + url.PathEscape(repository.Owner.Login) + "/" + url.PathEscape(repository.Name) + "/pulls/" + strconv.FormatInt(number, 10)
	return detail, client.get(ctx, endpoint, nil, &detail)
}

func (client *githubClient) pullRequestCommits(ctx context.Context, repository Repository, number int64, expected int) ([]PullRequestCommit, bool, error) {
	commits := make([]PullRequestCommit, 0)
	endpoint := "/repos/" + url.PathEscape(repository.Owner.Login) + "/" + url.PathEscape(repository.Name) + "/pulls/" + strconv.FormatInt(number, 10) + "/commits"
	for page := 1; page <= 3; page++ {
		var response []struct {
			SHA    string `json:"sha"`
			Author *struct {
				Login     string `json:"login"`
				AvatarURL string `json:"avatar_url"`
				Type      string `json:"type"`
			} `json:"author"`
			Commit struct {
				Message string `json:"message"`
				Author  struct {
					Name string `json:"name"`
				} `json:"author"`
			} `json:"commit"`
		}
		if err := client.get(ctx, endpoint, url.Values{"per_page": {"100"}, "page": {strconv.Itoa(page)}}, &response); err != nil {
			return nil, false, err
		}
		for _, item := range response {
			if len(commits) == 250 {
				return commits, expected > 0 && expected <= len(commits), nil
			}
			author := Person{Name: item.Commit.Author.Name}
			if item.Author != nil {
				author.Login, author.AvatarURL, author.Type = item.Author.Login, item.Author.AvatarURL, item.Author.Type
				if author.Name == "" {
					author.Name = item.Author.Login
				}
			}
			commits = append(commits, PullRequestCommit{SHA: item.SHA, Message: item.Commit.Message, Author: author})
		}
		if len(response) < 100 {
			return commits, expected == 0 || expected <= len(commits), nil
		}
	}
	return commits, expected > 0 && expected <= len(commits), nil
}

func (client *githubClient) pullRequestReviews(ctx context.Context, repository Repository, number int64) ([]PullRequestReview, error) {
	reviews := make([]PullRequestReview, 0)
	for page := 1; ; page++ {
		query := url.Values{"per_page": {"100"}, "page": {strconv.Itoa(page)}}
		var response []struct {
			ID          int64      `json:"id"`
			State       string     `json:"state"`
			SubmittedAt *time.Time `json:"submitted_at"`
			User        struct {
				Login     string `json:"login"`
				AvatarURL string `json:"avatar_url"`
				Type      string `json:"type"`
			} `json:"user"`
		}
		endpoint := "/repos/" + url.PathEscape(repository.Owner.Login) + "/" + url.PathEscape(repository.Name) + "/pulls/" + strconv.FormatInt(number, 10) + "/reviews"
		if err := client.get(ctx, endpoint, query, &response); err != nil {
			return nil, err
		}
		for _, item := range response {
			reviews = append(reviews, PullRequestReview{
				ID: item.ID, State: item.State, SubmittedAt: item.SubmittedAt,
				Author: Person{Login: item.User.Login, Name: item.User.Login, AvatarURL: item.User.AvatarURL, Type: item.User.Type},
			})
		}
		if len(response) < 100 {
			break
		}
	}
	return reviews, nil
}

func (client *githubClient) WorkflowRuns(ctx context.Context, repository Repository, previous ActionsCache, earliest time.Time) (ActionsCache, error) {
	checkpoint := time.Now().UTC()
	if earliest.IsZero() {
		earliest = checkpoint.AddDate(0, -6, 0)
	}
	earliest = time.Date(earliest.UTC().Year(), earliest.UTC().Month(), 1, 0, 0, 0, 0, time.UTC)
	runsByKey := make(map[string]WorkflowRun)
	previousPartitions := make(map[string]ActionsPartition)
	for _, partition := range previous.Partitions {
		previousPartitions[partition.From.Format(time.DateOnly)+".."+partition.To.Format(time.DateOnly)] = partition
	}
	partitions := make([]ActionsPartition, 0)
	truncated := false
	for start := earliest; !start.After(checkpoint); start = start.AddDate(0, 1, 0) {
		end := start.AddDate(0, 1, 0).Add(-time.Second)
		if end.After(checkpoint) {
			end = checkpoint
		}
		partitionKey := start.Format(time.DateOnly) + ".." + end.Format(time.DateOnly)
		prior := previousPartitions[partitionKey]
		runs, incomplete, etag, notModified, err := client.workflowRunsWindow(ctx, repository, start, end, prior.ETag)
		if err != nil {
			return ActionsCache{}, err
		}
		if notModified {
			etag = prior.ETag
			for _, run := range previous.Runs {
				if !run.CreatedAt.Before(start) && !run.CreatedAt.After(end) {
					runs = append(runs, run)
				}
			}
		}
		partitions = append(partitions, ActionsPartition{From: start, To: end, ETag: etag, ValidatedAt: checkpoint})
		truncated = truncated || incomplete
		for _, run := range runs {
			runsByKey[strconv.FormatInt(run.ID, 10)+":"+strconv.Itoa(run.Attempt)] = run
		}
	}
	latestRuns := make([]WorkflowRun, 0, len(runsByKey))
	for _, run := range runsByKey {
		latestRuns = append(latestRuns, run)
	}
	for _, run := range latestRuns {
		for attempt := 1; attempt < run.Attempt; attempt++ {
			key := strconv.FormatInt(run.ID, 10) + ":" + strconv.Itoa(attempt)
			if _, exists := runsByKey[key]; exists {
				continue
			}
			priorAttempt, err := client.workflowRunAttempt(ctx, repository, run.ID, attempt)
			if err != nil {
				return ActionsCache{}, err
			}
			runsByKey[key] = priorAttempt
		}
	}
	runs := make([]WorkflowRun, 0, len(runsByKey))
	for _, run := range runsByKey {
		runs = append(runs, run)
	}
	sort.Slice(runs, func(i, j int) bool {
		if runs[i].CreatedAt.Equal(runs[j].CreatedAt) {
			if runs[i].ID == runs[j].ID {
				return runs[i].Attempt < runs[j].Attempt
			}
			return runs[i].ID < runs[j].ID
		}
		return runs[i].CreatedAt.Before(runs[j].CreatedAt)
	})
	coverageFrom, coverageTo := earliest, checkpoint
	return ActionsCache{Version: 1, Checkpoint: checkpoint, CoverageFrom: &coverageFrom, CoverageTo: &coverageTo, Truncated: truncated, Partitions: partitions, Runs: runs}, nil
}

func (client *githubClient) workflowRunsWindow(ctx context.Context, repository Repository, start, end time.Time, etag string) ([]WorkflowRun, bool, string, bool, error) {
	type runResponse struct {
		TotalCount int `json:"total_count"`
		Runs       []struct {
			ID         int64     `json:"id"`
			Attempt    int       `json:"run_attempt"`
			WorkflowID int64     `json:"workflow_id"`
			Name       string    `json:"name"`
			Event      string    `json:"event"`
			HeadSHA    string    `json:"head_sha"`
			Status     string    `json:"status"`
			Conclusion string    `json:"conclusion"`
			CreatedAt  time.Time `json:"created_at"`
			UpdatedAt  time.Time `json:"updated_at"`
			Pulls      []struct {
				Number int64 `json:"number"`
			} `json:"pull_requests"`
		} `json:"workflow_runs"`
	}
	endpoint := "/repos/" + url.PathEscape(repository.Owner.Login) + "/" + url.PathEscape(repository.Name) + "/actions/runs"
	query := url.Values{
		"event":    {"pull_request"},
		"created":  {start.Format(time.DateOnly) + ".." + end.Format(time.DateOnly)},
		"per_page": {"100"},
		"page":     {"1"},
	}
	var first runResponse
	responseETag, notModified, err := client.getConditional(ctx, endpoint, query, etag, &first)
	if err != nil || notModified {
		return nil, false, responseETag, notModified, err
	}
	if first.TotalCount > 900 && end.Sub(start) > 24*time.Hour {
		midpoint := start.Add(end.Sub(start) / 2)
		left, leftIncomplete, _, _, err := client.workflowRunsWindow(ctx, repository, start, midpoint, "")
		if err != nil {
			return nil, false, responseETag, false, err
		}
		right, rightIncomplete, _, _, err := client.workflowRunsWindow(ctx, repository, midpoint.Add(time.Second), end, "")
		return append(left, right...), leftIncomplete || rightIncomplete, responseETag, false, err
	}
	convert := func(items []struct {
		ID         int64     `json:"id"`
		Attempt    int       `json:"run_attempt"`
		WorkflowID int64     `json:"workflow_id"`
		Name       string    `json:"name"`
		Event      string    `json:"event"`
		HeadSHA    string    `json:"head_sha"`
		Status     string    `json:"status"`
		Conclusion string    `json:"conclusion"`
		CreatedAt  time.Time `json:"created_at"`
		UpdatedAt  time.Time `json:"updated_at"`
		Pulls      []struct {
			Number int64 `json:"number"`
		} `json:"pull_requests"`
	}) []WorkflowRun {
		converted := make([]WorkflowRun, 0, len(items))
		for _, item := range items {
			attempt := item.Attempt
			if attempt == 0 {
				attempt = 1
			}
			run := WorkflowRun{ID: item.ID, Attempt: attempt, WorkflowID: item.WorkflowID, Name: item.Name, Event: item.Event, HeadSHA: item.HeadSHA, Status: item.Status, Conclusion: item.Conclusion, CreatedAt: item.CreatedAt, UpdatedAt: item.UpdatedAt}
			if strings.EqualFold(item.Status, "completed") {
				completed := item.UpdatedAt
				run.CompletedAt = &completed
			}
			for _, pull := range item.Pulls {
				run.PullNumbers = append(run.PullNumbers, pull.Number)
			}
			converted = append(converted, run)
		}
		return converted
	}
	runs := convert(first.Runs)
	pages := (first.TotalCount + 99) / 100
	for page := 2; page <= pages && page <= 10; page++ {
		query.Set("page", strconv.Itoa(page))
		var response runResponse
		if err := client.get(ctx, endpoint, query, &response); err != nil {
			return nil, false, responseETag, false, err
		}
		runs = append(runs, convert(response.Runs)...)
	}
	return runs, first.TotalCount > 1000, responseETag, false, nil
}

func (client *githubClient) workflowRunAttempt(ctx context.Context, repository Repository, runID int64, attempt int) (WorkflowRun, error) {
	var item struct {
		ID         int64     `json:"id"`
		Attempt    int       `json:"run_attempt"`
		WorkflowID int64     `json:"workflow_id"`
		Name       string    `json:"name"`
		Event      string    `json:"event"`
		HeadSHA    string    `json:"head_sha"`
		Status     string    `json:"status"`
		Conclusion string    `json:"conclusion"`
		CreatedAt  time.Time `json:"created_at"`
		UpdatedAt  time.Time `json:"updated_at"`
		Pulls      []struct {
			Number int64 `json:"number"`
		} `json:"pull_requests"`
	}
	endpoint := "/repos/" + url.PathEscape(repository.Owner.Login) + "/" + url.PathEscape(repository.Name) + "/actions/runs/" + strconv.FormatInt(runID, 10) + "/attempts/" + strconv.Itoa(attempt)
	if err := client.get(ctx, endpoint, nil, &item); err != nil {
		return WorkflowRun{}, err
	}
	if item.Attempt == 0 {
		item.Attempt = attempt
	}
	run := WorkflowRun{ID: item.ID, Attempt: item.Attempt, WorkflowID: item.WorkflowID, Name: item.Name, Event: item.Event, HeadSHA: item.HeadSHA, Status: item.Status, Conclusion: item.Conclusion, CreatedAt: item.CreatedAt, UpdatedAt: item.UpdatedAt}
	if strings.EqualFold(item.Status, "completed") {
		completed := item.UpdatedAt
		run.CompletedAt = &completed
	}
	for _, pull := range item.Pulls {
		run.PullNumbers = append(run.PullNumbers, pull.Number)
	}
	return run, nil
}

func (client *githubClient) get(ctx context.Context, path string, query url.Values, target any) error {
	_, _, err := client.getConditional(ctx, path, query, "", target)
	return err
}

func (client *githubClient) getConditional(ctx context.Context, path string, query url.Values, etag string, target any) (string, bool, error) {
	for attempt := 0; attempt < 4; attempt++ {
		if err := client.limiter.wait(ctx); err != nil {
			return "", false, err
		}
		endpoint := client.baseURL.ResolveReference(&url.URL{Path: strings.TrimSuffix(client.baseURL.Path, "/") + path})
		endpoint.RawQuery = query.Encode()
		request, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint.String(), nil)
		if err != nil {
			return "", false, fmt.Errorf("create GitHub request: %w", err)
		}
		request.Header.Set("Accept", "application/vnd.github+json")
		request.Header.Set("Authorization", "Bearer "+client.token)
		request.Header.Set("User-Agent", "gait-dashboard")
		request.Header.Set("X-GitHub-Api-Version", githubAPIVersion)
		if etag != "" {
			request.Header.Set("If-None-Match", etag)
		}

		response, err := client.httpClient.Do(request)
		if err != nil {
			return "", false, fmt.Errorf("request GitHub: %w", err)
		}
		if resetAt, limited := rateLimitReset(response); limited {
			_, _ = io.Copy(io.Discard, io.LimitReader(response.Body, 8<<10))
			_ = response.Body.Close()
			client.limiter.block(resetAt)
			continue
		}
		responseETag := response.Header.Get("ETag")
		if response.StatusCode == http.StatusNotModified {
			_ = response.Body.Close()
			return responseETag, true, nil
		}
		if response.StatusCode < 200 || response.StatusCode >= 300 {
			message := githubErrorMessage(response.Body)
			_ = response.Body.Close()
			return responseETag, false, &HTTPError{StatusCode: response.StatusCode, Message: message}
		}
		decodeErr := json.NewDecoder(io.LimitReader(response.Body, 128<<20)).Decode(target)
		closeErr := response.Body.Close()
		if decodeErr != nil {
			return responseETag, false, fmt.Errorf("decode GitHub response: %w", decodeErr)
		}
		if closeErr != nil {
			return responseETag, false, fmt.Errorf("close GitHub response: %w", closeErr)
		}
		return responseETag, false, nil
	}
	return "", false, errors.New("GitHub rate limit retry budget exhausted")
}

func githubErrorMessage(reader io.Reader) string {
	var body struct {
		Message string `json:"message"`
	}
	if err := json.NewDecoder(io.LimitReader(reader, 8<<10)).Decode(&body); err != nil {
		return "request failed"
	}
	return strings.TrimSpace(body.Message)
}

func rateLimitReset(response *http.Response) (time.Time, bool) {
	if response.StatusCode != http.StatusTooManyRequests && response.StatusCode != http.StatusForbidden {
		return time.Time{}, false
	}
	if retryAfter := response.Header.Get("Retry-After"); retryAfter != "" {
		seconds, err := strconv.Atoi(retryAfter)
		if err == nil {
			return time.Now().Add(time.Duration(seconds) * time.Second), true
		}
	}
	if response.Header.Get("X-RateLimit-Remaining") == "0" {
		reset, err := strconv.ParseInt(response.Header.Get("X-RateLimit-Reset"), 10, 64)
		if err == nil {
			return time.Unix(reset, 0), true
		}
	}
	if response.StatusCode == http.StatusTooManyRequests {
		return time.Now().Add(time.Minute), true
	}
	return time.Time{}, false
}

func (limiter *rateLimiter) block(until time.Time) {
	limiter.mu.Lock()
	if until.After(limiter.waitUntil) {
		limiter.waitUntil = until
	}
	limiter.mu.Unlock()
}

func (limiter *rateLimiter) wait(ctx context.Context) error {
	limiter.mu.Lock()
	waitUntil := limiter.waitUntil
	limiter.mu.Unlock()
	if !waitUntil.After(time.Now()) {
		return nil
	}
	if limiter.callbacks.Waiting != nil {
		limiter.callbacks.Waiting(waitUntil)
	}
	timer := time.NewTimer(time.Until(waitUntil))
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		if limiter.callbacks.Resumed != nil {
			limiter.callbacks.Resumed()
		}
		return nil
	}
}
