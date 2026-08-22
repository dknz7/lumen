package server

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"lumen/internal/config"
)

// safeThemeID turns a user-supplied id into a file name, so anything that
// escapes the themes directory or produces a surprising path has to be
// stripped rather than merely discouraged.
func TestSafeThemeIDStripsPathTraversal(t *testing.T) {
	cases := []struct {
		in, want string
	}{
		{"gruvbox-dark", "gruvbox-dark"},
		{"Gruvbox Dark", "gruvbox-dark"},
		{"  Spaced  Out  ", "spaced-out"},
		{"under_score", "under_score"},
		{"../../config", "config"},
		{`..\..\config`, "config"},
		{"C:/Windows/system32", "cwindowssystem32"},
		{"theme.json", "themejson"},
		{"../", ""},
		{"...", ""},
		{"", ""},
		{"!!!", ""},
		{"a/b/c", "abc"},
	}
	for _, c := range cases {
		if got := safeThemeID(c.in); got != c.want {
			t.Errorf("safeThemeID(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}

func TestSafeThemeIDNeverEscapesDir(t *testing.T) {
	dir := t.TempDir()
	for _, raw := range []string{"../../evil", `..\..\evil`, "/etc/passwd", "C:/Windows/x"} {
		id := safeThemeID(raw)
		if id == "" {
			continue // rejected outright, which is also fine
		}
		full := filepath.Join(dir, id+".json")
		rel, err := filepath.Rel(dir, full)
		if err != nil || filepath.IsAbs(rel) || len(rel) > 1 && rel[0] == '.' && rel[1] == '.' {
			t.Errorf("safeThemeID(%q) produced %q which escapes the themes dir (rel=%q)", raw, id, rel)
		}
	}
}

// A missing themes directory means "no custom themes", not an error — a fresh
// install has never had one.
func TestHandleThemesMissingDirIsNotAnError(t *testing.T) {
	t.Setenv("APPDATA", t.TempDir()) // ThemesDir() resolves under APPDATA

	s := &Server{}
	rec := httptest.NewRecorder()
	s.handleThemes(rec, httptest.NewRequest(http.MethodGet, "/api/themes", nil))

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	var out themesResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &out); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(out.Themes) != 0 {
		t.Errorf("themes = %v, want none", out.Themes)
	}
	if len(out.Errors) != 0 {
		t.Errorf("errors = %v, want none for a missing directory", out.Errors)
	}
}

// Malformed files are reported by name rather than silently skipped: someone
// hand-editing a theme needs to be told which file is wrong.
func TestHandleThemesReportsBadFilesByName(t *testing.T) {
	appdata := t.TempDir()
	t.Setenv("APPDATA", appdata)
	dir := config.ThemesDir()
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}

	write := func(name, body string) {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	write("good.json", `{"id":"good","name":"Good","tokens":{"bg":"#000"}}`)
	write("bad-json.json", `{"id":"x",}`)
	write("no-id.json", `{"name":"Nameless"}`)
	write("notes.txt", `ignored entirely`)

	s := &Server{}
	rec := httptest.NewRecorder()
	s.handleThemes(rec, httptest.NewRequest(http.MethodGet, "/api/themes", nil))

	var out themesResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &out); err != nil {
		t.Fatalf("decode: %v", err)
	}

	if len(out.Themes) != 1 || out.Themes[0].ID != "good" {
		t.Errorf("themes = %+v, want just the valid one", out.Themes)
	}
	if out.Themes[0].File != "good.json" {
		t.Errorf("file = %q, want good.json — the SPA names it when reporting errors", out.Themes[0].File)
	}

	byFile := map[string]string{}
	for _, e := range out.Errors {
		byFile[e.File] = e.Error
	}
	if _, ok := byFile["bad-json.json"]; !ok {
		t.Errorf("expected bad-json.json to be reported, got %+v", out.Errors)
	}
	if _, ok := byFile["no-id.json"]; !ok {
		t.Errorf("expected no-id.json to be reported, got %+v", out.Errors)
	}
	if _, ok := byFile["notes.txt"]; ok {
		t.Errorf("non-JSON files should be ignored, not reported")
	}
}

func TestHandleThemesRejectsNonGET(t *testing.T) {
	s := &Server{}
	rec := httptest.NewRecorder()
	s.handleThemes(rec, httptest.NewRequest(http.MethodPost, "/api/themes", nil))
	if rec.Code != http.StatusMethodNotAllowed {
		t.Errorf("status = %d, want 405", rec.Code)
	}
}
