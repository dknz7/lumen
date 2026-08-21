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
	// no-store keeps browsers from serving stale JSON state (e.g. /api/items/...)
	// after the user changes things in Plex Web and switches back to Lumen.
	// The image proxy sets its own long-lived cache headers and is unaffected.
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

// writeError writes {"error": msg} with the given status.
func writeError(w http.ResponseWriter, status int, msg string) {
	writeJSONStatus(w, status, map[string]string{"error": msg})
}

// --- Config access -----------------------------------------------------------
//
// s.cfg is shared mutable state and net/http runs every request on its own
// goroutine, so nothing may touch it outside these accessors. Two concurrent
// PUT /api/settings both writing cfg.UI.ShelfState — which the SPA does when
// autosaving shelf layout — is a `fatal error: concurrent map writes`, and that
// is not a panic you can recover from.
//
// Readers take RLock and return copies, so callers can use the result after the
// lock is released. Writers go through mutateCfg.

// serverByID looks up a server by MachineIdentifier and returns a *copy*.
// Returns nil if not found.
//
// The copy matters: handing out a pointer into cfg.Plex.Servers lets callers
// read fields long after the lock is gone, and /api/servers/refresh replaces
// that whole slice.
func (s *Server) serverByID(id string) *config.Server {
	s.cfgMu.RLock()
	defer s.cfgMu.RUnlock()
	for i := range s.cfg.Plex.Servers {
		if s.cfg.Plex.Servers[i].MachineIdentifier == id {
			cp := s.cfg.Plex.Servers[i]
			return &cp
		}
	}
	return nil
}

// serverList returns a snapshot of the configured servers.
func (s *Server) serverList() []config.Server {
	s.cfgMu.RLock()
	defer s.cfgMu.RUnlock()
	return append([]config.Server(nil), s.cfg.Plex.Servers...)
}

// accountToken returns the Plex account token, or "" when not linked yet.
func (s *Server) accountToken() string {
	s.cfgMu.RLock()
	defer s.cfgMu.RUnlock()
	return s.cfg.Plex.AccountToken
}

func (s *Server) omdbKey() string {
	s.cfgMu.RLock()
	defer s.cfgMu.RUnlock()
	return s.cfg.OMDBKey
}

func (s *Server) tmdbKey() string {
	s.cfgMu.RLock()
	defer s.cfgMu.RUnlock()
	return s.cfg.TMDBKey
}

// potPlayerPath returns the configured Pot Player override ("" = auto-detect).
func (s *Server) potPlayerPath() string {
	s.cfgMu.RLock()
	defer s.cfgMu.RUnlock()
	return s.cfg.UI.Playback.PotPlayerPath
}

// mutateCfg applies fn to the config under an exclusive lock and persists the
// result. fn must not call back into any other config accessor — the lock is
// not reentrant.
func (s *Server) mutateCfg(fn func(*config.Config)) error {
	s.cfgMu.Lock()
	defer s.cfgMu.Unlock()
	fn(s.cfg)
	return s.cfg.Save()
}

// marshalCfg runs fn under a read lock and returns what it produced. Used for
// responses that serialise config containing maps, where the marshal itself
// must happen inside the lock.
func (s *Server) marshalCfg(fn func(*config.Config) any) ([]byte, error) {
	s.cfgMu.RLock()
	defer s.cfgMu.RUnlock()
	return json.Marshal(fn(s.cfg))
}
