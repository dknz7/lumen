package server

import (
	"net/http"
	"sync"

	"lumen/internal/config"
	"lumen/internal/plex"
)

// handleServersRefresh re-discovers servers and re-picks connections.
// Mirrors what `lumen list` does but in-process, updating the live config.
func (s *Server) handleServersRefresh(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" {
		writeError(w, http.StatusMethodNotAllowed, "POST required")
		return
	}
	if s.cfg.Plex.AccountToken == "" {
		writeError(w, http.StatusUnauthorized, "no account token")
		return
	}
	if s.plex == nil {
		writeError(w, http.StatusInternalServerError, "plex client not initialised")
		return
	}

	servers, err := s.plex.DiscoverServers(s.cfg.Plex.AccountToken)
	if err != nil {
		writeError(w, http.StatusBadGateway, err.Error())
		return
	}

	var wg sync.WaitGroup
	for _, srv := range servers {
		wg.Add(1)
		go func(sr *plex.Server) {
			defer wg.Done()
			_, _ = s.plex.PickConnection(sr)
		}(srv)
	}
	wg.Wait()

	// Merge into config — preserve DisplayName overrides per server. The read
	// of the existing names and the replacement of the slice happen in one
	// critical section so a concurrent rename can't be lost or half-applied.
	var count int
	if err := s.mutateCfg(func(c *config.Config) {
		merged := make([]config.Server, 0, len(servers))
		for _, sr := range servers {
			var display string
			for _, prev := range c.Plex.Servers {
				if prev.MachineIdentifier == sr.MachineIdentifier {
					display = prev.DisplayName
					break
				}
			}
			merged = append(merged, config.Server{
				Name:               sr.Name,
				DisplayName:        display,
				MachineIdentifier:  sr.MachineIdentifier,
				AccessToken:        sr.AccessToken,
				LastGoodConnection: sr.BaseURL,
			})
		}
		c.Plex.Servers = merged
		count = len(merged)
	}); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, map[string]any{"status": "refreshed", "count": count})
}
