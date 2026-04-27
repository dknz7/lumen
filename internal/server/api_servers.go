package server

import (
	"encoding/json"
	"net/http"
	"strconv"
	"strings"

	"lumen/internal/config"
	"lumen/internal/plex"
)

// serverDTO is what the SPA consumes — never the raw config (tokens stay server-side).
type serverDTO struct {
	MachineID   string `json:"machineIdentifier"`
	Name        string `json:"name"`
	DisplayName string `json:"displayName"` // name, or machineIdentifier if name empty
	BaseURL     string `json:"baseURL"`
	Status      string `json:"status"` // "connected" | "offline"
}

// handleServers returns the full server list known to config.
func (s *Server) handleServers(w http.ResponseWriter, r *http.Request) {
	out := make([]serverDTO, 0, len(s.cfg.Plex.Servers))
	for _, srv := range s.cfg.Plex.Servers {
		status := "offline"
		if srv.LastGoodConnection != "" {
			status = "connected"
		}
		name := srv.Name
		// Resolution order for the SPA-facing label:
		//   1. local override (config.Server.DisplayName)
		//   2. Plex-returned name
		//   3. machineIdentifier fallback
		display := srv.DisplayName
		if display == "" {
			display = name
		}
		if display == "" {
			display = srv.MachineIdentifier
		}
		out = append(out, serverDTO{
			MachineID:   srv.MachineIdentifier,
			Name:        name,
			DisplayName: display,
			BaseURL:     srv.LastGoodConnection,
			Status:      status,
		})
	}
	writeJSON(w, out)
}

