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
		case "/orgs/org/repos":
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
	repositories, err := service.Repositories(context.Background(), OwnerIdentity{ID: 7, Login: "org", Type: "Organization"})
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

func TestGitHubClientDiscoversPaginatedActiveMembershipsForFineGrainedPATs(t *testing.T) {
	var requests atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
		if request.URL.Path == "/user/orgs" {
			t.Fatal("fine-grained discovery must not use /user/orgs")
		}
		if request.URL.Path != "/user/memberships/orgs" || request.URL.Query().Get("state") != "active" {
			http.NotFound(response, request)
			return
		}
		requests.Add(1)
		page, _ := strconv.Atoi(request.URL.Query().Get("page"))
		count := 100
		if page == 2 {
			count = 1
		}
		memberships := make([]map[string]any, count)
		for index := range count {
			id := int64((page-1)*100 + index + 1)
			memberships[index] = map[string]any{
				"state":        "active",
				"organization": map[string]any{"id": id, "login": fmt.Sprintf("org-%03d", id), "avatar_url": "avatar", "html_url": "profile"},
			}
		}
		_ = json.NewEncoder(response).Encode(memberships)
	}))
	defer server.Close()
	service, err := NewGitHubClient("fine-grained-token", server.URL, server.Client(), RateLimitCallbacks{})
	if err != nil {
		t.Fatal(err)
	}
	organizations, err := service.Organizations(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(organizations) != 101 || requests.Load() != 2 || organizations[0].Type != "Organization" {
		t.Fatalf("unexpected membership discovery: requests=%d organizations=%+v", requests.Load(), organizations)
	}
}

