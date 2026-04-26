package server

import (
	"encoding/json"
	"net/http"
)

// handleSettingsTMDB updates the TMDB API key in config. Mirrors the OMDB
// shape — top-level field, not inside UI, because TMDB is a Plex-adjacent
// integration concern.
func (s *Server) handleSettingsTMDB(w http.ResponseWriter, r *http.Request) {
	if r.Method != "PUT" {
		writeError(w, http.StatusMethodNotAllowed, "PUT required")
		return
	}
	var body struct {
		Key string `json:"key"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON")
		return
	}
	s.cfg.TMDBKey = body.Key
	if err := s.cfg.Save(); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, map[string]string{"status": "saved"})
}
