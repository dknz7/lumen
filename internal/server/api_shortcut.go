package server

import (
	"net/http"
	"os"

	"lumen/internal/shortcuts"
)

// handleShortcut creates a Desktop shortcut pointing at the current lumen.exe.
func (s *Server) handleShortcut(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" {
		writeError(w, http.StatusMethodNotAllowed, "POST required")
		return
	}
	exe, err := os.Executable()
	if err != nil {
		writeError(w, http.StatusInternalServerError, "resolve exe path: "+err.Error())
		return
	}
	path, err := shortcuts.CreateDesktop(exe, "serve")
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, map[string]string{"path": path})
}
