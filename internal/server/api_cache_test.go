package server

import (
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"lumen/internal/config"
)

func TestCacheSizeReportsBytes(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("APPDATA", tmp)

	// Write 3 files worth of image cache (total 300 bytes).
	dir := filepath.Join(tmp, "Lumen", "cache", "images")
	_ = os.MkdirAll(dir, 0o755)
	for i := 0; i < 3; i++ {
		_ = os.WriteFile(filepath.Join(dir, string(rune('a'+i))), []byte(strings.Repeat("x", 100)), 0o644)
	}

	cfg, _ := config.Load()
	s := New(cfg, nil, "127.0.0.1:0")

	req, _ := http.NewRequest("GET", "/api/cache/size", nil)
	w := newResponseRecorder()
	s.mux.ServeHTTP(w, req)
	if w.status != 200 {
		t.Fatalf("status: %d", w.status)
	}
	body := string(w.body.b)
	if !strings.Contains(body, `"images":300`) {
		t.Errorf("body missing images:300, got: %s", body)
	}
}

func TestCacheClearImagesWipesDirectory(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("APPDATA", tmp)
	dir := filepath.Join(tmp, "Lumen", "cache", "images")
	_ = os.MkdirAll(dir, 0o755)
	_ = os.WriteFile(filepath.Join(dir, "x"), []byte("hi"), 0o644)

	cfg, _ := config.Load()
	s := New(cfg, nil, "127.0.0.1:0")

	req, _ := http.NewRequest("POST", "/api/cache/clear?scope=images", nil)
	w := newResponseRecorder()
	s.mux.ServeHTTP(w, req)
	if w.status != 200 {
		t.Fatalf("status: %d — %s", w.status, w.body.b)
	}
	entries, _ := os.ReadDir(dir)
	if len(entries) != 0 {
		t.Errorf("images dir still has %d entries after clear", len(entries))
	}
}

func TestCacheClearRejectsBadScope(t *testing.T) {
	cfg := &config.Config{}
	s := New(cfg, nil, "127.0.0.1:0")
	req, _ := http.NewRequest("POST", "/api/cache/clear?scope=bogus", nil)
	w := newResponseRecorder()
	s.mux.ServeHTTP(w, req)
	if w.status != 400 {
		t.Errorf("status: %d, want 400", w.status)
	}
}
