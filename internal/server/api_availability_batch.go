package server

import (
	"encoding/json"
	"net/http"
	"sync"
	"time"

	"lumen/internal/plex"
)

// maxBatchGUIDs caps one request. The SPA sends viewport-sized chunks, so this
// is a sanity bound rather than an expected size.
const maxBatchGUIDs = 200

// availabilityConcurrency bounds in-flight Plex lookups across the whole batch.
//
// Each (guid, server) pair is its own round trip to /library/all?guid=..., and
// at least one server is typically remote, so this is latency-bound rather than
// CPU-bound — a higher number than you'd pick for local work is correct.
const availabilityConcurrency = 24

// availabilityTTL caches results in memory. Whether a title is in your library
// changes when you add media, not second to second, and the Watchlist re-asks
// on every visit and every window focus.
const availabilityTTL = 5 * time.Minute

type availabilityEntry struct {
	matches []plex.Match
	fetched time.Time
}

type availabilityCache struct {
	mu      sync.Mutex
	entries map[string]availabilityEntry
}

func newAvailabilityCache() *availabilityCache {
	return &availabilityCache{entries: map[string]availabilityEntry{}}
}

func (c *availabilityCache) get(guid string) ([]plex.Match, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	e, ok := c.entries[guid]
	if !ok || time.Since(e.fetched) > availabilityTTL {
		return nil, false
	}
	return e.matches, true
}

func (c *availabilityCache) put(guid string, matches []plex.Match) {
	c.mu.Lock()
	defer c.mu.Unlock()
	// Opportunistic sweep — this map is only ever as big as the user's library
	// interests, but nothing else prunes it.
	if len(c.entries) > 4000 {
		for k, v := range c.entries {
			if time.Since(v.fetched) > availabilityTTL {
				delete(c.entries, k)
			}
		}
	}
	c.entries[guid] = availabilityEntry{matches: matches, fetched: time.Now()}
}

// handleAvailabilityBatch resolves availability for many items in one request.
//
// The Watchlist previously issued one /api/availability call per card: 528
// concurrent requests, ~42 seconds to settle, repeated on every window focus,
// each one fanning out to every configured server. The SPA now requests only
// what scrolls into view, in chunks, and this endpoint resolves a chunk with
// bounded concurrency and a cache in front.
//
// Returns a map of guid -> matches; a guid with no copies maps to [].
func (s *Server) handleAvailabilityBatch(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "POST required")
		return
	}
	if s.plex == nil {
		writeError(w, http.StatusInternalServerError, "plex client not initialised")
		return
	}
	r.Body = http.MaxBytesReader(w, r.Body, 1<<20)

	var body struct {
		GUIDs []string `json:"guids"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON: "+err.Error())
		return
	}
	if len(body.GUIDs) > maxBatchGUIDs {
		writeError(w, http.StatusBadRequest, "too many guids in one request")
		return
	}

	servers := s.serverList()
	out := make(map[string][]plex.Match, len(body.GUIDs))
	var mu sync.Mutex

	// De-duplicate, and serve cache hits without touching the network. A
	// watchlist can list the same show more than once.
	pending := make([]string, 0, len(body.GUIDs))
	seen := make(map[string]bool, len(body.GUIDs))
	for _, g := range body.GUIDs {
		if g == "" || seen[g] {
			continue
		}
		seen[g] = true
		if cached, ok := s.availability.get(g); ok {
			out[g] = cached
			continue
		}
		pending = append(pending, g)
	}

	sem := make(chan struct{}, availabilityConcurrency)
	var wg sync.WaitGroup

	for _, guid := range pending {
		wg.Add(1)
		go func(guid string) {
			defer wg.Done()

			// Query every server in parallel rather than in sequence: the two
			// are independent, and doing them one after another doubled the
			// latency of every single item.
			var inner sync.WaitGroup
			results := make([][]plex.Match, len(servers))
			for i := range servers {
				inner.Add(1)
				go func(i int) {
					defer inner.Done()
					sem <- struct{}{}
					defer func() { <-sem }()
					if r.Context().Err() != nil {
						return
					}
					// An error means offline or no copy — both are simply
					// "not available", same as the single-item endpoint.
					if m, err := s.plex.GetAvailability(toPlexServer(&servers[i]), guid); err == nil {
						results[i] = m
					}
				}(i)
			}
			inner.Wait()

			// Non-nil so "no copies" marshals as [] rather than null — the SPA
			// reads null as "still loading" and would spin forever.
			matches := []plex.Match{}
			for _, m := range results {
				matches = append(matches, m...)
			}
			s.availability.put(guid, matches)

			mu.Lock()
			out[guid] = matches
			mu.Unlock()
		}(guid)
	}
	wg.Wait()

	if r.Context().Err() != nil {
		return
	}
	writeJSON(w, out)
}
