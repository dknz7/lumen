package server

import (
	"net/http"
	"strings"
)

func (s *Server) handleItem(w http.ResponseWriter, r *http.Request) {
	ratingKey := strings.TrimPrefix(r.URL.Path, "/api/items/")
	if ratingKey == "" || strings.Contains(ratingKey, "/") {
		writeError(w, http.StatusBadRequest, "ratingKey required")
		return
	}
	machineID := r.URL.Query().Get("server")
	if machineID == "" {
		writeError(w, http.StatusBadRequest, "server query param required")
		return
	}
	srv := s.serverByID(machineID)
	if srv == nil {
		writeError(w, http.StatusNotFound, "unknown server")
		return
	}
	if s.plex == nil {
		writeError(w, http.StatusInternalServerError, "plex client not initialised")
		return
	}
	item, err := s.plex.GetItem(toPlexServer(srv), ratingKey)
	if err != nil {
		writeError(w, http.StatusBadGateway, err.Error())
		return
	}
	// TODO Session 5: enrich with OMDB rating if item has an imdb:// GUID and
	// cfg.OMDBKey is set.
	writeJSON(w, item)
}
