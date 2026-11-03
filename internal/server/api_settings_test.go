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

// libraryPrefs and typePrefs go through PUT like shelfState: a PUT replaces
// the whole map, and GET returns it.
func TestPutSettingsLibraryAndTypePrefs(t *testing.T) {
	t.Setenv("APPDATA", t.TempDir())
	cfg, _ := config.Load()
	s := New(cfg, nil, "127.0.0.1:0")

	put := func(payload string) {
		t.Helper()
		req, _ := http.NewRequest("PUT", "/api/settings", bytes.NewBufferString(payload))
		w := newResponseRecorder()
		s.mux.ServeHTTP(w, req)
		if w.status != 200 {
			t.Fatalf("status: %d — %s", w.status, w.body.b)
		}
	}
	put(`{"libraryPrefs":{"abc:11":{"sort":"originallyAvailableAt:desc","viewMode":"shows"},"abc:1":{"sort":"titleSort:asc"}},
	      "typePrefs":{"show":{"sort":"originallyAvailableAt:desc","viewMode":"shows"}}}`)
	put(`{"libraryPrefs":{"abc:11":{"sort":"rating:desc"}}}`) // authoritative: abc:1 goes

	reloaded, _ := config.Load()
	if len(reloaded.UI.LibraryPrefs) != 1 || reloaded.UI.LibraryPrefs["abc:11"].Sort != "rating:desc" {
		t.Errorf("libraryPrefs after replace: %+v", reloaded.UI.LibraryPrefs)
	}
	if reloaded.UI.TypePrefs["show"].ViewMode != "shows" {
		t.Errorf("typePrefs untouched by a PUT that omits it: %+v", reloaded.UI.TypePrefs)
	}

	req, _ := http.NewRequest("GET", "/api/settings", nil)
	w := newResponseRecorder()
	s.mux.ServeHTTP(w, req)
	var got struct {
		LibraryPrefs map[string]map[string]string `json:"libraryPrefs"`
		TypePrefs    map[string]map[string]string `json:"typePrefs"`
	}
	body, _ := io.ReadAll(w.body)
	if err := json.Unmarshal(body, &got); err != nil {
		t.Fatal(err)
	}
	if got.LibraryPrefs["abc:11"]["sort"] != "rating:desc" || got.TypePrefs["show"]["sort"] != "originallyAvailableAt:desc" {
		t.Errorf("GET payload: %s", body)
	}
}
