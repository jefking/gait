package dashboard

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strconv"
	"sync/atomic"
	"testing"
	"time"
)

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
					"clone_url": "https://github.com/org/repo.git", "default_branch": "main",
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
}

func TestGitHubClientMergesPullRequestsUntilCheckpoint(t *testing.T) {
	checkpoint := time.Date(2025, time.January, 2, 0, 0, 0, 0, time.UTC)
	server := httptest.NewServer(http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
		if request.URL.Path != "/repos/org/repo/pulls" {
			http.NotFound(response, request)
			return
		}
		_ = json.NewEncoder(response).Encode([]map[string]any{
			{"number": 2, "state": "open", "created_at": "2025-01-03T00:00:00Z", "updated_at": "2025-01-03T00:00:00Z", "user": map[string]any{"login": "new"}},
			{"number": 1, "state": "closed", "created_at": "2025-01-01T00:00:00Z", "updated_at": "2025-01-02T00:00:00Z", "user": map[string]any{"login": "old"}},
		})
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
	if len(cache.PullRequests) != 2 || cache.PullRequests[0].State != "open" || cache.PullRequests[1].Author.Login != "new" {
		t.Fatalf("unexpected merged pull request cache: %+v", cache.PullRequests)
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
