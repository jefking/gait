package api

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/jefking/gait/backend/internal/dashboard"
)

type fakeDashboardService struct {
	response         dashboard.DashboardResponse
	started          string
	startErr         error
	query            dashboard.ActivityQuery
	events           chan dashboard.DashboardEvent
	insightQuery     dashboard.InsightQuery
	identityKey      string
	identityOverride dashboard.IdentityOverride
	activityErr      error
	insightErr       error
	identityErr      error
}

func (service *fakeDashboardService) Dashboard() dashboard.DashboardResponse {
	return service.response
}

func (service *fakeDashboardService) Activity(query dashboard.ActivityQuery) (dashboard.ActivityResponse, error) {
	service.query = query
	return dashboard.ActivityResponse{Group: query.Group, Metric: query.Metric, Series: []dashboard.ActivitySeries{}}, service.activityErr
}

func (service *fakeDashboardService) Start(token string) (dashboard.SyncStatus, error) {
	service.started = token
	return dashboard.SyncStatus{ID: "sync-1", State: dashboard.SyncDiscovering}, service.startErr
}

func (service *fakeDashboardService) Subscribe(context.Context) <-chan dashboard.DashboardEvent {
	if service.events == nil {
		service.events = make(chan dashboard.DashboardEvent)
	}
	return service.events
}

func (service *fakeDashboardService) InsightOverview(query dashboard.InsightQuery) (dashboard.OverviewResponse, error) {
	service.insightQuery = query
	return dashboard.OverviewResponse{Meta: dashboard.InsightMeta{Coverage: dashboard.InsightCoverage{}}}, service.insightErr
}
func (service *fakeDashboardService) InsightNetwork(query dashboard.InsightQuery) (dashboard.NetworkResponse, error) {
	service.insightQuery = query
	return dashboard.NetworkResponse{Meta: dashboard.InsightMeta{Coverage: dashboard.InsightCoverage{}}}, service.insightErr
}
func (service *fakeDashboardService) InsightRamps(query dashboard.InsightQuery) (dashboard.RampResponse, error) {
	service.insightQuery = query
	return dashboard.RampResponse{Meta: dashboard.InsightMeta{Coverage: dashboard.InsightCoverage{}}}, service.insightErr
}
func (service *fakeDashboardService) InsightRankings(query dashboard.InsightQuery) (dashboard.RankingResponse, error) {
	service.insightQuery = query
	return dashboard.RankingResponse{Meta: dashboard.InsightMeta{Coverage: dashboard.InsightCoverage{}}, Cohort: query.Cohort, Metric: query.Metric}, service.insightErr
}
func (service *fakeDashboardService) Identities() dashboard.IdentityResponse {
	return dashboard.IdentityResponse{Identities: []dashboard.IdentitySummary{}}
}
func (service *fakeDashboardService) UpdateIdentity(key string, override dashboard.IdentityOverride) (dashboard.IdentityResponse, error) {
	service.identityKey = key
	service.identityOverride = override
	return service.Identities(), service.identityErr
}

func TestStartSyncAPIAcceptsPATWithoutEchoingIt(t *testing.T) {
	service := &fakeDashboardService{}
	body := []byte(`{"pat":"top-secret"}`)
	request := httptest.NewRequest(http.MethodPost, "/api/sync", bytes.NewReader(body))
	request.Header.Set("Content-Type", "application/json")
	response := httptest.NewRecorder()

	NewRouter(t.TempDir(), service).ServeHTTP(response, request)

	if response.Code != http.StatusAccepted {
		t.Fatalf("expected status %d, got %d: %s", http.StatusAccepted, response.Code, response.Body.String())
	}
	if service.started != "top-secret" {
		t.Fatalf("service did not receive PAT")
	}
	if bytes.Contains(response.Body.Bytes(), []byte("top-secret")) {
		t.Fatalf("response echoed PAT: %s", response.Body.String())
	}
}

func TestStartSyncAPIAcceptsRefreshWithoutPAT(t *testing.T) {
	service := &fakeDashboardService{}
	request := httptest.NewRequest(http.MethodPost, "/api/sync", bytes.NewBufferString(`{}`))
	response := httptest.NewRecorder()

	NewRouter(t.TempDir(), service).ServeHTTP(response, request)

	if response.Code != http.StatusAccepted {
		t.Fatalf("expected status %d, got %d: %s", http.StatusAccepted, response.Code, response.Body.String())
	}
	if service.started != "" {
		t.Fatalf("refresh unexpectedly supplied PAT")
	}
}

