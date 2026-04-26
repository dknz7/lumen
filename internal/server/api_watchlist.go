package server

import (
	"encoding/json"
	"net/http"
	"sync"
	"time"

	"lumen/internal/plex"
)

// watchlistCache mirrors hubCache — 5 minute in-memory TTL keyed by token
// (so multiple users on the same machine — out of scope for v1.0 but
// future-proof — don't see each other's entries).
type watchlistCache struct {
	mu      sync.Mutex
	entries map[string]watchlistCacheEntry
}

type watchlistCacheEntry struct {
	items     []plex.WatchlistItem
	expiresAt time.Time
}

func newWatchlistCache() *watchlistCache {
	return &watchlistCache{entries: map[string]watchlistCacheEntry{}}
}

func (c *watchlistCache) get(key string) ([]plex.WatchlistItem, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	e, ok := c.entries[key]
	if !ok || time.Now().After(e.expiresAt) {
		delete(c.entries, key)
		return nil, false
	}
	return e.items, true
}

func (c *watchlistCache) set(key string, items []plex.WatchlistItem) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.entries[key] = watchlistCacheEntry{items: items, expiresAt: time.Now().Add(5 * time.Minute)}
}

func (c *watchlistCache) invalidate() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.entries = map[string]watchlistCacheEntry{}
}

func (s *Server) handleWatchlist(w http.ResponseWriter, r *http.Request) {
	if s.cfg.Plex.AccountToken == "" {
		writeError(w, http.StatusUnauthorized, "no account token — run lumen auth")
		return
	}
	if cached, ok := s.watchlist.get(s.cfg.Plex.AccountToken); ok {
		writeJSON(w, cached)
		return
	}
	items, err := s.plex.GetWatchlist(s.cfg.Plex.AccountToken)
	if err != nil {
		writeError(w, http.StatusBadGateway, err.Error())
		return
	}
	s.watchlist.set(s.cfg.Plex.AccountToken, items)
	writeJSON(w, items)
}

// handleWatchlistAdd takes JSON {"ratingKey": "<plexTvRatingKey>"} and
// PUTs the addToWatchlist action through. Invalidates the local
// watchlist cache on success so the SPA's next refetch sees fresh data.
func (s *Server) handleWatchlistAdd(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" {
		writeError(w, http.StatusMethodNotAllowed, "POST required")
		return
	}
	if s.cfg.Plex.AccountToken == "" {
		writeError(w, http.StatusUnauthorized, "no account token")
		return
	}
	var body struct {
		RatingKey string `json:"ratingKey"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON")
		return
	}
	if body.RatingKey == "" {
		writeError(w, http.StatusBadRequest, "ratingKey required")
		return
	}
	if err := s.plex.AddToWatchlist(s.cfg.Plex.AccountToken, body.RatingKey); err != nil {
		writeError(w, http.StatusBadGateway, err.Error())
		return
	}
	s.watchlist.invalidate()
	writeJSON(w, map[string]string{"status": "added"})
}

func (s *Server) handleWatchlistRemove(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" {
		writeError(w, http.StatusMethodNotAllowed, "POST required")
		return
	}
	if s.cfg.Plex.AccountToken == "" {
		writeError(w, http.StatusUnauthorized, "no account token")
		return
	}
	var body struct {
		RatingKey string `json:"ratingKey"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON")
		return
	}
	if body.RatingKey == "" {
		writeError(w, http.StatusBadRequest, "ratingKey required")
		return
	}
	if err := s.plex.RemoveFromWatchlist(s.cfg.Plex.AccountToken, body.RatingKey); err != nil {
		writeError(w, http.StatusBadGateway, err.Error())
		return
	}
	s.watchlist.invalidate()
	writeJSON(w, map[string]string{"status": "removed"})
}
