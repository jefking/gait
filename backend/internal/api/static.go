package api

import (
	"net/http"
	"os"
	"path"
	"path/filepath"
	"strings"
)

func newSPAHandler(staticDir string) http.Handler {
	fileServer := http.FileServer(http.Dir(staticDir))

	return http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
		if request.Method != http.MethodGet && request.Method != http.MethodHead {
			response.Header().Set("Allow", "GET, HEAD")
			http.Error(response, http.StatusText(http.StatusMethodNotAllowed), http.StatusMethodNotAllowed)
			return
		}

		cleanPath := path.Clean("/" + request.URL.Path)
		relativePath := strings.TrimPrefix(cleanPath, "/")
		requestedFile := filepath.Join(staticDir, filepath.FromSlash(relativePath))

		if fileInfo, err := os.Stat(requestedFile); err == nil && !fileInfo.IsDir() {
			if relativePath == "index.html" {
				response.Header().Set("Cache-Control", "no-cache, no-store, must-revalidate")
			}
			fileServer.ServeHTTP(response, request)
			return
		}

		// Missing paths that look like assets should remain 404s. Extensionless
		// paths are treated as future client-side routes and receive index.html.
		if filepath.Ext(relativePath) != "" {
			http.NotFound(response, request)
			return
		}

		indexPath := filepath.Join(staticDir, "index.html")
		if fileInfo, err := os.Stat(indexPath); err != nil || fileInfo.IsDir() {
			http.NotFound(response, request)
			return
		}

		indexRequest := request.Clone(request.Context())
		indexRequest.URL.Path = "/"
		response.Header().Set("Cache-Control", "no-cache, no-store, must-revalidate")
		fileServer.ServeHTTP(response, indexRequest)
	})
}
