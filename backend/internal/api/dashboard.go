package api

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/jefking/gait/backend/internal/dashboard"
)

func deliveryHandler(service InsightsService) http.HandlerFunc {
	return func(response http.ResponseWriter, request *http.Request) {
		query, err := parseScopeQuery(request)
		if err != nil {
			writeJSONValue(response, http.StatusBadRequest, map[string]string{"error": err.Error()})
			return
		}
		result, err := service.InsightDelivery(query)
		writeInsightResponse(response, result, err)
	}
}

func networkHandler(service InsightsService) http.HandlerFunc {
	return func(response http.ResponseWriter, request *http.Request) {
		query, err := parseScopeQuery(request)
		if err != nil {
			writeJSONValue(response, http.StatusBadRequest, map[string]string{"error": err.Error()})
			return
		}
		result, err := service.InsightNetwork(query)
		writeInsightResponse(response, result, err)
	}
}

func identitiesHandler(service InsightsService) http.HandlerFunc {
	return func(response http.ResponseWriter, request *http.Request) {
		query, err := parseScopeQuery(request)
		if err != nil {
			writeJSONValue(response, http.StatusBadRequest, map[string]string{"error": err.Error()})
			return
		}
		response.Header().Set("Cache-Control", "no-store")
		writeJSONValue(response, http.StatusOK, service.ScopedIdentities(query))
	}
}

func parseScopeQuery(request *http.Request) (dashboard.InsightQuery, error) {
	query := dashboard.InsightQuery{}
	var err error
	if request.URL.Query().Has("organization_id") {
		return query, errors.New("organization_id is no longer supported; GitHub target selection is configured in settings")
	}
	query.ExcludeDead, err = optionalBool(request.URL.Query().Get("exclude_dead"))
	if err != nil {
		return query, errors.New("exclude_dead must be true or false")
	}
	query.RepositoryID, err = optionalPositiveInt64(request.URL.Query().Get("repository_id"))
	if err != nil {
		return query, errors.New("repository_id must be a positive integer")
	}
	query.From, err = optionalDate(request.URL.Query().Get("from"))
	if err != nil {
		return query, errors.New("from must use YYYY-MM-DD")
	}
	query.To, err = optionalDate(request.URL.Query().Get("to"))
	if err != nil {
		return query, errors.New("to must use YYYY-MM-DD")
	}
	return query, nil
}

func updateIdentityHandler(service InsightsService) http.HandlerFunc {
	return func(response http.ResponseWriter, request *http.Request) {
		key, err := url.PathUnescape(chi.URLParam(request, "key"))
		if err != nil || strings.TrimSpace(key) == "" {
			writeJSONValue(response, http.StatusBadRequest, map[string]string{"error": "identity key is required"})
			return
		}
		request.Body = http.MaxBytesReader(response, request.Body, 16<<10)
		decoder := json.NewDecoder(request.Body)
		decoder.DisallowUnknownFields()
		var override dashboard.IdentityOverride
		if err := decoder.Decode(&override); err != nil {
			writeJSONValue(response, http.StatusBadRequest, map[string]string{"error": "request must contain an identity override"})
			return
		}
		if err := decoder.Decode(&struct{}{}); err != io.EOF {
			writeJSONValue(response, http.StatusBadRequest, map[string]string{"error": "request must contain one JSON object"})
			return
		}
		result, err := service.UpdateIdentity(key, override)
		if err != nil {
			writeJSONValue(response, http.StatusBadRequest, map[string]string{"error": err.Error()})
			return
		}
		response.Header().Set("Cache-Control", "no-store")
		writeJSONValue(response, http.StatusOK, result)
	}
}

