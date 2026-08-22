package server

import (
	"encoding/json"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strconv"
	"strings"

	"lumen/internal/config"
)

// Custom themes live as JSON files in %APPDATA%\Lumen\themes.
//
// This file is deliberately a dumb reader: it parses each file far enough to
// know it is an object carrying an id and a name, and hands the rest to the
// SPA untouched. The SPA owns validation because it owns the token list — it
// knows which keys exist, which built-in an "extends" refers to, and whether
// a value is something the browser will actually accept as a colour.
// Duplicating any of that here would be a second source of truth that drifts.
const (
	// A theme is 25 short strings. Anything approaching this is not a theme.
	maxThemeFileBytes = 64 << 10
	// Enough for a large collection, small enough that a directory full of
	// junk cannot stall startup.
	maxThemeFiles = 200
)

type themeFileDTO struct {
	File    string          `json:"file"`
	ID      string          `json:"id"`
	Name    string          `json:"name"`
	Extends string          `json:"extends,omitempty"`
	Tokens  json.RawMessage `json:"tokens,omitempty"`
}

type themeErrorDTO struct {
	File  string `json:"file"`
	Error string `json:"error"`
}

type themesResponse struct {
	Dir    string          `json:"dir"`
	Themes []themeFileDTO  `json:"themes"`
	Errors []themeErrorDTO `json:"errors"`
}

// handleThemes lists the user's custom themes.
//
// Malformed files are reported rather than swallowed. Someone hand-editing a
// theme wants to be told their JSON has a trailing comma, not to watch their
// theme quietly fail to appear in the picker.
func (s *Server) handleThemes(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "GET only")
		return
	}

	dir := config.ThemesDir()
	out := themesResponse{Dir: dir, Themes: []themeFileDTO{}, Errors: []themeErrorDTO{}}

	entries, err := os.ReadDir(dir)
	if err != nil {
		// A missing directory just means no custom themes yet.
		if !os.IsNotExist(err) {
			out.Errors = append(out.Errors, themeErrorDTO{File: ".", Error: err.Error()})
		}
		writeJSON(w, out)
		return
	}

	names := make([]string, 0, len(entries))
	for _, e := range entries {
		if e.IsDir() || !strings.EqualFold(filepath.Ext(e.Name()), ".json") {
			continue
		}
		names = append(names, e.Name())
	}
	sort.Strings(names)
	if len(names) > maxThemeFiles {
		names = names[:maxThemeFiles]
	}

	for _, name := range names {
		full := filepath.Join(dir, name)
		info, statErr := os.Stat(full)
		if statErr != nil {
			out.Errors = append(out.Errors, themeErrorDTO{File: name, Error: statErr.Error()})
			continue
		}
		if info.Size() > maxThemeFileBytes {
			out.Errors = append(out.Errors, themeErrorDTO{File: name, Error: "file is too large to be a theme"})
			continue
		}
		b, readErr := os.ReadFile(full)
		if readErr != nil {
			out.Errors = append(out.Errors, themeErrorDTO{File: name, Error: readErr.Error()})
			continue
		}
		var t themeFileDTO
		if jsonErr := json.Unmarshal(b, &t); jsonErr != nil {
			out.Errors = append(out.Errors, themeErrorDTO{File: name, Error: "invalid JSON: " + jsonErr.Error()})
			continue
		}
		if strings.TrimSpace(t.ID) == "" || strings.TrimSpace(t.Name) == "" {
			out.Errors = append(out.Errors, themeErrorDTO{File: name, Error: `needs both an "id" and a "name"`})
			continue
		}
		t.File = name
		out.Themes = append(out.Themes, t)
	}

	writeJSON(w, out)
}

// handleThemesReveal opens the themes folder in Explorer.
func (s *Server) handleThemesReveal(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	dir := config.ThemesDir()
	if err := os.MkdirAll(dir, 0o755); err != nil {
		writeError(w, http.StatusInternalServerError, "create themes folder: "+err.Error())
		return
	}
	// Fixed command, fixed path — nothing from the request reaches the shell.
	// Explorer exits non-zero even on success, so its status is ignored.
	_ = exec.Command("explorer", dir).Start()
	writeJSON(w, map[string]string{"dir": dir})
}

// handleThemesExport writes the posted theme into the themes folder as a
// complete, valid starting point. The SPA sends the fully resolved tokens of
// whatever theme is active, so the result needs no "extends" to work and can
// be edited immediately.
func (s *Server) handleThemesExport(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	r.Body = http.MaxBytesReader(w, r.Body, maxThemeFileBytes)

	var in struct {
		ID     string            `json:"id"`
		Name   string            `json:"name"`
		Tokens map[string]string `json:"tokens"`
	}
	if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON body")
		return
	}
	id := safeThemeID(in.ID)
	if id == "" || strings.TrimSpace(in.Name) == "" || len(in.Tokens) == 0 {
		writeError(w, http.StatusBadRequest, "id, name and tokens are all required")
		return
	}

	dir := config.ThemesDir()
	if err := os.MkdirAll(dir, 0o755); err != nil {
		writeError(w, http.StatusInternalServerError, "create themes folder: "+err.Error())
		return
	}

	body, err := json.MarshalIndent(struct {
		ID     string            `json:"id"`
		Name   string            `json:"name"`
		Tokens map[string]string `json:"tokens"`
	}{id, in.Name, in.Tokens}, "", "  ")
	if err != nil {
		writeError(w, http.StatusInternalServerError, "encode theme: "+err.Error())
		return
	}

	// Never clobber an existing file: someone exporting a second time has
	// probably already edited the first one.
	path := filepath.Join(dir, id+".json")
	for i := 2; ; i++ {
		if _, statErr := os.Stat(path); os.IsNotExist(statErr) {
			break
		}
		path = filepath.Join(dir, id+"-"+strconv.Itoa(i)+".json")
	}
	if err := os.WriteFile(path, append(body, '\n'), 0o644); err != nil {
		writeError(w, http.StatusInternalServerError, "write theme: "+err.Error())
		return
	}
	writeJSON(w, map[string]string{"path": path, "file": filepath.Base(path)})
}

// safeThemeID reduces an id to characters that are safe in a file name. The
// id becomes the file name, so a value like "../../config" must not survive.
func safeThemeID(raw string) string {
	var b strings.Builder
	lastDash := false
	for _, r := range strings.ToLower(strings.TrimSpace(raw)) {
		switch {
		case r >= 'a' && r <= 'z', r >= '0' && r <= '9':
			b.WriteRune(r)
			lastDash = false
		case r == '_':
			b.WriteRune(r)
			lastDash = false
		case r == '-', r == ' ':
			// Collapse runs: "Spaced  Out" should be spaced-out, not
			// spaced--out.
			if !lastDash {
				b.WriteRune('-')
				lastDash = true
			}
		}
	}
	out := strings.Trim(b.String(), "-_")
	if len(out) > 48 {
		out = out[:48]
	}
	return out
}
