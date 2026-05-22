package app

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"mime"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/go-chi/chi/v5"
)

func NewRouter(cfg Config, store *Store) http.Handler {
	router := chi.NewRouter()

	router.Route("/api", func(api chi.Router) {
		api.Use(jsonBodyLimit(cfg.MaxRequestBodyBytes))
		api.Get("/health", func(w http.ResponseWriter, r *http.Request) {
			writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
		})
		// Issue 01 exposes only enough API shape for the static app shell and
		// later migration slices; poll behavior remains on the Python server.
		api.Get("/polls", func(w http.ResponseWriter, r *http.Request) {
			writeJSON(w, http.StatusOK, map[string][]any{"polls": []any{}})
		})
		api.Post("/polls", func(w http.ResponseWriter, r *http.Request) {
			if err := discardBody(r.Body); err != nil {
				var maxBytesError *http.MaxBytesError
				if errors.As(err, &maxBytesError) {
					writeJSON(
						w,
						http.StatusRequestEntityTooLarge,
						map[string]string{"error": fmt.Sprintf("Request body must be %d bytes or fewer.", cfg.MaxRequestBodyBytes)},
					)
					return
				}
				writeJSON(w, http.StatusBadRequest, map[string]string{"error": "Request body could not be read."})
				return
			}
			writeJSON(w, http.StatusNotImplemented, map[string]string{"error": "Poll creation is not implemented yet."})
		})
	})

	// Keep static serving as an explicit allowlist so the Go server only
	// publishes the existing plain frontend assets during the migration.
	staticFiles := map[string]string{
		"/":             "index.html",
		"/index.html":   "index.html",
		"/vote.html":    "vote.html",
		"/results.html": "results.html",
		"/styles.css":   "styles.css",
		"/shared.js":    "shared.js",
		"/host.js":      "host.js",
		"/vote.js":      "vote.js",
		"/results.js":   "results.js",
	}
	for route, fileName := range staticFiles {
		fileName := fileName
		router.Get(route, func(w http.ResponseWriter, r *http.Request) {
			serveStaticFile(w, filepath.Join(cfg.StaticDir, fileName))
		})
	}

	router.NotFound(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasPrefix(r.URL.Path, "/api/") {
			writeJSON(w, http.StatusNotFound, map[string]string{"error": "Route not found."})
			return
		}
		http.NotFound(w, r)
	})

	// The router accepts the store now so later vertical slices can add poll
	// handlers without changing the construction path tested in this slice.
	_ = store
	return router
}

func serveStaticFile(w http.ResponseWriter, filePath string) {
	content, err := os.ReadFile(filePath)
	if err != nil {
		http.Error(w, "404 page not found", http.StatusNotFound)
		return
	}

	contentType := mime.TypeByExtension(filepath.Ext(filePath))
	if contentType == "" {
		contentType = http.DetectContentType(content)
	}
	if strings.HasPrefix(contentType, "text/") || contentType == "application/javascript" {
		contentType += "; charset=utf-8"
	}

	w.Header().Set("Content-Type", contentType)
	w.Header().Set("Content-Length", fmt.Sprintf("%d", len(content)))
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(content)
}

func NewHTTPServer(cfg Config, handler http.Handler) *http.Server {
	return &http.Server{
		Addr:              net.JoinHostPort(cfg.Host, cfg.Port),
		Handler:           handler,
		ReadHeaderTimeout: cfg.ReadHeaderTimeout,
		ReadTimeout:       cfg.ReadTimeout,
		WriteTimeout:      cfg.WriteTimeout,
		IdleTimeout:       cfg.IdleTimeout,
	}
}

func jsonBodyLimit(maxBodyBytes int64) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if isJSONRequestWithBody(r) {
				// Prefer rejecting known-oversize JSON requests before handlers
				// read the body, then cap streaming reads for missing lengths.
				if r.ContentLength > maxBodyBytes {
					writeJSON(
						w,
						http.StatusRequestEntityTooLarge,
						map[string]string{"error": fmt.Sprintf("Request body must be %d bytes or fewer.", maxBodyBytes)},
					)
					return
				}
				r.Body = http.MaxBytesReader(w, r.Body, maxBodyBytes)
			}
			next.ServeHTTP(w, r)
		})
	}
}

func isJSONRequestWithBody(r *http.Request) bool {
	if r.Body == nil || (r.Method != http.MethodPost && r.Method != http.MethodPut && r.Method != http.MethodPatch) {
		return false
	}
	contentType := strings.ToLower(r.Header.Get("Content-Type"))
	return strings.HasPrefix(contentType, "application/json")
}

func discardBody(body io.Reader) error {
	_, err := io.Copy(io.Discard, body)
	return err
}

func writeJSON(w http.ResponseWriter, status int, payload any) {
	content, err := json.Marshal(payload)
	if err != nil {
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Header().Set("Content-Length", fmt.Sprintf("%d", len(content)))
	w.WriteHeader(status)
	_, _ = w.Write(content)
}
