package server

import (
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
)

// handleImageProxy takes ?server=<machineID>&path=<plex path> and streams the
// image with Plex's token appended server-side. The SPA never sees the token.
//
// We fetch the direct thumb path (/library/metadata/<id>/thumb/<ts>) rather
// than routing through /photo/:/transcode, because some Plex deployments front
// their server with a CDN/proxy that rejects the /:/transcode path (the
// colon-slash-colon segment trips certain HTTP proxies). The direct path is
// what Plex's own web client uses and is universally supported.
//
// Trade-off: when a specific thumb variant is missing upstream, Plex returns
// 404 and we pass that through as 502 → the card falls back to the placeholder.
// The /photo/:/transcode approach would generate a fallback poster, but that
// feature isn't worth losing DKNZPLEX coverage.
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
		var snippet []byte
		if b, _ := io.ReadAll(io.LimitReader(resp.Body, 512)); len(b) > 0 {
			snippet = b
		}
		writeError(w, http.StatusBadGateway, fmt.Sprintf("upstream %s for %s — body: %q", resp.Status, target, snippet))
		return
	}

	if ct := resp.Header.Get("Content-Type"); ct != "" {
		w.Header().Set("Content-Type", ct)
	}
	w.Header().Set("Cache-Control", "public, max-age=2592000, immutable")
	w.WriteHeader(http.StatusOK)
	_, _ = io.Copy(w, resp.Body)
}
