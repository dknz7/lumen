package server

import (
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
)

// Plex hands out absolute URLs on its own metadata CDN for some artwork —
// notably cast and crew headshots, which appear on every detail page. Those
// carry no credentials and are not reachable through a server's
// /photo/:/transcode endpoint.
//
// cdnImageHosts is the allowlist that stops this from being an open proxy.
// Only Plex's own metadata hosts are permitted; anything else is refused, so a
// crafted `path` cannot make Lumen fetch arbitrary URLs on the user's behalf.
var cdnImageHosts = map[string]bool{
	"metadata-static.plex.tv": true,
	"provider-static.plex.tv": true,
	"images.plex.tv":          true,
}

func isAbsoluteURL(raw string) bool {
	return strings.HasPrefix(raw, "http://") || strings.HasPrefix(raw, "https://")
}

// serveCDNImage proxies an absolute Plex CDN image, with the same disk caching
// as server-hosted artwork.
func (s *Server) serveCDNImage(w http.ResponseWriter, r *http.Request, raw string) {
	u, err := url.Parse(raw)
	if err != nil {
		writeError(w, http.StatusBadRequest, "malformed image URL")
		return
	}
	if u.Scheme != "https" || !cdnImageHosts[strings.ToLower(u.Hostname())] {
		// Deliberately not echoing the URL back — this is the branch a crafted
		// request lands in, and the response should teach it nothing.
		writeError(w, http.StatusForbidden, "image host not allowed")
		return
	}
	// Never forward credentials to a third-party host, even if asked to.
	q := u.Query()
	q.Del("X-Plex-Token")
	u.RawQuery = q.Encode()

	width, height := imageDimsFromQuery(r.URL.Query())

	// "cdn" stands in for the machine identifier so these can't collide with a
	// server's cache entries.
	cacheKey := s.images.key("cdn", u.String(), width, height)
	if ct, data, ok := s.images.get(cacheKey); ok {
		writeImage(w, ct, data, "hit")
		return
	}

	req, err := http.NewRequestWithContext(r.Context(), "GET", u.String(), nil)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "could not build request")
		return
	}
	req.Header.Set("Accept", "image/avif,image/webp,image/png,image/*;q=0.8,*/*;q=0.5")
	req.Header.Set("User-Agent", "Lumen")

	resp, err := s.hlsClient.Do(req)
	if err != nil {
		writeError(w, http.StatusBadGateway, "could not reach the Plex image CDN")
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		writeError(w, http.StatusBadGateway,
			fmt.Sprintf("image CDN returned %s", resp.Status))
		return
	}

	// 16 MB ceiling: these are headshots, and an unbounded ReadAll on a remote
	// response is a memory-pressure lever.
	data, err := io.ReadAll(io.LimitReader(resp.Body, 16<<20))
	if err != nil {
		writeError(w, http.StatusBadGateway, "could not read image")
		return
	}
	ct := resp.Header.Get("Content-Type")
	if ct == "" {
		ct = "image/jpeg"
	}
	_ = s.images.put(cacheKey, ct, data)
	writeImage(w, ct, data, "miss")
}

func imageDimsFromQuery(q url.Values) (width, height int) {
	width, height = defaultImageWidth, defaultImageHeight
	if v, err := strconv.Atoi(q.Get("w")); err == nil && v > 0 && v <= 4096 {
		width = v
	}
	if v, err := strconv.Atoi(q.Get("h")); err == nil && v > 0 && v <= 4096 {
		height = v
	}
	return width, height
}

func writeImage(w http.ResponseWriter, contentType string, data []byte, cacheState string) {
	w.Header().Set("Content-Type", contentType)
	w.Header().Set("Cache-Control", "public, max-age=2592000, immutable")
	w.Header().Set("X-Lumen-Cache", cacheState)
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(data)
}
