package server

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
)

// Defaults match Plex Web's poster-cell request (240×360 — Session 5 post-smoke
// capture) so we share Stargaze's CDN cache instead of cold-missing on every
// first paint with our own dimension permutation.
const (
	defaultImageWidth  = 240
	defaultImageHeight = 360
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
//
// TODO(session-4.5+): Stargaze movie thumbnails 404 through this proxy
// while episodes from the same server load correctly. DKNZPLEX is fine for
// both. Hypothesis: Stargaze returns `thumb` paths in a different format
// for movies (type=1) vs episodes (type=4), OR this handler's URL-rewrite
// special-cases episode grandparentThumb fallback in a way that doesn't
// fire for movie thumb. Investigation steps documented in
// docs/session-4-findings.md → "Known Issues (Stargaze movie thumbnails —
// deferred)". Capture failing + working URLs from DevTools and compare.
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

	// Disk cache lookup before hitting Plex. Key includes size so different
	// resolutions of the same thumb cache separately.
	cacheKey := s.images.key(machineID, path, width, height)
	if ct, bytes, ok := s.images.get(cacheKey); ok {
		w.Header().Set("Content-Type", ct)
		w.Header().Set("Cache-Control", "public, max-age=2592000, immutable")
		w.Header().Set("X-Lumen-Cache", "hit")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write(bytes)
		return
	}

	// Strip the default HTTPS port so our URL matches Plex Web's format.
	base := strings.TrimSuffix(srv.LastGoodConnection, ":443")

	// Token try-with-fallback: Plex Web uses the per-server token for some
	// servers (Stargaze observed Session 5 post-smoke) and the account token
	// for others (DKNZPLEX, Session 2). Any non-200 (404 is the common case;
	// 401/403 also possible) is the signal to retry with the other. The
	// handler caching whichever worked for next time would be ideal but for
	// v1.0 we just try both per request — the disk cache hit rate makes the
	// cost negligible.
	resp, used, err := s.fetchImageProxyWithFallback(r.Context(), base, path, width, height, srv.AccessToken)
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
		writeError(w, http.StatusBadGateway, fmt.Sprintf("upstream %s for %s/%s [token=%s] — body: %q", resp.Status, base, path, used, snippet))
		return
	}

	// Buffer the body so we can both serve it and write it to the disk cache.
	// Thumbs are small (tens of KB typically); the RAM cost is negligible
	// compared to the round-trip savings on subsequent requests.
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		writeError(w, http.StatusBadGateway, "read body: "+err.Error())
		return
	}
	ct := resp.Header.Get("Content-Type")
	if ct == "" {
		ct = "image/jpeg"
	}

	// Best-effort cache write; if it fails (disk full, permissions) we still
	// serve the response. Logged silently — a cache miss on next request just
	// means we re-fetch.
	_ = s.images.put(cacheKey, ct, body)

	w.Header().Set("Content-Type", ct)
	w.Header().Set("Cache-Control", "public, max-age=2592000, immutable")
	w.Header().Set("X-Lumen-Cache", "miss")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(body)
}

// fetchImageProxyWithFallback attempts the /photo/:/transcode request with the
// account token first, then retries with the per-server token on any non-200
// or transport error (404 is the common case for Stargaze movie thumbs;
// 401/403 also possible per CDN policy). Returns the response, the token
// kind that worked ("account" or "server"), or an error that already
// carries enough context for diagnosis.
func (s *Server) fetchImageProxyWithFallback(ctx context.Context, base, path string, width, height int, serverToken string) (*http.Response, string, error) {
	tryToken := func(tokenRaw, kind string) (*http.Response, string, error) {
		if tokenRaw == "" {
			return nil, kind, fmt.Errorf("no %s token available", kind)
		}
		token := url.QueryEscape(tokenRaw)
		target := fmt.Sprintf(
			"%s/photo/:/transcode?width=%d&height=%d&minSize=1&upscale=1&url=%s?X-Plex-Token=%s&X-Plex-Token=%s",
			base, width, height, path, token, token,
		)
		req, err := http.NewRequestWithContext(ctx, "GET", target, nil)
		if err != nil {
			return nil, kind, err
		}
		// Match Plex Web's image-request header signature (Session 2 finding —
		// CDN rejects API-style headers). Keep what a browser naturally sends.
		req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64; rv:150.0) Gecko/20100101 Firefox/150.0")
		req.Header.Set("Accept", "image/avif,image/webp,image/png,image/svg+xml,image/*;q=0.8,*/*;q=0.5")
		req.Header.Set("Accept-Language", "en-US,en;q=0.9")
		req.Header.Set("Referer", "https://app.plex.tv/")
		req.Header.Set("Sec-Fetch-Dest", "image")
		req.Header.Set("Sec-Fetch-Mode", "no-cors")
		req.Header.Set("Sec-Fetch-Site", "cross-site")
		resp, err := http.DefaultClient.Do(req)
		return resp, kind, err
	}

	resp, kind, err := tryToken(s.cfg.Plex.AccountToken, "account")
	if err == nil && resp.StatusCode == http.StatusOK {
		return resp, kind, nil
	}
	// Account token failed (network err, 404, or other non-200). Drain + close
	// before retrying so the connection returns to the pool cleanly.
	if resp != nil {
		_ = resp.Body.Close()
	}
	resp2, kind2, err2 := tryToken(serverToken, "server")
	if err2 != nil {
		if resp2 != nil {
			_ = resp2.Body.Close()
		}
		return nil, kind2, err2
	}
	return resp2, kind2, nil
}
