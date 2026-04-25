package server

import "net/http"

func (s *Server) handlePlaybackState(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "GET only")
		return
	}
	writeJSON(w, s.playback.SnapshotState())
}