func TestStartSyncAPIRejectsConcurrentJob(t *testing.T) {
	service := &fakeDashboardService{startErr: dashboard.ErrSyncActive}
	request := httptest.NewRequest(http.MethodPost, "/api/sync", bytes.NewBufferString(`{"pat":"token"}`))
	response := httptest.NewRecorder()
	NewRouter(t.TempDir(), service).ServeHTTP(response, request)
	if response.Code != http.StatusConflict {
		t.Fatalf("expected conflict, got %d", response.Code)
	}
}

func TestActivityAPIParsesFilters(t *testing.T) {
	service := &fakeDashboardService{}
	request := httptest.NewRequest(http.MethodGet, "/api/activity?group_by=contributor&metric=pull_requests&owner_id=7&repository_id=9&exclude_dead=true&from=2024-01-02&to=2024-03-04", nil)
	response := httptest.NewRecorder()
	NewRouter(t.TempDir(), service).ServeHTTP(response, request)
	if response.Code != http.StatusOK {
		t.Fatalf("expected OK, got %d: %s", response.Code, response.Body.String())
	}
	if service.query.Group != dashboard.ActivityByContributor || service.query.Metric != dashboard.ActivityPullRequests ||
		service.query.OwnerID != 7 || service.query.RepositoryID != 9 || !service.query.ExcludeDead || service.query.From == nil || service.query.To == nil ||
		service.query.From.Format(time.DateOnly) != "2024-01-02" || service.query.To.Format(time.DateOnly) != "2024-03-04" {
		t.Fatalf("unexpected activity query: %+v", service.query)
	}
}

func TestActivityAPIRejectsInvalidDate(t *testing.T) {
	service := &fakeDashboardService{}
	request := httptest.NewRequest(http.MethodGet, "/api/activity?from=01-02-2024", nil)
	response := httptest.NewRecorder()
	NewRouter(t.TempDir(), service).ServeHTTP(response, request)
	if response.Code != http.StatusBadRequest {
		t.Fatalf("expected bad request, got %d: %s", response.Code, response.Body.String())
	}
}

func TestInsightAPIParsesSharedFiltersAndRankingSelection(t *testing.T) {
	service := &fakeDashboardService{}
	request := httptest.NewRequest(http.MethodGet, "/api/insights/rankings?owner_id=7&repository_id=9&actor_kind=agent&exclude_dead=true&from=2025-01-01&to=2025-02-01&session_hours=48&adoption_days=45&survival_days=60&cohort=human_agent&metric=handoffs", nil)
	response := httptest.NewRecorder()
	NewRouter(t.TempDir(), service).ServeHTTP(response, request)
	if response.Code != http.StatusOK {
		t.Fatalf("expected OK, got %d: %s", response.Code, response.Body.String())
	}
	query := service.insightQuery
	if query.OwnerID != 7 || query.RepositoryID != 9 || query.ActorKind != dashboard.ActorAgent || !query.ExcludeDead || query.SessionHours != 48 || query.AdoptionDays != 45 || query.SurvivalDays != 60 || query.Cohort != "human_agent" || query.Metric != "handoffs" {
		t.Fatalf("unexpected query: %+v", query)
	}
}

func TestIdentityAPIUpdatesAnEncodedIdentity(t *testing.T) {
	service := &fakeDashboardService{}
	request := httptest.NewRequest(http.MethodPatch, "/api/identities/github%3Ahelper%5Bbot%5D", bytes.NewBufferString(`{"kind":"agent","display_name":"Helper"}`))
	response := httptest.NewRecorder()
	NewRouter(t.TempDir(), service).ServeHTTP(response, request)
	if response.Code != http.StatusOK {
		t.Fatalf("expected OK, got %d: %s", response.Code, response.Body.String())
	}
	if service.identityKey != "github:helper[bot]" || service.identityOverride.Kind != dashboard.ActorAgent {
		t.Fatalf("unexpected update: %q %+v", service.identityKey, service.identityOverride)
	}
}

