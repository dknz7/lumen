package server

import (
	"encoding/json"
	"net/http"
	"strings"
)

// handleSettingsOMDB updates the OMDB API key in config. Top-level field, not
// inside UI, because OMDB is a Plex-adjacent integration concern.
func (s *Server) handleSettingsOMDB(w http.ResponseWriter, r *http.Request) {
	if r.Method != "PUT" {
		writeError(w, http.StatusMethodNotAllowed, "PUT required")
		return
	}
	r.Body = http.MaxBytesReader(w, r.Body, 1<<10)
	var body struct {
		Key string `json:"key"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON")
		return
	}
	s.cfg.OMDBKey = strings.TrimSpace(body.Key)
	if err := s.cfg.Save(); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, map[string]string{"status": "saved"})
}
