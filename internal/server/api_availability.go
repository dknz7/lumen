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
	// One snapshot, used for the channel size, the fan-out and the fan-in. Three
	// separate serverList() calls could disagree if /api/servers/refresh lands
	// mid-request, and a mismatched receive count deadlocks the handler.
	servers := s.serverList()
	results := make(chan result, len(servers))
	for _, srv := range servers {
		srv := srv
		go func() {
			matches, err := s.plex.GetAvailability(toPlexServer(&srv), guid)
			results <- result{matches: matches, err: err}
		}()
	}
	// Non-nil slice so an empty result marshals as `[]` rather than `null` —
	// the SPA's <Show when={availability()}> treats null as falsy and gets
	// stuck on "Checking your servers…" instead of rendering the empty state.
	all := []plex.Match{}
	for range servers {
		r := <-results
		if r.err != nil {
			continue // silent per spec — absence = offline or no match
		}
		all = append(all, r.matches...)
	}
	writeJSON(w, all)
}