// handleServerScoped dispatches everything under /api/servers/<id>/...
// Subpaths: libraries, libraries/<key>/items, ondeck, scrobble, unscrobble.
func (s *Server) handleServerScoped(w http.ResponseWriter, r *http.Request) {
	rest := strings.TrimPrefix(r.URL.Path, "/api/servers/")
	parts := strings.Split(rest, "/")
	if len(parts) < 2 {
		writeError(w, http.StatusNotFound, "path too short")
		return
	}
	machineID := parts[0]
	srv := s.serverByID(machineID)
	if srv == nil {
		writeError(w, http.StatusNotFound, "unknown server")
		return
	}
	switch {
	case len(parts) == 2 && parts[1] == "rename":
		if r.Method != "POST" {
			writeError(w, http.StatusMethodNotAllowed, "POST required")
			return
		}
		var body struct {
			DisplayName string `json:"displayName"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			writeError(w, http.StatusBadRequest, "invalid JSON")
			return
		}
		srv.DisplayName = body.DisplayName
		if err := s.cfg.Save(); err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
		writeJSON(w, map[string]string{"status": "renamed", "displayName": body.DisplayName})
	case len(parts) == 2 && parts[1] == "libraries":
		s.handleLibraries(w, r, srv)
	case len(parts) == 4 && parts[1] == "libraries" && parts[3] == "items":
		s.handleLibraryItems(w, r, srv, parts[2])
	case len(parts) == 4 && parts[1] == "libraries" && parts[3] == "recentlyAdded":
		s.handleLibraryRecentlyAdded(w, r, srv, parts[2])
	case len(parts) == 4 && parts[1] == "libraries" && parts[3] == "collections":
		s.handleLibraryCollections(w, r, srv, parts[2])
	case len(parts) == 4 && parts[1] == "collections" && parts[3] == "items":
		s.handleCollectionItems(w, r, srv, parts[2])
	case len(parts) == 3 && parts[1] == "seasons":
		s.handleSeasons(w, r, srv, parts[2])
	case len(parts) == 4 && parts[1] == "seasons" && parts[3] == "episodes":
		s.handleSeasonEpisodes(w, r, srv, parts[2])
	case len(parts) == 3 && parts[1] == "cw" && parts[2] == "remove":
		s.handleRemoveFromCW(w, r, srv)
	case len(parts) == 2 && parts[1] == "ondeck":
		s.handleOnDeck(w, r, srv)
	case len(parts) == 2 && parts[1] == "scrobble":
		s.handleScrobble(w, r, srv)
	case len(parts) == 2 && parts[1] == "unscrobble":
		s.handleUnscrobble(w, r, srv)
	default:
		writeError(w, http.StatusNotFound, "unknown server sub-path")
	}
}

func (s *Server) handleLibraries(w http.ResponseWriter, r *http.Request, srv *config.Server) {
	if s.plex == nil {
		writeError(w, http.StatusInternalServerError, "plex client not initialised")
		return
	}
	plexSrv := toPlexServer(srv)
	libs, err := s.plex.GetLibraries(plexSrv)
	if err != nil {
		writeError(w, http.StatusBadGateway, err.Error())
		return
	}
	writeJSON(w, libs)
}

func (s *Server) handleLibraryItems(w http.ResponseWriter, r *http.Request, srv *config.Server, libraryKey string) {
	if s.plex == nil {
		writeError(w, http.StatusInternalServerError, "plex client not initialised")
		return
	}
	q := r.URL.Query()
	iq := plex.ItemQuery{
		Sort:    q.Get("sort"),
		Filters: map[string]string{},
	}
	if v, err := strconv.Atoi(q.Get("start")); err == nil {
		iq.Start = v
	}
	if v, err := strconv.Atoi(q.Get("size")); err == nil {
		iq.Size = v
	}
	for key := range q {
		if strings.HasPrefix(key, "filter.") {
			iq.Filters[strings.TrimPrefix(key, "filter.")] = q.Get(key)
		}
	}
	plexSrv := toPlexServer(srv)
	items, err := s.plex.GetItems(plexSrv, libraryKey, iq)
	if err != nil {
		writeError(w, http.StatusBadGateway, err.Error())
		return
	}
	writeJSON(w, items)
}

// handleLibraryRecentlyAdded proxies /library/sections/<id>/recentlyAdded for
// Home shelves. Accepts ?size=<n> to cap the result count.
func (s *Server) handleLibraryRecentlyAdded(w http.ResponseWriter, r *http.Request, srv *config.Server, libraryKey string) {
	if s.plex == nil {
		writeError(w, http.StatusInternalServerError, "plex client not initialised")
		return
	}
	size := 20
	if v, err := strconv.Atoi(r.URL.Query().Get("size")); err == nil && v > 0 && v <= 200 {
		size = v
	}
	items, err := s.plex.GetRecentlyAdded(toPlexServer(srv), libraryKey, size)
	if err != nil {
		writeError(w, http.StatusBadGateway, err.Error())
		return
	}
	writeJSON(w, items)
}

func (s *Server) handleOnDeck(w http.ResponseWriter, r *http.Request, srv *config.Server) {
	if s.plex == nil {
		writeError(w, http.StatusInternalServerError, "plex client not initialised")
		return
	}
	items, err := s.plex.GetOnDeck(toPlexServer(srv))
	if err != nil {
		writeError(w, http.StatusBadGateway, err.Error())
		return
	}
	writeJSON(w, items)
}

// handleScrobble marks a Plex item as fully watched (Plex's /:/scrobble).
// Powers the "Mark as Watched" tick on Continue Watching cards. Side effect:
// the item leaves the onDeck list because watched items aren't on-deck.
// Accepts POST with ?ratingKey=<key>.
func (s *Server) handleScrobble(w http.ResponseWriter, r *http.Request, srv *config.Server) {
	if r.Method != "POST" {
		writeError(w, http.StatusMethodNotAllowed, "POST required")
		return
	}
	if s.plex == nil {
		writeError(w, http.StatusInternalServerError, "plex client not initialised")
		return
	}
	ratingKey := r.URL.Query().Get("ratingKey")
	if ratingKey == "" {
		writeError(w, http.StatusBadRequest, "ratingKey query param required")
		return
	}
	if err := s.plex.Scrobble(toPlexServer(srv), ratingKey); err != nil {
		writeError(w, http.StatusBadGateway, err.Error())
		return
	}
	writeJSON(w, map[string]string{"status": "scrobbled"})
}

// handleUnscrobble resets a Plex item's playback state (Plex's /:/unscrobble)
// — viewCount=0, viewOffset=0. Powers the "Remove from Continue Watching" bin
// on CW cards. Item leaves onDeck without entering watch history.
// Accepts POST with ?ratingKey=<key>.
func (s *Server) handleUnscrobble(w http.ResponseWriter, r *http.Request, srv *config.Server) {
	if r.Method != "POST" {
		writeError(w, http.StatusMethodNotAllowed, "POST required")
		return
	}
	if s.plex == nil {
		writeError(w, http.StatusInternalServerError, "plex client not initialised")
		return
	}
	ratingKey := r.URL.Query().Get("ratingKey")
	if ratingKey == "" {
		writeError(w, http.StatusBadRequest, "ratingKey query param required")
		return
	}
	if err := s.plex.Unscrobble(toPlexServer(srv), ratingKey); err != nil {
		writeError(w, http.StatusBadGateway, err.Error())
		return
	}
	writeJSON(w, map[string]string{"status": "unscrobbled"})
}

// handleRemoveFromCW proxies Plex's first-class
// PUT /actions/removeFromContinueWatching?ratingKey=<key> endpoint. Powers
// the trash icon on Continue Watching cards — Plex propagates the removal
// cross-device (Plex Web, mobile, smart-TV apps reflect it on next fetch).
// Pass the EPISODE ratingKey for shows; Plex's logic figures out the
// "remove the whole show" semantic.
// Accepts POST with ?ratingKey=<key>.
func (s *Server) handleRemoveFromCW(w http.ResponseWriter, r *http.Request, srv *config.Server) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "POST required")
		return
	}
	ratingKey := r.URL.Query().Get("ratingKey")
	if ratingKey == "" {
		writeError(w, http.StatusBadRequest, "ratingKey query param required")
		return
	}
	if s.plex == nil {
		writeError(w, http.StatusInternalServerError, "plex not initialised")
		return
	}
	if err := s.plex.RemoveFromContinueWatching(toPlexServer(srv), ratingKey); err != nil {
		writeError(w, http.StatusBadGateway, err.Error())
		return
	}
	writeJSON(w, map[string]string{"status": "removed"})
}

// handleSeasons proxies /library/metadata/<showKey>/children for the season
// tabs on the ItemDetail page. Returns the raw season list (synthetic season-0
// "All Episodes" included — SPA filters it out for the tab strip).
func (s *Server) handleSeasons(w http.ResponseWriter, r *http.Request, srv *config.Server, showRatingKey string) {
	if s.plex == nil {
		writeError(w, http.StatusInternalServerError, "plex not initialised")
		return
	}
	seasons, err := s.plex.GetSeasons(toPlexServer(srv), showRatingKey)
	if err != nil {
		writeError(w, http.StatusBadGateway, err.Error())
		return
	}
	writeJSON(w, seasons)
}

// handleSeasonEpisodes proxies /library/metadata/<seasonKey>/children for the
// per-season episode list under the season tabs on ItemDetail.
func (s *Server) handleSeasonEpisodes(w http.ResponseWriter, r *http.Request, srv *config.Server, seasonRatingKey string) {
	if s.plex == nil {
		writeError(w, http.StatusInternalServerError, "plex not initialised")
		return
	}
	eps, err := s.plex.GetSeasonEpisodes(toPlexServer(srv), seasonRatingKey)
	if err != nil {
		writeError(w, http.StatusBadGateway, err.Error())
		return
	}
	writeJSON(w, eps)
}

// toPlexServer maps a config.Server to the plex.Server shape the client needs.
// BaseURL is read from LastGoodConnection; AccessToken is propagated.
func toPlexServer(srv *config.Server) *plex.Server {
	return &plex.Server{
		Name:              srv.Name,
		MachineIdentifier: srv.MachineIdentifier,
		AccessToken:       srv.AccessToken,
		BaseURL:           srv.LastGoodConnection,
	}
}
