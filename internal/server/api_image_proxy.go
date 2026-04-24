package server

import (
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
)

// Poster sizing defaults. Plex's /photo/:/transcode endpoint renders at these
// dimensions, falling back gracefully when the exact source thumb is missing.
// 2x the rendered card width so retina displays look crisp.
const (
	defaultImageWidth  = 320
	defaultImageHeight = 480
)

// handleImageProxy takes ?server=<machineID>&path=<plex path>[&w=<px>&h=<px>]
// and streams the image via Plex's photo transcoder, which handles missing-thumb
// fallbacks and sizing server-side. The X-Plex-Token is appended server-side;
// the SPA never sees it.
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

	// Route through Plex's photo transcoder for reliable delivery + sizing.
	// Path format: /photo/:/transcode?url=<server-relative path>&width=W&height=H&minSize=1&upscale=1&X-Plex-Token=...
	tq := url.Values{
		"url":             []string{path},
		"width":           []string{strconv.Itoa(width)},
		"height":          []string{strconv.Itoa(height)},
		"minSize":         []string{"1"},
		"upscale":         []string{"1"},
		"X-Plex-Token":    []string{srv.AccessToken},
	}
	target := srv.LastGoodConnection + "/photo/:/transcode?" + tq.Encode()

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
		// Capture upstream body (truncated) so we can diagnose what Plex rejected.
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
	w.Header().Set("Cache-Control", "public, max-age=2592000, immutable") // 30 days
	w.WriteHeader(http.StatusOK)
	_, _ = io.Copy(w, resp.Body)
}
