package server

import (
	"embed"
	"io"
	"io/fs"
	"net/http"
	"strings"
)

//go:embed all:dist
var embeddedWebFS embed.FS

// spaHandler serves embedded React 19 single-page application static files.
func (s *Server) spaHandler() http.Handler {
	distFS, err := fs.Sub(embeddedWebFS, "dist")
	if err != nil {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			http.Error(w, "embedded web assets missing", http.StatusInternalServerError)
		})
	}

	fileServer := http.FileServer(http.FS(distFS))

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		path := r.URL.Path

		// Never intercept API or Prometheus requests
		if strings.HasPrefix(path, "/api") || strings.HasPrefix(path, "/metrics") {
			http.NotFound(w, r)
			return
		}

		cleanPath := strings.TrimPrefix(path, "/")
		if cleanPath == "" {
			cleanPath = "index.html"
		}

		// Check if file physically exists in embedded filesystem
		f, err := distFS.Open(cleanPath)
		if err == nil {
			_ = f.Close()
			fileServer.ServeHTTP(w, r)
			return
		}

		// Fallback to index.html for client-side SPA routing
		indexFile, err := distFS.Open("index.html")
		if err != nil {
			http.NotFound(w, r)
			return
		}
		defer indexFile.Close()

		stat, _ := indexFile.Stat()
		if rs, ok := indexFile.(io.ReadSeeker); ok {
			http.ServeContent(w, r, "index.html", stat.ModTime(), rs)
		} else {
			data, _ := io.ReadAll(indexFile)
			w.Header().Set("Content-Type", "text/html; charset=utf-8")
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write(data)
		}
	})
}
