package server

import (
	"net/http"
	"testing"

	"lumen/internal/config"
)

func TestTMDBTrailerRequiresKey(t *testing.T) {
	cfg := &config.Config{}
	s := New(cfg, nil, "127.0.0.1:0")
	req, _ := http.NewRequest("GET", "/api/tmdb/trailer/tt0111161?type=movie", nil)
	w := newResponseRecorder()
	s.mux.ServeHTTP(w, req)
	if w.status != http.StatusServiceUnavailable {
		t.Errorf("status %d, want 503", w.status)
	}
}

func TestTMDBTrailerValidatesIDShape(t *testing.T) {
	t.Setenv("APPDATA", t.TempDir())
	cfg := &config.Config{TMDBKey: "test-key"}
	s := New(cfg, nil, "127.0.0.1:0")
	for _, id := range []string{"", "abc", "tt0111161/extra", "0111161"} {
		req, _ := http.NewRequest("GET", "/api/tmdb/trailer/"+id+"?type=movie", nil)
		w := newResponseRecorder()
		s.mux.ServeHTTP(w, req)
		if w.status == http.StatusOK {
			t.Errorf("id=%q: expected non-200, got 200", id)
		}
	}
}

func TestTMDBTrailerValidatesType(t *testing.T) {
	t.Setenv("APPDATA", t.TempDir())
	cfg := &config.Config{TMDBKey: "test-key"}
	s := New(cfg, nil, "127.0.0.1:0")
	for _, ty := range []string{"", "season", "episode", "tv"} {
		req, _ := http.NewRequest("GET", "/api/tmdb/trailer/tt0111161?type="+ty, nil)
		w := newResponseRecorder()
		s.mux.ServeHTTP(w, req)
		if w.status != http.StatusBadRequest {
			t.Errorf("type=%q: status %d, want 400", ty, w.status)
		}
	}
}
