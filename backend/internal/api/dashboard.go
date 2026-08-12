package api

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strconv"
	"strings"

	"github.com/jefking/gait/backend/internal/dashboard"
)

const maximumPATLength = 4096

func dashboardHandler(service DashboardService) http.HandlerFunc {
	return func(response http.ResponseWriter, _ *http.Request) {
		response.Header().Set("Cache-Control", "no-store")
		writeJSONValue(response, http.StatusOK, service.Dashboard())
	}
}

func activityHandler(service DashboardService) http.HandlerFunc {
	return func(response http.ResponseWriter, request *http.Request) {
		query := dashboard.ActivityQuery{
			Group:  dashboard.ActivityGroup(request.URL.Query().Get("group_by")),
			Metric: dashboard.ActivityMetric(request.URL.Query().Get("metric")),
		}
		if query.Group == "" {
			query.Group = dashboard.ActivityByOwner
		}
		if query.Metric == "" {
			query.Metric = dashboard.ActivityCommits
		}
		var err error
		query.OwnerID, err = optionalPositiveInt64(request.URL.Query().Get("owner_id"))
		if err != nil {
			writeJSONValue(response, http.StatusBadRequest, map[string]string{"error": "owner_id must be a positive integer"})
			return
		}
		query.RepositoryID, err = optionalPositiveInt64(request.URL.Query().Get("repository_id"))
		if err != nil {
			writeJSONValue(response, http.StatusBadRequest, map[string]string{"error": "repository_id must be a positive integer"})
			return
		}
		activity, err := service.Activity(query)
		if err != nil {
			writeJSONValue(response, http.StatusBadRequest, map[string]string{"error": err.Error()})
			return
		}
		response.Header().Set("Cache-Control", "no-store")
		writeJSONValue(response, http.StatusOK, activity)
	}
}

func syncHandler(service DashboardService) http.HandlerFunc {
	type syncRequest struct {
		PAT string `json:"pat"`
	}
	return func(response http.ResponseWriter, request *http.Request) {
		request.Body = http.MaxBytesReader(response, request.Body, 8<<10)
		decoder := json.NewDecoder(request.Body)
		decoder.DisallowUnknownFields()
		var input syncRequest
		if err := decoder.Decode(&input); err != nil {
			writeJSONValue(response, http.StatusBadRequest, map[string]string{"error": "request must contain a PAT"})
			return
		}
		if err := decoder.Decode(&struct{}{}); err != io.EOF {
			writeJSONValue(response, http.StatusBadRequest, map[string]string{"error": "request must contain one JSON object"})
			return
		}
		input.PAT = strings.TrimSpace(input.PAT)
		if input.PAT == "" || len(input.PAT) > maximumPATLength {
			writeJSONValue(response, http.StatusBadRequest, map[string]string{"error": "PAT must be between 1 and 4096 characters"})
			return
		}
		status, err := service.Start(input.PAT)
		input.PAT = ""
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

func writeJSONValue(response http.ResponseWriter, status int, value any) {
	response.Header().Set("Content-Type", "application/json; charset=utf-8")
	response.WriteHeader(status)
	_ = json.NewEncoder(response).Encode(value)
}
