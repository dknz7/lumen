package server

import (
	"net/http"
	"testing"

	"lumen/internal/config"
)

func TestAuthStartRequiresPOST(t *testing.T) {
	cfg := &config.Config{}
	s := New(cfg, nil, "127.0.0.1:0")
	req, _ := http.NewRequest("GET", "/api/auth/start", nil)
	w := newResponseRecorder()
	s.mux.ServeHTTP(w, req)
	if w.status != http.StatusMethodNotAllowed {
		t.Errorf("status %d, want 405", w.status)
	}
}

func TestAuthPollRequiresPOST(t *testing.T) {
	cfg := &config.Config{}
	s := New(cfg, nil, "127.0.0.1:0")
	req, _ := http.NewRequest("GET", "/api/auth/poll", nil)
	w := newResponseRecorder()
	s.mux.ServeHTTP(w, req)
	if w.status != http.StatusMethodNotAllowed {
		t.Errorf("status %d, want 405", w.status)
	}
}

func TestAuthPollReturnsNoneWhenNoPendingPIN(t *testing.T) {
	cfg := &config.Config{}
	s := New(cfg, nil, "127.0.0.1:0")
	req, _ := http.NewRequest("POST", "/api/auth/poll", nil)
	w := newResponseRecorder()
	s.mux.ServeHTTP(w, req)
	if w.status != http.StatusOK {
		t.Errorf("status %d, want 200", w.status)
	}
	if got := decodeStatus(w.body.b); got != "none" {
		t.Errorf("status field %q, want 'none'", got)
	}
}
