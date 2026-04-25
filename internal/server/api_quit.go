package server

import (
	"net/http"
	"time"
)

// handleQuit lets the SPA request graceful shutdown of lumen.exe. Triggered
// by the "Close Lumen" confirmation in the top bar. We write the 200 response
// and flush it BEFORE signalling shutdown — the brief 150 ms delay in the
// goroutine gives the HTTP layer time to deliver the response to the SPA
// before the listener tears down.
func (s *Server) handleQuit(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	writeJSON(w, map[string]string{"status": "shutting down"})
	if f, ok := w.(http.Flusher); ok {
		f.Flush()
	}
	go func() {
		time.Sleep(150 * time.Millisecond)
		select {
		case s.quit <- struct{}{}:
		default:
			// Already signalled — buffered channel is full. Idempotent.
		}
	}()
}
