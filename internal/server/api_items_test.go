package server

import (
	"net/http"
	"testing"

	"lumen/internal/config"
)

func TestItemsRequiresServerQuery(t *testing.T) {
	cfg := &config.Config{Plex: config.PlexConfig{Servers: []config.Server{{MachineIdentifier: "abc", LastGoodConnection: "https://offline.example", AccessToken: "tok"}}}}
	s := New(cfg, nil, "127.0.0.1:0")

	req, _ := http.NewRequest("GET", "/api/items/12345", nil)
	w := newResponseRecorder()
	s.mux.ServeHTTP(w, req)
	if w.status != 400 {
		t.Errorf("missing server query: status %d, want 400", w.status)
	}

	req, _ = http.NewRequest("GET", "/api/items/12345?server=nonexistent", nil)
	w = newResponseRecorder()
	s.mux.ServeHTTP(w, req)
	if w.status != 404 {
		t.Errorf("unknown server: status %d, want 404", w.status)
	}
}
