package server

import (
	"io"
	"net/http"
	"net/url"
	"strings"
)

// handleImageProxy takes ?server=<machineID>&path=<plex path> and streams the image.
// The X-Plex-Token is appended server-side; the SPA never sees it.
func (s *Server) handleImageProxy(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	machineID := q.Get("server")
	path := q.Get("path")
	if machineID == "" {
		writeError(w, http.StatusBadRequest, "server query param required")
		return
	}
	if path == "" {
		writeError(w, http.StatusBadRequest, "path query param required")
		return
	}
	// Reject anything that doesn't look like a bare server-relative path.
	// Plex image paths always start with "/library/" or similar.
	if !strings.HasPrefix(path, "/") {
		writeError(w, http.StatusBadRequest, "path must start with /")
		return
	}
	if strings.Contains(path, "..") {
		writeError(w, http.StatusBadRequest, "path traversal not allowed")
		return
	}
	if u, err := url.Parse(path); err == nil && (u.Scheme != "" || u.Host != "") {
		writeError(w, http.StatusBadRequest, "path must be server-relative")
		return
	}
	srv := s.serverByID(machineID)
	if srv == nil {
		writeError(w, http.StatusNotFound, "unknown server")
		return
	}
	if srv.LastGoodConnection == "" {
		writeError(w, http.StatusBadGateway, "server has no cached connection")
		return
	}

	target := srv.LastGoodConnection + path
	if strings.Contains(path, "?") {
		target += "&X-Plex-Token=" + url.QueryEscape(srv.AccessToken)
	} else {
		target += "?X-Plex-Token=" + url.QueryEscape(srv.AccessToken)
	}

	req, err := http.NewRequestWithContext(r.Context(), "GET", target, nil)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		writeError(w, http.StatusBadGateway, err.Error())
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		writeError(w, http.StatusBadGateway, "upstream status "+resp.Status)
		return
	}

	// Forward Content-Type and set a long cache lifetime (posters are immutable).
	if ct := resp.Header.Get("Content-Type"); ct != "" {
		w.Header().Set("Content-Type", ct)
	}
	w.Header().Set("Cache-Control", "public, max-age=2592000, immutable") // 30 days
	w.WriteHeader(http.StatusOK)
	_, _ = io.Copy(w, resp.Body)
}
