package server

import (
	"log"
	"net/http"
	"sync"

	"lumen/internal/plex"
)

// searchServerBucket carries one server's search hits in the SearchResponse.
// machineIdentifier + displayName so the SPA can title each group's section.
type searchServerBucket struct {
	MachineIdentifier string      `json:"machineIdentifier"`
	DisplayName       string      `json:"displayName"`
	Items             []plex.Item `json:"items"`
}

// searchResponse is the SPA-facing shape — one bucket per server plus a
// flat discover list. Empty buckets stay in the response so the SPA can
// distinguish "this server returned nothing" from "this server didn't
// load yet" — empty array is unambiguous.
type searchResponse struct {
	Servers  []searchServerBucket `json:"servers"`
	Discover []plex.Item          `json:"discover"`
}

// handleSearch fans out a query to every connected Plex server's /search
// endpoint plus plex.tv's discover /library/search, in parallel. Per-source
// failures are logged server-side and surface as empty buckets — partial
// results stay useful (e.g. one server offline shouldn't kill the search).
//
// Boundary error scrub: per-source errors stay in the operator log;
// the SPA only ever sees successful buckets (possibly empty). Mirrors
// the pattern from /api/discover-item / /api/tmdb (Session 6 finding).
func (s *Server) handleSearch(w http.ResponseWriter, r *http.Request) {
	if s.plex == nil {
		writeError(w, http.StatusInternalServerError, "plex client not initialised")
		return
	}
	if s.cfg.Plex.AccountToken == "" {
		writeError(w, http.StatusUnauthorized, "no account token — run lumen auth")
		return
	}
	query := r.URL.Query().Get("q")
	if query == "" {
		writeError(w, http.StatusBadRequest, "q query param required")
		return
	}
	log.Printf("search: query=%q (servers=%d)", query, len(s.cfg.Plex.Servers))

	servers := s.cfg.Plex.Servers
	resp := searchResponse{
		Servers:  make([]searchServerBucket, len(servers)),
		Discover: nil,
	}

	var wg sync.WaitGroup
	wg.Add(len(servers) + 1)

	// Per-server fan-out — each server gets its own goroutine. Result slot
	// is pre-sized so we write into resp.Servers[i] without a mutex.
	for i := range servers {
		go func(idx int) {
			defer wg.Done()
			srv := &servers[idx]
			display := srv.DisplayName
			if display == "" {
				display = srv.Name
			}
			if display == "" {
				display = srv.MachineIdentifier
			}
			bucket := searchServerBucket{
				MachineIdentifier: srv.MachineIdentifier,
				DisplayName:       display,
				Items:             []plex.Item{}, // empty (never nil) so SPA renders an explicit "no results" affordance
			}
			plexSrv := toPlexServer(srv)
			items, err := s.plex.Search(plexSrv, query)
			if err != nil {
				// Log server-side, leave bucket empty for the SPA. Don't
				// short-circuit the whole response — other servers / discover
				// may still return useful results.
				log.Printf("search: server %s (%s) ERROR: %v", display, srv.MachineIdentifier, err)
			} else {
				bucket.Items = items
				log.Printf("search: server %s (%s) returned %d items", display, srv.MachineIdentifier, len(items))
			}
			resp.Servers[idx] = bucket
		}(i)
	}

	// Discover fan-out — sibling goroutine to the per-server loop above.
	var discoverItems []plex.Item
	go func() {
		defer wg.Done()
		items, err := s.plex.SearchDiscover(query, s.cfg.Plex.AccountToken)
		if err != nil {
			log.Printf("search: discover ERROR: %v", err)
			return
		}
		log.Printf("search: discover returned %d items", len(items))
		discoverItems = items
	}()

	wg.Wait()
	if discoverItems == nil {
		discoverItems = []plex.Item{}
	}
	resp.Discover = discoverItems
	writeJSON(w, resp)
}