func writeInsightResponse(response http.ResponseWriter, value any, err error) {
	if err != nil {
		writeJSONValue(response, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}
	response.Header().Set("Cache-Control", "no-store")
	writeJSONValue(response, http.StatusOK, value)
}

const maximumPATLength = 4096

func dashboardHandler(service DashboardService) http.HandlerFunc {
	return func(response http.ResponseWriter, _ *http.Request) {
		response.Header().Set("Cache-Control", "no-store")
		writeJSONValue(response, http.StatusOK, service.Dashboard())
	}
}

func eventsHandler(service DashboardService) http.HandlerFunc {
	return func(response http.ResponseWriter, request *http.Request) {
		flusher, ok := response.(http.Flusher)
		if !ok {
			http.Error(response, "streaming is unavailable", http.StatusInternalServerError)
			return
		}
		response.Header().Set("Content-Type", "text/event-stream")
		response.Header().Set("Cache-Control", "no-cache, no-store")
		response.Header().Set("Connection", "keep-alive")
		response.Header().Set("X-Accel-Buffering", "no")
		_ = http.NewResponseController(response).SetWriteDeadline(time.Time{})

		events := service.Subscribe(request.Context())
		keepAlive := time.NewTicker(15 * time.Second)
		defer keepAlive.Stop()
		for {
			select {
			case event, open := <-events:
				if !open {
					return
				}
				payload, err := json.Marshal(event)
				if err != nil {
					return
				}
				if _, err := fmt.Fprintf(response, "event: dashboard\ndata: %s\n\n", payload); err != nil {
					return
				}
				flusher.Flush()
			case <-keepAlive.C:
				if _, err := fmt.Fprint(response, ": keep-alive\n\n"); err != nil {
					return
				}
				flusher.Flush()
			case <-request.Context().Done():
				return
			}
		}
	}
}

func syncHandler(service DashboardService) http.HandlerFunc {
	return func(response http.ResponseWriter, request *http.Request) {
		request.Body = http.MaxBytesReader(response, request.Body, 8<<10)
		decoder := json.NewDecoder(request.Body)
		var input map[string]json.RawMessage
		if err := decoder.Decode(&input); err != nil || input == nil || len(input) != 0 {
			writeJSONValue(response, http.StatusBadRequest, map[string]string{"error": "request must contain one empty JSON object"})
			return
		}
		if err := decoder.Decode(&struct{}{}); err != io.EOF {
			writeJSONValue(response, http.StatusBadRequest, map[string]string{"error": "request must contain one JSON object"})
			return
		}
		status, err := service.Start()
		if errors.Is(err, dashboard.ErrSyncActive) {
			writeJSONValue(response, http.StatusConflict, map[string]any{"error": err.Error(), "sync": status})
			return
		}
		if err != nil {
			writeJSONValue(response, http.StatusBadRequest, map[string]string{"error": err.Error()})
			return
		}
		response.Header().Set("Cache-Control", "no-store")
		writeJSONValue(response, http.StatusAccepted, map[string]dashboard.SyncStatus{"sync": status})
	}
}

func githubTargetsHandler(service DashboardService) http.HandlerFunc {
	type targetRequest struct {
		PAT string `json:"pat"`
	}
	return func(response http.ResponseWriter, request *http.Request) {
		request.Body = http.MaxBytesReader(response, request.Body, 8<<10)
		decoder := json.NewDecoder(request.Body)
		decoder.DisallowUnknownFields()
		var input targetRequest
		if err := decoder.Decode(&input); err != nil {
			writeJSONValue(response, http.StatusBadRequest, map[string]string{"error": "request must contain a JSON object"})
			return
		}
		if err := decoder.Decode(&struct{}{}); err != io.EOF {
			writeJSONValue(response, http.StatusBadRequest, map[string]string{"error": "request must contain one JSON object"})
			return
		}
		input.PAT = strings.TrimSpace(input.PAT)
		if len(input.PAT) > maximumPATLength {
			writeJSONValue(response, http.StatusBadRequest, map[string]string{"error": "PAT must not exceed 4096 characters"})
			return
		}
		pat := input.PAT
		discovery, err := service.DiscoverTargets(pat)
		errorMessage := ""
		if err != nil {
			errorMessage = err.Error()
			if pat != "" {
				errorMessage = strings.ReplaceAll(errorMessage, pat, "[redacted]")
			}
		}
		input.PAT = ""
		pat = ""
		if errors.Is(err, dashboard.ErrSyncActive) {
			writeJSONValue(response, http.StatusConflict, map[string]string{"error": errorMessage})
			return
		}
		if err != nil {
			writeJSONValue(response, http.StatusBadRequest, map[string]string{"error": errorMessage})
			return
		}
		response.Header().Set("Cache-Control", "no-store")
		writeJSONValue(response, http.StatusOK, discovery)
	}
}

func selectGitHubTargetHandler(service DashboardService) http.HandlerFunc {
	type selectionRequest struct {
		TargetID int64 `json:"target_id"`
	}
	return func(response http.ResponseWriter, request *http.Request) {
		request.Body = http.MaxBytesReader(response, request.Body, 8<<10)
		decoder := json.NewDecoder(request.Body)
		decoder.DisallowUnknownFields()
		var input selectionRequest
		if err := decoder.Decode(&input); err != nil || input.TargetID <= 0 {
			writeJSONValue(response, http.StatusBadRequest, map[string]string{"error": "target_id must be a positive integer"})
			return
		}
		if err := decoder.Decode(&struct{}{}); err != io.EOF {
			writeJSONValue(response, http.StatusBadRequest, map[string]string{"error": "request must contain one JSON object"})
			return
		}
		result, err := service.SelectTarget(input.TargetID)
		if errors.Is(err, dashboard.ErrSyncActive) {
			writeJSONValue(response, http.StatusConflict, map[string]any{"error": err.Error(), "dashboard": result})
			return
		}
		if err != nil {
			writeJSONValue(response, http.StatusBadRequest, map[string]string{"error": err.Error()})
			return
		}
		response.Header().Set("Cache-Control", "no-store")
		writeJSONValue(response, http.StatusAccepted, result)
	}
}

func optionalPositiveInt64(value string) (int64, error) {
	if value == "" {
		return 0, nil
	}
	parsed, err := strconv.ParseInt(value, 10, 64)
	if err != nil || parsed <= 0 {
		return 0, errors.New("not a positive integer")
	}
	return parsed, nil
}

func optionalDate(value string) (*time.Time, error) {
	if value == "" {
		return nil, nil
	}
	parsed, err := time.Parse(time.DateOnly, value)
	if err != nil {
		return nil, err
	}
	return &parsed, nil
}

func optionalBool(value string) (bool, error) {
	if value == "" {
		return false, nil
	}
	return strconv.ParseBool(value)
}

func writeJSONValue(response http.ResponseWriter, status int, value any) {
	response.Header().Set("Content-Type", "application/json; charset=utf-8")
	response.WriteHeader(status)
	_ = json.NewEncoder(response).Encode(value)
}
