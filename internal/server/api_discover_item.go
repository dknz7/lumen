package server

import (
	"log"
	"net/http"
	"strings"
	"sync"
	"time"

	"lumen/internal/plex"
)

const discoverItemCacheTTL = 5 * time.Minute

// discoverItemCache is a small in-memory cache for the discover.provider.plex.tv
// /library/metadata/<rk> responses. Plex serves these reliably but the round
// trip is ~250-500 ms; caching for 5 minutes makes back-and-forth navigation
// in the SPA feel instant. Same TTL the hub cache uses.
type discoverItemCache struct {
	mu      sync.Mutex
	entries map[string]discoverItemEntry
}
type discoverItemEntry struct {
	item      *plex.DiscoverItem
	expiresAt time.Time
}

func newDiscoverItemCache() *discoverItemCache {
	return &discoverItemCache{entries: map[string]discoverItemEntry{}}
}

func (c *discoverItemCache) get(key string) (*plex.DiscoverItem, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	e, ok := c.entries[key]
	if !ok || time.Now().After(e.expiresAt) {
		delete(c.entries, key)
		return nil, false
	}
	return e.item, true
}

func (c *discoverItemCache) set(key string, item *plex.DiscoverItem) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.entries[key] = discoverItemEntry{item: item, expiresAt: time.Now().Add(discoverItemCacheTTL)}
}

// handleDiscoverItem returns rich metadata for a plex.tv ratingKey. Path:
// /api/discover-item/<plexTvRatingKey>. 5-minute in-memory cache. The SPA
// reads the response into the DiscoverItem page (Recommended/Discover/
// Watchlist tile click destinations).
func (s *Server) handleDiscoverItem(w http.ResponseWriter, r *http.Request) {
	if s.cfg.Plex.AccountToken == "" {
		writeError(w, http.StatusUnauthorized, "no account token — run lumen auth")
		return
	}
	rk := strings.TrimPrefix(r.URL.Path, "/api/discover-item/")
	if rk == "" || strings.Contains(rk, "/") {
		writeError(w, http.StatusBadRequest, "expected /api/discover-item/<ratingKey>")
		return
	}
	if cached, ok := s.discoverItems.get(rk); ok {
		writeJSON(w, cached)
		return
	}
	item, err := s.plex.GetDiscoverItem(s.cfg.Plex.AccountToken, rk)
	if err != nil {
		// Boundary error scrub — same idiom as handleIMDB / handleTMDBTrailer.
		// err.Error() may include the upstream URL (and could leak the
		// X-Plex-Token via redirect chains in some failure modes); log full
		// detail server-side, return a generic message to the SPA.
		log.Printf("discover-item lookup failed for %s: %v", rk, err)
		writeError(w, http.StatusBadGateway, "discover-item lookup failed")
		return
	}
	s.discoverItems.set(rk, item)
	writeJSON(w, item)
}
