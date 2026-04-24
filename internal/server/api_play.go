package server

import "net/http"

func (s *Server) handlePlay(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" {
		writeError(w, http.StatusMethodNotAllowed, "POST required")
		return
	}
	// TODO Session 4: parse {ratingKey, server, subtitleStreamID} and launch Pot Player.
	writeError(w, http.StatusNotImplemented, "playback launches in Session 4")
}
