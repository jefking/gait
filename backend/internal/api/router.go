package api

import (
	"context"
	"net/http"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/jefking/gait/backend/internal/dashboard"
)

type DashboardService interface {
	Dashboard() dashboard.DashboardResponse
	DiscoverTargets(string) (dashboard.TargetDiscovery, error)
	SelectTarget(int64) (dashboard.DashboardResponse, error)
	Start() (dashboard.SyncStatus, error)
	Subscribe(context.Context) <-chan dashboard.DashboardEvent
}

type InsightsService interface {
	InsightDelivery(dashboard.InsightQuery) (dashboard.DeliveryResponse, error)
	InsightNetwork(dashboard.InsightQuery) (dashboard.NetworkResponse, error)
	ScopedIdentities(dashboard.InsightQuery) dashboard.IdentityResponse
	UpdateIdentity(string, dashboard.IdentityOverride) (dashboard.IdentityResponse, error)
}

// NewRouter creates the application router. API routes are registered before
// the static fallback so they can never be mistaken for frontend routes.
func NewRouter(staticDir string, services ...DashboardService) http.Handler {
	router := chi.NewRouter()
	router.Use(middleware.RequestID)
	router.Use(middleware.RealIP)
	router.Use(middleware.Logger)
	router.Use(middleware.Recoverer)

	router.Get("/api/health", healthHandler)
	if len(services) > 0 && services[0] != nil {
		service := services[0]
		router.Get("/api/dashboard", dashboardHandler(service))
		if insights, ok := service.(InsightsService); ok {
			router.Get("/api/insights/delivery", deliveryHandler(insights))
			router.Get("/api/insights/network", networkHandler(insights))
			router.Get("/api/identities", identitiesHandler(insights))
			router.Patch("/api/identities/{key}", updateIdentityHandler(insights))
		}
		router.Get("/api/events", eventsHandler(service))
		router.Post("/api/github/targets", githubTargetsHandler(service))
		router.Put("/api/configuration/github-target", selectGitHubTargetHandler(service))
		router.Post("/api/sync", syncHandler(service))
	}

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
