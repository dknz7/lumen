package server

import (
	"embed"
	"io/fs"
	"net/http"
	"strings"
)

//go:embed all:web/dist
var spaFS embed.FS

func spaFilesystem() (fs.FS, error) {
	return fs.Sub(spaFS, "web/dist")
}

// handleSPA serves files from the embedded SPA bundle. For paths that don't
// match an asset (e.g. /library/abc/1), it falls back to index.html so the
// SolidJS client-side router can take over.
func (s *Server) handleSPA(w http.ResponseWriter, r *http.Request) {
	if strings.HasPrefix(r.URL.Path, "/api/") {
		writeError(w, http.StatusNotFound, "unknown endpoint")
		return
	}
	sub, err := spaFilesystem()
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	path := strings.TrimPrefix(r.URL.Path, "/")
	if path == "" {
		path = "index.html"
	}
	if _, err := fs.Stat(sub, path); err != nil {
		path = "index.html"
	}
	http.ServeFileFS(w, r, sub, path)
}
