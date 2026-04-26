package server

import (
	"net/http"
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