func TestGitHubClientUsesOwnerSpecificRepositoryEndpointsAndExactFiltering(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
		matchingOwner := map[string]any{"id": 9, "login": "viewer", "type": "User"}
		switch request.URL.Path {
		case "/user/repos":
			if request.URL.Query().Get("affiliation") != "owner" || request.URL.Query().Get("visibility") != "all" {
				t.Errorf("unexpected personal query: %s", request.URL.RawQuery)
			}
			_ = json.NewEncoder(response).Encode([]map[string]any{
				{"id": 1, "name": "private-fork", "full_name": "viewer/private-fork", "private": true, "fork": true, "archived": true, "default_branch": "main", "owner": matchingOwner},
				{"id": 2, "name": "member-repo", "full_name": "acme/member-repo", "default_branch": "main", "owner": map[string]any{"id": 7, "login": "acme", "type": "Organization"}},
			})
		case "/orgs/acme/repos":
			if request.URL.Query().Get("type") != "all" {
				t.Errorf("unexpected organization query: %s", request.URL.RawQuery)
			}
			_ = json.NewEncoder(response).Encode([]map[string]any{
				{"id": 3, "name": "archive", "full_name": "acme/archive", "fork": true, "archived": true, "default_branch": "trunk", "owner": map[string]any{"id": 7, "login": "acme", "type": "Organization"}},
				{"id": 4, "name": "wrong-id", "full_name": "impostor/wrong-id", "default_branch": "main", "owner": map[string]any{"id": 8, "login": "acme", "type": "Organization"}},
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
	personal, err := service.Repositories(context.Background(), OwnerIdentity{ID: 9, Login: "viewer", Type: "User"})
	if err != nil {
		t.Fatal(err)
	}
	organization, err := service.Repositories(context.Background(), OwnerIdentity{ID: 7, Login: "acme", Type: "Organization"})
	if err != nil {
		t.Fatal(err)
	}
	if len(personal) != 1 || !personal[0].Private || !personal[0].Fork || !personal[0].Archived {
		t.Fatalf("personal owner filtering lost repository visibility flags: %+v", personal)
	}
	if len(organization) != 1 || organization[0].ID != 3 || !organization[0].Fork || !organization[0].Archived {
		t.Fatalf("organization owner filtering was not exact: %+v", organization)
	}
}

func TestGitHubClientConditionallyRefreshesEveryCachedCatalogPage(t *testing.T) {
	var requests atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
		requests.Add(1)
		if request.URL.Path != "/orgs/acme/repos" {
			http.NotFound(response, request)
			return
		}
		page, _ := strconv.Atoi(request.URL.Query().Get("page"))
		etag := fmt.Sprintf(`"catalog-page-%d"`, page)
		if request.Header.Get("If-None-Match") == etag {
			response.Header().Set("ETag", etag)
			response.WriteHeader(http.StatusNotModified)
			return
		}
		response.Header().Set("ETag", etag)
		count := 100
		if page == 2 {
			count = 1
		}
		items := make([]map[string]any, count)
		for index := range count {
			id := int64((page-1)*100 + index + 1)
			items[index] = map[string]any{
				"id": id, "name": fmt.Sprintf("repo-%d", id), "full_name": fmt.Sprintf("acme/repo-%d", id), "default_branch": "main", "pushed_at": "2025-01-01T00:00:00Z",
				"owner": map[string]any{"id": 7, "login": "acme", "type": "Organization"},
			}
		}
		_ = json.NewEncoder(response).Encode(items)
	}))
	defer server.Close()
	service, err := NewGitHubClient("token", server.URL, server.Client(), RateLimitCallbacks{})
	if err != nil {
		t.Fatal(err)
	}
	catalogService := service.(GitHubRepositoryCatalogService)
	first, err := catalogService.RefreshRepositories(context.Background(), OwnerIdentity{ID: 7, Login: "acme", Type: "Organization"}, RepositoryCatalog{})
	if err != nil {
		t.Fatal(err)
	}
	second, err := catalogService.RefreshRepositories(context.Background(), OwnerIdentity{ID: 7, Login: "acme", Type: "Organization"}, first)
	if err != nil {
		t.Fatal(err)
	}
	if requests.Load() != 4 || len(second.Repositories()) != 101 || len(second.Pages) != 2 || second.Pages[1].ETag != `"catalog-page-2"` {
		t.Fatalf("conditional repository catalog refresh lost cached data: requests=%d catalog=%+v", requests.Load(), second)
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

func TestGitHubClientReenrichesEveryPullRequestWhenUpgradingV2Cache(t *testing.T) {
	checkpoint := time.Date(2025, time.January, 3, 0, 0, 0, 0, time.UTC)
	var detailRequests atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
		switch request.URL.Path {
		case "/repos/org/repo/pulls":
			_ = json.NewEncoder(response).Encode([]map[string]any{{
				"number": 1, "state": "closed", "merged_at": "2025-01-02T00:00:00Z", "created_at": "2025-01-01T00:00:00Z", "updated_at": "2025-01-02T00:00:00Z", "user": map[string]any{"login": "human", "type": "User"},
			}})
		case "/repos/org/repo/pulls/1":
			detailRequests.Add(1)
			_ = json.NewEncoder(response).Encode(map[string]any{"additions": 8, "deletions": 2, "changed_files": 1, "commits": 1, "merge_commit_sha": "merge-sha"})
		case "/repos/org/repo/pulls/1/reviews":
			_ = json.NewEncoder(response).Encode([]map[string]any{})
		case "/repos/org/repo/pulls/1/commits":
			_ = json.NewEncoder(response).Encode([]map[string]any{{
				"sha": "commit-sha", "commit": map[string]any{"message": "ship\n\nCo-authored-by: agent[bot] <agent[bot]@users.noreply.github.com>"}, "author": map[string]any{"login": "human", "type": "User"},
			}})
		default:
			http.NotFound(response, request)
		}
	}))
	defer server.Close()

	service, err := NewGitHubClient("token", server.URL, server.Client(), RateLimitCallbacks{})
	if err != nil {
		t.Fatal(err)
	}
	cache, err := service.PullRequests(context.Background(), Repository{Name: "repo", Owner: OwnerIdentity{Login: "org"}}, PullCache{
		Version: 2, Checkpoint: checkpoint,
		PullRequests: []PullRequest{{Number: 1, State: "closed", CreatedAt: checkpoint.Add(-48 * time.Hour), UpdatedAt: checkpoint.Add(-24 * time.Hour), Author: Person{Login: "human", Type: "User"}}},
	})
	if err != nil {
		t.Fatalf("upgrade pull cache: %v", err)
	}
	if cache.Version != pullCacheVersion || detailRequests.Load() != 1 || len(cache.PullRequests) != 1 || !cache.PullRequests[0].DetailComplete || !cache.PullRequests[0].CommitEvidenceComplete || len(cache.PullRequests[0].CommitEvidence) != 1 {
		t.Fatalf("v2 cache was not fully re-enriched: version=%d detail_requests=%d pulls=%+v", cache.Version, detailRequests.Load(), cache.PullRequests)
	}
}

func TestPullCacheEnrichmentCompleteness(t *testing.T) {
	completePull := PullRequest{DetailComplete: true, CommitEvidenceComplete: true}
	truncatedPull := PullRequest{DetailComplete: true, Commits: 251, CommitEvidence: make([]PullRequestCommit, 250)}
	if !pullCacheEnrichmentComplete(PullCache{Version: pullCacheVersion, PullRequests: []PullRequest{completePull, truncatedPull}}) {
		t.Fatal("complete and intentionally capped commit evidence should allow incremental refresh")
	}
	if pullCacheEnrichmentComplete(PullCache{Version: pullCacheVersion, PullRequests: []PullRequest{{DetailComplete: true}}}) {
		t.Fatal("incomplete v3 commit evidence should force full re-enrichment")
	}
	if pullCacheEnrichmentComplete(PullCache{Version: pullCacheVersion - 1, PullRequests: []PullRequest{completePull}}) {
		t.Fatal("legacy cache version should force full re-enrichment")
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
