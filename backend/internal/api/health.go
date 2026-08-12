package api

import "net/http"

func healthHandler(response http.ResponseWriter, _ *http.Request) {
	writeJSON(response, http.StatusOK, []byte(`{"status":"ok"}`))
}

func writeJSON(response http.ResponseWriter, status int, body []byte) {
	response.Header().Set("Content-Type", "application/json; charset=utf-8")
	response.WriteHeader(status)
	_, _ = response.Write(append(body, '\n'))
}
