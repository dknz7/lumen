package server

import (
	"encoding/json"
	"log"
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
	if s.accountToken() == "" {
		writeError(w, http.StatusUnauthorized, "no account token — run lumen auth")
		return
	}
	if cached, ok := s.watchlist.get(s.accountToken()); ok {
		writeJSON(w, cached)
		return
	}
	items, err := s.plex.GetWatchlist(s.accountToken())
	if err != nil {
		writeError(w, http.StatusBadGateway, err.Error())
		return
	}
	s.watchlist.set(s.accountToken(), items)
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
	if s.accountToken() == "" {
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
	if err := s.plex.AddToWatchlist(s.accountToken(), body.RatingKey); err != nil {
		writeError(w, http.StatusBadGateway, err.Error())
		return
	}
	s.watchlist.invalidate()
	writeJSON(w, map[string]string{"status": "added"})
}

// handleWatchlistAddFromItem accepts {server, ratingKey} JSON identifying a
// server-local item, resolves the right plex.tv catalog target (rolling up
// episodes/seasons to the parent show), and adds it to the watchlist. Used
// by the Library card's "Add to Watchlist" hover button — the SPA doesn't
// need to know the resolution rules or surface parent GUIDs.
func (s *Server) handleWatchlistAddFromItem(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" {
		writeError(w, http.StatusMethodNotAllowed, "POST required")
		return
	}
	if s.accountToken() == "" {
		writeError(w, http.StatusUnauthorized, "no account token")
		return
	}
	if s.plex == nil {
		writeError(w, http.StatusInternalServerError, "plex client not initialised")
		return
	}
	var body struct {
		Server    string `json:"server"`
		RatingKey string `json:"ratingKey"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON")
		return
	}
	if body.Server == "" || body.RatingKey == "" {
		writeError(w, http.StatusBadRequest, "server and ratingKey required")
		return
	}
	srv := s.serverByID(body.Server)
	if srv == nil {
		writeError(w, http.StatusNotFound, "unknown server")
		return
	}
	if err := s.plex.AddItemToWatchlist(toPlexServer(srv), s.accountToken(), body.RatingKey); err != nil {
		// Boundary scrub — log full detail server-side, hand the SPA a
		// generic message. Resolution failures (missing parent ratingKey,
		// non-plex GUID) carry actionable hints in the operator log.
		log.Printf("watchlist add-from-item server=%s rk=%s: %v", body.Server, body.RatingKey, err)
		writeError(w, http.StatusBadGateway, "watchlist add failed")
		return
	}
	s.watchlist.invalidate()
	writeJSON(w, map[string]string{"status": "added"})
}

// handleWatchlistRemoveFromItem mirrors handleWatchlistAddFromItem for
// removal — accepts {server, ratingKey} for a server-local item and walks
// up to the parent show before calling removeFromWatchlist. Used by
// ItemDetail's watchlist toggle so the same roll-up applies symmetrically.
func (s *Server) handleWatchlistRemoveFromItem(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" {
		writeError(w, http.StatusMethodNotAllowed, "POST required")
		return
	}
	if s.accountToken() == "" {
		writeError(w, http.StatusUnauthorized, "no account token")
		return
	}
	if s.plex == nil {
		writeError(w, http.StatusInternalServerError, "plex client not initialised")
		return
	}
	var body struct {
		Server    string `json:"server"`
		RatingKey string `json:"ratingKey"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON")
		return
	}
	if body.Server == "" || body.RatingKey == "" {
		writeError(w, http.StatusBadRequest, "server and ratingKey required")
		return
	}
	srv := s.serverByID(body.Server)
	if srv == nil {
		writeError(w, http.StatusNotFound, "unknown server")
		return
	}
	if err := s.plex.RemoveItemFromWatchlist(toPlexServer(srv), s.accountToken(), body.RatingKey); err != nil {
		log.Printf("watchlist remove-from-item server=%s rk=%s: %v", body.Server, body.RatingKey, err)
		writeError(w, http.StatusBadGateway, "watchlist remove failed")
		return
	}
	s.watchlist.invalidate()
	writeJSON(w, map[string]string{"status": "removed"})
}

func (s *Server) handleWatchlistRemove(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" {
		writeError(w, http.StatusMethodNotAllowed, "POST required")
		return
	}
	if s.accountToken() == "" {
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
	if err := s.plex.RemoveFromWatchlist(s.accountToken(), body.RatingKey); err != nil {
		writeError(w, http.StatusBadGateway, err.Error())
		return
	}
	s.watchlist.invalidate()
	writeJSON(w, map[string]string{"status": "removed"})
}
