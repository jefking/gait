package api

import (
	"net/http"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
)

// NewRouter creates the application router. API routes are registered before
// the static fallback so they can never be mistaken for frontend routes.
func NewRouter(staticDir string) http.Handler {
	router := chi.NewRouter()
	router.Use(middleware.RequestID)
	router.Use(middleware.RealIP)
	router.Use(middleware.Logger)
	router.Use(middleware.Recoverer)

	router.Get("/api/health", healthHandler)

	frontend := newSPAHandler(staticDir)
	router.NotFound(func(response http.ResponseWriter, request *http.Request) {
		if isAPIPath(request.URL.Path) {
			writeJSON(response, http.StatusNotFound, []byte(`{"error":"not found"}`))
			return
		}

		frontend.ServeHTTP(response, request)
	})

	return router
}

func isAPIPath(requestPath string) bool {
	return requestPath == "/api" || strings.HasPrefix(requestPath, "/api/")
}
