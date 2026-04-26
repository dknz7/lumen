package server

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"lumen/internal/config"
)

func TestIMDBRequiresOMDBKey(t *testing.T) {
	cfg := &config.Config{} // OMDBKey unset
	s := New(cfg, nil, "127.0.0.1:0")
	req, _ := http.NewRequest("GET", "/api/imdb/tt0111161", nil)
	w := newResponseRecorder()
	s.mux.ServeHTTP(w, req)
	if w.status != http.StatusServiceUnavailable {
		t.Errorf("status %d, want 503", w.status)
	}
}

func TestIMDBValidatesIDShape(t *testing.T) {
	t.Setenv("APPDATA", t.TempDir())
	cfg := &config.Config{OMDBKey: "abc12345"}
	s := New(cfg, nil, "127.0.0.1:0")

	for _, id := range []string{"", "abc", "tt0111161/extra", "0111161"} {
		req, _ := http.NewRequest("GET", "/api/imdb/"+id, nil)
		w := newResponseRecorder()
		s.mux.ServeHTTP(w, req)
		if w.status == http.StatusOK {
			t.Errorf("id=%q: expected non-200, got 200", id)
		}
	}
}

func TestIMDBServesCachedHit(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("APPDATA", tmp)
	dir := filepath.Join(tmp, "Lumen", "cache", "omdb")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	cached := []byte(`{"imdbID":"tt0111161","imdbRating":"9.3"}`)
	if err := os.WriteFile(filepath.Join(dir, "tt0111161.json"), cached, 0o644); err != nil {
		t.Fatal(err)
	}
	cfg := &config.Config{OMDBKey: "abc12345"}
	s := New(cfg, nil, "127.0.0.1:0")
	req := httptest.NewRequest("GET", "/api/imdb/tt0111161", nil)
	w := newResponseRecorder()
	s.mux.ServeHTTP(w, req)
	if w.status != http.StatusOK {
		t.Fatalf("status %d, want 200", w.status)
	}
	if string(w.body.b) != string(cached) {
		t.Errorf("body mismatch: got %q want %q", string(w.body.b), string(cached))
	}
}
