package server

import (
	"net/http"
	"strings"
	"sync"
	"time"

	"lumen/internal/plex"
)

// hubCache is a simple in-memory 5-minute TTL cache keyed by "namespace/slug".
type hubCache struct {
	mu      sync.Mutex
	entries map[string]hubCacheEntry
}

type hubCacheEntry struct {
	items     []plex.HubItem
	expiresAt time.Time
}

func newHubCache() *hubCache { return &hubCache{entries: map[string]hubCacheEntry{}} }

func (c *hubCache) get(key string) ([]plex.HubItem, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	e, ok := c.entries[key]
	if !ok || time.Now().After(e.expiresAt) {
		delete(c.entries, key)
		return nil, false
	}
	return e.items, true
}

func (c *hubCache) set(key string, items []plex.HubItem) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.entries[key] = hubCacheEntry{items: items, expiresAt: time.Now().Add(5 * time.Minute)}
}

func (s *Server) handleHub(w http.ResponseWriter, r *http.Request) {
	if s.cfg.Plex.AccountToken == "" {
		writeError(w, http.StatusUnauthorized, "no account token — run lumen auth")
		return
	}
	rest := strings.TrimPrefix(r.URL.Path, "/api/hubs/")
	parts := strings.SplitN(rest, "/", 2)
	if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
		writeError(w, http.StatusBadRequest, "expected /api/hubs/<namespace>/<slug>")
		return
	}
	namespace, slug := parts[0], parts[1]
	if namespace != "home" && namespace != "watchlist" {
		writeError(w, http.StatusBadRequest, "namespace must be 'home' or 'watchlist'")
		return
	}
	key := namespace + "/" + slug
	if cached, ok := s.hubs.get(key); ok {
		writeJSON(w, cached)
		return
	}
	items, err := s.plex.GetHub(namespace, slug, s.cfg.Plex.AccountToken)
	if err != nil {
		writeError(w, http.StatusBadGateway, err.Error())
		return
	}
	s.hubs.set(key, items)
	writeJSON(w, items)
}
