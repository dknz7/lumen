# Lumen — Session 3 Implementation Plan (Theme, Settings Modal, Persistence, Drag-Reorder)

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Upgrade Lumen from "it works" to "it's yours". Ship a pluggable theme registry (Pure OLED wired, slots ready for more), a full Settings modal with six sections, `config.json`-backed persistence replacing Session 2's localStorage shims, drag-to-reorder shelves and groups via `@thisbeyond/solid-dnd`, inline library visibility toggles, cache management with size readout + clear buttons, and a "Create Desktop Shortcut" one-click helper (replacing the autostart spec item — Byron opts to launch on demand).

**Architecture:** Config grows a `UI` sub-struct carrying every persistable preference. A new `/api/settings` GET/PUT endpoint is the single source of truth; the SPA reads it on boot, writes it on every change. A new theme registry lives in `web/src/themes/` — Pure OLED is the only defined theme for v1, but the shape and apply-mechanism support arbitrary future themes without refactoring. Shelf/group ordering state is stored per "page key" (home, recommended, discover — the pages that have shelves). Drag-reorder uses `@thisbeyond/solid-dnd`'s sortable primitives. Spec §13's "OLED Protection" section is deliberately **dropped** — Byron's call: theme switching already rotates pixels; pixel-shift + auto-hide are net complexity without gain.

**Tech Stack additions:**
- `@thisbeyond/solid-dnd` ^0.7.x — Solid-native drag/drop + sortable presets (~5 KB gzipped).
- `lucide-solid` ^0.x — SVG icon library. Replaces every emoji in the codebase (Taste rule: anti-emoji policy is critical).
- `@fontsource/geist-sans` + `@fontsource/geist-mono` — self-hosted Geist font family. Replaces the generic system-font stack (Taste rule: Inter is banned; Geist / Satoshi / Cabinet Grotesk preferred for dashboards).
- `@motionone/solid` — Solid adapter for Motion One. Drives modal fade+scale and subtle layout transitions with spring physics.
- Go stdlib only for new endpoints.

**Taste rules carried forward** (applied in Task 21, enforced from Session 3 onwards):
- **No emoji icons** anywhere — everything goes to Lucide SVGs with stable stroke-width 1.5.
- **No Inter, no Arial, no system stack for the primary font** — Geist Sans is the daily driver.
- **Skeleton loaders, not "Loading…" text** — card-shaped shimmers matching the target layout.
- **Focus rings via `:focus-visible`** on every interactive element.
- **Max 1 accent colour** (our dark navy `#0f1729` — desaturated, singular).
- **Pure `#000000` is retained** — Byron's OLED burn-in mitigation overrides the skill's "no pure black" guidance. Documented exception.
- **No perpetual micro-animations** on every card — Lumen is a functional media browser, not a Vercel landing demo. Motion is reserved for deliberate moments (modal open, drag, hover dim).

**Carry-ins from Sessions 0–2:**
- All `internal/config`, `internal/plex`, `internal/server`, `internal/potplayer` packages.
- Full Session 2 SPA under `web/src/`.
- `docs/session-2-findings.md` — notes the localStorage prefs that Session 3 migrates to config.json, and the DKNZPLEX CDN thumb issue deferred as out-of-scope.
- `lumen rename` CLI — Session 3's Settings "per-server display name" control replaces this as the primary UX; `rename` stays for power users.

