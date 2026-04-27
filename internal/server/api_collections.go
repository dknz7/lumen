package server

import (
	"net/http"
	"strconv"

	"lumen/internal/config"
)

// handleLibraryCollections proxies /library/sections/<id>/collections.
// Powers the Home page's "server-collection" shelf — SPA lists collections
// in a library section, then matches by title to find the one to render
// (admin-rename tolerant: reshuffling a collection's ratingKey doesn't
// break Lumen as long as the title stays stable).
func (s *Server) handleLibraryCollections(w http.ResponseWriter, r *http.Request, srv *config.Server, libraryKey string) {
	if s.plex == nil {
		writeError(w, http.StatusInternalServerError, "plex client not initialised")
		return
	}
	cols, err := s.plex.GetCollections(toPlexServer(srv), libraryKey)
	if err != nil {
		writeError(w, http.StatusBadGateway, err.Error())
		return
	}
	writeJSON(w, cols)
}

// handleCollectionItems proxies /library/collections/<rk>/children with an
// optional size cap (?size=<n>, capped at 200 to mirror handleLibraryRecentlyAdded).
func (s *Server) handleCollectionItems(w http.ResponseWriter, r *http.Request, srv *config.Server, collectionRatingKey string) {
	if s.plex == nil {
		writeError(w, http.StatusInternalServerError, "plex client not initialised")
		return
	}
	size := 0
	if v, err := strconv.Atoi(r.URL.Query().Get("size")); err == nil && v > 0 && v <= 200 {
		size = v
	}
	items, err := s.plex.GetCollectionItems(toPlexServer(srv), collectionRatingKey, size)
	if err != nil {
		writeError(w, http.StatusBadGateway, err.Error())
		return
	}
	writeJSON(w, items)
}
