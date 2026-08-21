package server

import "net/http"

// handleUser returns the current Plex account info. Empty response when no token.
func (s *Server) handleUser(w http.ResponseWriter, r *http.Request) {
	if s.accountToken() == "" {
		writeError(w, http.StatusUnauthorized, "no account token — run lumen auth")
		return
	}
	if s.plex == nil {
		writeError(w, http.StatusInternalServerError, "plex client not initialised")
		return
	}
	info, err := s.plex.GetAccount(s.accountToken())
	if err != nil {
		writeError(w, http.StatusBadGateway, err.Error())
		return
	}
	writeJSON(w, info)
}
