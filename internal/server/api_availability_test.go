package server

import (
	"net/http"
	"testing"

	"lumen/internal/config"
)

func TestAvailabilityRequiresGUID(t *testing.T) {
	cfg := &config.Config{Plex: config.PlexConfig{Servers: []config.Server{{MachineIdentifier: "abc", LastGoodConnection: "https://offline.example", AccessToken: "tok"}}}}
	s := New(cfg, nil, "127.0.0.1:0")

	req, _ := http.NewRequest("GET", "/api/availability", nil)
	w := newResponseRecorder()
	s.mux.ServeHTTP(w, req)
	if w.status != 400 {
		t.Errorf("status %d, want 400", w.status)
	}
}
