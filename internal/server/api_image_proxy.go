package server

import (
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
)

const (
	defaultImageWidth  = 320
	defaultImageHeight = 480
)

// handleImageProxy fetches a Plex poster/art via the server's own
// /photo/:/transcode endpoint, matching the exact URL format Plex Web uses.
// The SPA never sees the token.
//
// Critical format details (discovered Session 2 against DKNZPLEX's Level 3 CDN):
//  1. The inner "url" query param MUST keep raw slashes — URL-encoded %2F
//     causes the CDN to return 404. This bypasses Go's url.Values.Encode()
//     which would otherwise escape them.
//  2. The X-Plex-Token must appear TWICE: once inside the inner url value
//     (authenticates the inner fetch) and once as an outer query param
//     (authenticates the transcode request itself). Plex Web does this.
//
// This specific format is load-bearing for CDN-fronted Plex deployments.
// Don't simplify it with url.Values without re-verifying against Byron's setup.
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

	width := defaultImageWidth
	if v, err := strconv.Atoi(q.Get("w")); err == nil && v > 0 && v <= 4096 {
		width = v
	}
	height := defaultImageHeight
	if v, err := strconv.Atoi(q.Get("h")); err == nil && v > 0 && v <= 4096 {
		height = v
	}

	// Strip the default HTTPS port so our URL matches Plex Web's format.
	base := strings.TrimSuffix(srv.LastGoodConnection, ":443")

	// Manual query-string build — preserves raw slashes in `url=` param and
	// places X-Plex-Token both inside the inner url AND as an outer param.
	token := url.QueryEscape(srv.AccessToken)
	target := fmt.Sprintf(
		"%s/photo/:/transcode?width=%d&height=%d&minSize=1&upscale=1&url=%s?X-Plex-Token=%s&X-Plex-Token=%s",
		base,
		width,
		height,
		path, // raw — not url-encoded so / stays as /
		token,
		token,
	)

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
