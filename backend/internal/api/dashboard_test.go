package api

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/jefking/gait/backend/internal/dashboard"
)

type fakeDashboardService struct {
	response dashboard.DashboardResponse
	started  string
	startErr error
	query    dashboard.ActivityQuery
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
	request := httptest.NewRequest(http.MethodGet, "/api/activity?group_by=contributor&metric=pull_requests&owner_id=7&repository_id=9", nil)
	response := httptest.NewRecorder()
	NewRouter(t.TempDir(), service).ServeHTTP(response, request)
	if response.Code != http.StatusOK {
		t.Fatalf("expected OK, got %d: %s", response.Code, response.Body.String())
	}
	if service.query != (dashboard.ActivityQuery{Group: dashboard.ActivityByContributor, Metric: dashboard.ActivityPullRequests, OwnerID: 7, RepositoryID: 9}) {
		t.Fatalf("unexpected activity query: %+v", service.query)
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