func TestDashboardAPIReturnsNullableSnapshotAndSync(t *testing.T) {
	service := &fakeDashboardService{response: dashboard.DashboardResponse{Sync: dashboard.SyncStatus{State: dashboard.SyncIdle}}}
	request := httptest.NewRequest(http.MethodGet, "/api/dashboard", nil)
	response := httptest.NewRecorder()
	NewRouter(t.TempDir(), service).ServeHTTP(response, request)
	var body dashboard.DashboardResponse
	if err := json.NewDecoder(response.Body).Decode(&body); err != nil {
		t.Fatalf("decode dashboard: %v", err)
	}
	if response.Code != http.StatusOK || body.Snapshot != nil || body.Sync.State != dashboard.SyncIdle {
		t.Fatalf("unexpected dashboard response: %d %+v", response.Code, body)
	}
}

func TestStartSyncAPIRejectsUnknownFields(t *testing.T) {
	service := &fakeDashboardService{}
	request := httptest.NewRequest(http.MethodPost, "/api/sync", bytes.NewBufferString(`{"pat":"token","persist":true}`))
	response := httptest.NewRecorder()
	NewRouter(t.TempDir(), service).ServeHTTP(response, request)
	if response.Code != http.StatusBadRequest {
		t.Fatalf("expected bad request, got %d", response.Code)
	}
	if service.started != "" {
		t.Fatalf("service should not have been called")
	}
}

func TestDashboardEventsStreamsInvalidations(t *testing.T) {
	events := make(chan dashboard.DashboardEvent, 1)
	events <- dashboard.DashboardEvent{
		Type: "snapshot", Revision: 7,
		Repository: &dashboard.RepositoryEventMetadata{
			ID: 9, FullName: "org/repo", SyncStatus: "synced",
			Liveness: dashboard.RepositoryLiveness{State: dashboard.RepositoryDead, IsDead: true, Basis: "default_branch_commits"},
		},
	}
	close(events)
	service := &fakeDashboardService{events: events}
	request := httptest.NewRequest(http.MethodGet, "/api/events", nil)
	response := httptest.NewRecorder()

	NewRouter(t.TempDir(), service).ServeHTTP(response, request)

	if response.Header().Get("Content-Type") != "text/event-stream" {
		t.Fatalf("expected event stream, got %q", response.Header().Get("Content-Type"))
	}
	want := "event: dashboard\ndata: {\"type\":\"snapshot\",\"revision\":7,\"repository\":{\"id\":9,\"full_name\":\"org/repo\",\"sync_status\":\"synced\",\"liveness\":{\"state\":\"dead\",\"is_dead\":true,\"basis\":\"default_branch_commits\",\"evaluated_at\":\"0001-01-01T00:00:00Z\"}}}\n\n"
	if response.Body.String() != want {
		t.Fatalf("unexpected event body: %q", response.Body.String())
	}
}

func TestActivityAPIRejectsInvalidFiltersAndServiceErrors(t *testing.T) {
	tests := []struct {
		name string
		url  string
		err  error
		want string
	}{
		{"invalid boolean", "/api/activity?exclude_dead=sometimes", nil, "exclude_dead must be true or false"},
		{"invalid owner", "/api/activity?owner_id=0", nil, "owner_id must be a positive integer"},
		{"invalid repository", "/api/activity?repository_id=-1", nil, "repository_id must be a positive integer"},
		{"invalid end date", "/api/activity?to=tomorrow", nil, "to must use YYYY-MM-DD"},
		{"service validation", "/api/activity", errors.New("unsupported group"), "unsupported group"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			service := &fakeDashboardService{activityErr: test.err}
			response := httptest.NewRecorder()
			NewRouter(t.TempDir(), service).ServeHTTP(response, httptest.NewRequest(http.MethodGet, test.url, nil))
			if response.Code != http.StatusBadRequest || !bytes.Contains(response.Body.Bytes(), []byte(test.want)) {
				t.Fatalf("expected bad request containing %q, got %d: %s", test.want, response.Code, response.Body.String())
			}
		})
	}
}

