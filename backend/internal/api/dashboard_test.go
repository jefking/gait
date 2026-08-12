package api

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/jefking/gait/backend/internal/dashboard"
)

type fakeDashboardService struct {
	response dashboard.DashboardResponse
	started  string
	startErr error
	query    dashboard.ActivityQuery
	events   chan dashboard.DashboardEvent
}

func (service *fakeDashboardService) Dashboard() dashboard.DashboardResponse {
	return service.response
}

func (service *fakeDashboardService) Activity(query dashboard.ActivityQuery) (dashboard.ActivityResponse, error) {
	service.query = query
	return dashboard.ActivityResponse{Group: query.Group, Metric: query.Metric, Series: []dashboard.ActivitySeries{}}, nil
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
	request := httptest.NewRequest(http.MethodGet, "/api/activity?group_by=contributor&metric=pull_requests&owner_id=7&repository_id=9&from=2024-01-02&to=2024-03-04", nil)
	response := httptest.NewRecorder()
	NewRouter(t.TempDir(), service).ServeHTTP(response, request)
	if response.Code != http.StatusOK {
		t.Fatalf("expected OK, got %d: %s", response.Code, response.Body.String())
	}
	if service.query.Group != dashboard.ActivityByContributor || service.query.Metric != dashboard.ActivityPullRequests ||
		service.query.OwnerID != 7 || service.query.RepositoryID != 9 || service.query.From == nil || service.query.To == nil ||
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
