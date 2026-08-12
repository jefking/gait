package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestHealth(t *testing.T) {
	request := httptest.NewRequest(http.MethodGet, "/api/health", nil)
	response := httptest.NewRecorder()

	NewRouter(t.TempDir()).ServeHTTP(response, request)

	if response.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, response.Code)
	}

	if contentType := response.Header().Get("Content-Type"); contentType != "application/json; charset=utf-8" {
		t.Fatalf("expected JSON content type, got %q", contentType)
	}

	var body struct {
		Status string `json:"status"`
	}
	if err := json.NewDecoder(response.Body).Decode(&body); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if body.Status != "ok" {
		t.Fatalf("expected status body to be ok, got %q", body.Status)
	}
}

func TestUnknownAPIRouteReturnsJSON404(t *testing.T) {
	request := httptest.NewRequest(http.MethodGet, "/api/missing", nil)
	response := httptest.NewRecorder()

	NewRouter(t.TempDir()).ServeHTTP(response, request)

	if response.Code != http.StatusNotFound {
		t.Fatalf("expected status %d, got %d", http.StatusNotFound, response.Code)
	}

	if contentType := response.Header().Get("Content-Type"); contentType != "application/json; charset=utf-8" {
		t.Fatalf("expected JSON content type, got %q", contentType)
	}
}

func TestStaticAndSPARoutes(t *testing.T) {
	staticDir := t.TempDir()
	imagesDir := filepath.Join(staticDir, "images")
	if err := os.MkdirAll(imagesDir, 0o755); err != nil {
		t.Fatalf("create images directory: %v", err)
	}
	if err := os.WriteFile(filepath.Join(staticDir, "index.html"), []byte("<h1>Gait</h1>"), 0o644); err != nil {
		t.Fatalf("write index: %v", err)
	}
	if err := os.WriteFile(filepath.Join(imagesDir, "example.txt"), []byte("static image"), 0o644); err != nil {
		t.Fatalf("write static fixture: %v", err)
	}

	handler := NewRouter(staticDir)
	tests := []struct {
		name    string
		path    string
		status  int
		body    string
		noCache bool
	}{
		{name: "root", path: "/", status: http.StatusOK, body: "<h1>Gait</h1>", noCache: true},
		{name: "static file", path: "/images/example.txt", status: http.StatusOK, body: "static image"},
		{name: "client route", path: "/future/route", status: http.StatusOK, body: "<h1>Gait</h1>", noCache: true},
		{name: "missing asset", path: "/images/missing.png", status: http.StatusNotFound, body: "404 page not found"},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			request := httptest.NewRequest(http.MethodGet, test.path, nil)
			response := httptest.NewRecorder()

			handler.ServeHTTP(response, request)

			if response.Code != test.status {
				t.Fatalf("expected status %d, got %d", test.status, response.Code)
			}
			if !strings.Contains(response.Body.String(), test.body) {
				t.Fatalf("expected body to contain %q, got %q", test.body, response.Body.String())
			}
			if test.noCache && response.Header().Get("Cache-Control") != "no-cache, no-store, must-revalidate" {
				t.Fatalf("expected HTML shell to disable caching, got %q", response.Header().Get("Cache-Control"))
			}
		})
	}
}

func TestFrontendRejectsUnsupportedMethods(t *testing.T) {
	request := httptest.NewRequest(http.MethodPost, "/future/route", nil)
	response := httptest.NewRecorder()

	NewRouter(t.TempDir()).ServeHTTP(response, request)

	if response.Code != http.StatusMethodNotAllowed {
		t.Fatalf("expected status %d, got %d", http.StatusMethodNotAllowed, response.Code)
	}
	if allow := response.Header().Get("Allow"); allow != "GET, HEAD" {
		t.Fatalf("expected Allow header, got %q", allow)
	}
}
