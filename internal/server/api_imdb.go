package server

import (
	"encoding/json"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"lumen/internal/config"
	"lumen/internal/plex"
)

const omdbCacheTTL = 30 * 24 * time.Hour // 30 days per spec §6

// handleIMDB serves cached OMDB lookups by IMDB id. Path: /api/imdb/<imdbId>.
// On miss, calls OMDB, writes the response to %APPDATA%\Lumen\cache\omdb\<id>.json
// with file mtime as the freshness anchor. Returns 404 when OMDB has no data
// (Response: False) — SPA renders the IMDB pill as "—" in that case.
func (s *Server) handleIMDB(w http.ResponseWriter, r *http.Request) {
	if s.cfg.OMDBKey == "" {
		writeError(w, http.StatusServiceUnavailable, "no OMDB key configured")
		return
	}
	id := strings.TrimPrefix(r.URL.Path, "/api/imdb/")
	if id == "" || strings.Contains(id, "/") || !strings.HasPrefix(id, "tt") {
		writeError(w, http.StatusBadRequest, "expected /api/imdb/ttNNNNNNN")
		return
	}
	dir := filepath.Join(config.CacheDir(), "omdb")
	path := filepath.Join(dir, id+".json")
	if data, ok := readFreshOMDBCache(path, omdbCacheTTL); ok {
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("Cache-Control", "no-store")
		_, _ = w.Write(data)
		return
	}

	client := plex.NewOMDBClient(s.cfg.OMDBKey)
	rating, err := client.LookupByIMDBId(id)
	if err != nil {
		writeError(w, http.StatusBadGateway, err.Error())
		return
	}
	if rating == nil {
		writeJSONStatus(w, http.StatusNotFound, map[string]string{"error": "not found"})
		return
	}
	if err := os.MkdirAll(dir, 0o755); err == nil {
		buf, _ := json.Marshal(rating)
		_ = os.WriteFile(path, buf, 0o644)
	}
	writeJSON(w, rating)
}

// readFreshOMDBCache returns the cached bytes if the file exists and its
// mtime is within ttl. Returns (nil, false) on any miss or staleness.
func readFreshOMDBCache(path string, ttl time.Duration) ([]byte, bool) {
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
	// Best-effort sanity check that the body is JSON.
	if !json.Valid(data) {
		return nil, false
	}
	return data, true
}
