package server

import (
	"net/http"
	"sync"
)

// windowController lets the SPA and a second launched instance drive the
// native window.
//
// It is wired in by cmd/lumen rather than imported, because internal/shell is
// Windows-only and depends on WebView2 — the server package has to stay
// buildable (and testable) without it, and in --browser mode there is no
// window at all.
type windowController struct {
	mu   sync.RWMutex
	show func()
	hide func()
}

// SetWindowController registers the desktop shell's show/hide hooks. Passing
// nils (browser mode) leaves the window endpoints reporting "no window".
func (s *Server) SetWindowController(show, hide func()) {
	s.window.mu.Lock()
	defer s.window.mu.Unlock()
	s.window.show = show
	s.window.hide = hide
}

func (s *Server) windowHooks() (show, hide func()) {
	s.window.mu.RLock()
	defer s.window.mu.RUnlock()
	return s.window.show, s.window.hide
}

// handleWindowShow brings the window to the front. Also the mechanism by which
// launching Lumen a second time surfaces the instance already running.
func (s *Server) handleWindowShow(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "POST required")
		return
	}
	show, _ := s.windowHooks()
	if show == nil {
		writeJSON(w, map[string]string{"status": "no-window"})
		return
	}
	show()
	writeJSON(w, map[string]string{"status": "shown"})
}

// handleWindowHide sends the window to the tray, leaving Lumen running.
func (s *Server) handleWindowHide(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "POST required")
		return
	}
	_, hide := s.windowHooks()
	if hide == nil {
		writeJSON(w, map[string]string{"status": "no-window"})
		return
	}
	hide()
	writeJSON(w, map[string]string{"status": "hidden"})
}
