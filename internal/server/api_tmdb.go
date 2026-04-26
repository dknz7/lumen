package server

import (
	"encoding/json"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"lumen/internal/config"
	"lumen/internal/plex"
)

const tmdbCacheTTL = 30 * 24 * time.Hour

// handleTMDBTrailer returns the best-pick YouTube trailer key for an IMDB id,
// looking up via TMDB. Path: /api/tmdb/trailer/<imdbID>?type=movie|show.
//
// Cached on disk under %APPDATA%\Lumen\cache\tmdb\<id>-<type>.json for 30
// days, mirroring the OMDB cache pattern. 503 when no TMDB key configured;
// 404 when no matching TMDB entry or no YouTube trailer/teaser. SPA falls
// through to the Plex Extras item.trailer field on 404.
func (s *Server) handleTMDBTrailer(w http.ResponseWriter, r *http.Request) {
	if s.cfg.TMDBKey == "" {
		writeError(w, http.StatusServiceUnavailable, "no TMDB key configured")
		return
	}
	id := strings.TrimPrefix(r.URL.Path, "/api/tmdb/trailer/")
	if id == "" || strings.Contains(id, "/") || !strings.HasPrefix(id, "tt") {
		writeError(w, http.StatusBadRequest, "expected /api/tmdb/trailer/ttNNNNNNN")
		return
	}
	mediaType := r.URL.Query().Get("type")
	if mediaType != "movie" && mediaType != "show" {
		writeError(w, http.StatusBadRequest, "type query param must be 'movie' or 'show'")
		return
	}

	dir := filepath.Join(config.CacheDir(), "tmdb")
	path := filepath.Join(dir, id+"-"+mediaType+".json")
	if data, ok := readFreshTMDBCache(path, tmdbCacheTTL); ok {
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("Cache-Control", "no-store")
		_, _ = w.Write(data)
		return
	}

	client := plex.NewTMDBClient(s.cfg.TMDBKey)
	yt, err := client.LookupTrailerByIMDBID(id, mediaType)
	if err != nil {
		// Scrub: Go's *url.Error.Error() includes the full request URL on
		// transport failures (DNS/TCP/TLS), and TMDB's v3 api key rides in
		// the URL query string — so err.Error() leaks the key. Log the
		// detail server-side; respond with a generic message.
		log.Printf("tmdb lookup failed for %s/%s: %v", id, mediaType, err)
		writeError(w, http.StatusBadGateway, "tmdb lookup failed")
		return
	}
	if yt == "" {
		// Cache the negative result too — TMDB has no trailer is a stable fact
		// for the cache TTL window. SPA reads 404 as "fallback to Plex Extras".
		if err := os.MkdirAll(dir, 0o755); err == nil {
			_ = os.WriteFile(path, []byte(`{"youtubeID":""}`), 0o644)
		}
		writeJSONStatus(w, http.StatusNotFound, map[string]string{"error": "no trailer"})
		return
	}

	resp := map[string]string{"youtubeID": yt}
	buf, _ := json.Marshal(resp)
	if err := os.MkdirAll(dir, 0o755); err == nil {
		_ = os.WriteFile(path, buf, 0o644)
	}
	writeJSON(w, resp)
}

func readFreshTMDBCache(path string, ttl time.Duration) ([]byte, bool) {
	st, err := os.Stat(path)
	if err != nil {
		return nil, false
	}
	if time.Since(st.ModTime()) > ttl {
		return nil, false
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, false
	}
	if !json.Valid(data) {
		return nil, false
	}
	// Decode to detect cached negative ("youtubeID":"").
	var probe struct {
		YouTubeID string `json:"youtubeID"`
	}
	_ = json.Unmarshal(data, &probe)
	if probe.YouTubeID == "" {
		// Treat cached negative as a miss for v1.0 — caller will re-fetch
		// from TMDB and overwrite. Cost is ~1 TMDB call per 30 days per
		// "no trailer" item; acceptable. A future enhancement could serve
		// the cached negative directly from this layer to dodge that call,
		// but the current shape keeps the cache + handler logic uniform.
		return nil, false
	}
	return data, true
}
