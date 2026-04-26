package server

import (
	"encoding/json"
	"net/http"
	"sync"
	"time"

	"lumen/internal/plex"
)

// pendingPIN tracks an in-flight PIN poll launched by /api/auth/start.
// Only one is active at a time; a second /api/auth/start replaces it.
type pendingPIN struct {
	pin     plex.PIN
	created time.Time
}

type authState struct {
	mu      sync.Mutex
	pending *pendingPIN
}

func newAuthState() *authState { return &authState{} }

// handleAuthStart creates a PIN at plex.tv and returns the code + a link
// URL the SPA can show or open. Does NOT block waiting for the user to
// link — the SPA polls /api/auth/poll on a 2s tick.
func (s *Server) handleAuthStart(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" {
		writeError(w, http.StatusMethodNotAllowed, "POST required")
		return
	}
	pin, err := s.plex.CreatePIN()
	if err != nil {
		writeError(w, http.StatusBadGateway, err.Error())
		return
	}
	s.auth.mu.Lock()
	s.auth.pending = &pendingPIN{pin: pin, created: time.Now()}
	s.auth.mu.Unlock()
	writeJSON(w, map[string]any{
		"pinId":   pin.ID,
		"code":    pin.Code,
		"linkURL": plex.ForceBrowserURL(pin.Code),
	})
}

// handleAuthPoll checks whether the pending PIN has been claimed. Returns
// {status:"pending"}, {status:"linked"} after a successful link (token is
// also persisted to config), or {status:"none"} if no PIN is currently
// pending. 410 once the PIN expires (15 minutes since /api/auth/start).
func (s *Server) handleAuthPoll(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" {
		writeError(w, http.StatusMethodNotAllowed, "POST required")
		return
	}
	s.auth.mu.Lock()
	pending := s.auth.pending
	s.auth.mu.Unlock()
	if pending == nil {
		writeJSON(w, map[string]string{"status": "none"})
		return
	}
	if time.Since(pending.created) > 15*time.Minute {
		s.auth.mu.Lock()
		s.auth.pending = nil
		s.auth.mu.Unlock()
		writeJSONStatus(w, http.StatusGone, map[string]string{"status": "expired"})
		return
	}
	// Single-shot poll (2s budget) — keeps the handler responsive without
	// holding an HTTP connection open for the full 5-minute deadline.
	token, err := s.plex.PollPIN(pending.pin, 2*time.Second)
	if err != nil {
		// Timed-out polls return an error from PollPIN; surface them as pending.
		writeJSON(w, map[string]string{"status": "pending"})
		return
	}
	if token == "" {
		writeJSON(w, map[string]string{"status": "pending"})
		return
	}
	// Linked. Persist + clear pending.
	s.cfg.Plex.AccountToken = token
	if err := s.cfg.Save(); err != nil {
		writeError(w, http.StatusInternalServerError, "save config: "+err.Error())
		return
	}
	s.auth.mu.Lock()
	s.auth.pending = nil
	s.auth.mu.Unlock()
	writeJSON(w, map[string]string{"status": "linked"})
}

// decodeStatus is a small helper used by tests to read the status field
// out of a JSON response body.
func decodeStatus(b []byte) string {
	var resp map[string]string
	_ = json.Unmarshal(b, &resp)
	return resp["status"]
}
