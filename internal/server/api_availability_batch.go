package server

import (
	"encoding/json"
	"net/http"
	"sync"

	"lumen/internal/plex"
)

// maxBatchGUIDs caps a single batch request. A 528-item watchlist is real —
// that is what prompted this endpoint — so the ceiling is generous, but
// unbounded would let one request fan out without limit.
const maxBatchGUIDs = 1000

// availabilityConcurrency bounds how many Plex lookups run at once across the
// whole batch. Without it, 528 items × 2 servers is 1056 simultaneous requests
// at one Plex server, which is how the un-batched version took 42 seconds to
// settle and hammered the server on every visit to the page.
const availabilityConcurrency = 8

// handleAvailabilityBatch resolves availability for many items in one request.
//
// The Watchlist page previously issued one /api/availability call per card:
// measured at 528 concurrent requests taking 42 seconds to fully settle, and
// repeated on every window focus. Cards rendered immediately but their Play
// state trickled in for the better part of a minute.
//
// Response shape is a map of guid -> matches, so the SPA can resolve one
// resource and index into it.
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

	// De-duplicate: a watchlist can hold the same show under several entries,
	// and there is no point resolving it twice.
	unique := make([]string, 0, len(body.GUIDs))
	seen := make(map[string]bool, len(body.GUIDs))
	for _, g := range body.GUIDs {
		if g == "" || seen[g] {
			continue
		}
		seen[g] = true
		unique = append(unique, g)
	}

	servers := s.serverList()
	out := make(map[string][]plex.Match, len(unique))
	var mu sync.Mutex

	sem := make(chan struct{}, availabilityConcurrency)
	var wg sync.WaitGroup

	for _, guid := range unique {
		wg.Add(1)
		go func(guid string) {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()

			// Non-nil so an item with no matches marshals as [] rather than
			// null — the SPA treats null as "still loading" and would sit on a
			// spinner forever.
			matches := []plex.Match{}
			for i := range servers {
				// A failure here means that server is offline or doesn't have
				// the item; both are simply "not available", same as the
				// single-item endpoint.
				if m, err := s.plex.GetAvailability(toPlexServer(&servers[i]), guid); err == nil {
					matches = append(matches, m...)
				}
				if r.Context().Err() != nil {
					return // client navigated away; stop doing work for it
				}
			}

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