**Deliberately excluded from Session 3:**
- OLED pixel-shift / auto-hide chrome (dropped per Byron's design call).
- Autostart on Windows (dropped — desktop shortcut instead).
- Actual kiosk launch logic (Session 5 — the Settings checkbox just writes config).
- Actual OMDB lookup (Session 5 — the API key field just persists).
- Actual Pot Player path auto-detection (Session 4 — the field just persists; rename/override works).
- Re-authenticate button wiring (stubbed; calls out to an in-session PIN flow — Session 5 polishes).

**Pre-flight:**
- Working directory: `C:\Users\dicke\Desktop\Dump Zone\STACK\04-DEV\lumen`
- Stay on `main`.
- Node ≥ 20, npm ≥ 10 already available.
- Start from a clean `go test ./...` (33+ tests passing) and a clean `npm run build`.

---

## File Structure

**Go additions:**

```
internal/
├── config/
│   └── config.go                   # Extended with UI struct + ShelfState types
├── server/
│   ├── api_settings.go             # GET/PUT /api/settings
│   ├── api_settings_test.go
│   ├── api_cache.go                # GET /api/cache/size + POST /api/cache/clear
│   ├── api_cache_test.go
│   └── api_user.go                 # GET /api/user (plex.tv account info)
└── shortcuts/
    ├── windows.go                  # CreateDesktopShortcut via PowerShell WScript.Shell
    └── windows_test.go             # stubbed test that only runs on windows
```

**New CLI command:**

```
cmd/lumen/
└── install_shortcut.go             # `lumen install-shortcut` subcommand
```

**SPA additions:**

```
web/src/
├── api/
│   └── settings.ts                 # Typed client wrapping /api/settings
├── state/
│   └── settings.ts                 # Solid store holding the current settings
├── themes/
│   ├── index.ts                    # registry + applyTheme helper
│   └── pure-oled.ts                # theme definition (only entry for v1)
├── components/
│   ├── Settings/
│   │   ├── SettingsModal.tsx + .css        # modal overlay shell
│   │   ├── Section.tsx + .css              # reusable section wrapper
│   │   ├── Appearance.tsx + .css
│   │   ├── KioskShortcuts.tsx + .css
│   │   ├── AccountsServers.tsx + .css
│   │   ├── Playback.tsx + .css
│   │   ├── DataCache.tsx + .css
│   │   └── About.tsx + .css
│   ├── Shelf.tsx                   # extended: draggable + hide button
│   └── Group.tsx                   # extended: draggable + collapsible (persisted)
└── pages/
    └── Home.tsx                    # uses solid-dnd SortableProvider for shelf/group reorder
```

---

## Task 1: Extend `config.Config` with UI preferences

**Files:**
- Modify: `internal/config/config.go`
- Modify: `internal/config/config_test.go`

**Context:** Session 2 punted UI prefs to `localStorage`. Time to formalize. Every preference the Settings modal controls lives here. Existing fields (`ClientIdentifier`, `Plex.*`, `OMDBKey`) stay unchanged.

- [ ] **Step 1: Write failing tests**

Append to `internal/config/config_test.go`:
```go
func TestUIDefaultsPopulatedOnFreshLoad(t *testing.T) {
	t.Setenv("APPDATA", t.TempDir())
	c, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	if c.UI.Theme != "pure-oled" {
		t.Errorf("default theme: got %q want pure-oled", c.UI.Theme)
	}
	if c.UI.Zoom != 100 {
		t.Errorf("default zoom: got %d want 100", c.UI.Zoom)
	}
	if c.UI.RowsPerShelf != 3 {
		t.Errorf("default rows: got %d want 3", c.UI.RowsPerShelf)
	}
	if c.UI.CardSize != "m" {
		t.Errorf("default card size: got %q want m", c.UI.CardSize)
	}
	if c.UI.DefaultViewMode != "episodes" {
		t.Errorf("default view mode: got %q want episodes", c.UI.DefaultViewMode)
	}
	if c.UI.ShelfState == nil {
		t.Errorf("ShelfState should be initialised to empty map, not nil")
	}
	if c.UI.HiddenLibraries == nil {
		t.Errorf("HiddenLibraries should be initialised to empty slice, not nil")
	}
}

func TestUIRoundTripsThroughSave(t *testing.T) {
	t.Setenv("APPDATA", t.TempDir())
	c1, _ := Load()
	c1.UI.Theme = "high-contrast"
	c1.UI.Zoom = 120
	c1.UI.RowsPerShelf = 4
	c1.UI.CardSize = "l"
	c1.UI.HiddenLibraries = []string{"abc:5", "def:7"}
	c1.UI.ShelfState = map[string]PageShelfState{
		"home": {
			GroupOrder:    []string{"dknzplex", "stargaze"},
			GroupCollapsed: map[string]bool{"stargaze": true},
			ShelfOrder:    map[string][]string{"stargaze": {"s-movies", "s-tv"}},
			ShelfPrefs:    map[string]ShelfPref{"s-movies": {Hidden: true, Collapsed: false}},
		},
	}
	if err := c1.Save(); err != nil {
		t.Fatal(err)
	}
	c2, _ := Load()
	if c2.UI.Theme != "high-contrast" {
		t.Errorf("theme round-trip: %q", c2.UI.Theme)
	}
	if c2.UI.Zoom != 120 {
		t.Errorf("zoom round-trip: %d", c2.UI.Zoom)
	}
	if len(c2.UI.HiddenLibraries) != 2 {
		t.Errorf("hidden libs: %+v", c2.UI.HiddenLibraries)
	}
	if c2.UI.ShelfState["home"].GroupOrder[0] != "dknzplex" {
		t.Errorf("group order: %+v", c2.UI.ShelfState["home"].GroupOrder)
	}
	if !c2.UI.ShelfState["home"].ShelfPrefs["s-movies"].Hidden {
		t.Errorf("shelf pref hidden lost: %+v", c2.UI.ShelfState["home"].ShelfPrefs)
	}
}
```

- [ ] **Step 2: Confirm tests fail**

```bash
go test ./internal/config/...
```
Expected: FAIL — `UI`, `PageShelfState`, `ShelfPref` undefined.

- [ ] **Step 3: Add UI struct to Config**

Modify `internal/config/config.go`. Add types above `Config`:
```go
// UIConfig holds every user-tunable UI preference. Persisted in config.json.
type UIConfig struct {
	Theme            string                     `json:"theme"`            // "pure-oled" | future themes
	Zoom             int                        `json:"zoom"`             // viewport zoom percentage, 80-150
	CardSize         string                     `json:"cardSize"`         // "s" | "m" | "l" | "xl"
	CardDensity      int                        `json:"cardDensity"`      // 0-100, grid gap slider
	RowsPerShelf     int                        `json:"rowsPerShelf"`     // 1-4
	FontSize         int                        `json:"fontSize"`         // base rem in px, default 14
	CardLayout       string                     `json:"cardLayout"`       // "poster" | "landscape"
	DefaultSort      string                     `json:"defaultSort"`      // library default sort, Session 3 shim
	DefaultViewMode  string                     `json:"defaultViewMode"`  // "shows" | "episodes" for TV libraries
	Kiosk            KioskConfig                `json:"kiosk"`
	Playback         PlaybackUIConfig           `json:"playback"`
	HiddenLibraries  []string                   `json:"hiddenLibraries"`  // "serverID:libraryKey" entries
	ShelfState       map[string]PageShelfState  `json:"shelfState"`       // keyed by page: "home", "recommended", "discover"
}

// KioskConfig — launched for real in Session 5; Session 3 just persists the toggles.
type KioskConfig struct {
	EnableOnStartup bool   `json:"enableOnStartup"`
	Browser         string `json:"browser"` // "edge" | "chrome" | "system"
}

// PlaybackUIConfig — persists the Pot Player override path (Session 4 reads it).
type PlaybackUIConfig struct {
	PotPlayerPath string `json:"potPlayerPath"`
}

// PageShelfState stores per-page shelf/group order + visibility + collapse.
type PageShelfState struct {
	GroupOrder     []string              `json:"groupOrder,omitempty"`     // order of groups on pages that have groups (Home)
	GroupCollapsed map[string]bool       `json:"groupCollapsed,omitempty"` // group ID → collapsed?
	ShelfOrder     map[string][]string   `json:"shelfOrder,omitempty"`     // group ID (or "" for ungrouped) → ordered shelf IDs
	ShelfPrefs     map[string]ShelfPref  `json:"shelfPrefs,omitempty"`     // shelf ID → pref
}

// ShelfPref is per-shelf visibility/collapse state.
type ShelfPref struct {
	Hidden    bool `json:"hidden,omitempty"`
	Collapsed bool `json:"collapsed,omitempty"`
}
```

Add `UI UIConfig `json:"ui"`` to the `Config` struct:
```go
type Config struct {
	ClientIdentifier string     `json:"clientIdentifier"`
	OMDBKey          string     `json:"omdbKey,omitempty"`
	Plex             PlexConfig `json:"plex"`
	UI               UIConfig   `json:"ui"`
}
```

Add a mirror `wireUIConfig` for the JSON round-trip (nothing to encrypt in UI, so it's just copy-through):
```go
type wireConfig struct {
	ClientIdentifier string         `json:"clientIdentifier"`
	OMDBKey          string         `json:"omdbKey,omitempty"`
	Plex             wirePlexConfig `json:"plex"`
	UI               UIConfig       `json:"ui"`
}
```

- [ ] **Step 4: Populate defaults + wire round-trip**

In `Load()`, after the existing defaults block, add UI defaults:
```go
	// Apply UI defaults for missing fields. Preserves any persisted values.
	if c.UI.Theme == "" {
		c.UI.Theme = "pure-oled"
	}
	if c.UI.Zoom == 0 {
		c.UI.Zoom = 100
	}
	if c.UI.CardSize == "" {
		c.UI.CardSize = "m"
	}
	if c.UI.CardDensity == 0 {
		c.UI.CardDensity = 50
	}
	if c.UI.RowsPerShelf == 0 {
		c.UI.RowsPerShelf = 3
	}
	if c.UI.FontSize == 0 {
		c.UI.FontSize = 14
	}
	if c.UI.CardLayout == "" {
		c.UI.CardLayout = "poster"
	}
	if c.UI.DefaultSort == "" {
		c.UI.DefaultSort = "addedAt:desc"
	}
	if c.UI.DefaultViewMode == "" {
		c.UI.DefaultViewMode = "episodes"
	}
	if c.UI.Kiosk.Browser == "" {
		c.UI.Kiosk.Browser = "edge"
	}
	if c.UI.ShelfState == nil {
		c.UI.ShelfState = map[string]PageShelfState{}
	}
	if c.UI.HiddenLibraries == nil {
		c.UI.HiddenLibraries = []string{}
	}
```

Also propagate the Load reads the UI field from wireConfig:
```go
	c := &Config{
		ClientIdentifier: w.ClientIdentifier,
		OMDBKey:          w.OMDBKey,
		UI:               w.UI,
	}
```

And `Save()` populates `w.UI = c.UI` before marshal.

Modify `newDefault()` to also apply UI defaults so fresh-load path matches:
```go
func newDefault() *Config {
	c := &Config{ClientIdentifier: uuid.NewString()}
	c.UI = UIConfig{
		Theme:           "pure-oled",
		Zoom:            100,
		CardSize:        "m",
		CardDensity:     50,
		RowsPerShelf:    3,
		FontSize:        14,
		CardLayout:      "poster",
		DefaultSort:     "addedAt:desc",
		DefaultViewMode: "episodes",
		Kiosk:           KioskConfig{Browser: "edge"},
		ShelfState:      map[string]PageShelfState{},
		HiddenLibraries: []string{},
	}
	return c
}
```

- [ ] **Step 5: Verify all config tests pass**

```bash
go test ./internal/config/...
```
Expected: PASS — both new tests plus all existing.

- [ ] **Step 6: Commit**

```bash
git add internal/config/
git commit -m "feat(config): UIConfig persists theme/zoom/shelf-state/hidden-libs/kiosk/playback"
```

---

## Task 2: `/api/settings` GET + PUT

**Files:**
- Create: `internal/server/api_settings.go`
- Create: `internal/server/api_settings_test.go`

**Context:** SPA reads settings on boot, writes them on every change. Full payload replacement each PUT — no PATCH semantics. Server merges into existing config and re-saves.

- [ ] **Step 1: Write failing test**

Create `internal/server/api_settings_test.go`:
```go
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
```

- [ ] **Step 2: Confirm fail**

```bash
go test ./internal/server/...
```
Expected: FAIL — route not registered.

- [ ] **Step 3: Implement the handler**

Create `internal/server/api_settings.go`:
```go
package server

import (
	"encoding/json"
	"net/http"
)

// handleSettings dispatches GET (returns UI config) or PUT (replaces it).
// Non-UI config fields (tokens, servers, client identifier) are never exposed.
func (s *Server) handleSettings(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case "GET":
		writeJSON(w, s.cfg.UI)
	case "PUT":
		var incoming struct {
			Theme           *string  `json:"theme"`
			Zoom            *int     `json:"zoom"`
			CardSize        *string  `json:"cardSize"`
			CardDensity     *int     `json:"cardDensity"`
			RowsPerShelf    *int     `json:"rowsPerShelf"`
			FontSize        *int     `json:"fontSize"`
			CardLayout      *string  `json:"cardLayout"`
			DefaultSort     *string  `json:"defaultSort"`
			DefaultViewMode *string  `json:"defaultViewMode"`
			Kiosk           *struct {
				EnableOnStartup *bool   `json:"enableOnStartup"`
				Browser         *string `json:"browser"`
			} `json:"kiosk"`
			Playback *struct {
				PotPlayerPath *string `json:"potPlayerPath"`
			} `json:"playback"`
			HiddenLibraries *[]string                          `json:"hiddenLibraries"`
			ShelfState      *map[string]any                    `json:"shelfState"`
		}
		if err := json.NewDecoder(r.Body).Decode(&incoming); err != nil {
			writeError(w, http.StatusBadRequest, "invalid JSON: "+err.Error())
			return
		}

		// Apply any fields provided. Untouched fields keep their current values.
		if incoming.Theme != nil {
			s.cfg.UI.Theme = *incoming.Theme
		}
		if incoming.Zoom != nil {
			s.cfg.UI.Zoom = *incoming.Zoom
		}
		if incoming.CardSize != nil {
			s.cfg.UI.CardSize = *incoming.CardSize
		}
		if incoming.CardDensity != nil {
			s.cfg.UI.CardDensity = *incoming.CardDensity
		}
		if incoming.RowsPerShelf != nil {
			s.cfg.UI.RowsPerShelf = *incoming.RowsPerShelf
		}
		if incoming.FontSize != nil {
			s.cfg.UI.FontSize = *incoming.FontSize
		}
		if incoming.CardLayout != nil {
			s.cfg.UI.CardLayout = *incoming.CardLayout
		}
		if incoming.DefaultSort != nil {
			s.cfg.UI.DefaultSort = *incoming.DefaultSort
		}
		if incoming.DefaultViewMode != nil {
			s.cfg.UI.DefaultViewMode = *incoming.DefaultViewMode
		}
		if incoming.Kiosk != nil {
			if incoming.Kiosk.EnableOnStartup != nil {
				s.cfg.UI.Kiosk.EnableOnStartup = *incoming.Kiosk.EnableOnStartup
			}
			if incoming.Kiosk.Browser != nil {
				s.cfg.UI.Kiosk.Browser = *incoming.Kiosk.Browser
			}
		}
		if incoming.Playback != nil && incoming.Playback.PotPlayerPath != nil {
			s.cfg.UI.Playback.PotPlayerPath = *incoming.Playback.PotPlayerPath
		}
		if incoming.HiddenLibraries != nil {
			s.cfg.UI.HiddenLibraries = *incoming.HiddenLibraries
		}
		if incoming.ShelfState != nil {
			// Re-marshal + unmarshal to coerce map[string]any → typed PageShelfState.
			raw, _ := json.Marshal(*incoming.ShelfState)
			_ = json.Unmarshal(raw, &s.cfg.UI.ShelfState)
		}

		if err := s.cfg.Save(); err != nil {
			writeError(w, http.StatusInternalServerError, "save: "+err.Error())
			return
		}
		writeJSON(w, s.cfg.UI)
	default:
		writeError(w, http.StatusMethodNotAllowed, "GET or PUT only")
	}
}
```

Register in `registerRoutes`:
```go
	s.mux.HandleFunc("/api/settings", s.handleSettings)
```

- [ ] **Step 4: Verify tests pass**

```bash
go test ./internal/server/...
```
Expected: PASS for all three settings tests plus existing suite.

- [ ] **Step 5: Commit**

```bash
git add internal/server/api_settings.go internal/server/api_settings_test.go internal/server/server.go
git commit -m "feat(server): /api/settings GET/PUT persists UI config to disk"
```

---

## Task 3: `/api/cache/size` + `/api/cache/clear`

**Files:**
- Create: `internal/server/api_cache.go`
- Create: `internal/server/api_cache_test.go`

**Context:** Spec §13.6 Settings Data & Cache. Needs a size readout ("Image cache: 412 MB") and clear buttons. Scope param: `images` (our disk image cache), `omdb` (ready for Session 5's OMDB cache), or `all`.

- [ ] **Step 1: Write failing test**

Create `internal/server/api_cache_test.go`:
```go
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
```

- [ ] **Step 2: Confirm fail**

- [ ] **Step 3: Implement**

Create `internal/server/api_cache.go`:
```go
package server

import (
	"net/http"
	"os"
	"path/filepath"

	"lumen/internal/config"
)

// handleCacheSize walks cache subdirectories and returns per-scope byte totals.
func (s *Server) handleCacheSize(w http.ResponseWriter, r *http.Request) {
	images := dirSize(filepath.Join(config.CacheDir(), "images"))
	omdb := dirSize(filepath.Join(config.CacheDir(), "omdb"))
	writeJSON(w, map[string]int64{
		"images": images,
		"omdb":   omdb,
		"total":  images + omdb,
	})
}

// handleCacheClear wipes one of the named cache subdirectories.
// scope: "images" | "omdb" | "all"
func (s *Server) handleCacheClear(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" {
		writeError(w, http.StatusMethodNotAllowed, "POST required")
		return
	}
	scope := r.URL.Query().Get("scope")
	switch scope {
	case "images":
		clearDir(filepath.Join(config.CacheDir(), "images"))
	case "omdb":
		clearDir(filepath.Join(config.CacheDir(), "omdb"))
	case "all":
		clearDir(filepath.Join(config.CacheDir(), "images"))
		clearDir(filepath.Join(config.CacheDir(), "omdb"))
	default:
		writeError(w, http.StatusBadRequest, `scope must be one of "images", "omdb", "all"`)
		return
	}
	writeJSON(w, map[string]string{"status": "cleared", "scope": scope})
}

func dirSize(dir string) int64 {
	var total int64
	_ = filepath.Walk(dir, func(_ string, info os.FileInfo, err error) error {
		if err != nil || info == nil || info.IsDir() {
			return nil
		}
		total += info.Size()
		return nil
	})
	return total
}

func clearDir(dir string) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return
	}
	for _, e := range entries {
		_ = os.RemoveAll(filepath.Join(dir, e.Name()))
	}
}
```

Register in `registerRoutes`:
```go
	s.mux.HandleFunc("/api/cache/size", s.handleCacheSize)
	s.mux.HandleFunc("/api/cache/clear", s.handleCacheClear)
```

- [ ] **Step 4: Verify + commit**

```bash
go test ./internal/server/...
git add internal/server/api_cache.go internal/server/api_cache_test.go internal/server/server.go
git commit -m "feat(server): /api/cache/size + /api/cache/clear for Settings Data & Cache section"
```

---

## Task 4: `/api/user` — Plex account info for Settings header

**Files:**
- Create: `internal/server/api_user.go`
- Modify: `internal/plex/auth.go` (extract account info method)

**Context:** Settings → Accounts & Servers shows "Logged in as <username>". Requires calling `GET https://plex.tv/api/v2/user` with the account token.

- [ ] **Step 1: Add plex method**

Append to `internal/plex/auth.go`:
```go
// AccountInfo holds a subset of plex.tv/api/v2/user response.
type AccountInfo struct {
	ID       int    `json:"id"`
	Username string `json:"username"`
	Email    string `json:"email"`
	Thumb    string `json:"thumb"`
}

// GetAccount fetches the current user's Plex account info for display.
func (c *Client) GetAccount(accountToken string) (AccountInfo, error) {
	u := c.plexTVBase + "/api/v2/user"
	req, err := c.NewRequest("GET", u, nil)
	if err != nil {
		return AccountInfo{}, err
	}
	c.SetToken(req, accountToken)
	resp, err := c.Do(req)
	if err != nil {
		return AccountInfo{}, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return AccountInfo{}, fmt.Errorf("get account: status %d", resp.StatusCode)
	}
	var info AccountInfo
	if err := json.NewDecoder(resp.Body).Decode(&info); err != nil {
		return AccountInfo{}, err
	}
	return info, nil
}
```

- [ ] **Step 2: Implement server handler**

Create `internal/server/api_user.go`:
```go
package server

import "net/http"

// handleUser returns the current Plex account info. Empty response when no token.
func (s *Server) handleUser(w http.ResponseWriter, r *http.Request) {
	if s.cfg.Plex.AccountToken == "" {
		writeError(w, http.StatusUnauthorized, "no account token — run lumen auth")
		return
	}
	if s.plex == nil {
		writeError(w, http.StatusInternalServerError, "plex client not initialised")
		return
	}
	info, err := s.plex.GetAccount(s.cfg.Plex.AccountToken)
	if err != nil {
		writeError(w, http.StatusBadGateway, err.Error())
		return
	}
	writeJSON(w, info)
}
```

Register in `registerRoutes`:
```go
	s.mux.HandleFunc("/api/user", s.handleUser)
```

- [ ] **Step 3: Build + commit (no unit test — live network dependency)**

```bash
go build ./...
git add internal/plex/auth.go internal/server/api_user.go internal/server/server.go
git commit -m "feat(server): /api/user exposes Plex account info for Settings header"
```

---

## Task 5: Install `@thisbeyond/solid-dnd`

**Files:**
- Modify: `web/package.json`

- [ ] **Step 1: Install**

```bash
cd web && npm install @thisbeyond/solid-dnd
```

- [ ] **Step 2: Verify build still works**

```bash
npm run build
```
Expected: succeeds.

- [ ] **Step 3: Commit**

```bash
cd ..
git add web/package.json web/package-lock.json
git commit -m "feat(web): add @thisbeyond/solid-dnd for shelf and group reorder"
```

---

## Task 6: Theme registry + `applyTheme` helper

**Files:**
- Create: `web/src/themes/index.ts`
- Create: `web/src/themes/pure-oled.ts`

**Context:** A theme is a JS object describing CSS custom-property values. `applyTheme(theme)` writes those values to `:root`. A registry maps theme IDs to definitions. Session 3 ships only Pure OLED but the structure is ready for Byron's future additions — adding a theme is just dropping a file into `web/src/themes/` and registering it.

- [ ] **Step 1: Theme shape + registry**

Create `web/src/themes/index.ts`:
```typescript
export interface ThemeTokens {
  bg: string;
  bgMenu: string;
  bgElevated: string;
  bgInverse: string;
  text: string;
  textMuted: string;
  textInverse: string;
  menuIcon: string;
  border: string;
  borderSoft: string;
  stroke: string;
  statusOnline: string;
  statusOffline: string;
  shadow: string;
}

export interface Theme {
  id: string;
  name: string;      // display label for the picker
  tokens: ThemeTokens;
}

import { pureOled } from "./pure-oled";

// Registry — add new themes here. The picker reads this list.
export const THEMES: Theme[] = [pureOled];

export function themeByID(id: string): Theme {
  return THEMES.find((t) => t.id === id) ?? pureOled;
}

/**
 * Applies a theme's tokens to :root as CSS custom properties.
 * Call on boot (from the loaded settings) and again on every theme change.
 */
export function applyTheme(theme: Theme) {
  const root = document.documentElement;
  const t = theme.tokens;
  root.style.setProperty("--bg", t.bg);
  root.style.setProperty("--bg-menu", t.bgMenu);
  root.style.setProperty("--bg-elevated", t.bgElevated);
  root.style.setProperty("--bg-inverse", t.bgInverse);
  root.style.setProperty("--text", t.text);
  root.style.setProperty("--text-muted", t.textMuted);
  root.style.setProperty("--text-inverse", t.textInverse);
  root.style.setProperty("--menu-icon", t.menuIcon);
  root.style.setProperty("--border", t.border);
  root.style.setProperty("--border-soft", t.borderSoft);
  root.style.setProperty("--stroke", t.stroke);
  root.style.setProperty("--status-online", t.statusOnline);
  root.style.setProperty("--status-offline", t.statusOffline);
  root.style.setProperty("--shadow", t.shadow);
}
```

- [ ] **Step 2: Pure OLED definition (mirror of current theme.css)**

Create `web/src/themes/pure-oled.ts`:
```typescript
import type { Theme } from "./index";

export const pureOled: Theme = {
  id: "pure-oled",
  name: "Pure OLED",
  tokens: {
    bg:             "#000000",
    bgMenu:         "#1a1a1a",
    bgElevated:     "#0f1729",
    bgInverse:      "#ffffff",
    text:           "#ffffff",
    textMuted:      "#9ca3af",
    textInverse:    "#000000",
    menuIcon:       "#d1d5db",
    border:         "#262626",
    borderSoft:     "rgba(255, 255, 255, 0.08)",
    stroke:         "#ffffff",
    statusOnline:   "#4caf50",
    statusOffline:  "#6b7280",
    shadow:         "0 2px 14px rgba(0, 0, 0, 0.7)",
  },
};
```

- [ ] **Step 3: Build + commit**

```bash
cd web && npm run build
cd ..
git add web/src/themes/
git commit -m "feat(web): theme registry with applyTheme helper; Pure OLED wired"
```

---

## Task 7: Settings store + wiring to /api/settings

**Files:**
- Create: `web/src/state/settings.ts`
- Create: `web/src/api/settings.ts`

**Context:** Central Solid store that holds the in-memory settings. On boot: fetches `/api/settings` and applies theme. On any change: writes back via PUT with a debounced flush so rapid slider drags don't spam the server.

- [ ] **Step 1: API client**

Create `web/src/api/settings.ts`:
```typescript
export interface UISettings {
  theme: string;
  zoom: number;
  cardSize: "s" | "m" | "l" | "xl";
  cardDensity: number;
  rowsPerShelf: 1 | 2 | 3 | 4;
  fontSize: number;
  cardLayout: "poster" | "landscape";
  defaultSort: string;
  defaultViewMode: "shows" | "episodes" | "";
  kiosk: { enableOnStartup: boolean; browser: "edge" | "chrome" | "system" };
  playback: { potPlayerPath: string };
  hiddenLibraries: string[];
  shelfState: Record<string, PageShelfState>;
}

export interface PageShelfState {
  groupOrder?: string[];
  groupCollapsed?: Record<string, boolean>;
  shelfOrder?: Record<string, string[]>;
  shelfPrefs?: Record<string, { hidden?: boolean; collapsed?: boolean }>;
}

export const settingsAPI = {
  get: async (): Promise<UISettings> => {
    const res = await fetch("/api/settings");
    if (!res.ok) throw new Error(`GET settings: ${res.status}`);
    return res.json();
  },
  put: async (patch: Partial<UISettings>): Promise<UISettings> => {
    const res = await fetch("/api/settings", {
      method: "PUT",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify(patch),
    });
    if (!res.ok) throw new Error(`PUT settings: ${res.status} ${await res.text()}`);
    return res.json();
  },
};
```

- [ ] **Step 2: Solid store wrapping the settings**

Create `web/src/state/settings.ts`:
```typescript
import { createRoot, createSignal } from "solid-js";
import { settingsAPI, UISettings } from "../api/settings";
import { applyTheme, themeByID } from "../themes";

// Debounce helper for PUT coalescing.
function debounce<T extends (...args: any[]) => any>(fn: T, ms: number): T {
  let t: number | undefined;
  return ((...args: Parameters<T>) => {
    if (t !== undefined) clearTimeout(t);
    t = setTimeout(() => fn(...args), ms) as unknown as number;
  }) as T;
}

function createSettingsStore() {
  const [settings, setSettings] = createSignal<UISettings | null>(null);
  const [loaded, setLoaded] = createSignal(false);

  // Initial fetch. Called once at app boot from main.tsx.
  async function load() {
    const s = await settingsAPI.get();
    setSettings(s);
    setLoaded(true);
    applyTheme(themeByID(s.theme));
    document.documentElement.style.setProperty("zoom", String(s.zoom / 100));
  }

  const flushDebounced = debounce(async (patch: Partial<UISettings>) => {
    try {
      const updated = await settingsAPI.put(patch);
      setSettings(updated);
    } catch (e) {
      console.error("settings PUT failed:", e);
    }
  }, 300);

  // Patch mutates the store locally (optimistic) AND schedules a server write.
  function patch(update: Partial<UISettings>) {
    const current = settings();
    if (!current) return;
    const next = { ...current, ...update };
    setSettings(next);

    // Side-effects that should happen immediately (not wait for debounce):
    if (update.theme && update.theme !== current.theme) {
      applyTheme(themeByID(update.theme));
    }
    if (update.zoom !== undefined && update.zoom !== current.zoom) {
      document.documentElement.style.setProperty("zoom", String(update.zoom / 100));
    }

    flushDebounced(update);
  }

  return { settings, loaded, load, patch };
}

export const store = createRoot(createSettingsStore);
```

- [ ] **Step 3: Wire store into main.tsx boot**

Modify `web/src/main.tsx` — import and call `store.load()` before rendering:
```typescript
import { render } from "solid-js/web";
import { Router, Route } from "@solidjs/router";
import App from "./App";
import Home from "./pages/Home";
import Library from "./pages/Library";
import ItemDetail from "./pages/ItemDetail";
import Placeholder from "./pages/Placeholder";
import { store as settingsStore } from "./state/settings";
import "./theme.css";

// Fire-and-forget — settings load populates the store and applies theme.
// The UI renders defaults until the load resolves; no blocking splash.
settingsStore.load().catch((e) => console.error("initial settings load failed:", e));

render(() => (
  <Router root={App}>
    <Route path="/" component={Home} />
    <Route path="/library/:serverID/:libraryID" component={Library} />
    <Route path="/item/:serverID/:ratingKey" component={ItemDetail} />
    <Route path="/watchlist"   component={() => <Placeholder name="Watchlist"   session="Session 5" />} />
    <Route path="/recommended" component={() => <Placeholder name="Recommended" session="Session 5" />} />
    <Route path="/discover"    component={() => <Placeholder name="Discover"    session="Session 5" />} />
    <Route path="/settings"    component={() => <Placeholder name="Settings"    session="opens modal instead" />} />
  </Router>
), document.getElementById("root")!);
```

- [ ] **Step 4: Build + commit**

```bash
cd web && npm run build
cd ..
git add web/src/api/settings.ts web/src/state/settings.ts web/src/main.tsx
git commit -m "feat(web): settings store + /api/settings client, applies theme on boot"
```

---

## Task 8: Settings modal shell

**Files:**
- Create: `web/src/components/Settings/SettingsModal.tsx`
- Create: `web/src/components/Settings/SettingsModal.css`
- Create: `web/src/components/Settings/Section.tsx`
- Create: `web/src/components/Settings/Section.css`

**Context:** Overlay modal that fills most of the viewport. Left rail lists the 6 sections; right pane shows the active section's contents. Backdrop click + Escape key close the modal.

- [ ] **Step 1: Implement Section wrapper**

Create `web/src/components/Settings/Section.tsx`:
```tsx
import { JSX, ParentProps } from "solid-js";
import "./Section.css";

export interface SectionProps {
  title: string;
  description?: string;
  children?: JSX.Element;
}

export default function Section(props: ParentProps<SectionProps>) {
  return (
    <section class="settings-section">
      <header class="settings-section-header">
        <h2>{props.title}</h2>
        {props.description && <p>{props.description}</p>}
      </header>
      <div class="settings-section-body">{props.children}</div>
    </section>
  );
}
```

Create `web/src/components/Settings/Section.css`:
```css
.settings-section {
  padding: 24px 32px;
  max-width: 720px;
}

.settings-section-header {
  border-bottom: 1px solid var(--border);
  padding-bottom: 12px;
  margin-bottom: 24px;
}

.settings-section-header h2 {
  margin: 0;
  font-size: 20px;
  color: var(--text);
  font-weight: 600;
}

.settings-section-header p {
  margin: 6px 0 0;
  color: var(--text-muted);
  font-size: 13px;
}

.settings-section-body {
  display: flex;
  flex-direction: column;
  gap: 20px;
}

/* Common form-control layout reused across sections. */
.settings-row {
  display: grid;
  grid-template-columns: 220px 1fr;
  gap: 16px;
  align-items: center;
}

.settings-row > label:first-child {
  color: var(--text-muted);
  font-size: 13px;
}

.settings-control select,
.settings-control input[type="text"],
.settings-control input[type="number"],
.settings-control input[type="password"] {
  width: 100%;
  background: var(--bg);
  color: var(--text);
  border: 1px solid var(--border);
  border-radius: var(--radius-sm);
  padding: 8px 10px;
  font-size: 13px;
}

.settings-control input[type="range"] {
  width: 100%;
  accent-color: var(--text);
}

.settings-btn {
  background: var(--bg-elevated);
  color: var(--text);
  border: 1px solid var(--border-soft);
  border-radius: var(--radius-pill);
  padding: 8px 16px;
  font-size: 13px;
  cursor: pointer;
}

.settings-btn:hover:not(:disabled) {
  border-color: var(--stroke);
}

.settings-btn:disabled {
  opacity: 0.5;
  cursor: not-allowed;
}

.settings-btn-primary {
  background: var(--bg-inverse);
  color: var(--text-inverse);
  border-color: var(--bg-inverse);
  font-weight: 600;
}

.settings-btn-primary:hover:not(:disabled) {
  opacity: 0.92;
}

.settings-btn-danger:hover:not(:disabled) {
  border-color: #c83c3c;
  color: #f07878;
}
```

- [ ] **Step 2: Implement Modal shell**

Create `web/src/components/Settings/SettingsModal.tsx`:
```tsx
import { createSignal, For, onCleanup, onMount, Show } from "solid-js";
import Appearance from "./Appearance";
import KioskShortcuts from "./KioskShortcuts";
import AccountsServers from "./AccountsServers";
import Playback from "./Playback";
import DataCache from "./DataCache";
import About from "./About";
import "./SettingsModal.css";

const SECTIONS = [
  { id: "appearance",    label: "Appearance",         component: Appearance },
  { id: "kiosk",         label: "Kiosk & Shortcuts",  component: KioskShortcuts },
  { id: "accounts",      label: "Accounts & Servers", component: AccountsServers },
  { id: "playback",      label: "Playback",           component: Playback },
  { id: "cache",         label: "Data & Cache",       component: DataCache },
  { id: "about",         label: "About",              component: About },
] as const;

export default function SettingsModal(props: { open: boolean; onClose: () => void }) {
  const [activeID, setActiveID] = createSignal<string>(SECTIONS[0].id);

  function onKeyDown(e: KeyboardEvent) {
    if (e.key === "Escape") props.onClose();
  }

  onMount(() => {
    document.addEventListener("keydown", onKeyDown);
    onCleanup(() => document.removeEventListener("keydown", onKeyDown));
  });

  const activeSection = () => SECTIONS.find((s) => s.id === activeID())!;

  return (
    <Show when={props.open}>
      <div class="settings-backdrop" onClick={props.onClose} role="presentation">
        <div class="settings-modal" onClick={(e) => e.stopPropagation()} role="dialog" aria-label="Settings">
          <aside class="settings-nav">
            <header class="settings-nav-header">
              <span class="settings-nav-title">Settings</span>
              <button class="settings-close-btn" onClick={props.onClose} aria-label="Close settings">✕</button>
            </header>
            <nav>
              <For each={SECTIONS}>
                {(s) => (
                  <button
                    class="settings-nav-item"
                    classList={{ active: activeID() === s.id }}
                    onClick={() => setActiveID(s.id)}
                  >
                    {s.label}
                  </button>
                )}
              </For>
            </nav>
          </aside>
          <main class="settings-detail">
            {activeSection().component({})}
          </main>
        </div>
      </div>
    </Show>
  );
}
```

Create `web/src/components/Settings/SettingsModal.css`:
```css
.settings-backdrop {
  position: fixed;
  inset: 0;
  background: rgba(0, 0, 0, 0.65);
  display: grid;
  place-items: center;
  z-index: 100;
  backdrop-filter: blur(4px);
}

.settings-modal {
  display: grid;
  grid-template-columns: 220px 1fr;
  width: min(960px, 92vw);
  height: min(720px, 90vh);
  background: var(--bg);
  border-radius: var(--radius-lg);
  box-shadow: var(--shadow);
  overflow: hidden;
  border: 1px solid var(--border);
}

.settings-nav {
  background: var(--bg-menu);
  border-right: 1px solid var(--border);
  display: flex;
  flex-direction: column;
}

.settings-nav-header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  padding: 16px 18px;
  border-bottom: 1px solid var(--border);
}

.settings-nav-title {
  font-weight: 600;
  color: var(--text);
  font-size: 15px;
  letter-spacing: 0.3px;
}

.settings-close-btn {
  width: 28px;
  height: 28px;
  display: grid;
  place-items: center;
  background: transparent;
  color: var(--text-muted);
  border: none;
  border-radius: 50%;
  cursor: pointer;
  font-size: 14px;
}

.settings-close-btn:hover {
  background: rgba(255, 255, 255, 0.08);
  color: var(--text);
}

.settings-nav nav {
  padding: 8px;
  display: flex;
  flex-direction: column;
  gap: 2px;
}

.settings-nav-item {
  text-align: left;
  padding: 8px 12px;
  background: transparent;
  color: var(--menu-icon);
  border: none;
  border-radius: var(--radius-sm);
  cursor: pointer;
  font-size: 13px;
}

.settings-nav-item:hover {
  background: var(--bg-elevated);
  color: var(--text);
}

.settings-nav-item.active {
  background: var(--bg-elevated);
  color: var(--text);
}

.settings-detail {
  overflow-y: auto;
}
```

- [ ] **Step 3: Stub placeholder section components**

Create 6 files — each an identical stub so the modal compiles before the real implementations land:

`web/src/components/Settings/Appearance.tsx`:
```tsx
import Section from "./Section";
export default function Appearance() {
  return <Section title="Appearance" description="Theme, card sizing, layout.">(Task 9 populates)</Section>;
}
```

Repeat the same stub pattern (just rename title + file) for:
- `KioskShortcuts.tsx` ("Kiosk & Shortcuts", "Desktop shortcut, kiosk preferences.")
- `AccountsServers.tsx` ("Accounts & Servers", "Plex account and server overrides.")
- `Playback.tsx` ("Playback", "Pot Player configuration.")
- `DataCache.tsx` ("Data & Cache", "Cache size and clear controls.")
- `About.tsx` ("About", "Version, paths, repository.")

- [ ] **Step 4: Build + commit**

```bash
cd web && npm run build
cd ..
git add web/src/components/Settings/
git commit -m "feat(web): Settings modal shell with section nav and 6 placeholder sections"
```

---

## Task 9: Settings → Appearance section

**Files:**
- Modify: `web/src/components/Settings/Appearance.tsx`

**Context:** Spec §13.1 controls: theme picker, card size, card density slider, rows per shelf, font size, card layout. All values bound to the settings store; changes PUT to server (debounced).

- [ ] **Step 1: Implement Appearance**

Replace `web/src/components/Settings/Appearance.tsx`:
```tsx
import { For, Show } from "solid-js";
import Section from "./Section";
import { store } from "../../state/settings";
import { THEMES } from "../../themes";

export default function Appearance() {
  const s = store.settings;
  const patch = store.patch;

  return (
    <Section title="Appearance" description="Theme, card sizing, layout. Changes apply instantly.">
      <Show when={s()} fallback={<p>Loading…</p>}>
        {(settings) => (
          <>
            <div class="settings-row">
              <label for="theme">Theme</label>
              <div class="settings-control">
                <select id="theme" value={settings().theme} onChange={(e) => patch({ theme: e.currentTarget.value })}>
                  <For each={THEMES}>
                    {(t) => <option value={t.id}>{t.name}</option>}
                  </For>
                </select>
              </div>
            </div>

            <div class="settings-row">
              <label for="cardSize">Card size</label>
              <div class="settings-control">
                <select id="cardSize" value={settings().cardSize} onChange={(e) => patch({ cardSize: e.currentTarget.value as any })}>
                  <option value="s">Small</option>
                  <option value="m">Medium</option>
                  <option value="l">Large</option>
                  <option value="xl">Extra Large</option>
                </select>
              </div>
            </div>

            <div class="settings-row">
              <label for="rowsPerShelf">Rows per shelf</label>
              <div class="settings-control">
                <select id="rowsPerShelf" value={String(settings().rowsPerShelf)} onChange={(e) => patch({ rowsPerShelf: Number(e.currentTarget.value) as any })}>
                  <option value="1">1</option>
                  <option value="2">2</option>
                  <option value="3">3</option>
                  <option value="4">4</option>
                </select>
              </div>
            </div>

            <div class="settings-row">
              <label for="cardDensity">Card density</label>
              <div class="settings-control">
                <input
                  id="cardDensity"
                  type="range"
                  min="0" max="100"
                  value={settings().cardDensity}
                  onInput={(e) => patch({ cardDensity: Number(e.currentTarget.value) })}
                />
              </div>
            </div>

            <div class="settings-row">
              <label for="cardLayout">Card layout</label>
              <div class="settings-control">
                <select id="cardLayout" value={settings().cardLayout} onChange={(e) => patch({ cardLayout: e.currentTarget.value as any })}>
                  <option value="poster">Poster (2:3)</option>
                  <option value="landscape">Landscape (16:9)</option>
                </select>
              </div>
            </div>

            <div class="settings-row">
              <label for="fontSize">Font size</label>
              <div class="settings-control">
                <input
                  id="fontSize"
                  type="range"
                  min="11" max="18"
                  value={settings().fontSize}
                  onInput={(e) => patch({ fontSize: Number(e.currentTarget.value) })}
                />
              </div>
            </div>
          </>
        )}
      </Show>
    </Section>
  );
}
```

- [ ] **Step 2: Wire cardSize + rowsPerShelf + fontSize into CSS**

CardSize and fontSize should affect rendered cards. Add CSS variables driven by settings. Modify `web/src/theme.css` — append:
```css
/* Card sizing, driven by Settings → Appearance. Values are CSS vars so changes
   from the settings store propagate without component re-renders. */
:root {
  --card-width-s: 120px;
  --card-width-m: 160px;
  --card-width-l: 200px;
  --card-width-xl: 240px;

  --card-width: var(--card-width-m);
}

:root[data-card-size="s"]  { --card-width: var(--card-width-s); }
:root[data-card-size="m"]  { --card-width: var(--card-width-m); }
:root[data-card-size="l"]  { --card-width: var(--card-width-l); }
:root[data-card-size="xl"] { --card-width: var(--card-width-xl); }

:root {
  font-size: var(--font-size);
}
```

Modify `web/src/components/Card.css` — change `.card { width: 160px; }` to `width: var(--card-width);`.
Modify `web/src/components/Shelf.css` `grid-template-columns` to use `minmax(var(--card-width), 1fr)`.
Modify `web/src/pages/Library.css` `grid-template-columns` same update.

- [ ] **Step 3: Propagate settings changes to :root**

In `web/src/state/settings.ts`, extend the `applyTheme + zoom` block in `load()` and `patch()` to also set `data-card-size` and `--font-size`. Change the `load()` function's side-effects:
```typescript
  async function load() {
    const s = await settingsAPI.get();
    setSettings(s);
    setLoaded(true);
    applyTheme(themeByID(s.theme));
    applyRootDerived(s);
  }

  function applyRootDerived(s: UISettings) {
    const root = document.documentElement;
    root.style.setProperty("zoom", String(s.zoom / 100));
    root.setAttribute("data-card-size", s.cardSize);
    root.style.setProperty("--font-size", `${s.fontSize}px`);
  }
```

And in `patch`:
```typescript
  function patch(update: Partial<UISettings>) {
    const current = settings();
    if (!current) return;
    const next = { ...current, ...update };
    setSettings(next);

    if (update.theme && update.theme !== current.theme) {
      applyTheme(themeByID(update.theme));
    }
    applyRootDerived(next);

    flushDebounced(update);
  }
```

- [ ] **Step 4: Build + commit**

```bash
cd web && npm run build
cd ..
git add web/src/
git commit -m "feat(web): Settings Appearance section — theme/card-size/rows/density/layout/font"
```

---

## Task 10: Settings → Kiosk & Shortcuts section

**Files:**
- Modify: `web/src/components/Settings/KioskShortcuts.tsx`

**Context:** Spec §13.3 renamed per Byron: autostart removed, desktop shortcut added. Three controls: kiosk-on-startup toggle (writes config; Session 5 wires launch), kiosk browser preference, Create Desktop Shortcut button.

- [ ] **Step 1: Implement KioskShortcuts**

Replace `web/src/components/Settings/KioskShortcuts.tsx`:
```tsx
import { createSignal, Show } from "solid-js";
import Section from "./Section";
import { store } from "../../state/settings";

export default function KioskShortcuts() {
  const s = store.settings;
  const patch = store.patch;
  const [shortcutStatus, setShortcutStatus] = createSignal<string>("");

  async function createShortcut() {
    setShortcutStatus("Creating…");
    try {
      const res = await fetch("/api/shortcut", { method: "POST" });
      if (!res.ok) throw new Error(`${res.status} ${await res.text()}`);
      const body = await res.json();
      setShortcutStatus(`Created at ${body.path}`);
    } catch (e) {
      setShortcutStatus(`Failed: ${(e as Error).message}`);
    }
  }

  return (
    <Section
      title="Kiosk & Shortcuts"
      description="Launch behaviour. Kiosk mode actually starts Lumen in Session 5; these toggles save the preference."
    >
      <Show when={s()} fallback={<p>Loading…</p>}>
        {(settings) => (
          <>
            <div class="settings-row">
              <label for="kioskOnStart">Launch in kiosk mode on startup</label>
              <div class="settings-control">
                <input
                  id="kioskOnStart"
                  type="checkbox"
                  checked={settings().kiosk.enableOnStartup}
                  onChange={(e) =>
                    patch({ kiosk: { ...settings().kiosk, enableOnStartup: e.currentTarget.checked } })
                  }
                />
              </div>
            </div>

            <div class="settings-row">
              <label for="kioskBrowser">Kiosk browser</label>
              <div class="settings-control">
                <select
                  id="kioskBrowser"
                  value={settings().kiosk.browser}
                  onChange={(e) =>
                    patch({ kiosk: { ...settings().kiosk, browser: e.currentTarget.value as any } })
                  }
                >
                  <option value="edge">Microsoft Edge</option>
                  <option value="chrome">Google Chrome</option>
                  <option value="system">System default</option>
                </select>
              </div>
            </div>

            <div class="settings-row">
              <label>Desktop shortcut</label>
              <div class="settings-control">
                <button class="settings-btn" onClick={createShortcut}>Create Desktop Shortcut</button>
                {shortcutStatus() && (
                  <div style={{ "margin-top": "8px", "color": "var(--text-muted)", "font-size": "12px" }}>
                    {shortcutStatus()}
                  </div>
                )}
              </div>
            </div>
          </>
        )}
      </Show>
    </Section>
  );
}
```

- [ ] **Step 2: Build + commit**

```bash
cd web && npm run build
cd ..
git add web/src/components/Settings/KioskShortcuts.tsx
git commit -m "feat(web): Settings Kiosk & Shortcuts section"
```

---

## Task 11: Settings → Accounts & Servers section

**Files:**
- Modify: `web/src/components/Settings/AccountsServers.tsx`

**Context:** Spec §13.4. Shows Plex username (from /api/user), Re-authenticate button (stubbed — opens terminal reminder), server list with display-name overrides, Refresh Connections button, OMDB API key field.

- [ ] **Step 1: Implement AccountsServers**

Replace `web/src/components/Settings/AccountsServers.tsx`:
```tsx
import { createResource, createSignal, For, Show } from "solid-js";
import Section from "./Section";
import { api } from "../../api/client";
import { store } from "../../state/settings";

interface Account {
  username: string;
  email: string;
}

export default function AccountsServers() {
  const s = store.settings;
  const [account] = createResource<Account>(async () => {
    const res = await fetch("/api/user");
    if (!res.ok) throw new Error(`${res.status}`);
    return res.json();
  });
  const [servers, { refetch: refetchServers }] = createResource(() => api.servers());
  const [refreshing, setRefreshing] = createSignal(false);
  const [refreshStatus, setRefreshStatus] = createSignal("");
  const [omdbKey, setOmdbKey] = createSignal("");

  // Per-server display-name overrides get written via /api/servers/:id/rename
  // (new endpoint added in Task 14) OR a PUT to /api/settings if we stored overrides there.
  // Using a new dedicated endpoint keeps the display-name concept close to the server record.
  async function renameServer(machineID: string, newName: string) {
    const res = await fetch(`/api/servers/${encodeURIComponent(machineID)}/rename`, {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({ displayName: newName }),
    });
    if (!res.ok) throw new Error(`${res.status} ${await res.text()}`);
    refetchServers();
  }

  async function refreshConnections() {
    setRefreshing(true);
    setRefreshStatus("Re-discovering…");
    try {
      const res = await fetch("/api/servers/refresh", { method: "POST" });
      if (!res.ok) throw new Error(`${res.status}`);
      refetchServers();
      setRefreshStatus("Done.");
    } catch (e) {
      setRefreshStatus(`Failed: ${(e as Error).message}`);
    } finally {
      setRefreshing(false);
    }
  }

  // OMDB key lives at config root, not inside UI — so it goes via its own
  // endpoint rather than the settings store's patch(). Wired up in Task 14.
  function saveOMDB() {
    fetch("/api/settings/omdb", {
      method: "PUT",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({ key: omdbKey() }),
    });
  }

  return (
    <Section title="Accounts & Servers" description="Plex account, server names, and OMDB integration.">
      <Show when={s()}>
        <div class="settings-row">
          <label>Plex account</label>
          <div class="settings-control">
            <Show when={account()} fallback={<span>Loading…</span>}>
              {(a) => (
                <div style={{ "display": "flex", "gap": "12px", "align-items": "center" }}>
                  <strong>{(a() as Account).username}</strong>
                  <span style={{ "color": "var(--text-muted)", "font-size": "12px" }}>{(a() as Account).email}</span>
                </div>
              )}
            </Show>
          </div>
        </div>

        <div class="settings-row">
          <label>Re-authenticate</label>
          <div class="settings-control">
            <button
              class="settings-btn"
              onClick={() => alert("To re-authenticate, run `lumen auth` in a terminal. UI flow lands in Session 5.")}
            >
              Re-authenticate
            </button>
          </div>
        </div>

        <div class="settings-row">
          <label>Servers</label>
          <div class="settings-control">
            <Show when={servers()}>
              {(srvs) => (
                <div style={{ "display": "flex", "flex-direction": "column", "gap": "8px" }}>
                  <For each={srvs() as any[]}>
                    {(srv) => (
                      <div style={{ "display": "grid", "grid-template-columns": "1fr 160px 80px", "gap": "8px", "align-items": "center" }}>
                        <input
                          type="text"
                          value={srv.displayName}
                          onChange={(e) => renameServer(srv.machineIdentifier, e.currentTarget.value).catch((err) => alert(err.message))}
                        />
                        <span style={{ "color": srv.status === "connected" ? "var(--status-online)" : "var(--status-offline)", "font-size": "12px" }}>
                          {srv.status}
                        </span>
                        <span style={{ "color": "var(--text-muted)", "font-size": "11px", "overflow": "hidden", "text-overflow": "ellipsis" }}>
                          {srv.machineIdentifier.slice(0, 8)}…
                        </span>
                      </div>
                    )}
                  </For>
                  <button class="settings-btn" disabled={refreshing()} onClick={refreshConnections}>
                    {refreshing() ? "Refreshing…" : "Refresh connections"}
                  </button>
                  {refreshStatus() && (
                    <div style={{ "color": "var(--text-muted)", "font-size": "12px" }}>{refreshStatus()}</div>
                  )}
                </div>
              )}
            </Show>
          </div>
        </div>

        <div class="settings-row">
          <label for="omdbKey">OMDB API key</label>
          <div class="settings-control">
            <input
              id="omdbKey"
              type="password"
              placeholder="Paste key to enable IMDB ratings (Session 5)"
              value={omdbKey()}
              onInput={(e) => setOmdbKey(e.currentTarget.value)}
              onBlur={saveOMDB}
            />
            <div style={{ "margin-top": "4px", "font-size": "11px" }}>
              <a href="https://www.omdbapi.com/apikey.aspx" target="_blank" rel="noreferrer" style={{ "color": "var(--text-muted)" }}>
                Get a free key →
              </a>
            </div>
          </div>
        </div>
      </Show>
    </Section>
  );
}
```

- [ ] **Step 2: Commit** (backend endpoints added in Task 14)

```bash
cd web && npm run build
cd ..
git add web/src/components/Settings/AccountsServers.tsx
git commit -m "feat(web): Settings Accounts & Servers section — backend wiring in Task 14"
```

---

## Task 12: Settings → Playback section

**Files:**
- Modify: `web/src/components/Settings/Playback.tsx`

**Context:** Spec §13.5. Pot Player path override field (auto-detection is Session 4's job), plus read-only display of direct-play timeout (10 s, fixed by policy).

- [ ] **Step 1: Implement Playback**

Replace `web/src/components/Settings/Playback.tsx`:
```tsx
import { Show } from "solid-js";
import Section from "./Section";
import { store } from "../../state/settings";

export default function Playback() {
  const s = store.settings;
  const patch = store.patch;

  return (
    <Section title="Playback" description="Pot Player integration. Playback itself lands in Session 4.">
      <Show when={s()} fallback={<p>Loading…</p>}>
        {(settings) => (
          <>
            <div class="settings-row">
              <label for="potPath">Pot Player path</label>
              <div class="settings-control">
                <input
                  id="potPath"
                  type="text"
                  placeholder="Leave blank to auto-detect (Session 4)"
                  value={settings().playback.potPlayerPath}
                  onChange={(e) =>
                    patch({ playback: { ...settings().playback, potPlayerPath: e.currentTarget.value } })
                  }
                />
              </div>
            </div>

            <div class="settings-row">
              <label>Direct-play timeout</label>
              <div class="settings-control">
                <span style={{ "color": "var(--text-muted)" }}>10 seconds (fixed policy)</span>
              </div>
            </div>
          </>
        )}
      </Show>
    </Section>
  );
}
```

- [ ] **Step 2: Commit**

```bash
cd web && npm run build
cd ..
git add web/src/components/Settings/Playback.tsx
git commit -m "feat(web): Settings Playback section"
```

---

## Task 13: Settings → Data & Cache section

**Files:**
- Modify: `web/src/components/Settings/DataCache.tsx`

**Context:** Spec §13.6. Shows current cache size per scope, three clear buttons.

- [ ] **Step 1: Implement DataCache**

Replace `web/src/components/Settings/DataCache.tsx`:
```tsx
import { createResource, createSignal, Show } from "solid-js";
import Section from "./Section";

interface CacheSize {
  images: number;
  omdb: number;
  total: number;
}

function formatBytes(n: number): string {
  if (n === 0) return "0 B";
  const units = ["B", "KB", "MB", "GB"];
  let u = 0;
  let v = n;
  while (v >= 1024 && u < units.length - 1) { v /= 1024; u++; }
  return `${v.toFixed(u === 0 ? 0 : 1)} ${units[u]}`;
}

export default function DataCache() {
  const [size, { refetch }] = createResource<CacheSize>(async () => {
    const res = await fetch("/api/cache/size");
    if (!res.ok) throw new Error(`${res.status}`);
    return res.json();
  });
  const [clearingScope, setClearingScope] = createSignal<string | null>(null);

  async function clear(scope: "images" | "omdb" | "all") {
    setClearingScope(scope);
    try {
      const res = await fetch(`/api/cache/clear?scope=${scope}`, { method: "POST" });
      if (!res.ok) throw new Error(`${res.status}`);
      refetch();
    } catch (e) {
      alert((e as Error).message);
    } finally {
      setClearingScope(null);
    }
  }

  return (
    <Section title="Data & Cache" description="Proxied images and OMDB metadata cache.">
      <Show when={size()} fallback={<p>Loading…</p>}>
        {(cs) => (
          <>
            <div class="settings-row">
              <label>Image cache</label>
              <div class="settings-control" style={{ "display": "flex", "gap": "12px", "align-items": "center" }}>
                <strong>{formatBytes((cs() as CacheSize).images)}</strong>
                <button class="settings-btn settings-btn-danger" disabled={clearingScope() === "images"} onClick={() => clear("images")}>
                  {clearingScope() === "images" ? "Clearing…" : "Clear"}
                </button>
              </div>
            </div>

            <div class="settings-row">
              <label>Metadata cache (OMDB)</label>
              <div class="settings-control" style={{ "display": "flex", "gap": "12px", "align-items": "center" }}>
                <strong>{formatBytes((cs() as CacheSize).omdb)}</strong>
                <button class="settings-btn settings-btn-danger" disabled={clearingScope() === "omdb"} onClick={() => clear("omdb")}>
                  {clearingScope() === "omdb" ? "Clearing…" : "Clear"}
                </button>
              </div>
            </div>

            <div class="settings-row">
              <label>All caches</label>
              <div class="settings-control" style={{ "display": "flex", "gap": "12px", "align-items": "center" }}>
                <strong>{formatBytes((cs() as CacheSize).total)}</strong>
                <button class="settings-btn settings-btn-danger" disabled={clearingScope() === "all"} onClick={() => clear("all")}>
                  {clearingScope() === "all" ? "Clearing…" : "Clear All"}
                </button>
              </div>
            </div>
          </>
        )}
      </Show>
    </Section>
  );
}
```

- [ ] **Step 2: Commit**

```bash
cd web && npm run build
cd ..
git add web/src/components/Settings/DataCache.tsx
git commit -m "feat(web): Settings Data & Cache section with size + clear per scope"
```

---

## Task 14: Backend endpoints for Accounts & Servers section

**Files:**
- Create: `internal/server/api_servers_rename.go`
- Create: `internal/server/api_servers_refresh.go`
- Create: `internal/server/api_settings_omdb.go`

**Context:** Task 11 referenced `/api/servers/:id/rename`, `/api/servers/refresh`, and `/api/settings/omdb`. Implement them now.

- [ ] **Step 1: Add the `rename` case to `handleServerScoped`**

Server rename routes through the existing `/api/servers/<id>/...` dispatcher. The dispatcher already resolves the `srv` handle from the machineID, so we just add a `rename` case.

Modify `internal/server/api_servers.go` — in `handleServerScoped`'s switch block, add:
```go
	case len(parts) == 2 && parts[1] == "rename":
		if r.Method != "POST" {
			writeError(w, http.StatusMethodNotAllowed, "POST required")
			return
		}
		var body struct {
			DisplayName string `json:"displayName"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			writeError(w, http.StatusBadRequest, "invalid JSON")
			return
		}
		srv.DisplayName = body.DisplayName
		if err := s.cfg.Save(); err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
		writeJSON(w, map[string]string{"status": "renamed", "displayName": body.DisplayName})
```

Add `"encoding/json"` to `api_servers.go`'s import block if it's not already there.

Note: `srv` is a `*config.Server` pointer into `s.cfg.Plex.Servers` (via `serverByID`), so mutating `srv.DisplayName` updates the live config — then `cfg.Save()` writes to disk.

- [ ] **Step 2: Refresh connections endpoint**

Create `internal/server/api_servers_refresh.go`:
```go
package server

import (
	"net/http"
	"sync"

	"lumen/internal/config"
	"lumen/internal/plex"
)

// handleServersRefresh re-discovers servers and re-picks connections.
// Mirrors what `lumen list` does but in-process, updating the live config.
func (s *Server) handleServersRefresh(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" {
		writeError(w, http.StatusMethodNotAllowed, "POST required")
		return
	}
	if s.cfg.Plex.AccountToken == "" {
		writeError(w, http.StatusUnauthorized, "no account token")
		return
	}
	if s.plex == nil {
		writeError(w, http.StatusInternalServerError, "plex client not initialised")
		return
	}

	servers, err := s.plex.DiscoverServers(s.cfg.Plex.AccountToken)
	if err != nil {
		writeError(w, http.StatusBadGateway, err.Error())
		return
	}

	var wg sync.WaitGroup
	for _, srv := range servers {
		wg.Add(1)
		go func(sr *plex.Server) {
			defer wg.Done()
			_, _ = s.plex.PickConnection(sr)
		}(srv)
	}
	wg.Wait()

	// Merge into config — preserve DisplayName overrides per server.
	var merged []config.Server
	for _, sr := range servers {
		var display string
		for _, prev := range s.cfg.Plex.Servers {
			if prev.MachineIdentifier == sr.MachineIdentifier {
				display = prev.DisplayName
				break
			}
		}
		merged = append(merged, config.Server{
			Name:               sr.Name,
			DisplayName:        display,
			MachineIdentifier:  sr.MachineIdentifier,
			AccessToken:        sr.AccessToken,
			LastGoodConnection: sr.BaseURL,
		})
	}
	s.cfg.Plex.Servers = merged
	if err := s.cfg.Save(); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, map[string]any{"status": "refreshed", "count": len(merged)})
}
```

Register in `registerRoutes`:
```go
	s.mux.HandleFunc("/api/servers/refresh", s.handleServersRefresh)
```

(No conflict with the existing `/api/servers/` prefix handler: Go's `http.ServeMux` prefers exact-path matches over prefix matches, so `/api/servers/refresh` without a trailing slash goes to this handler; `/api/servers/<machineID>/...` falls through to `handleServerScoped`.)

- [ ] **Step 3: OMDB settings endpoint**

Create `internal/server/api_settings_omdb.go`:
```go
package server

import (
	"encoding/json"
	"net/http"
)

// handleSettingsOMDB updates the OMDB API key in config. Top-level field, not
// inside UI, because OMDB is a Plex-adjacent integration concern.
func (s *Server) handleSettingsOMDB(w http.ResponseWriter, r *http.Request) {
	if r.Method != "PUT" {
		writeError(w, http.StatusMethodNotAllowed, "PUT required")
		return
	}
	var body struct {
		Key string `json:"key"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON")
		return
	}
	s.cfg.OMDBKey = body.Key
	if err := s.cfg.Save(); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, map[string]string{"status": "saved"})
}
```

Register: `s.mux.HandleFunc("/api/settings/omdb", s.handleSettingsOMDB)`.

- [ ] **Step 4: Build + commit**

```bash
go test ./...
cd web && npm run build
cd ..
git add internal/server/
git commit -m "feat(server): /api/servers/:id/rename, /api/servers/refresh, /api/settings/omdb"
```

---

## Task 15: `shortcuts` package + `/api/shortcut` endpoint + `lumen install-shortcut` CLI

**Files:**
- Create: `internal/shortcuts/windows.go`
- Create: `internal/server/api_shortcut.go`
- Create: `cmd/lumen/install_shortcut.go`

**Context:** Desktop shortcut creation. PowerShell's `WScript.Shell` COM wrapper is the most portable way — call out via `exec.Command`. Creates `%USERPROFILE%\Desktop\Lumen.lnk` pointing at the current `lumen.exe` with `serve` as args.

- [ ] **Step 1: Shortcut package**

Create `internal/shortcuts/windows.go`:
```go
// Package shortcuts creates Windows desktop shortcuts (.lnk files).
package shortcuts

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
)

// CreateDesktop drops a Lumen.lnk on the user's Desktop, targeting the given
// executable with the given arguments. Uses PowerShell's WScript.Shell COM
// object — it's available on every Windows install since XP.
func CreateDesktop(exePath, args string) (string, error) {
	exePath, err := filepath.Abs(exePath)
	if err != nil {
		return "", fmt.Errorf("abs path: %w", err)
	}
	workDir := filepath.Dir(exePath)

	desktop := filepath.Join(os.Getenv("USERPROFILE"), "Desktop")
	if info, err := os.Stat(desktop); err != nil || !info.IsDir() {
		return "", fmt.Errorf("desktop not found at %s", desktop)
	}
	lnkPath := filepath.Join(desktop, "Lumen.lnk")

	script := fmt.Sprintf(`
$wsh = New-Object -ComObject WScript.Shell
$sc = $wsh.CreateShortcut(%q)
$sc.TargetPath = %q
$sc.Arguments = %q
$sc.WorkingDirectory = %q
$sc.IconLocation = %q
$sc.Description = "Launch Lumen"
$sc.Save()
`, lnkPath, exePath, args, workDir, exePath)

	cmd := exec.Command("powershell", "-NoProfile", "-ExecutionPolicy", "Bypass", "-Command", script)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("powershell: %w — %s", err, out)
	}
	return lnkPath, nil
}
```

- [ ] **Step 2: `/api/shortcut` endpoint**

Create `internal/server/api_shortcut.go`:
```go
package server

import (
	"net/http"
	"os"

	"lumen/internal/shortcuts"
)

// handleShortcut creates a Desktop shortcut pointing at the current lumen.exe.
func (s *Server) handleShortcut(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" {
		writeError(w, http.StatusMethodNotAllowed, "POST required")
		return
	}
	exe, err := os.Executable()
	if err != nil {
		writeError(w, http.StatusInternalServerError, "resolve exe path: "+err.Error())
		return
	}
	path, err := shortcuts.CreateDesktop(exe, "serve")
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, map[string]string{"path": path})
}
```

Register: `s.mux.HandleFunc("/api/shortcut", s.handleShortcut)`.

- [ ] **Step 3: `lumen install-shortcut` CLI subcommand**

Create `cmd/lumen/install_shortcut.go`:
```go
package main

import (
	"fmt"
	"os"

	"lumen/internal/shortcuts"
)

func runInstallShortcut(args []string) {
	exe, err := os.Executable()
	if err != nil {
		fmt.Fprintf(os.Stderr, "resolve exe path: %v\n", err)
		os.Exit(1)
	}
	path, err := shortcuts.CreateDesktop(exe, "serve")
	if err != nil {
		fmt.Fprintf(os.Stderr, "create shortcut: %v\n", err)
		os.Exit(1)
	}
	fmt.Printf("Created desktop shortcut at %s\n", path)
}
```

Wire into `cmd/lumen/main.go`:
```go
	case "install-shortcut":
		runInstallShortcut(os.Args[2:])
```

Add to usage text:
```
  install-shortcut  Create a Lumen shortcut on your Desktop
```

- [ ] **Step 4: Build + commit (no unit tests — WScript.Shell is a live Windows COM call)**

```bash
go build ./...
git add internal/shortcuts/ internal/server/api_shortcut.go cmd/lumen/install_shortcut.go cmd/lumen/main.go
git commit -m "feat: shortcuts package + /api/shortcut + lumen install-shortcut CLI"
```

---

## Task 16: Wire Settings modal open/close into the left menu

**Files:**
- Modify: `web/src/components/LeftMenu.tsx`
- Modify: `web/src/App.tsx`

**Context:** Settings link currently navigates to a placeholder route. Replace with a button that opens the SettingsModal overlay.

- [ ] **Step 1: Lift modal state to App + pass toggle down**

Replace `web/src/App.tsx`:
```tsx
import { createSignal, ParentProps } from "solid-js";
import TopBar from "./components/TopBar";
import LeftMenu from "./components/LeftMenu";
import SettingsModal from "./components/Settings/SettingsModal";

export default function App(props: ParentProps) {
  const [settingsOpen, setSettingsOpen] = createSignal(false);
  return (
    <div class="app-shell">
      <TopBar />
      <div class="app-body">
        <LeftMenu onOpenSettings={() => setSettingsOpen(true)} />
        <main class="content">{props.children}</main>
      </div>
      <SettingsModal open={settingsOpen()} onClose={() => setSettingsOpen(false)} />
    </div>
  );
}
```

- [ ] **Step 2: Update LeftMenu to call the toggle**

In `web/src/components/LeftMenu.tsx`, change the Settings `<A>` link to a button:
```tsx
export default function LeftMenu(props: { onOpenSettings: () => void }) {
  // ...existing code...
  return (
    <nav class="left-menu">
      {/* ...existing menu items and Libraries section... */}
      <div class="menu-spacer" />
      <ul class="menu-bottom">
        <li>
          <button class="menu-settings-btn" onClick={props.onOpenSettings}>⚙ Settings</button>
        </li>
      </ul>
    </nav>
  );
}
```

Add to `LeftMenu.css`:
```css
.menu-settings-btn {
  display: block;
  width: 100%;
  text-align: left;
  padding: 6px 10px;
  background: transparent;
  color: var(--menu-icon);
  border: none;
  border-radius: var(--radius-sm);
  cursor: pointer;
  font-size: 13px;
}

.menu-settings-btn:hover {
  background: var(--bg-elevated);
  color: var(--text);
}
```

- [ ] **Step 3: Build + commit**

```bash
cd web && npm run build
cd ..
git add web/src/App.tsx web/src/components/LeftMenu.tsx web/src/components/LeftMenu.css
git commit -m "feat(web): Settings menu button opens the modal instead of routing"
```

---

## Task 17: Migrate localStorage prefs → config.json

**Files:**
- Modify: `web/src/pages/Library.tsx`

**Context:** Library.tsx currently reads/writes `lumen.library.sort` and `lumen.library.viewMode` from localStorage. Session 3 moves them to the settings store → config.json.

- [ ] **Step 1: Update Library.tsx**

Replace the localStorage helpers with settings-store access:
```tsx
import { store as settingsStore } from "../state/settings";

// Remove: LS_SORT, LS_VIEW, loadLS, saveLS.

// Replace the signal inits:
const [sort, setSort] = createSignal(settingsStore.settings()?.defaultSort ?? SORT_OPTIONS[0].value);
const [viewMode, setViewMode] = createSignal(settingsStore.settings()?.defaultViewMode ?? "4");

// Replace the effect blocks that called saveLS:
createEffect(() => {
  const s = sort();
  settingsStore.patch({ defaultSort: s });
  setPage(0);
});
createEffect(() => {
  const v = viewMode();
  settingsStore.patch({ defaultViewMode: v as any });
  setPage(0);
});
```

Note that `defaultViewMode` values are `"shows" | "episodes" | ""` per our type, but we've been using `"4"` in the Library page. Normalize — on the Plex server side, `4` is the "episodes" type code. Map here:
```tsx
const viewModeToFilter = (mode: string) => (mode === "episodes" ? "4" : "");
const filterToViewMode = (filter: string) => (filter === "4" ? "episodes" : "");
```

And update the items fetch:
```tsx
if (isTV && viewMode() === "episodes") opts.filters = { type: "4" };
```

- [ ] **Step 2: Build + commit**

```bash
cd web && npm run build
cd ..
git add web/src/pages/Library.tsx
git commit -m "feat(web): Library sort/viewMode migrate from localStorage to settings store"
```

---

## Task 18: Shelf drag-reorder with @thisbeyond/solid-dnd

**Files:**
- Modify: `web/src/components/Shelf.tsx`
- Modify: `web/src/pages/Home.tsx`

**Context:** Shelves within a Home group are reorderable. Reordering persists to `config.UI.ShelfState[pageKey].ShelfOrder[groupID]`. Use solid-dnd's `DragDropProvider` + `SortableProvider` + `createSortable` pattern.

- [ ] **Step 1: Wrap Home ServerGroup's shelf loop in SortableProvider**

In `web/src/pages/Home.tsx`, change `ServerGroup`:
```tsx
import { DragDropProvider, DragDropSensors, SortableProvider, createSortable, closestCenter } from "@thisbeyond/solid-dnd";
import { store as settingsStore } from "../state/settings";

function ServerGroup(props: { srvs: Server[]; logicalName: string; shelves: ShelfDef[] }) {
  const matched = () =>
    props.srvs.find((s) =>
      s.displayName.toLowerCase().includes(props.logicalName.toLowerCase())
    );

  const pageKey = "home";
  const groupID = props.logicalName.toLowerCase();

  // Resolve the persisted shelf order for this group — fall back to the spec's default.
  const persistedOrder = () => {
    const order = settingsStore.settings()?.shelfState?.[pageKey]?.shelfOrder?.[groupID];
    if (!order || order.length === 0) return props.shelves.map((s) => s.id);
    // Include any new shelves that weren't in the persisted list (appended).
    const missing = props.shelves.map((s) => s.id).filter((id) => !order.includes(id));
    return [...order.filter((id) => props.shelves.some((s) => s.id === id)), ...missing];
  };

  function onShelfDragEnd(e: any) {
    const { draggable, droppable } = e;
    if (!draggable || !droppable) return;
    const current = persistedOrder();
    const from = current.indexOf(draggable.id as string);
    const to = current.indexOf(droppable.id as string);
    if (from === -1 || to === -1 || from === to) return;
    const next = [...current];
    next.splice(from, 1);
    next.splice(to, 0, draggable.id as string);

    const state = settingsStore.settings()?.shelfState ?? {};
    const pageState = state[pageKey] ?? {};
    const shelfOrder = { ...(pageState.shelfOrder ?? {}), [groupID]: next };
    settingsStore.patch({
      shelfState: { ...state, [pageKey]: { ...pageState, shelfOrder } },
    });
  }

  return (
    <Group id={`group-${groupID}`} title={props.logicalName}>
      <Show when={matched()} fallback={<div class="group-missing">{props.logicalName} not found — run `lumen list`</div>}>
        {(srv) => (
          <DragDropProvider onDragEnd={onShelfDragEnd} collisionDetector={closestCenter}>
            <DragDropSensors />
            <SortableProvider ids={persistedOrder()}>
              <For each={persistedOrder()}>
                {(id) => {
                  const def = props.shelves.find((s) => s.id === id);
                  return def ? <ShelfLoader server={srv() as Server} def={def} /> : null;
                }}
              </For>
            </SortableProvider>
          </DragDropProvider>
        )}
      </Show>
    </Group>
  );
}
```

- [ ] **Step 2: Make Shelf draggable**

Modify `web/src/components/Shelf.tsx` to create a sortable within its render:
```tsx
import { createSortable } from "@thisbeyond/solid-dnd";

export default function Shelf(props: ShelfProps) {
  const [collapsed, setCollapsed] = createSignal(!!props.initialCollapsed);
  const rowsPerShelf = () => props.rowsPerShelf ?? 3;
  const sortable = createSortable(props.id);

  return (
    <section
      ref={sortable.ref}
      class="shelf"
      classList={{ "is-dragging": sortable.isActiveDraggable }}
      style={sortable.transform ? { transform: `translate(${sortable.transform.x}px, ${sortable.transform.y}px)` } : {}}
      data-shelf-id={props.id}
    >
      <header class="shelf-header">
        <button
          class="shelf-collapse-btn"
          aria-expanded={!collapsed()}
          onClick={() => setCollapsed(!collapsed())}
        >
          <span class="caret">{collapsed() ? "▸" : "▾"}</span>
          <h2 class="shelf-title">{props.title}</h2>
        </button>
        <span class="shelf-drag-handle" {...sortable.dragActivators} title="Drag to reorder" aria-label="Drag handle">⋮⋮</span>
      </header>
      <Show when={!collapsed()}>
        <div class="shelf-cards" style={{ "--rows-per-shelf": rowsPerShelf() }}>
          {props.children}
        </div>
      </Show>
    </section>
  );
}
```

Add drag-handle styles to `web/src/components/Shelf.css`:
```css
.shelf-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
}

.shelf-drag-handle {
  cursor: grab;
  color: var(--text-muted);
  padding: 6px 8px;
  user-select: none;
  opacity: 0;
  transition: opacity 0.15s ease;
}

.shelf:hover .shelf-drag-handle {
  opacity: 1;
}

.shelf.is-dragging {
  opacity: 0.5;
}
```

- [ ] **Step 3: Build + commit**

```bash
cd web && npm run build
cd ..
git add web/src/
git commit -m "feat(web): shelf drag-reorder within Home groups, persisted to config"
```

---

## Task 19: Group drag-reorder on Home

**Files:**
- Modify: `web/src/pages/Home.tsx`
- Modify: `web/src/components/Group.tsx`

**Context:** Same pattern as Task 18 but at the group level. Wrap the two server groups on Home in a SortableProvider so Byron can reorder DKNZPLEX above Stargaze.

- [ ] **Step 1: Update Home to wrap ServerGroups**

In Home.tsx, replace the `<ServerGroup>` pair with a sortable list:
```tsx
export default function Home() {
  const [servers] = createResource(() => api.servers());
  const pageKey = "home";

  const groupDefs = [
    { id: "stargaze", logicalName: "Stargaze", shelves: STARGAZE_SHELVES },
    { id: "dknzplex", logicalName: "DKNZPLEX", shelves: DKNZPLEX_SHELVES },
  ];

  const persistedGroupOrder = () => {
    const persisted = settingsStore.settings()?.shelfState?.[pageKey]?.groupOrder;
    if (!persisted || persisted.length === 0) return groupDefs.map((g) => g.id);
    const missing = groupDefs.map((g) => g.id).filter((id) => !persisted.includes(id));
    return [...persisted.filter((id) => groupDefs.some((g) => g.id === id)), ...missing];
  };

  function onGroupDragEnd(e: any) {
    const { draggable, droppable } = e;
    if (!draggable || !droppable) return;
    const current = persistedGroupOrder();
    const from = current.indexOf(draggable.id as string);
    const to = current.indexOf(droppable.id as string);
    if (from === -1 || to === -1 || from === to) return;
    const next = [...current];
    next.splice(from, 1);
    next.splice(to, 0, draggable.id as string);

    const state = settingsStore.settings()?.shelfState ?? {};
    const pageState = state[pageKey] ?? {};
    settingsStore.patch({
      shelfState: { ...state, [pageKey]: { ...pageState, groupOrder: next } },
    });
  }

  return (
    <div class="home-page">
      <Show when={servers()}>
        {(srvs) => (
          <>
            <ContinueWatching servers={srvs() as Server[]} />
            <DragDropProvider onDragEnd={onGroupDragEnd} collisionDetector={closestCenter}>
              <DragDropSensors />
              <SortableProvider ids={persistedGroupOrder()}>
                <For each={persistedGroupOrder()}>
                  {(id) => {
                    const def = groupDefs.find((g) => g.id === id)!;
                    return <ServerGroup srvs={srvs() as Server[]} logicalName={def.logicalName} shelves={def.shelves} />;
                  }}
                </For>
              </SortableProvider>
            </DragDropProvider>
          </>
        )}
      </Show>
    </div>
  );
}
```

- [ ] **Step 2: Make Group draggable**

Modify `web/src/components/Group.tsx`:
```tsx
import { createSortable } from "@thisbeyond/solid-dnd";

export default function Group(props: GroupProps) {
  const [collapsed, setCollapsed] = createSignal(!!props.initialCollapsed);
  const sortable = createSortable(props.id);
  return (
    <section
      ref={sortable.ref}
      class="group"
      classList={{ "is-dragging": sortable.isActiveDraggable }}
      style={sortable.transform ? { transform: `translate(${sortable.transform.x}px, ${sortable.transform.y}px)` } : {}}
      data-group-id={props.id}
    >
      <header class="group-header-wrap" style={{ "display": "flex", "align-items": "center", "gap": "8px" }}>
        <button
          class="group-header"
          aria-expanded={!collapsed()}
          onClick={() => setCollapsed(!collapsed())}
        >
          <span class="caret">{collapsed() ? "▸" : "▾"}</span>
          <h1 class="group-title">{props.title}</h1>
        </button>
        <span class="group-drag-handle" {...sortable.dragActivators} title="Drag to reorder group">⋮⋮</span>
      </header>
      <Show when={!collapsed()}>
        <div class="group-body">{props.children}</div>
      </Show>
    </section>
  );
}
```

Add to `Group.css`:
```css
.group-drag-handle {
  cursor: grab;
  color: var(--text-muted);
  padding: 8px;
  opacity: 0;
  transition: opacity 0.15s ease;
}

.group:hover .group-drag-handle {
  opacity: 1;
}

.group.is-dragging {
  opacity: 0.5;
}
```

- [ ] **Step 3: Build + commit**

```bash
cd web && npm run build
cd ..
git add web/src/
git commit -m "feat(web): group drag-reorder on Home persists to config"
```

---

## Task 20: Library visibility toggles in LeftMenu

**Files:**
- Modify: `web/src/components/LeftMenu.tsx`
- Modify: `web/src/components/LeftMenu.css`
- Modify: `web/src/pages/Home.tsx`

**Context:** Spec §10.2: "each library row has an inline toggle to hide/show that library across Lumen". Inline eye-icon button per library; click toggles `config.UI.HiddenLibraries`. Hidden libraries disappear from left menu nav, and any Home shelves referencing them show nothing.

- [ ] **Step 1: Update LeftMenu's library rows**

In `web/src/components/LeftMenu.tsx`, wrap the library `<A>` with a row that includes a toggle:
```tsx
import { store as settingsStore } from "../state/settings";

function ServerLibraries(props: { server: Server }) {
  const [libs] = createResource(() => api.libraries(props.server.machineIdentifier));
  const [expanded, setExpanded] = createSignal(true);
  const hiddenSet = () => new Set(settingsStore.settings()?.hiddenLibraries ?? []);
  const key = (libKey: string) => `${props.server.machineIdentifier}:${libKey}`;
  const isHidden = (libKey: string) => hiddenSet().has(key(libKey));

  function toggleHidden(libKey: string) {
    const set = new Set(hiddenSet());
    const k = key(libKey);
    if (set.has(k)) set.delete(k); else set.add(k);
    settingsStore.patch({ hiddenLibraries: Array.from(set) });
  }

  return (
    <div class="server-group">
      <button class="server-group-header" onClick={() => setExpanded(!expanded())}>
        <span class="caret">{expanded() ? "▾" : "▸"}</span>
        <span>{props.server.displayName}</span>
        <span class="server-status" data-status={props.server.status} />
      </button>
      <Show when={expanded() && libs()}>
        {(libList) => (
          <ul class="library-list">
            <For each={libList() as Library[]}>
              {(l) => (
                <li class="library-row" classList={{ "is-hidden": isHidden(l.key) }}>
                  <A href={`/library/${props.server.machineIdentifier}/${l.key}`} activeClass="active">
                    {l.title}
                  </A>
                  <button
                    class="library-eye"
                    onClick={(e) => { e.preventDefault(); e.stopPropagation(); toggleHidden(l.key); }}
                    title={isHidden(l.key) ? "Show library" : "Hide library"}
                    aria-label="Toggle library visibility"
                  >
                    {isHidden(l.key) ? "🙈" : "👁"}
                  </button>
                </li>
              )}
            </For>
          </ul>
        )}
      </Show>
    </div>
  );
}
```

Add to `LeftMenu.css`:
```css
.library-row {
  display: flex;
  align-items: center;
  gap: 4px;
}

.library-row > a {
  flex: 1;
  min-width: 0;
}

.library-eye {
  padding: 2px 6px;
  background: transparent;
  border: none;
  color: var(--text-muted);
  cursor: pointer;
  font-size: 12px;
  opacity: 0;
  transition: opacity 0.15s ease;
}

.library-row:hover .library-eye {
  opacity: 1;
}

.library-row.is-hidden > a {
  opacity: 0.5;
  font-style: italic;
}

.library-row.is-hidden .library-eye {
  opacity: 1;
}
```

- [ ] **Step 2: Filter hidden libraries from Home shelves**

In `web/src/pages/Home.tsx`'s `ShelfLoader`, skip shelves whose library is hidden:
```tsx
function ShelfLoader(props: { server: Server; def: ShelfDef }) {
  if (props.def.kind === "stub") { /* unchanged */ }
  if (props.def.kind === "ondeck-merged") return null;

  const hiddenSet = () => new Set(settingsStore.settings()?.hiddenLibraries ?? []);

  const [libs] = createResource(() => api.libraries(props.server.machineIdentifier));
  return (
    <Shelf id={props.def.id} title={props.def.title}>
      <Show when={libs()}>
        {(libList) => {
          const lib = (libList() as Library[]).find((l) => l.title === props.def.libraryName);
          if (!lib) return <div class="shelf-stub">(library "{props.def.libraryName}" not found on {props.server.displayName})</div>;
          const hidden = hiddenSet().has(`${props.server.machineIdentifier}:${lib.key}`);
          if (hidden) return <div class="shelf-stub">(library hidden — toggle in left menu)</div>;
          return <LibraryCards server={props.server} libraryKey={lib.key} />;
        }}
      </Show>
    </Shelf>
  );
}
```

- [ ] **Step 3: Build + commit**

```bash
cd web && npm run build
cd ..
git add web/src/
git commit -m "feat(web): inline library hide toggles in LeftMenu; Home shelves honour hidden state"
```

---

## Task 21: Taste polish pass — Lucide icons, Geist typography, Motion One, skeletons, focus rings

**Files touched:** most SPA components (TopBar, Shelf, Group, Card, LeftMenu, Library, ItemDetail, Settings/*), `web/src/theme.css`, `web/src/main.tsx`, `web/package.json`.

**Context:** Taste-skill pass against the Lumen codebase flagged several AI-slop patterns that need correcting before v1.0 ships. This task bundles the fixes: SVG icons (not emojis), Geist font family (not system stack), Motion One modal transitions, skeleton shimmer placeholders, proper focus rings, and inner-refraction border on the Settings modal. Done as a single task at end of Session 3 so it applies to the full surface area at once.

- [ ] **Step 1: Install dependencies**

```bash
cd web && npm install lucide-solid @fontsource/geist-sans @fontsource/geist-mono @motionone/solid
```

- [ ] **Step 2: Switch the font stack to Geist**

Modify `web/src/main.tsx` — add font imports at the top:
```typescript
import "@fontsource/geist-sans/400.css";
import "@fontsource/geist-sans/500.css";
import "@fontsource/geist-sans/600.css";
import "@fontsource/geist-sans/700.css";
import "@fontsource/geist-mono/400.css";
import "@fontsource/geist-mono/500.css";
```

Modify `web/src/theme.css` — update the font token:
```css
  --font-base:      "Geist Sans", -apple-system, BlinkMacSystemFont, "Segoe UI", Roboto, sans-serif;
  --font-mono:      "Geist Mono", "JetBrains Mono", Consolas, Menlo, monospace;
```

- [ ] **Step 3: Create a shared Icon import file**

Create `web/src/components/icons.ts`:
```typescript
// Lumen's icon set — every visible icon in the UI. Import from here so we
// control stroke-width consistency and can swap libraries in one place later
// if needed. Standardise on stroke-width 1.75 for mid-weight lines; explicitly
// pass width/height via className (w-N h-N) at call site.
export {
  ArrowLeft,
  Home,
  Maximize2,      // kiosk / fullscreen toggle
  Minus,          // zoom out
  Plus,           // zoom in (paired with slider)
  X,              // close buttons
  Search,         // search input adornment
  ChevronDown,    // expanded caret
  ChevronRight,   // collapsed caret
  GripVertical,   // drag handle on shelves + groups
  Trash2,         // remove from Continue Watching
  Eye,            // library visible
  EyeOff,         // library hidden
  Settings,       // left-menu settings entry
  Play,           // primary playback action
  Sparkles,       // brand logo placeholder
  ExternalLink,   // "Get a free key" link in OMDB field
  RefreshCw,      // refresh connections button
} from "lucide-solid";
```

Set a default stroke width via CSS — append to `web/src/theme.css`:
```css
/* All Lucide icons share stroke-width 1.75 unless overridden. */
.lucide {
  stroke-width: 1.75;
}
```

- [ ] **Step 4: Replace emoji in TopBar**

Modify `web/src/components/TopBar.tsx` — replace the emoji usages:
```tsx
import { ArrowLeft, Home, Maximize2, Search, Sparkles, X } from "./icons";

// inside the JSX:
<span class="logo"><Sparkles size={18} /></span>
// ...
<button class="icon-btn" title="Back" aria-label="Back" onClick={() => navigate(-1)}>
  <ArrowLeft size={16} />
</button>
<button class="icon-btn" title="Home" aria-label="Home" onClick={() => navigate("/")}>
  <Home size={16} />
</button>
// ...
<button class="icon-btn" title="Kiosk mode (Session 5)" aria-label="Kiosk mode">
  <Maximize2 size={16} />
</button>
<span class="zoom-icon" aria-hidden="true"><Search size={12} /></span>
// ...
<button class="icon-btn" title="Close Lumen" aria-label="Close" onClick={() => window.close()}>
  <X size={16} />
</button>
```

- [ ] **Step 5: Replace emoji in Shelf + Group headers (caret + drag handle)**

Modify `web/src/components/Shelf.tsx`:
```tsx
import { ChevronDown, ChevronRight, GripVertical } from "./icons";

// inside the header JSX:
<span class="caret">
  {collapsed() ? <ChevronRight size={14} /> : <ChevronDown size={14} />}
</span>
// ...
<span class="shelf-drag-handle" {...sortable.dragActivators} title="Drag to reorder" aria-label="Drag handle">
  <GripVertical size={16} />
</span>
```

Same pattern in `web/src/components/Group.tsx` — swap `▾ ▸` for `ChevronDown / ChevronRight` and `⋮⋮` for `GripVertical`.

- [ ] **Step 6: Replace emoji in Card + LeftMenu + Library**

- `web/src/components/Card.tsx` — bin icon `🗑` → `<Trash2 size={14} />`
- `web/src/components/LeftMenu.tsx` — caret `▾ ▸` → ChevronDown/Right; settings `⚙` → `<Settings size={14} />`
- Library visibility toggle `🙈 / 👁` → `<EyeOff size={12} /> / <Eye size={12} />`

All call-site updates: import from `"../components/icons"` (adjust relative path per file).

- [ ] **Step 7: Replace emoji in Settings modal close button**

`web/src/components/Settings/SettingsModal.tsx`:
```tsx
import { X } from "../icons";
// ...
<button class="settings-close-btn" onClick={props.onClose} aria-label="Close settings">
  <X size={14} />
</button>
```

- [ ] **Step 8: Motion One fade+scale on Settings modal open/close**

Modify `SettingsModal.tsx` to wrap the modal contents in a `motion.div`:
```tsx
import { Motion, Presence } from "@motionone/solid";

// replace the <Show>-wrapped outer structure with:
<Presence>
  <Show when={props.open}>
    <Motion.div
      class="settings-backdrop"
      onClick={props.onClose}
      role="presentation"
      initial={{ opacity: 0 }}
      animate={{ opacity: 1 }}
      exit={{ opacity: 0 }}
      transition={{ duration: 0.18, easing: [0.16, 1, 0.3, 1] }}
    >
      <Motion.div
        class="settings-modal"
        onClick={(e) => e.stopPropagation()}
        role="dialog" aria-label="Settings"
        initial={{ opacity: 0, scale: 0.96, y: 8 }}
        animate={{ opacity: 1, scale: 1, y: 0 }}
        exit={{ opacity: 0, scale: 0.96, y: 8 }}
        transition={{ duration: 0.22, easing: "spring(1, 100, 14, 0)" }}
      >
        {/* existing nav + detail panes */}
      </Motion.div>
    </Motion.div>
  </Show>
</Presence>
```

- [ ] **Step 9: Inner-refraction border on Settings modal**

Modify `SettingsModal.css`:
```css
.settings-modal {
  /* ...existing rules... */
  border: 1px solid rgba(255, 255, 255, 0.08);
  box-shadow:
    var(--shadow),
    inset 0 1px 0 rgba(255, 255, 255, 0.08);
}
```

(Remove the existing `border: 1px solid var(--border);` line — the refraction border replaces it.)

- [ ] **Step 10: Global `:focus-visible` treatment**

Append to `web/src/theme.css`:
```css
/* Keyboard-visible focus rings. Mouse clicks don't trigger :focus-visible;
   only keyboard nav + programmatic focus do. Matches system expectations. */
:focus-visible {
  outline: 2px solid var(--stroke);
  outline-offset: 2px;
  border-radius: 4px;
}

/* Remove mouse-click focus outlines where intrinsic feedback exists
   (buttons have their own hover/active states). */
button:focus:not(:focus-visible),
a:focus:not(:focus-visible) {
  outline: none;
}
```

- [ ] **Step 11: Skeleton component for loading states**

Create `web/src/components/Skeleton.tsx`:
```tsx
import "./Skeleton.css";

export interface SkeletonProps {
  /** "card" renders a poster-aspect placeholder; "line" renders a short text line. */
  kind?: "card" | "line";
  count?: number;
}

export default function Skeleton(props: SkeletonProps) {
  const n = () => props.count ?? 1;
  const kind = () => props.kind ?? "line";
  return (
    <>
      {Array.from({ length: n() }).map(() => (
        <div class={`skeleton skeleton-${kind()}`} />
      ))}
    </>
  );
}
```

Create `web/src/components/Skeleton.css`:
```css
/* Shimmer animation — a light-coloured gradient sweeping across a muted base. */
@keyframes skeleton-shimmer {
  0%   { background-position: -200% 0; }
  100% { background-position: 200% 0; }
}

.skeleton {
  background: linear-gradient(
    90deg,
    var(--bg-elevated) 0%,
    rgba(255, 255, 255, 0.05) 50%,
    var(--bg-elevated) 100%
  );
  background-size: 200% 100%;
  animation: skeleton-shimmer 1.6s ease-in-out infinite;
  border-radius: var(--radius-sm);
}

.skeleton-card {
  aspect-ratio: 2 / 3;
  width: var(--card-width, 160px);
}

.skeleton-line {
  height: 1em;
  width: 100%;
  max-width: 240px;
}

@media (prefers-reduced-motion: reduce) {
  .skeleton {
    animation: none;
  }
}
```

- [ ] **Step 12: Use Skeleton in loading fallbacks**

Replace "Loading…" text instances with skeleton cards in:

`web/src/pages/Home.tsx` — both `ContinueWatching` and `LibraryCards` components. Example for `LibraryCards`:
```tsx
import Skeleton from "../components/Skeleton";
// ...
<Show when={items()} fallback={<Skeleton kind="card" count={6} />}>
```

`web/src/pages/Library.tsx` — replace `<div class="library-loading">Loading…</div>` with `<Skeleton kind="card" count={12} />`.

`web/src/pages/ItemDetail.tsx` — replace the `item-loading` div with a hero-shaped skeleton + a few line skeletons for the overview.

- [ ] **Step 13: OMDB key inline validation**

Modify `web/src/components/Settings/AccountsServers.tsx` — extend the OMDB field with validation:
```tsx
const [omdbError, setOmdbError] = createSignal("");
function validateAndSaveOMDB() {
  const k = omdbKey().trim();
  if (k === "") {
    setOmdbError("");
    saveOMDB();
    return;
  }
  // OMDB keys are exactly 8 hex chars.
  if (!/^[a-f0-9]{8}$/i.test(k)) {
    setOmdbError("Expected 8 hexadecimal characters (e.g. 1a2b3c4d).");
    return;
  }
  setOmdbError("");
  saveOMDB();
}
```

In the OMDB field JSX:
```tsx
<input
  id="omdbKey"
  type="password"
  placeholder="8-char hex key (Session 5 enables IMDB ratings)"
  value={omdbKey()}
  onInput={(e) => setOmdbKey(e.currentTarget.value)}
  onBlur={validateAndSaveOMDB}
  aria-invalid={omdbError() !== ""}
  aria-describedby={omdbError() ? "omdbError" : undefined}
/>
{omdbError() && (
  <div id="omdbError" role="alert" style={{ "margin-top": "4px", "color": "#f07878", "font-size": "12px" }}>
    {omdbError()}
  </div>
)}
```

- [ ] **Step 14: Build + run tests + commit**

```bash
cd web && npm run build && cd ..
go test ./...
go build -o lumen.exe ./cmd/lumen
git add web/ internal/
git commit -m "feat(web): taste pass — Lucide icons, Geist fonts, Motion One modal, skeletons, focus rings"
```

---

## Task 22: End-to-end verification

**Files:** none.

**Context:** Byron runs Lumen, exercises every setting, confirms persistence, confirms drag-reorder survives refresh, confirms theme swaps don't require rebuild.

- [ ] **Step 1: Clean rebuild**

```bash
cd web && npm run build && cd ..
go build -o lumen.exe ./cmd/lumen
```

- [ ] **Step 2: Launch and exercise**

```bash
./lumen.exe serve
```

Checklist:
- Settings gear opens the modal; Escape + backdrop close it; section nav switches panes.
- Appearance: theme dropdown shows Pure OLED. Card size changes resize cards instantly. Rows per shelf updates Home's grid. Font size slider resizes text live. Kill the process and relaunch — values persist.
- Kiosk & Shortcuts: toggle saves; browser preference saves; **Create Desktop Shortcut** drops `Lumen.lnk` on the Desktop pointing at `lumen.exe serve`. Double-click the `.lnk` — Lumen launches.
- Accounts & Servers: Plex username shows at top. Server display-name edits persist (change Stargaze's label, refresh, still there). Refresh Connections triggers a re-discovery. OMDB key saves on blur.
- Playback: Pot Player path saves.
- Data & Cache: size readouts are real. **Clear Image Cache** empties `%APPDATA%\Lumen\cache\images\`. Clear All wipes both.
- About: version prints, file/folder links open Explorer.
- Home drag-reorder: drag the Stargaze group below DKNZPLEX → sticks across refresh. Drag a shelf within a group → sticks.
- Library visibility: eye-toggle in LeftMenu hides a library; corresponding Home shelf shows "library hidden" stub; toggle visibility back.

- [ ] **Step 3: `lumen install-shortcut` CLI**

```bash
./lumen.exe install-shortcut
```
Expected: prints `Created desktop shortcut at C:\Users\<you>\Desktop\Lumen.lnk`. Double-click → Lumen launches.

- [ ] **Step 4: Report back**

Paste observations + any breakage for Session 3 findings.

---

## Self-review checklist (for the executing agent)

Before reporting Session 3 done:

- [ ] Every Task 1–21 step checkbox ticked.
- [ ] No emojis remain in any SPA component — grep `rg "[\u2600-\u27BF]|🗑|👁|🙈" web/src` should return nothing.
- [ ] Geist Sans visibly active (inspect any text in DevTools → computed font-family).
- [ ] `:focus-visible` rings appear when tabbing through the app.
- [ ] Settings modal fades + scales on open/close with spring feel.
- [ ] Loading placeholders are shimmering skeleton cards, not "Loading…" text.
- [ ] `go test ./...` passes (expect ~36+ tests).
- [ ] `cd web && npm run build` succeeds.
- [ ] `go build ./cmd/lumen` clean.
- [ ] All 6 Settings sections render (Appearance / Kiosk & Shortcuts / Accounts & Servers / Playback / Data & Cache / About).
- [ ] Theme apply via `applyTheme` — Pure OLED tokens on `:root` after boot.
- [ ] No `localStorage` references in SPA source except possibly a comment noting migration.
- [ ] `probe/` untouched; Session 0/1/2 findings untouched.
- [ ] Spec §13 Section 2 "OLED Protection" **not** implemented (deliberately dropped per Byron).
- [ ] Autostart **not** implemented (dropped; replaced by `lumen install-shortcut` + Settings button).
- [ ] Commits sequenced and atomic — no "miscellaneous fixes" commits.

## Design notes for later sessions (carried from Session 2)

These stay honoured through Session 3:

- Cast/Crew (Session 5): character names in muted body-text grey, actor names in white.
- Episode rows (Session 5): episode title white, description/duration/date muted.
- Primary action buttons: inverse fill (white bg, black text, pill).
- Top-bar pill stays floating, dark navy surface.
- No pixel-shift / auto-hide chrome.
