package dashboard

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

func TestHTTPErrorFormattingAndPermissionClassification(t *testing.T) {
	tests := []struct {
		name       string
		err        error
		want       string
		permission bool
	}{
		{"message", &HTTPError{StatusCode: http.StatusUnauthorized, Message: "bad credentials"}, "GitHub returned HTTP 401: bad credentials", false},
		{"empty message", &HTTPError{StatusCode: http.StatusNotFound}, "GitHub returned HTTP 404", true},
		{"wrapped forbidden", fmt.Errorf("pulls: %w", &HTTPError{StatusCode: http.StatusForbidden}), "pulls: GitHub returned HTTP 403", true},
		{"unrelated", errors.New("offline"), "offline", false},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := test.err.Error(); got != test.want {
				t.Fatalf("unexpected error text: %q", got)
			}
			if got := IsPullPermissionError(test.err); got != test.permission {
				t.Fatalf("permission classification = %t, want %t", got, test.permission)
			}
		})
	}
}

func TestGitHubErrorMessageParsesAndSanitizesResponse(t *testing.T) {
	if got := githubErrorMessage(strings.NewReader(`{"message":"  access denied  "}`)); got != "access denied" {
		t.Fatalf("unexpected parsed message: %q", got)
	}
	if got := githubErrorMessage(strings.NewReader("not JSON")); got != "request failed" {
		t.Fatalf("unexpected malformed-body fallback: %q", got)
	}
}

func TestGitHubClientPaginatesRepositoriesAndUsesBearerToken(t *testing.T) {
	var repoRequests atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
		if request.Header.Get("Authorization") != "Bearer secret-token" {
			t.Errorf("unexpected authorization header %q", request.Header.Get("Authorization"))
		}
		switch request.URL.Path {
		case "/user":
			_ = json.NewEncoder(response).Encode(map[string]any{"login": "viewer", "name": "Viewer"})
		case "/user/repos":
			repoRequests.Add(1)
			page, _ := strconv.Atoi(request.URL.Query().Get("page"))
			count := 100
			if page == 2 {
				count = 1
			}
			items := make([]map[string]any, count)
			for index := range count {
				id := int64((page-1)*100 + index + 1)
				items[index] = map[string]any{
					"id": id, "name": fmt.Sprintf("repo-%d", id), "full_name": fmt.Sprintf("org/repo-%d", id),
					"clone_url": "https://github.com/org/repo.git", "default_branch": "main", "created_at": "2020-01-02T00:00:00Z",
					"owner": map[string]any{"id": 7, "login": "org", "type": "Organization"},
				}
			}
			_ = json.NewEncoder(response).Encode(items)
		default:
			http.NotFound(response, request)
		}
	}))
	defer server.Close()

	service, err := NewGitHubClient("secret-token", server.URL, server.Client(), RateLimitCallbacks{})
	if err != nil {
		t.Fatalf("create GitHub client: %v", err)
	}
	viewer, err := service.Viewer(context.Background())
	if err != nil || viewer.Login != "viewer" {
		t.Fatalf("load viewer: %+v, %v", viewer, err)
	}
	repositories, err := service.Repositories(context.Background())
	if err != nil {
		t.Fatalf("list repositories: %v", err)
	}
	if len(repositories) != 101 || repoRequests.Load() != 2 {
		t.Fatalf("expected 101 repositories over two pages, got %d over %d requests", len(repositories), repoRequests.Load())
	}
	if repositories[0].CreatedAt.Format(time.DateOnly) != "2020-01-02" {
		t.Fatalf("repository creation metadata was not retained: %+v", repositories[0])
	}
}

func TestGitHubClientMergesPullRequestsUntilCheckpoint(t *testing.T) {
	checkpoint := time.Date(2025, time.January, 2, 0, 0, 0, 0, time.UTC)
	server := httptest.NewServer(http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
		switch request.URL.Path {
		case "/repos/org/repo/pulls":
			_ = json.NewEncoder(response).Encode([]map[string]any{
				{"number": 2, "state": "open", "created_at": "2025-01-03T00:00:00Z", "updated_at": "2025-01-03T00:00:00Z", "user": map[string]any{"login": "new"}},
				{"number": 1, "state": "closed", "created_at": "2025-01-01T00:00:00Z", "updated_at": "2025-01-02T00:00:00Z", "user": map[string]any{"login": "old"}},
			})
		case "/repos/org/repo/pulls/1/reviews", "/repos/org/repo/pulls/2/reviews":
			_ = json.NewEncoder(response).Encode([]map[string]any{{"id": 10, "state": "APPROVED", "submitted_at": "2025-01-04T00:00:00Z", "user": map[string]any{"login": "reviewer", "type": "User"}}})
		case "/repos/org/repo/pulls/1", "/repos/org/repo/pulls/2":
			_ = json.NewEncoder(response).Encode(map[string]any{"additions": 8, "deletions": 3, "changed_files": 2, "commits": 1, "merge_commit_sha": "merge-sha"})
		case "/repos/org/repo/pulls/1/commits", "/repos/org/repo/pulls/2/commits":
			_ = json.NewEncoder(response).Encode([]map[string]any{{"sha": "commit-sha", "commit": map[string]any{"message": "ship"}, "author": map[string]any{"login": "new", "type": "User"}}})
		default:
			http.NotFound(response, request)
		}
	}))
	defer server.Close()
	service, err := NewGitHubClient("token", server.URL, server.Client(), RateLimitCallbacks{})
	if err != nil {
		t.Fatalf("create GitHub client: %v", err)
	}
	cache, err := service.PullRequests(context.Background(), Repository{Name: "repo", Owner: OwnerIdentity{Login: "org"}}, PullCache{
		Checkpoint:   checkpoint,
		PullRequests: []PullRequest{{Number: 1, State: "open", CreatedAt: checkpoint.Add(-24 * time.Hour), UpdatedAt: checkpoint, Author: Person{Login: "old"}}},
	})
	if err != nil {
		t.Fatalf("list pull requests: %v", err)
	}
	if cache.Version != 3 || len(cache.PullRequests) != 2 || cache.PullRequests[0].State != "closed" || cache.PullRequests[1].Author.Login != "new" || len(cache.PullRequests[1].Reviews) != 1 || cache.PullRequests[1].Additions != 8 || !cache.PullRequests[1].CommitEvidenceComplete {
		t.Fatalf("unexpected merged pull request cache: %+v", cache.PullRequests)
	}
}

