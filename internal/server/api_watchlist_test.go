package server

import (
	"net/http"
	"strings"
	"testing"

	"lumen/internal/config"
)

func TestWatchlistRequiresAccountToken(t *testing.T) {
	cfg := &config.Config{}
	s := New(cfg, nil, "127.0.0.1:0")
	req, _ := http.NewRequest("GET", "/api/watchlist", nil)
	w := newResponseRecorder()
	s.mux.ServeHTTP(w, req)
	if w.status != http.StatusUnauthorized {
		t.Errorf("status %d, want 401", w.status)
	}
}

func TestWatchlistAddRequiresPOST(t *testing.T) {
	cfg := &config.Config{Plex: config.PlexConfig{AccountToken: "tok"}}
	s := New(cfg, nil, "127.0.0.1:0")
	req, _ := http.NewRequest("GET", "/api/watchlist/add", nil)
	w := newResponseRecorder()
	s.mux.ServeHTTP(w, req)
	if w.status != http.StatusMethodNotAllowed {
		t.Errorf("status %d, want 405", w.status)
	}
}

func TestWatchlistAddRequiresRatingKey(t *testing.T) {
	cfg := &config.Config{Plex: config.PlexConfig{AccountToken: "tok"}}
	s := New(cfg, nil, "127.0.0.1:0")
	req, _ := http.NewRequest("POST", "/api/watchlist/add", strings.NewReader(`{}`))
	req.Header.Set("Content-Type", "application/json")
	w := newResponseRecorder()
	s.mux.ServeHTTP(w, req)
	if w.status != http.StatusBadRequest {
		t.Errorf("status %d, want 400", w.status)
	}
}

func TestWatchlistRemoveRequiresPOST(t *testing.T) {
	cfg := &config.Config{Plex: config.PlexConfig{AccountToken: "tok"}}
	s := New(cfg, nil, "127.0.0.1:0")
	req, _ := http.NewRequest("GET", "/api/watchlist/remove", nil)
	w := newResponseRecorder()
	s.mux.ServeHTTP(w, req)
	if w.status != http.StatusMethodNotAllowed {
		t.Errorf("status %d, want 405", w.status)
	}
}
