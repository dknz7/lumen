package server

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"testing"

	"lumen/internal/config"
)

func TestGetSettingsReturnsUI(t *testing.T) {
	cfg := &config.Config{
		UI: config.UIConfig{Theme: "pure-oled", Zoom: 120, RowsPerShelf: 4},
	}
	s := New(cfg, nil, "127.0.0.1:0")

	req, _ := http.NewRequest("GET", "/api/settings", nil)
	w := newResponseRecorder()
	s.mux.ServeHTTP(w, req)
	if w.status != 200 {
		t.Fatalf("status: %d", w.status)
	}
	var got map[string]any
	body, _ := io.ReadAll(w.body)
	if err := json.Unmarshal(body, &got); err != nil {
		t.Fatal(err)
	}
	if got["theme"] != "pure-oled" || got["zoom"] != float64(120) || got["rowsPerShelf"] != float64(4) {
		t.Errorf("payload: %+v", got)
	}
}

func TestPutSettingsUpdatesConfigAndPersists(t *testing.T) {
	t.Setenv("APPDATA", t.TempDir())
	cfg, _ := config.Load()
	s := New(cfg, nil, "127.0.0.1:0")

	payload := `{"theme":"high-contrast","zoom":130,"cardSize":"l","rowsPerShelf":2,"hiddenLibraries":["abc:5"]}`
	req, _ := http.NewRequest("PUT", "/api/settings", bytes.NewBufferString(payload))
	w := newResponseRecorder()
	s.mux.ServeHTTP(w, req)
	if w.status != 200 {
		t.Fatalf("status: %d — %s", w.status, w.body.b)
	}

	// In-memory mutated.
	if cfg.UI.Theme != "high-contrast" || cfg.UI.Zoom != 130 {
		t.Errorf("in-memory: %+v", cfg.UI)
	}
	// On-disk persisted.
	reloaded, _ := config.Load()
	if reloaded.UI.Theme != "high-contrast" {
		t.Errorf("on-disk: %+v", reloaded.UI)
	}
}

func TestPutSettingsRejectsInvalidJSON(t *testing.T) {
	cfg := &config.Config{}
	s := New(cfg, nil, "127.0.0.1:0")
	req, _ := http.NewRequest("PUT", "/api/settings", bytes.NewBufferString("not json"))
	w := newResponseRecorder()
	s.mux.ServeHTTP(w, req)
	if w.status != 400 {
		t.Errorf("status: %d, want 400", w.status)
	}
}
