package server

import (
	"encoding/json"
	"net/http"

	"lumen/internal/config"
)

// writeJSON marshals v as JSON and writes it with status 200 and the right Content-Type.
func writeJSON(w http.ResponseWriter, v any) {
	writeJSONStatus(w, http.StatusOK, v)
}

func writeJSONStatus(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

// writeError writes {"error": msg} with the given status.
func writeError(w http.ResponseWriter, status int, msg string) {
	writeJSONStatus(w, status, map[string]string{"error": msg})
}

// serverByID looks up a server in the loaded config by MachineIdentifier.
// Returns nil if not found.
func (s *Server) serverByID(id string) *config.Server {
	for i := range s.cfg.Plex.Servers {
		if s.cfg.Plex.Servers[i].MachineIdentifier == id {
			return &s.cfg.Plex.Servers[i]
		}
	}
	return nil
}