func TestGitHubClientActionsRevalidatesPartitionsAndRetainsRerunAttempts(t *testing.T) {
	now := time.Now().UTC()
	earliest := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, time.UTC)
	var conditionalRequests, attemptRequests atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
		switch request.URL.Path {
		case "/repos/org/repo/actions/runs":
			if request.Header.Get("If-None-Match") == `"partition-v1"` {
				conditionalRequests.Add(1)
				response.WriteHeader(http.StatusNotModified)
				return
			}
			response.Header().Set("ETag", `"partition-v1"`)
			_ = json.NewEncoder(response).Encode(map[string]any{
				"total_count": 1,
				"workflow_runs": []map[string]any{{
					"id": 55, "run_attempt": 2, "workflow_id": 8, "name": "build", "event": "pull_request", "head_sha": "sha", "status": "completed", "conclusion": "success",
					"created_at": now.Format(time.RFC3339), "updated_at": now.Format(time.RFC3339), "pull_requests": []map[string]any{{"number": 7}},
				}},
			})
		case "/repos/org/repo/actions/runs/55/attempts/1":
			attemptRequests.Add(1)
			_ = json.NewEncoder(response).Encode(map[string]any{
				"id": 55, "run_attempt": 1, "workflow_id": 8, "name": "build", "event": "pull_request", "head_sha": "sha", "status": "completed", "conclusion": "failure",
				"created_at": now.Add(-time.Minute).Format(time.RFC3339), "updated_at": now.Add(-30 * time.Second).Format(time.RFC3339), "pull_requests": []map[string]any{{"number": 7}},
			})
		default:
			http.NotFound(response, request)
		}
	}))
	defer server.Close()
	service, err := NewGitHubClient("token", server.URL, server.Client(), RateLimitCallbacks{})
	if err != nil {
		t.Fatal(err)
	}
	actions := service.(GitHubActionsService)
	cache, err := actions.WorkflowRuns(context.Background(), Repository{Name: "repo", Owner: OwnerIdentity{Login: "org"}}, ActionsCache{}, earliest)
	if err != nil {
		t.Fatalf("first Actions sync: %v", err)
	}
	if len(cache.Runs) != 2 || len(cache.Partitions) != 1 || cache.Partitions[0].ETag != `"partition-v1"` || attemptRequests.Load() != 1 {
		t.Fatalf("rerun attempts or partition ETag missing: %+v", cache)
	}
	cache, err = actions.WorkflowRuns(context.Background(), Repository{Name: "repo", Owner: OwnerIdentity{Login: "org"}}, cache, earliest)
	if err != nil {
		t.Fatalf("conditional Actions sync: %v", err)
	}
	if len(cache.Runs) != 2 || conditionalRequests.Load() != 1 || attemptRequests.Load() != 1 {
		t.Fatalf("conditional revalidation lost cached attempts: %+v", cache)
	}
}

func TestGitHubClientCapsPullCommitEvidenceAt250(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
		items := make([]map[string]any, 100)
		for index := range items {
			items[index] = map[string]any{"sha": fmt.Sprintf("%s-%d", request.URL.Query().Get("page"), index), "commit": map[string]any{"message": "ship"}, "author": map[string]any{"login": "human", "type": "User"}}
		}
		_ = json.NewEncoder(response).Encode(items)
	}))
	defer server.Close()
	service, err := NewGitHubClient("token", server.URL, server.Client(), RateLimitCallbacks{})
	if err != nil {
		t.Fatal(err)
	}
	commits, complete, err := service.(*githubClient).pullRequestCommits(context.Background(), Repository{Name: "repo", Owner: OwnerIdentity{Login: "org"}}, 1, 300)
	if err != nil {
		t.Fatal(err)
	}
	if len(commits) != 250 || complete {
		t.Fatalf("commit evidence len=%d complete=%v, want capped incomplete evidence", len(commits), complete)
	}
}

func TestGitHubClientRetriesRateLimitedRequest(t *testing.T) {
	var requests atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(response http.ResponseWriter, _ *http.Request) {
		if requests.Add(1) == 1 {
			response.Header().Set("Retry-After", "0")
			response.WriteHeader(http.StatusTooManyRequests)
			return
		}
		_ = json.NewEncoder(response).Encode(map[string]any{"login": "viewer"})
	}))
	defer server.Close()
	service, _ := NewGitHubClient("token", server.URL, server.Client(), RateLimitCallbacks{})
	if _, err := service.Viewer(context.Background()); err != nil {
		t.Fatalf("retry viewer request: %v", err)
	}
	if requests.Load() != 2 {
		t.Fatalf("expected one rate-limit retry, got %d requests", requests.Load())
	}
}
