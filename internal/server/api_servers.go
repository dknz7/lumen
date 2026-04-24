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
// Subpaths: libraries, libraries/<key>/items, ondeck.
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
	case len(parts) == 2 && parts[1] == "ondeck":
		s.handleOnDeck(w, r, srv)
	case len(parts) == 3 && parts[1] == "ondeck" && parts[2] == "remove":
		s.handleOnDeckRemove(w, r, srv)
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

// handleOnDeckRemove removes an item from the server's Continue Watching list
// by marking it as watched via Plex's scrobble endpoint. Accepts POST with
// ?ratingKey=<key>.
func (s *Server) handleOnDeckRemove(w http.ResponseWriter, r *http.Request, srv *config.Server) {
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
	writeJSON(w, map[string]string{"status": "removed"})
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
