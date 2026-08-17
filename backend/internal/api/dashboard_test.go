package api

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/jefking/gait/backend/internal/dashboard"
)

type fakeDashboardService struct {
	response         dashboard.DashboardResponse
	started          bool
	discovered       string
	selected         int64
	startErr         error
	events           chan dashboard.DashboardEvent
	insightQuery     dashboard.InsightQuery
	identityKey      string
	identityOverride dashboard.IdentityOverride
	insightErr       error
	identityErr      error
}

func (service *fakeDashboardService) Dashboard() dashboard.DashboardResponse {
	return service.response
}

func (service *fakeDashboardService) Start() (dashboard.SyncStatus, error) {
	service.started = true
	return dashboard.SyncStatus{ID: "sync-1", State: dashboard.SyncDiscovering}, service.startErr
}

func (service *fakeDashboardService) DiscoverTargets(token string) (dashboard.TargetDiscovery, error) {
	service.discovered = token
	return dashboard.TargetDiscovery{Viewer: dashboard.Viewer{ID: 9, Login: "viewer"}, Targets: []dashboard.OwnerIdentity{{ID: 9, Login: "viewer", Type: "User"}}}, service.startErr
}

func (service *fakeDashboardService) SelectTarget(targetID int64) (dashboard.DashboardResponse, error) {
	service.selected = targetID
	return dashboard.DashboardResponse{Sync: dashboard.SyncStatus{ID: "sync-1", State: dashboard.SyncDiscovering}}, service.startErr
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
func (service *fakeDashboardService) InsightDelivery(query dashboard.InsightQuery) (dashboard.DeliveryResponse, error) {
	service.insightQuery = query
	return dashboard.DeliveryResponse{Meta: dashboard.DeliveryMeta{Coverage: dashboard.DeliveryCoverage{}}}, service.insightErr
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
func (service *fakeDashboardService) ScopedIdentities(query dashboard.InsightQuery) dashboard.IdentityResponse {
	service.insightQuery = query
	return service.Identities()
}
func (service *fakeDashboardService) UpdateIdentity(key string, override dashboard.IdentityOverride) (dashboard.IdentityResponse, error) {
	service.identityKey = key
	service.identityOverride = override
	return service.Identities(), service.identityErr
}

func TestGitHubTargetsAPIAcceptsPATWithoutEchoingIt(t *testing.T) {
	service := &fakeDashboardService{}
	body := []byte(`{"pat":"top-secret"}`)
	request := httptest.NewRequest(http.MethodPost, "/api/github/targets", bytes.NewReader(body))
	request.Header.Set("Content-Type", "application/json")
	response := httptest.NewRecorder()

	NewRouter(t.TempDir(), service).ServeHTTP(response, request)

	if response.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d: %s", http.StatusOK, response.Code, response.Body.String())
	}
	if service.discovered != "top-secret" {
		t.Fatalf("service did not receive PAT")
	}
	if bytes.Contains(response.Body.Bytes(), []byte("top-secret")) {
		t.Fatalf("response echoed PAT: %s", response.Body.String())
	}
}

func TestGitHubTargetsAPIRedactsPATFromFailures(t *testing.T) {
	service := &fakeDashboardService{startErr: errors.New("GitHub rejected top-secret")}
	request := httptest.NewRequest(http.MethodPost, "/api/github/targets", bytes.NewBufferString(`{"pat":"top-secret"}`))
	response := httptest.NewRecorder()
	NewRouter(t.TempDir(), service).ServeHTTP(response, request)
	if response.Code != http.StatusBadRequest || bytes.Contains(response.Body.Bytes(), []byte("top-secret")) || !bytes.Contains(response.Body.Bytes(), []byte("[redacted]")) {
		t.Fatalf("PAT was not redacted from failure: %d %s", response.Code, response.Body.String())
	}
}

func TestSelectGitHubTargetAPIValidatesAndStartsSelection(t *testing.T) {
	service := &fakeDashboardService{}
	request := httptest.NewRequest(http.MethodPut, "/api/configuration/github-target", bytes.NewBufferString(`{"target_id":7}`))
	response := httptest.NewRecorder()
	NewRouter(t.TempDir(), service).ServeHTTP(response, request)
	if response.Code != http.StatusAccepted || service.selected != 7 {
		t.Fatalf("target selection was not accepted: %d %s", response.Code, response.Body.String())
	}

	invalid := &fakeDashboardService{}
	response = httptest.NewRecorder()
	NewRouter(t.TempDir(), invalid).ServeHTTP(response, httptest.NewRequest(http.MethodPut, "/api/configuration/github-target", bytes.NewBufferString(`{"target_id":0}`)))
	if response.Code != http.StatusBadRequest || invalid.selected != 0 {
		t.Fatalf("invalid target selection was not rejected: %d %s", response.Code, response.Body.String())
	}

	conflict := &fakeDashboardService{startErr: dashboard.ErrSyncActive}
	response = httptest.NewRecorder()
	NewRouter(t.TempDir(), conflict).ServeHTTP(response, httptest.NewRequest(http.MethodPut, "/api/configuration/github-target", bytes.NewBufferString(`{"target_id":7}`)))
	if response.Code != http.StatusConflict {
		t.Fatalf("concurrent target selection did not conflict: %d %s", response.Code, response.Body.String())
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
	if !service.started {
		t.Fatalf("refresh did not start")
	}
}

func TestStartSyncAPIRejectsConcurrentJob(t *testing.T) {
	service := &fakeDashboardService{startErr: dashboard.ErrSyncActive}
	request := httptest.NewRequest(http.MethodPost, "/api/sync", bytes.NewBufferString(`{}`))
	response := httptest.NewRecorder()
	NewRouter(t.TempDir(), service).ServeHTTP(response, request)
	if response.Code != http.StatusConflict {
		t.Fatalf("expected conflict, got %d", response.Code)
	}
}

func TestDeliveryAPIParsesGlobalScope(t *testing.T) {
	service := &fakeDashboardService{}
	request := httptest.NewRequest(http.MethodGet, "/api/insights/delivery?exclude_dead=true&from=2025-01-01&to=2025-02-01", nil)
	response := httptest.NewRecorder()
	NewRouter(t.TempDir(), service).ServeHTTP(response, request)
	if response.Code != http.StatusOK {
		t.Fatalf("expected OK, got %d: %s", response.Code, response.Body.String())
	}
	query := service.insightQuery
	if query.OwnerID != 0 || !query.ExcludeDead || query.From == nil || query.To == nil {
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
	if service.started {
		t.Fatalf("service should not have been called")
	}
}

func TestStartSyncAPIRejectsNull(t *testing.T) {
	service := &fakeDashboardService{}
	request := httptest.NewRequest(http.MethodPost, "/api/sync", bytes.NewBufferString(`null`))
	response := httptest.NewRecorder()
	NewRouter(t.TempDir(), service).ServeHTTP(response, request)
	if response.Code != http.StatusBadRequest || service.started {
		t.Fatalf("expected null refresh to be rejected, got %d", response.Code)
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

func TestInsightAPIRoutesAndRejectsInvalidQueries(t *testing.T) {
	for _, route := range []string{"delivery", "network"} {
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
		{"organization", "organization_id=7", "organization_id is no longer supported"},
		{"repository", "repository_id=0", "repository_id must be a positive integer"},
		{"from", "from=yesterday", "from must use YYYY-MM-DD"},
		{"to", "to=2025-13-01", "to must use YYYY-MM-DD"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			response := httptest.NewRecorder()
			request := httptest.NewRequest(http.MethodGet, "/api/insights/delivery?"+test.query, nil)
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
	for _, obsolete := range []string{"overview", "ramps", "rankings"} {
		response := httptest.NewRecorder()
		NewRouter(t.TempDir(), &fakeDashboardService{}).ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/api/insights/"+obsolete, nil))
		if response.Code != http.StatusNotFound {
			t.Fatalf("obsolete %s route remains registered: %d", obsolete, response.Code)
		}
	}
	response = httptest.NewRecorder()
	NewRouter(t.TempDir(), &fakeDashboardService{}).ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/api/activity?group_by=contributor", nil))
	if response.Code != http.StatusNotFound {
		t.Fatalf("contributor activity route remains registered: %d", response.Code)
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

func TestStartSyncAPIRejectsMalformedAndServiceFailures(t *testing.T) {
	tests := []struct {
		name string
		body string
		err  error
		want string
	}{
		{"malformed", `{`, nil, "request must contain one empty JSON object"},
		{"multiple objects", `{} {}`, nil, "request must contain one JSON object"},
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
