package server

import (
	"net/http"
	"os"
	"path/filepath"
	"strings"
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

// TestTMDBTrailerServesCachedHit pre-seeds the on-disk cache and verifies the
// handler short-circuits to serve the cached youtubeID at 200 — i.e. the
// positive-path that the existing validation tests don't exercise. Keeps the
// suite from regressing the cache-read branch.
func TestTMDBTrailerServesCachedHit(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("APPDATA", tmp)
	dir := filepath.Join(tmp, "Lumen", "cache", "tmdb")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	cached := []byte(`{"youtubeID":"NmzuHjWmXOc"}`)
	if err := os.WriteFile(filepath.Join(dir, "tt0111161-movie.json"), cached, 0o644); err != nil {
		t.Fatalf("write cache: %v", err)
	}

	cfg := &config.Config{TMDBKey: "test-key"}
	s := New(cfg, nil, "127.0.0.1:0")
	req, _ := http.NewRequest("GET", "/api/tmdb/trailer/tt0111161?type=movie", nil)
	w := newResponseRecorder()
	s.mux.ServeHTTP(w, req)

	if w.status != http.StatusOK {
		t.Fatalf("status %d, want 200", w.status)
	}
	body := string(w.body.b)
	if !strings.Contains(body, `"youtubeID":"NmzuHjWmXOc"`) {
		t.Errorf("body %q missing youtubeID", body)
	}
}
