package server

import (
	"net/http"

	"lumen/internal/plex"
)

func (s *Server) handleAvailability(w http.ResponseWriter, r *http.Request) {
	guid := r.URL.Query().Get("guid")
	if guid == "" {
		writeError(w, http.StatusBadRequest, "guid query param required")
		return
	}
	if s.plex == nil {
		writeError(w, http.StatusInternalServerError, "plex client not initialised")
		return
	}
	// Walk all configured servers in parallel; any failures just mean that server
	// doesn't have this item (or is offline) — collect whatever succeeds.
	type result struct {
		matches []plex.Match
		err     error
	}
	results := make(chan result, len(s.cfg.Plex.Servers))
	for _, srv := range s.cfg.Plex.Servers {
		srv := srv
		go func() {
			matches, err := s.plex.GetAvailability(toPlexServer(&srv), guid)
			results <- result{matches: matches, err: err}
		}()
	}
	var all []plex.Match
	for range s.cfg.Plex.Servers {
		r := <-results
		if r.err != nil {
			continue // silent per spec — absence = offline or no match
		}
		all = append(all, r.matches...)
	}
	writeJSON(w, all)
}
