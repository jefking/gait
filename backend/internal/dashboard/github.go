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

const githubAPIVersion = "2026-03-10"

type GitHubService interface {
	Viewer(context.Context) (Viewer, error)
	Repositories(context.Context) ([]Repository, error)
	PullRequests(context.Context, Repository, PullCache) (PullCache, error)
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
			if previous.Version >= 2 && !previous.Checkpoint.IsZero() && !item.UpdatedAt.After(previous.Checkpoint) {
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
				pull.Reviews = previousPull.Reviews
			}
			if reviews, reviewErr := client.pullRequestReviews(ctx, repository, item.Number); reviewErr == nil {
				pull.Reviews = reviews
			} else if !IsPullPermissionError(reviewErr) {
				return previous, reviewErr
			}
			byNumber[item.Number] = pull
		}
		if len(response) < 100 {
			break
		}
	}

	cache := PullCache{Version: 2, Checkpoint: startedAt, PullRequests: make([]PullRequest, 0, len(byNumber))}
	for _, pull := range byNumber {
		cache.PullRequests = append(cache.PullRequests, pull)
	}
	sort.Slice(cache.PullRequests, func(left, right int) bool {
		return cache.PullRequests[left].Number < cache.PullRequests[right].Number
	})
	return cache, nil
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

func (client *githubClient) get(ctx context.Context, path string, query url.Values, target any) error {
	for attempt := 0; attempt < 4; attempt++ {
		if err := client.limiter.wait(ctx); err != nil {
			return err
		}
		endpoint := client.baseURL.ResolveReference(&url.URL{Path: strings.TrimSuffix(client.baseURL.Path, "/") + path})
		endpoint.RawQuery = query.Encode()
		request, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint.String(), nil)
		if err != nil {
			return fmt.Errorf("create GitHub request: %w", err)
		}
		request.Header.Set("Accept", "application/vnd.github+json")
		request.Header.Set("Authorization", "Bearer "+client.token)
		request.Header.Set("User-Agent", "gait-dashboard")
		request.Header.Set("X-GitHub-Api-Version", githubAPIVersion)

		response, err := client.httpClient.Do(request)
		if err != nil {
			return fmt.Errorf("request GitHub: %w", err)
		}
		if resetAt, limited := rateLimitReset(response); limited {
			_, _ = io.Copy(io.Discard, io.LimitReader(response.Body, 8<<10))
			_ = response.Body.Close()
			client.limiter.block(resetAt)
			continue
		}
		if response.StatusCode < 200 || response.StatusCode >= 300 {
			message := githubErrorMessage(response.Body)
			_ = response.Body.Close()
			return &HTTPError{StatusCode: response.StatusCode, Message: message}
		}
		decodeErr := json.NewDecoder(io.LimitReader(response.Body, 128<<20)).Decode(target)
		closeErr := response.Body.Close()
		if decodeErr != nil {
			return fmt.Errorf("decode GitHub response: %w", decodeErr)
		}
		if closeErr != nil {
			return fmt.Errorf("close GitHub response: %w", closeErr)
		}
		return nil
	}
	return errors.New("GitHub rate limit retry budget exhausted")
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