func TestInsightAPIRoutesAndRejectsInvalidQueries(t *testing.T) {
	for _, route := range []string{"overview", "network", "ramps"} {
		t.Run(route+" success", func(t *testing.T) {
			response := httptest.NewRecorder()
			NewRouter(t.TempDir(), &fakeDashboardService{}).ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/api/insights/"+route, nil))
			if response.Code != http.StatusOK || response.Header().Get("Cache-Control") != "no-store" {
				t.Fatalf("unexpected response: %d %s", response.Code, response.Body.String())
			}
		})
	}
	tests := []struct{ name, query, want string }{
		{"boolean", "exclude_dead=nope", "exclude_dead must be true or false"},
		{"owner", "owner_id=x", "owner_id must be a positive integer"},
		{"repository", "repository_id=0", "repository_id must be a positive integer"},
		{"from", "from=yesterday", "from must use YYYY-MM-DD"},
		{"to", "to=2025-13-01", "to must use YYYY-MM-DD"},
		{"session", "session_hours=soon", "session_hours must be an integer"},
		{"adoption", "adoption_days=many", "adoption_days must be an integer"},
		{"survival", "survival_days=forever", "survival_days must be an integer"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			response := httptest.NewRecorder()
			request := httptest.NewRequest(http.MethodGet, "/api/insights/overview?"+test.query, nil)
			NewRouter(t.TempDir(), &fakeDashboardService{}).ServeHTTP(response, request)
			if response.Code != http.StatusBadRequest || !bytes.Contains(response.Body.Bytes(), []byte(test.want)) {
				t.Fatalf("expected %q, got %d: %s", test.want, response.Code, response.Body.String())
			}
		})
	}
	response := httptest.NewRecorder()
	service := &fakeDashboardService{insightErr: errors.New("range unavailable")}
	NewRouter(t.TempDir(), service).ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/api/insights/network", nil))
	if response.Code != http.StatusBadRequest || !bytes.Contains(response.Body.Bytes(), []byte("range unavailable")) {
		t.Fatalf("service error not returned: %d %s", response.Code, response.Body.String())
	}
}

func TestIdentityAPIHandlesReadsAndInvalidUpdates(t *testing.T) {
	response := httptest.NewRecorder()
	NewRouter(t.TempDir(), &fakeDashboardService{}).ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/api/identities", nil))
	if response.Code != http.StatusOK || response.Header().Get("Cache-Control") != "no-store" {
		t.Fatalf("unexpected identities response: %d %s", response.Code, response.Body.String())
	}
	tests := []struct {
		name string
		body string
		err  error
		want string
	}{
		{"malformed", `{`, nil, "request must contain an identity override"},
		{"unknown field", `{"kind":"agent","extra":true}`, nil, "request must contain an identity override"},
		{"multiple objects", `{} {}`, nil, "request must contain one JSON object"},
		{"service rejection", `{"kind":"robot"}`, errors.New("kind must be human, agent, or unknown"), "kind must be human, agent, or unknown"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			response := httptest.NewRecorder()
			service := &fakeDashboardService{identityErr: test.err}
			request := httptest.NewRequest(http.MethodPatch, "/api/identities/github:user", bytes.NewBufferString(test.body))
			NewRouter(t.TempDir(), service).ServeHTTP(response, request)
			if response.Code != http.StatusBadRequest || !bytes.Contains(response.Body.Bytes(), []byte(test.want)) {
				t.Fatalf("expected %q, got %d: %s", test.want, response.Code, response.Body.String())
			}
		})
	}
}

func TestStartSyncAPIRejectsMalformedOversizedAndServiceFailures(t *testing.T) {
	tests := []struct {
		name string
		body string
		err  error
		want string
	}{
		{"malformed", `{`, nil, "request must contain a PAT"},
		{"multiple objects", `{} {}`, nil, "request must contain one JSON object"},
		{"oversized token", `{"pat":"` + string(bytes.Repeat([]byte{'x'}, maximumPATLength+1)) + `"}`, nil, "PAT must not exceed 4096 characters"},
		{"service failure", `{}`, errors.New("no retained PAT"), "no retained PAT"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			response := httptest.NewRecorder()
			service := &fakeDashboardService{startErr: test.err}
			NewRouter(t.TempDir(), service).ServeHTTP(response, httptest.NewRequest(http.MethodPost, "/api/sync", bytes.NewBufferString(test.body)))
			if response.Code != http.StatusBadRequest || !bytes.Contains(response.Body.Bytes(), []byte(test.want)) {
				t.Fatalf("expected %q, got %d: %s", test.want, response.Code, response.Body.String())
			}
		})
	}
}
