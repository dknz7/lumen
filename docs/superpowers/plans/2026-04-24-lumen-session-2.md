# Lumen — Session 2 Implementation Plan (Web Shell & Library Browsing)

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Stand up the embedded Lumen web app — a SolidJS SPA served by the Go binary on `localhost:7832` — with a full navigation shell, a two-level shelf-and-card system, Home (14 shelves), Library grid views, and an Item Detail skeleton. End state: `lumen serve` launches, opens the default browser, and Byron can navigate every top-level page and drill into item detail. Images render via a token-stripping proxy; DevTools Network must show no raw Plex URLs.

**Architecture:** Go backend adds an HTTP server (`internal/server`) that mounts a `/api/*` router and serves the embedded SPA at `/`. The SPA (SolidJS + Vite + TypeScript under `web/`) is built into `web/dist/` and embedded into the Go binary via `//go:embed all:web/dist`. All Plex calls stay server-side — the browser only ever talks to Lumen's own API. The image proxy (`/api/image-proxy`) is the load-bearing security piece: the SPA gets opaque local paths, the Go backend fetches from Plex with the real token, and ciphered tokens never reach the browser. Session 3 will add theming/OLED/persistence on top; Session 4 wires real playback in. Session 2 keeps playback stubbed.

**Tech Stack:**
- Go 1.26.2 (already installed).
- Go stdlib `net/http` + `http.ServeMux` — no third-party router.
- SolidJS 1.9+ (reactive UI, smaller than React, matches spec).
- Vite 6+ (build tool).
- TypeScript 5+ (catches component wiring errors).
- `@solidjs/router` (client-side routing).
- `github.com/pkg/browser` (already pulled in Session 1; reused for auto-opening the browser on `lumen serve`).
- No UI framework. Vanilla CSS with custom properties on `:root` (matches spec §14).
- No state management library. Solid's built-in stores + resources are enough.

**Carry-ins from Sessions 0–1:**
- All `internal/config` and `internal/plex` packages stay as-is — Session 2 consumes them via new HTTP handlers.
- `internal/potplayer` stays a skeleton; Session 2's `/api/play` endpoint returns HTTP 501 "Session 4 populates this" for now.
- Session 1 findings (`docs/session-1-findings.md`) pin the watchlist slugs including `continueWatching` — Session 2 wires those into the Recommended page **in Session 5 per spec §3**. Session 2 only implements Home + Library + Item Detail (not Recommended/Discover/Watchlist pages).

**Pre-flight — confirm before Task 1:**

- Working directory: `C:\Users\dicke\Desktop\Dump Zone\STACK\04-DEV\lumen`.
- Stay on `main` branch. Solo repo.
- Node.js ≥ 20 and npm ≥ 10 installed (Byron has Node 22.21 / npm 11.7 — confirmed at planning time).
- Windows Defender exclusion for the Lumen folder still in place (Session 1 fix).
- `go test ./...` should pass all 23 existing tests before starting.

---

## File Structure

**Go additions:**

```
lumen/
├── cmd/lumen/
│   └── serve.go                        # `lumen serve` subcommand
├── internal/server/
│   ├── server.go                       # HTTP server struct, lifecycle, mux wiring
│   ├── server_test.go
│   ├── spa.go                          # Serve embedded SPA + SPA fallback routing
│   ├── api.go                          # Shared helpers (writeJSON, errors)
│   ├── api_servers.go                  # /api/servers, /api/servers/:id/libraries, .../items
│   ├── api_servers_test.go
│   ├── api_items.go                    # /api/items/:ratingKey (cross-server aware)
│   ├── api_items_test.go
│   ├── api_hubs.go                     # /api/hubs/:namespace/:slug with 5-min cache
│   ├── api_hubs_test.go
│   ├── api_availability.go             # /api/availability?guid=...
│   ├── api_availability_test.go
│   ├── api_image_proxy.go              # /api/image-proxy?server=...&path=... (CRITICAL)
│   ├── api_image_proxy_test.go
│   └── api_play.go                     # /api/play stub — returns 501 until Session 4
└── web/                                # SolidJS SPA (populated Task 9+)
```

**SolidJS SPA:**

```
web/
├── package.json
├── tsconfig.json
├── vite.config.ts
├── index.html
├── src/
│   ├── main.tsx                        # App bootstrap
│   ├── App.tsx                         # Shell: <TopBar> <LeftMenu> <main><Outlet/></main>
│   ├── theme.css                       # :root custom properties + Pure OLED defaults
│   ├── api/
│   │   ├── client.ts                   # fetch wrapper + typed endpoint helpers
│   │   └── types.ts                    # Server, Library, Item, HubItem shapes
│   ├── components/
│   │   ├── TopBar.tsx + .css
│   │   ├── LeftMenu.tsx + .css
│   │   ├── Card.tsx + .css
│   │   ├── Shelf.tsx + .css
│   │   └── Group.tsx + .css
│   └── pages/
│       ├── Home.tsx + .css
│       ├── Library.tsx + .css           # grid + sort + filter dropdowns
│       └── ItemDetail.tsx + .css        # hero + action row + availability block
└── dist/                                # Vite output (gitignored; embedded into Go)
```

**Gitignore additions** (new file `.gitignore` if not already present):

```
# Vite build output — embedded into lumen.exe, not checked in
web/dist/
web/node_modules/

# Compiled binaries
lumen.exe
probe/probe.exe
```

---

## Task 1: Go HTTP server scaffold with graceful shutdown

**Files:**
- Create: `internal/server/server.go`
- Create: `internal/server/server_test.go`

**Context:** The `Server` struct owns the `http.Server`, the `http.ServeMux`, and a reference to the Plex `Client` and the loaded `Config`. Bind strictly to `127.0.0.1:7832` per spec §16. Graceful shutdown via `server.Shutdown(ctx)` on SIGINT/SIGTERM so `lumen serve` can be Ctrl-C'd cleanly.

- [ ] **Step 1: Write the failing test**

Create `internal/server/server_test.go`:
```go
package server

import (
	"net"
	"net/http"
	"testing"
	"time"
)

func TestServerBindsToLoopbackOnly(t *testing.T) {
	s := New(nil, nil, "127.0.0.1:0") // port 0 = pick any free port
	go func() { _ = s.ListenAndServe() }()
	t.Cleanup(func() { _ = s.Shutdown() })

	// Give it a moment to bind.
	time.Sleep(50 * time.Millisecond)

	addr := s.Addr()
	if addr == "" {
		t.Fatal("Addr() returned empty after ListenAndServe")
	}

	// Confirm the listener is on loopback.
	host, _, err := net.SplitHostPort(addr)
	if err != nil {
		t.Fatal(err)
	}
	if host != "127.0.0.1" {
		t.Errorf("bound to %q; must be 127.0.0.1", host)
	}

	// Confirm /api/health responds 200.
	resp, err := http.Get("http://" + addr + "/api/health")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		t.Errorf("GET /api/health: status %d", resp.StatusCode)
	}
}
```

- [ ] **Step 2: Run the test and confirm it fails**

```bash
go test ./internal/server/...
```
Expected: FAIL — package has no non-test files or `New`, `ListenAndServe`, `Shutdown`, `Addr` not defined.

- [ ] **Step 3: Implement server.go**

Create `internal/server/server.go`:
```go
// Package server hosts Lumen's loopback HTTP server — it serves the embedded
// SolidJS SPA at /, and /api/* routes for Plex proxying.
package server

import (
	"context"
	"net"
	"net/http"
	"time"

	"lumen/internal/config"
	"lumen/internal/plex"
)

// Server bundles the http.Server, mux, Plex client, and loaded config into one
// lifecycle-managed unit.
type Server struct {
	cfg    *config.Config
	plex   *plex.Client
	mux    *http.ServeMux
	http   *http.Server
	ln     net.Listener
}

// New constructs the Server but does not bind yet. addr is in "host:port" form
// (e.g. "127.0.0.1:7832" in production; "127.0.0.1:0" in tests to pick any port).
func New(cfg *config.Config, c *plex.Client, addr string) *Server {
	mux := http.NewServeMux()
	s := &Server{
		cfg:  cfg,
		plex: c,
		mux:  mux,
		http: &http.Server{
			Addr:         addr,
			Handler:      mux,
			ReadTimeout:  15 * time.Second,
			WriteTimeout: 30 * time.Second,
			IdleTimeout:  60 * time.Second,
		},
	}
	s.registerRoutes()
	return s
}

// registerRoutes wires every /api/* endpoint. Later tasks register more.
func (s *Server) registerRoutes() {
	s.mux.HandleFunc("/api/health", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"status":"ok"}`))
	})
}

// ListenAndServe binds and serves. Blocks until Shutdown is called.
func (s *Server) ListenAndServe() error {
	ln, err := net.Listen("tcp", s.http.Addr)
	if err != nil {
		return err
	}
	s.ln = ln
	return s.http.Serve(ln)
}

// Shutdown gracefully stops the server.
func (s *Server) Shutdown() error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	return s.http.Shutdown(ctx)
}

// Addr returns the actual bound address. Useful for tests using port 0.
func (s *Server) Addr() string {
	if s.ln == nil {
		return ""
	}
	return s.ln.Addr().String()
}
```

- [ ] **Step 4: Verify the test passes**

```bash
go test ./internal/server/...
```
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/server/server.go internal/server/server_test.go
git commit -m "feat(server): loopback HTTP server scaffold with /api/health and graceful shutdown"
```

---

## Task 2: Shared API helpers + error types

**Files:**
- Create: `internal/server/api.go`

**Context:** Every JSON handler uses the same plumbing: set `Content-Type: application/json`, marshal, write. Error responses use `{"error": "message"}` shape. Keep this in one place.

- [ ] **Step 1: Implement api.go**

Create `internal/server/api.go`:
```go
package server

import (
	"encoding/json"
	"net/http"

	"lumen/internal/config"
)

// writeJSON marshals v as JSON and writes it with status 200 and the right Content-Type.
func writeJSON(w http.ResponseWriter, v any) {
	writeJSONStatus(w, http.StatusOK, v)
}

func writeJSONStatus(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

// writeError writes {"error": msg} with the given status.
func writeError(w http.ResponseWriter, status int, msg string) {
	writeJSONStatus(w, status, map[string]string{"error": msg})
}

// serverByID looks up a server in the loaded config by MachineIdentifier.
// Returns nil if not found.
func (s *Server) serverByID(id string) *config.Server {
	for i := range s.cfg.Plex.Servers {
		if s.cfg.Plex.Servers[i].MachineIdentifier == id {
			return &s.cfg.Plex.Servers[i]
		}
	}
	return nil
}
```

- [ ] **Step 2: Build to confirm it compiles**

```bash
go build ./internal/server/...
```
Expected: succeeds.

- [ ] **Step 3: Commit**

```bash
git add internal/server/api.go
git commit -m "feat(server): shared JSON helpers and server-by-ID lookup"
```

---

## Task 3: `/api/servers` endpoint

**Files:**
- Create: `internal/server/api_servers.go`
- Create: `internal/server/api_servers_test.go`

**Context:** Returns `[{name, machineIdentifier, baseURL, status}]` for the SPA's left menu + settings. Status is derived from whether `LastGoodConnection` resolves via a quick `HEAD /identity` — but Session 2 doesn't re-probe on every request; it just returns the cached `LastGoodConnection` and a static status of `"connected"` (or `"offline"` if the field is empty). Re-probing on demand comes in Session 3 or 5 as part of Settings' "Refresh connections" button.

- [ ] **Step 1: Write the failing test**

Create `internal/server/api_servers_test.go`:
```go
package server

import (
	"encoding/json"
	"io"
	"net/http"
	"testing"

	"lumen/internal/config"
)

func TestAPIServersReturnsConfiguredServers(t *testing.T) {
	cfg := &config.Config{
		ClientIdentifier: "x",
		Plex: config.PlexConfig{
			Servers: []config.Server{
				{Name: "Stargaze", MachineIdentifier: "abc", LastGoodConnection: "https://a.example"},
				{Name: "", MachineIdentifier: "def", LastGoodConnection: "https://b.example"}, // empty name → falls through
			},
		},
	}
	s := New(cfg, nil, "127.0.0.1:0")
	req, _ := http.NewRequest("GET", "/api/servers", nil)
	w := newResponseRecorder()
	s.mux.ServeHTTP(w, req)

	if w.status != 200 {
		t.Fatalf("status: %d", w.status)
	}
	var got []map[string]any
	body, _ := io.ReadAll(w.body)
	if err := json.Unmarshal(body, &got); err != nil {
		t.Fatal(err)
	}
	if len(got) != 2 {
		t.Fatalf("got %d servers, want 2", len(got))
	}
	if got[0]["name"] != "Stargaze" || got[0]["status"] != "connected" {
		t.Errorf("server 0: %+v", got[0])
	}
	// Empty name should fall through to machineIdentifier — Session 1 finding.
	if got[1]["displayName"] != "def" {
		t.Errorf("server 1 displayName fallback failed: %+v", got[1])
	}
}

// Small in-package httptest replacement so we don't need httptest.Server for unit tests.
type responseRecorder struct {
	status int
	body   *byteBuffer
	headers http.Header
}

type byteBuffer struct{ b []byte }

func (b *byteBuffer) Write(p []byte) (int, error) { b.b = append(b.b, p...); return len(p), nil }
func (b *byteBuffer) Read(p []byte) (int, error) {
	n := copy(p, b.b)
	b.b = b.b[n:]
	if n == 0 {
		return 0, io.EOF
	}
	return n, nil
}

func newResponseRecorder() *responseRecorder {
	return &responseRecorder{status: 200, body: &byteBuffer{}, headers: http.Header{}}
}
func (r *responseRecorder) Header() http.Header { return r.headers }
func (r *responseRecorder) Write(p []byte) (int, error) { return r.body.Write(p) }
func (r *responseRecorder) WriteHeader(s int) { r.status = s }
```

**NOTE:** the `responseRecorder` above is intentional — `httptest.ResponseRecorder` works fine too. Use whichever is cleaner for the subagent. If using `httptest.ResponseRecorder`, replace `newResponseRecorder` calls with `httptest.NewRecorder()` and read `.Body.String()` and `.Code`.

- [ ] **Step 2: Run the test and confirm it fails**

```bash
go test ./internal/server/...
```
Expected: FAIL — route `/api/servers` not registered.

- [ ] **Step 3: Implement api_servers.go**

Create `internal/server/api_servers.go`:
```go
package server

import (
	"net/http"
)

// serverDTO is what the SPA consumes — never the raw config (tokens stay server-side).
type serverDTO struct {
	MachineID   string `json:"machineIdentifier"`
	Name        string `json:"name"`
	DisplayName string `json:"displayName"` // name, or machineIdentifier if name empty
	BaseURL     string `json:"baseURL"`
	Status      string `json:"status"` // "connected" | "offline"
}

// handleServers returns the full server list known to config.
func (s *Server) handleServers(w http.ResponseWriter, r *http.Request) {
	out := make([]serverDTO, 0, len(s.cfg.Plex.Servers))
	for _, srv := range s.cfg.Plex.Servers {
		status := "offline"
		if srv.LastGoodConnection != "" {
			status = "connected"
		}
		name := srv.Name
		display := name
		if display == "" {
			display = srv.MachineIdentifier
		}
		out = append(out, serverDTO{
			MachineID:   srv.MachineIdentifier,
			Name:        name,
			DisplayName: display,
			BaseURL:     srv.LastGoodConnection,
			Status:      status,
		})
	}
	writeJSON(w, out)
}
```

And register it in `server.go` — extend `registerRoutes`:
```go
	s.mux.HandleFunc("/api/servers", s.handleServers)
```

- [ ] **Step 4: Verify tests pass**

```bash
go test ./internal/server/...
```
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/server/api_servers.go internal/server/api_servers_test.go internal/server/server.go
git commit -m "feat(server): /api/servers returns configured servers with displayName fallback"
```

---

## Task 4: `/api/servers/:id/libraries` and library items

**Files:**
- Modify: `internal/server/api_servers.go`
- Modify: `internal/server/api_servers_test.go`

**Context:** Two endpoints:
- `GET /api/servers/<machineID>/libraries` → the libraries on that server.
- `GET /api/servers/<machineID>/libraries/<libraryKey>/items?sort=...&start=0&size=50` → a page of items in a library.

Handled via path prefix `GET /api/servers/` and manual path splitting. No third-party router.

- [ ] **Step 1: Extend the test file**

Append to `internal/server/api_servers_test.go`:
```go
import "strings"

func TestLibrariesAndItemsRoutesNeedLiveClient(t *testing.T) {
	// We can't exercise the real plex.Client without a live server, so this test
	// confirms the router routes these paths and returns 500/404 deterministically
	// rather than crashing. Live integration happens in Task 23.

	cfg := &config.Config{Plex: config.PlexConfig{Servers: []config.Server{{MachineIdentifier: "abc", LastGoodConnection: "https://offline.example", AccessToken: "tok"}}}}
	s := New(cfg, nil, "127.0.0.1:0")

	// Unknown server → 404.
	req, _ := http.NewRequest("GET", "/api/servers/nonexistent/libraries", nil)
	w := newResponseRecorder()
	s.mux.ServeHTTP(w, req)
	if w.status != 404 {
		t.Errorf("unknown server: status %d, want 404", w.status)
	}

	// Invalid sub-path → 404.
	req, _ = http.NewRequest("GET", "/api/servers/abc/something-weird", nil)
	w = newResponseRecorder()
	s.mux.ServeHTTP(w, req)
	if w.status != 404 {
		t.Errorf("bogus sub-path: status %d, want 404", w.status)
	}

	// Known server, libraries path — will attempt to hit live Plex and fail with 502.
	// That's fine for this unit test. Integration test does the real run.
	req, _ = http.NewRequest("GET", "/api/servers/abc/libraries", nil)
	w = newResponseRecorder()
	s.mux.ServeHTTP(w, req)
	if !(w.status == 500 || w.status == 502) {
		t.Errorf("live-call attempt: status %d, want 500 or 502", w.status)
	}
	body := string(w.body.b)
	if !strings.Contains(body, "error") {
		t.Errorf("expected error payload, got %q", body)
	}
}
```

- [ ] **Step 2: Run the test and confirm it fails**

```bash
go test ./internal/server/...
```
Expected: FAIL — route not registered.

- [ ] **Step 3: Implement the server-scoped router**

Modify `internal/server/api_servers.go` — add:
```go
import (
	"net/http"
	"strconv"
	"strings"

	"lumen/internal/config"
	"lumen/internal/plex"
)

// handleServerScoped dispatches everything under /api/servers/<id>/...
// Subpaths: libraries, libraries/<key>/items.
func (s *Server) handleServerScoped(w http.ResponseWriter, r *http.Request) {
	rest := strings.TrimPrefix(r.URL.Path, "/api/servers/")
	parts := strings.Split(rest, "/")
	if len(parts) < 2 {
		writeError(w, http.StatusNotFound, "path too short")
		return
	}
	machineID := parts[0]
	srv := s.serverByID(machineID)
	if srv == nil {
		writeError(w, http.StatusNotFound, "unknown server")
		return
	}
	switch {
	case len(parts) == 2 && parts[1] == "libraries":
		s.handleLibraries(w, r, srv)
	case len(parts) == 4 && parts[1] == "libraries" && parts[3] == "items":
		s.handleLibraryItems(w, r, srv, parts[2])
	default:
		writeError(w, http.StatusNotFound, "unknown server sub-path")
	}
}

func (s *Server) handleLibraries(w http.ResponseWriter, r *http.Request, srv *config.Server) {
	plexSrv := toPlexServer(srv)
	libs, err := s.plex.GetLibraries(plexSrv)
	if err != nil {
		writeError(w, http.StatusBadGateway, err.Error())
		return
	}
	writeJSON(w, libs)
}

func (s *Server) handleLibraryItems(w http.ResponseWriter, r *http.Request, srv *config.Server, libraryKey string) {
	q := r.URL.Query()
	iq := plex.ItemQuery{
		Sort:    q.Get("sort"),
		Filters: map[string]string{},
	}
	if v, err := strconv.Atoi(q.Get("start")); err == nil {
		iq.Start = v
	}
	if v, err := strconv.Atoi(q.Get("size")); err == nil {
		iq.Size = v
	}
	for key := range q {
		if strings.HasPrefix(key, "filter.") {
			iq.Filters[strings.TrimPrefix(key, "filter.")] = q.Get(key)
		}
	}
	plexSrv := toPlexServer(srv)
	items, err := s.plex.GetItems(plexSrv, libraryKey, iq)
	if err != nil {
		writeError(w, http.StatusBadGateway, err.Error())
		return
	}
	writeJSON(w, items)
}

// toPlexServer maps a config.Server to the plex.Server shape the client needs.
// BaseURL is read from LastGoodConnection; AccessToken is propagated.
func toPlexServer(srv *config.Server) *plex.Server {
	return &plex.Server{
		Name:              srv.Name,
		MachineIdentifier: srv.MachineIdentifier,
		AccessToken:       srv.AccessToken,
		BaseURL:           srv.LastGoodConnection,
	}
}
```

Register the route in `server.go` — add to `registerRoutes`:
```go
	s.mux.HandleFunc("/api/servers/", s.handleServerScoped)
```

(Note the trailing slash — this is Go's way to say "prefix match".)

- [ ] **Step 4: Verify tests pass**

```bash
go test ./internal/server/...
```
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/server/api_servers.go internal/server/api_servers_test.go internal/server/server.go
git commit -m "feat(server): /api/servers/:id/libraries and .../items with sort+filter+paging"
```

---

## Task 5: `/api/items/:ratingKey?server=<machineID>` — item detail

**Files:**
- Create: `internal/server/api_items.go`
- Create: `internal/server/api_items_test.go`

**Context:** Spec §12.6 item detail page. The SPA passes both the ratingKey and the origin server via `?server=<machineID>`. Session 2 returns raw Plex metadata — OMDB enrichment lands in Session 5 (noted as TODO in handler).

- [ ] **Step 1: Write the test**

Create `internal/server/api_items_test.go`:
```go
package server

import (
	"net/http"
	"testing"

	"lumen/internal/config"
)

func TestItemsRequiresServerQuery(t *testing.T) {
	cfg := &config.Config{Plex: config.PlexConfig{Servers: []config.Server{{MachineIdentifier: "abc", LastGoodConnection: "https://offline.example", AccessToken: "tok"}}}}
	s := New(cfg, nil, "127.0.0.1:0")

	req, _ := http.NewRequest("GET", "/api/items/12345", nil)
	w := newResponseRecorder()
	s.mux.ServeHTTP(w, req)
	if w.status != 400 {
		t.Errorf("missing server query: status %d, want 400", w.status)
	}

	req, _ = http.NewRequest("GET", "/api/items/12345?server=nonexistent", nil)
	w = newResponseRecorder()
	s.mux.ServeHTTP(w, req)
	if w.status != 404 {
		t.Errorf("unknown server: status %d, want 404", w.status)
	}
}
```

- [ ] **Step 2: Confirm fail**

```bash
go test ./internal/server/...
```
Expected: FAIL.

- [ ] **Step 3: Implement api_items.go**

Create `internal/server/api_items.go`:
```go
package server

import (
	"net/http"
	"strings"
)

func (s *Server) handleItem(w http.ResponseWriter, r *http.Request) {
	ratingKey := strings.TrimPrefix(r.URL.Path, "/api/items/")
	if ratingKey == "" || strings.Contains(ratingKey, "/") {
		writeError(w, http.StatusBadRequest, "ratingKey required")
		return
	}
	machineID := r.URL.Query().Get("server")
	if machineID == "" {
		writeError(w, http.StatusBadRequest, "server query param required")
		return
	}
	srv := s.serverByID(machineID)
	if srv == nil {
		writeError(w, http.StatusNotFound, "unknown server")
		return
	}
	item, err := s.plex.GetItem(toPlexServer(srv), ratingKey)
	if err != nil {
		writeError(w, http.StatusBadGateway, err.Error())
		return
	}
	// TODO Session 5: enrich with OMDB rating if item has an imdb:// GUID and
	// cfg.OMDBKey is set.
	writeJSON(w, item)
}
```

Register in `registerRoutes`:
```go
	s.mux.HandleFunc("/api/items/", s.handleItem)
```

- [ ] **Step 4: Verify and commit**

```bash
go test ./internal/server/...
git add internal/server/api_items.go internal/server/api_items_test.go internal/server/server.go
git commit -m "feat(server): /api/items/:ratingKey scoped to originating server"
```

---

## Task 6: `/api/hubs/:namespace/:slug` with 5-minute in-memory cache

**Files:**
- Create: `internal/server/api_hubs.go`
- Create: `internal/server/api_hubs_test.go`

**Context:** Spec §5.2 — cloud Discover hubs need an in-memory 5-min cache to avoid hammering plex.tv. Cache keyed per `(namespace, slug)`. Session 2 uses this for Home's **Continue Watching** pinned shelf (which spec §12.1 says is `server_hub` merged across servers — that's a *different* thing, driven by per-server onDeck, so the hub endpoint here isn't strictly needed for Session 2's pages). Build it anyway — Session 5's Recommended/Discover pages will consume it.

- [ ] **Step 1: Write test**

Create `internal/server/api_hubs_test.go`:
```go
package server

import (
	"net/http"
	"testing"

	"lumen/internal/config"
)

func TestHubsRouteValidatesNamespace(t *testing.T) {
	cfg := &config.Config{Plex: config.PlexConfig{AccountToken: "tok"}}
	s := New(cfg, nil, "127.0.0.1:0")

	for _, ns := range []string{"bogus", "random", ""} {
		path := "/api/hubs/" + ns + "/whatever"
		req, _ := http.NewRequest("GET", path, nil)
		w := newResponseRecorder()
		s.mux.ServeHTTP(w, req)
		if w.status != 400 {
			t.Errorf("ns=%q: status %d, want 400", ns, w.status)
		}
	}
}

func TestHubsRequiresAccountToken(t *testing.T) {
	cfg := &config.Config{} // no AccountToken
	s := New(cfg, nil, "127.0.0.1:0")
	req, _ := http.NewRequest("GET", "/api/hubs/home/trending-plex", nil)
	w := newResponseRecorder()
	s.mux.ServeHTTP(w, req)
	if w.status != 401 {
		t.Errorf("status %d, want 401", w.status)
	}
}
```

- [ ] **Step 2: Confirm fail**

- [ ] **Step 3: Implement api_hubs.go**

Create `internal/server/api_hubs.go`:
```go
package server

import (
	"net/http"
	"strings"
	"sync"
	"time"

	"lumen/internal/plex"
)

// hubCache is a simple in-memory 5-minute TTL cache keyed by "namespace/slug".
type hubCache struct {
	mu      sync.Mutex
	entries map[string]hubCacheEntry
}

type hubCacheEntry struct {
	items     []plex.HubItem
	expiresAt time.Time
}

func newHubCache() *hubCache { return &hubCache{entries: map[string]hubCacheEntry{}} }

func (c *hubCache) get(key string) ([]plex.HubItem, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	e, ok := c.entries[key]
	if !ok || time.Now().After(e.expiresAt) {
		delete(c.entries, key)
		return nil, false
	}
	return e.items, true
}

func (c *hubCache) set(key string, items []plex.HubItem) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.entries[key] = hubCacheEntry{items: items, expiresAt: time.Now().Add(5 * time.Minute)}
}

func (s *Server) handleHub(w http.ResponseWriter, r *http.Request) {
	if s.cfg.Plex.AccountToken == "" {
		writeError(w, http.StatusUnauthorized, "no account token — run lumen auth")
		return
	}
	rest := strings.TrimPrefix(r.URL.Path, "/api/hubs/")
	parts := strings.SplitN(rest, "/", 2)
	if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
		writeError(w, http.StatusBadRequest, "expected /api/hubs/<namespace>/<slug>")
		return
	}
	namespace, slug := parts[0], parts[1]
	if namespace != "home" && namespace != "watchlist" {
		writeError(w, http.StatusBadRequest, "namespace must be 'home' or 'watchlist'")
		return
	}
	key := namespace + "/" + slug
	if cached, ok := s.hubs.get(key); ok {
		writeJSON(w, cached)
		return
	}
	items, err := s.plex.GetHub(namespace, slug, s.cfg.Plex.AccountToken)
	if err != nil {
		writeError(w, http.StatusBadGateway, err.Error())
		return
	}
	s.hubs.set(key, items)
	writeJSON(w, items)
}
```

Modify `server.go` to own the cache — add to the `Server` struct:
```go
	hubs *hubCache
```

Populate in `New`:
```go
	s.hubs = newHubCache()
```

Register in `registerRoutes`:
```go
	s.mux.HandleFunc("/api/hubs/", s.handleHub)
```

- [ ] **Step 4: Verify and commit**

```bash
go test ./internal/server/...
git add internal/server/api_hubs.go internal/server/api_hubs_test.go internal/server/server.go
git commit -m "feat(server): /api/hubs/:namespace/:slug with 5-min in-memory cache"
```

---

## Task 7: `/api/availability?guid=` — GUID → server match list

**Files:**
- Create: `internal/server/api_availability.go`
- Create: `internal/server/api_availability_test.go`

**Context:** Spec §5.4. Takes a plex.tv GUID, asks every configured server `GET /library/all?guid=<guid>`, and returns the match list with per-server metadata (resolution, codec, size). Session 2 uses this on the Item Detail page's "More Ways to Watch" block.

Server-side Plex API: `GET {server.BaseURL}/library/all?guid=<guid>` with the per-server token. Response is a `MediaContainer` envelope with `Metadata[]` — each entry gives us the ratingKey, and its `Media[0].Part[0]` fields give us resolution/codec/size.

This is the first endpoint that needs a new Plex-client method (`GetAvailability`). Since Session 1 deliberately didn't pre-build every plex.Client method, we add the one we need now.

- [ ] **Step 1: Add the plex client method — write the test first**

Create `internal/plex/availability_test.go`:
```go
package plex

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestGetAvailabilityBuildsURLAndParses(t *testing.T) {
	var gotQuery string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotQuery = r.URL.RawQuery
		w.Write([]byte(`{"MediaContainer":{"Metadata":[
			{"ratingKey":"100","guid":"plex://movie/abc","title":"Dune","Media":[
				{"container":"mkv","videoResolution":"2160","bitrate":25000,
				 "Part":[{"key":"/library/parts/1/1/file.mkv","size":9876543210,"container":"mkv"}]}
			]}
		]}}`))
	}))
	defer srv.Close()

	c := NewClient("id", "1.0.0")
	s := &Server{BaseURL: srv.URL, AccessToken: "srv-tok", Name: "Test"}
	matches, err := c.GetAvailability(s, "plex://movie/abc")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(gotQuery, "guid=plex") {
		t.Errorf("query: %q", gotQuery)
	}
	if len(matches) != 1 {
		t.Fatalf("len=%d", len(matches))
	}
	m := matches[0]
	if m.RatingKey != "100" || m.Resolution != "2160" || m.Container != "mkv" || m.Size != 9876543210 {
		t.Errorf("match: %+v", m)
	}
}
```

- [ ] **Step 2: Implement plex.GetAvailability**

Create `internal/plex/availability.go`:
```go
package plex

import (
	"encoding/json"
	"fmt"
	"net/url"
)

// Match is what GetAvailability returns per server that has a matching GUID.
type Match struct {
	ServerName  string `json:"serverName"`
	MachineID   string `json:"machineIdentifier"`
	RatingKey   string `json:"ratingKey"`
	LibraryName string `json:"libraryName,omitempty"`
	Resolution  string `json:"resolution"` // "2160" | "1080" | "720" | ...
	Container   string `json:"container"`  // "mkv" | "mp4" | ...
	Bitrate     int    `json:"bitrate"`
	Size        int64  `json:"size"`
	Codec       string `json:"codec,omitempty"`
}

type availabilityWire struct {
	MediaContainer struct {
		Metadata []struct {
			RatingKey           string `json:"ratingKey"`
			GUID                string `json:"guid"`
			Title               string `json:"title"`
			LibrarySectionTitle string `json:"librarySectionTitle"`
			Media               []struct {
				Container       string `json:"container"`
				VideoResolution string `json:"videoResolution"`
				Bitrate         int    `json:"bitrate"`
				VideoCodec      string `json:"videoCodec"`
				Part            []struct {
					Size      int64  `json:"size"`
					Container string `json:"container"`
				} `json:"Part"`
			} `json:"Media"`
		} `json:"Metadata"`
	} `json:"MediaContainer"`
}

// GetAvailability queries one server's /library/all?guid=<guid> and returns zero
// or more Match entries. Normal case is 0 (not on this server) or 1 (one copy).
func (c *Client) GetAvailability(s *Server, guid string) ([]Match, error) {
	q := url.Values{"guid": []string{guid}}
	u := s.BaseURL + "/library/all?" + q.Encode()
	req, err := c.NewRequest("GET", u, nil)
	if err != nil {
		return nil, err
	}
	c.SetToken(req, s.AccessToken)
	resp, err := c.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("plex: %s returned status %d", u, resp.StatusCode)
	}
	var mc availabilityWire
	if err := json.NewDecoder(resp.Body).Decode(&mc); err != nil {
		return nil, err
	}
	out := make([]Match, 0, len(mc.MediaContainer.Metadata))
	for _, m := range mc.MediaContainer.Metadata {
		match := Match{
			ServerName:  s.Name,
			MachineID:   s.MachineIdentifier,
			RatingKey:   m.RatingKey,
			LibraryName: m.LibrarySectionTitle,
		}
		if len(m.Media) > 0 {
			match.Container = m.Media[0].Container
			match.Resolution = m.Media[0].VideoResolution
			match.Bitrate = m.Media[0].Bitrate
			match.Codec = m.Media[0].VideoCodec
			if len(m.Media[0].Part) > 0 {
				match.Size = m.Media[0].Part[0].Size
			}
		}
		out = append(out, match)
	}
	return out, nil
}
```

- [ ] **Step 3: Run the plex test**

```bash
go test ./internal/plex/...
```
Expected: PASS (including the new availability test).

- [ ] **Step 4: Now wire the server handler**

Create `internal/server/api_availability_test.go`:
```go
package server

import (
	"net/http"
	"testing"

	"lumen/internal/config"
)

func TestAvailabilityRequiresGUID(t *testing.T) {
	cfg := &config.Config{Plex: config.PlexConfig{Servers: []config.Server{{MachineIdentifier: "abc", LastGoodConnection: "https://offline.example", AccessToken: "tok"}}}}
	s := New(cfg, nil, "127.0.0.1:0")

	req, _ := http.NewRequest("GET", "/api/availability", nil)
	w := newResponseRecorder()
	s.mux.ServeHTTP(w, req)
	if w.status != 400 {
		t.Errorf("status %d, want 400", w.status)
	}
}
```

Create `internal/server/api_availability.go`:
```go
package server

import (
	"net/http"

	"lumen/internal/plex"
)

func (s *Server) handleAvailability(w http.ResponseWriter, r *http.Request) {
	guid := r.URL.Query().Get("guid")
	if guid == "" {
		writeError(w, http.StatusBadRequest, "guid query param required")
		return
	}
	// Walk all configured servers in parallel; any failures just mean that server
	// doesn't have this item (or is offline) — collect whatever succeeds.
	type result struct {
		matches []plex.Match
		err     error
	}
	results := make(chan result, len(s.cfg.Plex.Servers))
	for _, srv := range s.cfg.Plex.Servers {
		srv := srv
		go func() {
			matches, err := s.plex.GetAvailability(toPlexServer(&srv), guid)
			results <- result{matches: matches, err: err}
		}()
	}
	var all []plex.Match
	for range s.cfg.Plex.Servers {
		r := <-results
		if r.err != nil {
			continue // silent per spec — absence = offline or no match
		}
		all = append(all, r.matches...)
	}
	writeJSON(w, all)
}
```

Register in `registerRoutes`:
```go
	s.mux.HandleFunc("/api/availability", s.handleAvailability)
```

- [ ] **Step 5: Verify and commit**

```bash
go test ./...
git add internal/plex/availability.go internal/plex/availability_test.go internal/server/api_availability.go internal/server/api_availability_test.go internal/server/server.go
git commit -m "feat(server): /api/availability fans out to all servers for GUID match"
```

---

## Task 8: `/api/image-proxy` — token-stripping image pass-through

**Files:**
- Create: `internal/server/api_image_proxy.go`
- Create: `internal/server/api_image_proxy_test.go`

**Context:** The most security-critical endpoint in Lumen. Spec §16: "Every poster, backdrop, and metadata request goes through `/api/image-proxy` which strips tokens and proxies server-side." SPA calls `/api/image-proxy?server=<machineID>&path=/library/metadata/12345/thumb/1234567890`. The backend appends the correct per-server token, fetches, and streams the response back. **Under no circumstances does `X-Plex-Token=` appear in the SPA's browser view.**

Implementation: `httputil.ReverseProxy` would be overkill; a manual `http.Get` + `io.Copy` is cleaner. Forward the `Content-Type` and `Cache-Control` headers; set a generous 30-day browser cache since Plex posters are immutable once published.

- [ ] **Step 1: Write the test**

Create `internal/server/api_image_proxy_test.go`:
```go
package server

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"lumen/internal/config"
	"lumen/internal/plex"
)

func TestImageProxyForwardsWithTokenServerSide(t *testing.T) {
	// Fake Plex server — confirm it sees the token, SPA-facing response doesn't.
	var gotToken string
	plexFake := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotToken = r.URL.Query().Get("X-Plex-Token")
		w.Header().Set("Content-Type", "image/jpeg")
		_, _ = w.Write([]byte("FAKE-JPEG-BYTES"))
	}))
	defer plexFake.Close()

	cfg := &config.Config{Plex: config.PlexConfig{
		Servers: []config.Server{{MachineIdentifier: "abc", AccessToken: "secret-token", LastGoodConnection: plexFake.URL}},
	}}
	c := plex.NewClient("id", "1.0.0")
	s := New(cfg, c, "127.0.0.1:0")

	req, _ := http.NewRequest("GET", "/api/image-proxy?server=abc&path=/library/metadata/1/thumb/1", nil)
	w := newResponseRecorder()
	s.mux.ServeHTTP(w, req)

	if w.status != 200 {
		t.Fatalf("status %d", w.status)
	}
	if gotToken != "secret-token" {
		t.Errorf("plex server saw token %q, want 'secret-token'", gotToken)
	}
	body, _ := io.ReadAll(w.body)
	if string(body) != "FAKE-JPEG-BYTES" {
		t.Errorf("body: %q", body)
	}
	// The response back to the SPA must NOT contain the token anywhere.
	if strings.Contains(string(body), "secret-token") {
		t.Error("response leaks token")
	}
	if ct := w.headers.Get("Content-Type"); !strings.HasPrefix(ct, "image/") {
		t.Errorf("Content-Type: %q", ct)
	}
	// Cache-Control should be long — posters don't change.
	if cc := w.headers.Get("Cache-Control"); cc == "" {
		t.Error("missing Cache-Control header")
	}
}

func TestImageProxyValidatesPath(t *testing.T) {
	cfg := &config.Config{Plex: config.PlexConfig{Servers: []config.Server{{MachineIdentifier: "abc"}}}}
	s := New(cfg, nil, "127.0.0.1:0")

	cases := []struct {
		path string
		want int
		name string
	}{
		{"?server=abc", 400, "missing path"},
		{"?server=abc&path=http://evil.com/exfil", 400, "absolute URL path"},
		{"?server=abc&path=../../../etc/passwd", 400, "path traversal"},
		{"?server=abc&path=/library/metadata/1/thumb/1", 200, "valid — but 500-502 is also fine since server has no BaseURL"},
		{"?server=nonexistent&path=/library/metadata/1/thumb/1", 404, "unknown server"},
		{"?path=/library/metadata/1/thumb/1", 400, "missing server"},
	}
	for _, tc := range cases {
		req, _ := http.NewRequest("GET", "/api/image-proxy"+tc.path, nil)
		w := newResponseRecorder()
		s.mux.ServeHTTP(w, req)
		if tc.want == 200 && (w.status == 500 || w.status == 502) {
			continue // offline Plex server, handler still rejected correctly
		}
		if w.status != tc.want {
			t.Errorf("%s: status %d, want %d", tc.name, w.status, tc.want)
		}
	}
}
```

- [ ] **Step 2: Confirm fail**

- [ ] **Step 3: Implement api_image_proxy.go**

Create `internal/server/api_image_proxy.go`:
```go
package server

import (
	"io"
	"net/http"
	"net/url"
	"strings"
)

// handleImageProxy takes ?server=<machineID>&path=<plex path> and streams the image.
// The X-Plex-Token is appended server-side; the SPA never sees it.
func (s *Server) handleImageProxy(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	machineID := q.Get("server")
	path := q.Get("path")
	if machineID == "" {
		writeError(w, http.StatusBadRequest, "server query param required")
		return
	}
	if path == "" {
		writeError(w, http.StatusBadRequest, "path query param required")
		return
	}
	// Reject anything that doesn't look like a bare server-relative path.
	// Plex image paths always start with "/library/" or similar.
	if !strings.HasPrefix(path, "/") {
		writeError(w, http.StatusBadRequest, "path must start with /")
		return
	}
	if strings.Contains(path, "..") {
		writeError(w, http.StatusBadRequest, "path traversal not allowed")
		return
	}
	if u, err := url.Parse(path); err == nil && (u.Scheme != "" || u.Host != "") {
		writeError(w, http.StatusBadRequest, "path must be server-relative")
		return
	}
	srv := s.serverByID(machineID)
	if srv == nil {
		writeError(w, http.StatusNotFound, "unknown server")
		return
	}
	if srv.LastGoodConnection == "" {
		writeError(w, http.StatusBadGateway, "server has no cached connection")
		return
	}

	target := srv.LastGoodConnection + path
	if strings.Contains(path, "?") {
		target += "&X-Plex-Token=" + url.QueryEscape(srv.AccessToken)
	} else {
		target += "?X-Plex-Token=" + url.QueryEscape(srv.AccessToken)
	}

	req, err := http.NewRequestWithContext(r.Context(), "GET", target, nil)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		writeError(w, http.StatusBadGateway, err.Error())
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		writeError(w, http.StatusBadGateway, "upstream status "+resp.Status)
		return
	}

	// Forward Content-Type and set a long cache lifetime (posters are immutable).
	if ct := resp.Header.Get("Content-Type"); ct != "" {
		w.Header().Set("Content-Type", ct)
	}
	w.Header().Set("Cache-Control", "public, max-age=2592000, immutable") // 30 days
	w.WriteHeader(http.StatusOK)
	_, _ = io.Copy(w, resp.Body)
}
```

Register in `registerRoutes`:
```go
	s.mux.HandleFunc("/api/image-proxy", s.handleImageProxy)
```

- [ ] **Step 4: Verify and commit**

```bash
go test ./internal/server/...
git add internal/server/api_image_proxy.go internal/server/api_image_proxy_test.go internal/server/server.go
git commit -m "feat(server): /api/image-proxy strips tokens and streams with long cache"
```

---

## Task 9: `/api/play` stub (HTTP 501 until Session 4)

**Files:**
- Create: `internal/server/api_play.go`

**Context:** Spec §3 Session 2 deliverables: "Play button present but stubbed (logs to console; real launch lands in Session 4)." The SPA's play button fires POST `/api/play` with a JSON body; Session 2 returns `501 Not Implemented`. This makes the integration contract visible NOW so Session 4 just has to fill in the body.

- [ ] **Step 1: Implement the stub**

Create `internal/server/api_play.go`:
```go
package server

import "net/http"

func (s *Server) handlePlay(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" {
		writeError(w, http.StatusMethodNotAllowed, "POST required")
		return
	}
	// TODO Session 4: parse {ratingKey, server, subtitleStreamID} and launch Pot Player.
	writeError(w, http.StatusNotImplemented, "playback launches in Session 4")
}
```

Register in `registerRoutes`:
```go
	s.mux.HandleFunc("/api/play", s.handlePlay)
```

- [ ] **Step 2: Build and commit**

```bash
go build ./internal/server/...
git add internal/server/api_play.go internal/server/server.go
git commit -m "feat(server): /api/play stub returns 501 until Session 4"
```

---

## Task 10: SolidJS + Vite + TypeScript scaffold

**Files:**
- Create: `web/package.json`, `web/tsconfig.json`, `web/vite.config.ts`, `web/index.html`
- Create: `web/src/main.tsx`, `web/src/App.tsx`, `web/src/theme.css`

**Context:** `npm create vite@latest` generates a Solid template. We'll initialise it manually to keep control. Entry point is `web/src/main.tsx`; Vite builds into `web/dist/`.

- [ ] **Step 1: Initialise package.json**

Create `web/package.json`:
```json
{
  "name": "lumen-web",
  "private": true,
  "version": "0.1.0",
  "type": "module",
  "scripts": {
    "dev": "vite",
    "build": "vite build",
    "preview": "vite preview"
  },
  "dependencies": {
    "@solidjs/router": "^0.15.0",
    "solid-js": "^1.9.0"
  },
  "devDependencies": {
    "typescript": "^5.6.0",
    "vite": "^6.0.0",
    "vite-plugin-solid": "^2.11.0"
  }
}
```

- [ ] **Step 2: Install dependencies**

```bash
cd web && npm install
```
Expected: creates `web/node_modules/` and `web/package-lock.json` without errors. If npm warns about "old lockfileVersion" or similar, ignore.

- [ ] **Step 3: TypeScript config**

Create `web/tsconfig.json`:
```json
{
  "compilerOptions": {
    "target": "ES2020",
    "module": "ESNext",
    "moduleResolution": "bundler",
    "jsx": "preserve",
    "jsxImportSource": "solid-js",
    "strict": true,
    "esModuleInterop": true,
    "skipLibCheck": true,
    "allowSyntheticDefaultImports": true,
    "isolatedModules": true,
    "noEmit": true,
    "types": ["vite/client"]
  },
  "include": ["src"]
}
```

- [ ] **Step 4: Vite config**

Create `web/vite.config.ts`:
```typescript
import { defineConfig } from "vite";
import solid from "vite-plugin-solid";

export default defineConfig({
  plugins: [solid()],
  build: {
    outDir: "dist",
    emptyOutDir: true,
    sourcemap: false,
  },
  server: {
    // Dev-mode only — proxy /api to the Go backend running on 7832.
    proxy: {
      "/api": "http://127.0.0.1:7832",
    },
  },
});
```

- [ ] **Step 5: index.html**

Create `web/index.html`:
```html
<!DOCTYPE html>
<html lang="en">
  <head>
    <meta charset="UTF-8" />
    <meta name="viewport" content="width=device-width, initial-scale=1.0" />
    <title>Lumen</title>
    <link rel="stylesheet" href="/src/theme.css" />
  </head>
  <body>
    <div id="root"></div>
    <script type="module" src="/src/main.tsx"></script>
  </body>
</html>
```

- [ ] **Step 6: Theme CSS (Pure OLED defaults)**

Create `web/src/theme.css`:
```css
:root {
  /* Pure OLED — spec §14 + Byron's design call (2026-04-24):
     - Black canvas, dark grey left menu, dark navy for any "coloured" surface.
     - Primary action uses INVERSE fill (white bg, black text) per Plezy reference.
     - Body text (descriptions, durations, character names) is muted grey. */
  --bg:             #000000;   /* primary canvas */
  --bg-menu:        #1a1a1a;   /* left menu only */
  --bg-elevated:    #0f1729;   /* dark navy — top bar pill, pills, coloured surfaces */
  --bg-inverse:     #ffffff;   /* inverse surfaces — primary action button, selected tab */
  --text:           #ffffff;   /* primary white — titles, top-bar icons, section headers */
  --text-muted:     #9ca3af;   /* body text — descriptions, durations, dates, character names */
  --text-inverse:   #000000;   /* text/icon on inverse surfaces */
  --menu-icon:      #d1d5db;   /* left menu — chevrons, idle nav links (light grey, distinct from body text) */
  --border:         #262626;   /* hard divider between sections */
  --border-soft:    rgba(255, 255, 255, 0.08); /* in-pill dividers, subtle separators */
  --stroke:         #ffffff;   /* white stroke — hover outlines, secondary button borders */
  --status-online:  #4caf50;   /* connected server indicator */
  --status-offline: #6b7280;   /* offline server indicator */
  --shadow:         0 2px 14px rgba(0, 0, 0, 0.7);

  --font-base:      -apple-system, BlinkMacSystemFont, "Segoe UI", Roboto, Helvetica, Arial, sans-serif;
  --font-size:      14px;

  --radius-sm:      4px;
  --radius-md:      8px;
  --radius-lg:      12px;
  --radius-pill:    24px;      /* top bar pill + circular icon buttons */

  --top-bar-height: 48px;
  --top-bar-margin: 12px;      /* pill's gap from viewport edges */
  --left-menu-width: 220px;
}

* {
  box-sizing: border-box;
}

html, body, #root {
  height: 100%;
  margin: 0;
  padding: 0;
}

body {
  background: var(--bg);
  color: var(--text);
  font-family: var(--font-base);
  font-size: var(--font-size);
  overflow: hidden;
}

a {
  color: var(--text);
  text-decoration: none;
}

a:hover {
  color: var(--text-muted);
}

button {
  background: none;
  color: inherit;
  border: none;
  font: inherit;
  cursor: pointer;
  padding: 0;
}
```

- [ ] **Step 7: Bootstrap**

Create `web/src/main.tsx`:
```tsx
import { render } from "solid-js/web";
import { Router, Route } from "@solidjs/router";
import App from "./App";
import Home from "./pages/Home";
import Library from "./pages/Library";
import ItemDetail from "./pages/ItemDetail";
import "./theme.css";

render(() => (
  <Router root={App}>
    <Route path="/" component={Home} />
    <Route path="/library/:serverID/:libraryID" component={Library} />
    <Route path="/item/:serverID/:ratingKey" component={ItemDetail} />
  </Router>
), document.getElementById("root")!);
```

Create a placeholder `web/src/App.tsx`:
```tsx
import { ParentProps } from "solid-js";

// Task 13/14 populate this with TopBar + LeftMenu + content area.
export default function App(props: ParentProps) {
  return (
    <div style={{ "padding": "20px" }}>
      <h1>Lumen</h1>
      {props.children}
    </div>
  );
}
```

Create empty placeholder pages so main.tsx compiles. `web/src/pages/Home.tsx`:
```tsx
export default function Home() { return <div>Home placeholder — Task 18 populates</div>; }
```

`web/src/pages/Library.tsx`:
```tsx
export default function Library() { return <div>Library placeholder — Task 19 populates</div>; }
```

`web/src/pages/ItemDetail.tsx`:
```tsx
export default function ItemDetail() { return <div>ItemDetail placeholder — Tasks 20–21 populate</div>; }
```

- [ ] **Step 8: Build to confirm everything compiles**

```bash
cd web && npm run build
```
Expected: `vite build` succeeds, writes `web/dist/index.html` + `web/dist/assets/*.js,css`.

- [ ] **Step 9: .gitignore update**

Create or append to `.gitignore` at the repo root:
```gitignore
# Vite build output — embedded into lumen.exe, not checked in
web/dist/
web/node_modules/

# Compiled binaries
lumen.exe
probe/probe.exe
```

- [ ] **Step 10: Commit**

```bash
cd ..
git add web/package.json web/package-lock.json web/tsconfig.json web/vite.config.ts web/index.html web/src/ .gitignore
git commit -m "feat(web): SolidJS + Vite + TypeScript scaffold with Pure OLED theme"
```

---

## Task 11: Embed built SPA + serve at `/`

**Files:**
- Create: `internal/server/spa.go`
- Create: `internal/server/web/dist/index.html` (placeholder — Vite overwrites on `npm run build`)
- Modify: `internal/server/server.go`
- Modify: `web/vite.config.ts`
- Modify: `.gitignore`

**Context:** Go's `//go:embed` directive forbids `..` in paths — the embedded directory must be a sibling or descendant of the Go file with the directive. `web/dist/` at the repo root can't be embedded from `internal/server/`. Fix: change Vite's output directory to `internal/server/web/dist/` so the embed directive can reference it as a local subpath. The SPA code stays under `web/src/` (source of truth); only the build output relocates.

**Critical build order:** `cd web && npm run build` MUST run before `go build`. The compiler checks `//go:embed` paths at compile time — a missing directory breaks the build. We commit a placeholder `index.html` to the embed target so a fresh checkout can still `go build` (it'll just serve the placeholder until someone runs `npm run build`).

- [ ] **Step 1: Change Vite's output directory**

Modify `web/vite.config.ts` — set `build.outDir`:
```typescript
import { defineConfig } from "vite";
import solid from "vite-plugin-solid";

export default defineConfig({
  plugins: [solid()],
  build: {
    outDir: "../internal/server/web/dist",
    emptyOutDir: true,
    sourcemap: false,
  },
  server: {
    proxy: { "/api": "http://127.0.0.1:7832" },
  },
});
```

- [ ] **Step 2: Commit a placeholder index.html so the embed path always exists**

Create `internal/server/web/dist/index.html`:
```html
<!DOCTYPE html>
<title>Lumen</title>
<p>Web assets not built. Run <code>cd web &amp;&amp; npm run build</code>.</p>
```

- [ ] **Step 3: Implement spa.go**

Create `internal/server/spa.go`:
```go
package server

import (
	"embed"
	"io/fs"
	"net/http"
	"strings"
)

//go:embed all:web/dist
var spaFS embed.FS

func spaFilesystem() (fs.FS, error) {
	return fs.Sub(spaFS, "web/dist")
}

// handleSPA serves files from the embedded SPA bundle. For paths that don't
// match an asset (e.g. /library/abc/1), it falls back to index.html so the
// SolidJS client-side router can take over.
func (s *Server) handleSPA(w http.ResponseWriter, r *http.Request) {
	if strings.HasPrefix(r.URL.Path, "/api/") {
		writeError(w, http.StatusNotFound, "unknown endpoint")
		return
	}
	sub, err := spaFilesystem()
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	path := strings.TrimPrefix(r.URL.Path, "/")
	if path == "" {
		path = "index.html"
	}
	if _, err := fs.Stat(sub, path); err != nil {
		path = "index.html"
	}
	http.ServeFileFS(w, r, sub, path)
}
```

- [ ] **Step 4: Register the root handler**

Modify `internal/server/server.go` — in `registerRoutes`, add **at the very end** (after all `/api/*` routes):
```go
	s.mux.HandleFunc("/", s.handleSPA)
```

The `/` prefix match is the catch-all that Go's ServeMux uses for SPA fallback routing.

- [ ] **Step 5: Update .gitignore**

Append to `.gitignore`:
```gitignore
# Vite output lives inside the Go embed target.
# Commit only the placeholder index.html so a fresh checkout builds.
internal/server/web/dist/*
!internal/server/web/dist/index.html

web/node_modules/

lumen.exe
probe/probe.exe
```

- [ ] **Step 6: Rebuild both**

```bash
cd web && npm run build && cd ..
go build -o lumen.exe ./cmd/lumen
```
Expected: Vite overwrites `internal/server/web/dist/` with real assets; Go build succeeds.

- [ ] **Step 7: Run tests**

```bash
go test ./...
```
Expected: every test still passes.

- [ ] **Step 8: Commit**

```bash
git add internal/server/spa.go internal/server/server.go internal/server/web/dist/index.html web/vite.config.ts .gitignore
git commit -m "feat(server): embed SolidJS SPA via //go:embed and serve at /"
```

---

## Task 12: `lumen serve` subcommand

**Files:**
- Create: `cmd/lumen/serve.go`
- Modify: `cmd/lumen/main.go`

**Context:** Ties Session 1's config + plex client + Session 2's server together. On run: load config, assert account token exists, build the plex client, build the server, `ListenAndServe` on `127.0.0.1:7832`, open the default browser to `http://127.0.0.1:7832`, trap SIGINT for graceful shutdown.

- [ ] **Step 1: Implement serve.go**

Create `cmd/lumen/serve.go`:
```go
package main

import (
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/pkg/browser"

	"lumen/internal/config"
	"lumen/internal/plex"
	"lumen/internal/server"
)

func runServe(args []string) {
	cfg, err := config.Load()
	if err != nil {
		fmt.Fprintf(os.Stderr, "load config: %v\n", err)
		os.Exit(1)
	}
	if cfg.Plex.AccountToken == "" {
		fmt.Fprintln(os.Stderr, "no Plex account token — run `lumen auth` first")
		os.Exit(1)
	}
	if len(cfg.Plex.Servers) == 0 {
		fmt.Fprintln(os.Stderr, "no servers discovered — run `lumen list` first")
		os.Exit(1)
	}

	c := plex.NewClient(cfg.ClientIdentifier, version)
	s := server.New(cfg, c, "127.0.0.1:7832")

	errCh := make(chan error, 1)
	go func() { errCh <- s.ListenAndServe() }()

	// Give it 200 ms to bind, then open the browser.
	time.Sleep(200 * time.Millisecond)
	url := "http://127.0.0.1:7832"
	fmt.Printf("Lumen is serving at %s\n", url)
	if err := browser.OpenURL(url); err != nil {
		fmt.Fprintf(os.Stderr, "(couldn't open browser automatically: %v)\n", err)
	}

	// Trap SIGINT / SIGTERM for graceful shutdown.
	sig := make(chan os.Signal, 1)
	signal.Notify(sig, os.Interrupt, syscall.SIGTERM)
	select {
	case <-sig:
		fmt.Println("\nshutting down...")
		_ = s.Shutdown()
	case err := <-errCh:
		fmt.Fprintf(os.Stderr, "server error: %v\n", err)
		os.Exit(1)
	}
}
```

- [ ] **Step 2: Wire the subcommand**

Modify `cmd/lumen/main.go` — add to the `switch os.Args[1]` in `main()`:
```go
	case "serve":
		runServe(os.Args[2:])
```

And in `usage()` add the `serve` line:
```
  serve        Start the Lumen web app (HTTP server on 127.0.0.1:7832)
```

- [ ] **Step 3: Build**

```bash
go build -o lumen.exe ./cmd/lumen
```
Expected: succeeds.

- [ ] **Step 4: Commit**

```bash
git add cmd/lumen/serve.go cmd/lumen/main.go
git commit -m "feat(cli): lumen serve launches HTTP server and opens browser"
```

---

## Task 13: Typed API client in the SPA

**Files:**
- Create: `web/src/api/types.ts`
- Create: `web/src/api/client.ts`

**Context:** Every fetch from the SPA goes through a thin typed wrapper so components don't duplicate URL-building. Types mirror the Go DTOs.

- [ ] **Step 1: Define the types**

Create `web/src/api/types.ts`:
```typescript
export interface Server {
  machineIdentifier: string;
  name: string;
  displayName: string;
  baseURL: string;
  status: "connected" | "offline";
}

export interface Library {
  id: string;
  key: string;
  title: string;
  type: string;
}

export interface Item {
  ratingKey: string;
  guid?: string;
  title: string;
  type: string;
  year?: number;
  summary?: string;
}

export interface HubItem {
  guid?: string;
  ratingKey: string;
  title: string;
  type: string;
  year?: number;
}

export interface Match {
  serverName: string;
  machineIdentifier: string;
  ratingKey: string;
  libraryName?: string;
  resolution: string;
  container: string;
  bitrate: number;
  size: number;
  codec?: string;
}
```

- [ ] **Step 2: Implement client.ts**

Create `web/src/api/client.ts`:
```typescript
import type { Server, Library, Item, HubItem, Match } from "./types";

async function getJSON<T>(url: string): Promise<T> {
  const res = await fetch(url);
  if (!res.ok) {
    const body = await res.text();
    throw new Error(`${res.status} ${url}: ${body}`);
  }
  return res.json() as Promise<T>;
}

export const api = {
  servers: () => getJSON<Server[]>("/api/servers"),

  libraries: (serverID: string) =>
    getJSON<Library[]>(`/api/servers/${encodeURIComponent(serverID)}/libraries`),

  items: (serverID: string, libraryID: string, opts?: { sort?: string; start?: number; size?: number }) => {
    const qs = new URLSearchParams();
    if (opts?.sort) qs.set("sort", opts.sort);
    if (opts?.start !== undefined) qs.set("start", String(opts.start));
    if (opts?.size !== undefined) qs.set("size", String(opts.size));
    const s = qs.toString();
    return getJSON<Item[]>(
      `/api/servers/${encodeURIComponent(serverID)}/libraries/${encodeURIComponent(libraryID)}/items${s ? "?" + s : ""}`
    );
  },

  item: (serverID: string, ratingKey: string) =>
    getJSON<Item>(`/api/items/${encodeURIComponent(ratingKey)}?server=${encodeURIComponent(serverID)}`),

  hub: (namespace: "home" | "watchlist", slug: string) =>
    getJSON<HubItem[]>(`/api/hubs/${namespace}/${encodeURIComponent(slug)}`),

  availability: (guid: string) =>
    getJSON<Match[]>(`/api/availability?guid=${encodeURIComponent(guid)}`),

  // Session 2 just needs the path-building helper — actual images are <img src=...>.
  image: (serverID: string, path: string) =>
    `/api/image-proxy?server=${encodeURIComponent(serverID)}&path=${encodeURIComponent(path)}`,
};
```

- [ ] **Step 3: Verify build**

```bash
cd web && npm run build
```
Expected: succeeds.

- [ ] **Step 4: Commit**

```bash
cd ..
git add web/src/api/
git commit -m "feat(web): typed API client wrapping Lumen's /api endpoints"
```

---

## Task 14: Top bar component — floating pill with back/home/search/zoom/close

**Files:**
- Create: `web/src/components/TopBar.tsx`
- Create: `web/src/components/TopBar.css`

**Context:** Spec §10.1 + Byron's 2026-04-24 design call. The top bar is a **floating pill** — dark navy fill, rounded corners, small margin from viewport edges. Layout (left → right):

`[logo + wordmark]` → `[Back ←]` `[Home 🏠]` → `[search — flex-fill]` → `[Kiosk ⛶]` `[Zoom 🔍 + slider]` → `[Close ✕]`

Soft dividers separate functional groups inside the pill. Zoom slider is **usable in-session** (adjusts `document.documentElement.style.zoom`); Session 3 persists the value.

- [ ] **Step 1: Implement TopBar.tsx**

Create `web/src/components/TopBar.tsx`:
```tsx
import { createSignal } from "solid-js";
import { useNavigate } from "@solidjs/router";
import "./TopBar.css";

export default function TopBar() {
  const navigate = useNavigate();
  const [query, setQuery] = createSignal("");
  const [zoom, setZoom] = createSignal(100);

  function onSearch(e: SubmitEvent) {
    e.preventDefault();
    // Session 5 wires real search; Session 2 just logs.
    console.log("search:", query());
  }

  function applyZoom(v: number) {
    setZoom(v);
    // CSS zoom on :root scales the whole viewport. Session 3 will persist this.
    document.documentElement.style.setProperty("zoom", String(v / 100));
  }

  return (
    <header class="top-bar">
      <div class="top-bar-pill">
        <div class="tb-group tb-brand">
          <span class="logo">✦</span>
          <span class="wordmark">Lumen</span>
        </div>
        <div class="tb-divider" />
        <div class="tb-group tb-nav">
          <button class="icon-btn" title="Back" aria-label="Back" onClick={() => navigate(-1)}>←</button>
          <button class="icon-btn" title="Home" aria-label="Home" onClick={() => navigate("/")}>⌂</button>
        </div>
        <div class="tb-divider" />
        <form class="tb-search" onSubmit={onSearch}>
          <input
            type="search"
            placeholder="Search across servers and Discover..."
            value={query()}
            onInput={(e) => setQuery(e.currentTarget.value)}
            aria-label="Search"
          />
        </form>
        <div class="tb-divider" />
        <div class="tb-group tb-zoom">
          <button class="icon-btn" title="Kiosk mode (Session 5)" aria-label="Kiosk mode">⛶</button>
          <span class="zoom-icon" aria-hidden="true">🔍</span>
          <input
            type="range"
            min="80"
            max="150"
            value={zoom()}
            class="zoom-slider"
            title={`Viewport zoom: ${zoom()}%`}
            onInput={(e) => applyZoom(Number(e.currentTarget.value))}
          />
        </div>
        <div class="tb-divider" />
        <div class="tb-group tb-close">
          <button class="icon-btn" title="Close Lumen" aria-label="Close" onClick={() => window.close()}>✕</button>
        </div>
      </div>
    </header>
  );
}
```

- [ ] **Step 2: Implement TopBar.css**

Create `web/src/components/TopBar.css`:
```css
.top-bar {
  padding: var(--top-bar-margin) var(--top-bar-margin) 0 var(--top-bar-margin);
  position: sticky;
  top: 0;
  z-index: 10;
  background: var(--bg);
}

.top-bar-pill {
  display: flex;
  align-items: center;
  gap: 10px;
  height: var(--top-bar-height);
  padding: 0 14px;
  background: var(--bg-elevated);
  border-radius: var(--radius-pill);
  box-shadow: var(--shadow);
}

.tb-group {
  display: flex;
  align-items: center;
  gap: 6px;
  flex: 0 0 auto;
}

.tb-divider {
  width: 1px;
  align-self: stretch;
  background: var(--border-soft);
  margin: 6px 2px;
}

.tb-brand .logo {
  color: var(--text);
  font-size: 18px;
}

.tb-brand .wordmark {
  color: var(--text);
  font-weight: 600;
  letter-spacing: 0.5px;
  margin-left: 4px;
}

.tb-search {
  display: flex;
  flex: 1 1 auto;
  min-width: 200px;
}

.tb-search input {
  flex: 1 1 auto;
  background: rgba(0, 0, 0, 0.25);
  color: var(--text);
  border: 1px solid transparent;
  border-radius: var(--radius-pill);
  padding: 6px 14px;
  outline: none;
  font-size: 13px;
}

.tb-search input::placeholder {
  color: var(--text-muted);
}

.tb-search input:focus {
  border-color: var(--stroke);
}

.icon-btn {
  width: 32px;
  height: 32px;
  display: inline-grid;
  place-items: center;
  color: var(--text);
  background: transparent;
  border-radius: 50%;
  font-size: 15px;
  line-height: 1;
  transition: background 0.15s ease;
}

.icon-btn:hover {
  background: rgba(255, 255, 255, 0.08);
}

.zoom-icon {
  color: var(--text-muted);
  font-size: 12px;
  margin-left: 4px;
}

.zoom-slider {
  width: 84px;
  accent-color: var(--text); /* white thumb + fill on the track */
}
```

- [ ] **Step 3: Build**

```bash
cd web && npm run build
```

- [ ] **Step 4: Commit**

```bash
cd ..
git add web/src/components/TopBar.tsx web/src/components/TopBar.css
git commit -m "feat(web): TopBar with logo, search bar, kiosk/zoom/close placeholders"
```

---

## Task 15: Left menu + router integration

**Files:**
- Create: `web/src/components/LeftMenu.tsx`
- Create: `web/src/components/LeftMenu.css`
- Modify: `web/src/App.tsx`

**Context:** Spec §10.2. Flat menu: Home, Watchlist (Session 5), Recommended (Session 5), Discover (Session 5), Libraries (expandable — Stargaze + DKNZPLEX groups with library rows), spacer, Settings (Session 3 opens real modal; Session 2 stub). Session 2 implements Home + Libraries live; other menu items navigate to a "Coming in Session N" placeholder route.

- [ ] **Step 1: Implement LeftMenu.tsx**

Create `web/src/components/LeftMenu.tsx`:
```tsx
import { createResource, createSignal, For, Show } from "solid-js";
import { A } from "@solidjs/router";
import { api } from "../api/client";
import type { Library, Server } from "../api/types";
import "./LeftMenu.css";

export default function LeftMenu() {
  const [servers] = createResource(() => api.servers());
  return (
    <nav class="left-menu">
      <ul class="menu-top">
        <li><A href="/" activeClass="active" end>Home</A></li>
        <li><A href="/watchlist" activeClass="active">Watchlist</A></li>
        <li><A href="/recommended" activeClass="active">Recommended</A></li>
        <li><A href="/discover" activeClass="active">Discover</A></li>
      </ul>
      <div class="libraries-section">
        <div class="libraries-label">LIBRARIES</div>
        <Show when={servers()}>
          {(srvs) => (
            <For each={srvs()}>
              {(srv) => <ServerLibraries server={srv} />}
            </For>
          )}
        </Show>
      </div>
      <div class="menu-spacer" />
      <ul class="menu-bottom">
        <li><A href="/settings" activeClass="active">⚙ Settings</A></li>
      </ul>
    </nav>
  );
}

function ServerLibraries(props: { server: Server }) {
  const [libs] = createResource(() => api.libraries(props.server.machineIdentifier));
  const [expanded, setExpanded] = createSignal(true);
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
                <li>
                  <A
                    href={`/library/${props.server.machineIdentifier}/${l.key}`}
                    activeClass="active"
                  >
                    {l.title}
                  </A>
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

- [ ] **Step 2: Implement LeftMenu.css**

Create `web/src/components/LeftMenu.css`:
```css
.left-menu {
  width: var(--left-menu-width);
  background: var(--bg-menu);
  border-right: 1px solid var(--border);
  overflow-y: auto;
  padding: 12px 8px;
  display: flex;
  flex-direction: column;
  /* The top bar pill has its own margin; left menu sits underneath it,
     flush to the left viewport edge. */
  height: calc(100vh - var(--top-bar-height) - var(--top-bar-margin));
}

.menu-top, .menu-bottom, .library-list {
  list-style: none;
  padding: 0;
  margin: 0;
}

.menu-top li a,
.menu-bottom li a,
.library-list li a {
  display: block;
  padding: 6px 10px;
  border-radius: var(--radius-sm);
  color: var(--menu-icon);
}

.menu-top li a:hover,
.menu-bottom li a:hover,
.library-list li a:hover {
  background: var(--bg-elevated);
  color: var(--text);
}

.menu-top li a.active,
.menu-bottom li a.active,
.library-list li a.active {
  background: var(--bg-elevated);
  color: var(--text);
}

.libraries-section {
  margin-top: 20px;
  flex: 1;
  overflow-y: auto;
}

.libraries-label {
  font-size: 10px;
  letter-spacing: 1.5px;
  color: var(--text-muted);
  padding: 0 10px 6px 10px;
}

.server-group-header {
  width: 100%;
  display: flex;
  align-items: center;
  gap: 6px;
  padding: 6px 10px;
  color: var(--text);
  font-weight: 600;
  border-radius: var(--radius-sm);
}

.server-group-header:hover {
  background: var(--bg-elevated);
}

.caret {
  color: var(--menu-icon);
  font-size: 10px;
}

.server-status {
  margin-left: auto;
  width: 8px;
  height: 8px;
  border-radius: 50%;
}
.server-status[data-status="connected"] { background: var(--status-online); }
.server-status[data-status="offline"]   { background: var(--status-offline); }

.library-list li a {
  padding-left: 28px;
  font-size: 13px;
}

.menu-spacer {
  flex: 1;
  min-height: 20px;
}
```

- [ ] **Step 3: Wire TopBar + LeftMenu into App.tsx**

Replace `web/src/App.tsx`:
```tsx
import { ParentProps } from "solid-js";
import TopBar from "./components/TopBar";
import LeftMenu from "./components/LeftMenu";

export default function App(props: ParentProps) {
  return (
    <div class="app-shell">
      <TopBar />
      <div class="app-body">
        <LeftMenu />
        <main class="content">
          {props.children}
        </main>
      </div>
    </div>
  );
}
```

Add shell CSS — modify `web/src/theme.css` — append:
```css
.app-shell {
  display: flex;
  flex-direction: column;
  height: 100vh;
  background: var(--bg);
}

.app-body {
  display: flex;
  flex: 1;
  min-height: 0;
}

.content {
  flex: 1;
  overflow-y: auto;
  padding: 16px 24px;
}
```

- [ ] **Step 4: Build**

```bash
cd web && npm run build
```

- [ ] **Step 5: Commit**

```bash
cd ..
git add web/src/components/LeftMenu.tsx web/src/components/LeftMenu.css web/src/App.tsx web/src/theme.css
git commit -m "feat(web): LeftMenu with routing + server/library tree, wired into App shell"
```

---

## Task 16: Card component

**Files:**
- Create: `web/src/components/Card.tsx`
- Create: `web/src/components/Card.css`

**Context:** Single poster card. Props: title, year, poster path (server-relative), server ID (for building the image proxy URL), link target (item detail). Hover state surfaces a focus ring.

- [ ] **Step 1: Implement Card.tsx**

Create `web/src/components/Card.tsx`:
```tsx
import { A } from "@solidjs/router";
import { api } from "../api/client";
import "./Card.css";

export interface CardProps {
  title: string;
  year?: number;
  thumb?: string;       // server-relative path, e.g. /library/metadata/123/thumb/1234
  serverID: string;
  ratingKey: string;
  subtitle?: string;    // optional: for episodes, "Show Name — E03"
}

export default function Card(props: CardProps) {
  return (
    <A
      class="card"
      href={`/item/${props.serverID}/${props.ratingKey}`}
    >
      <div class="card-poster">
        {props.thumb ? (
          <img
            src={api.image(props.serverID, props.thumb)}
            alt={props.title}
            loading="lazy"
          />
        ) : (
          <div class="card-poster-placeholder">
            <span>{props.title.slice(0, 1)}</span>
          </div>
        )}
      </div>
      <div class="card-meta">
        <div class="card-title">{props.title}</div>
        {props.subtitle && <div class="card-subtitle">{props.subtitle}</div>}
        {props.year && <div class="card-year">{props.year}</div>}
      </div>
    </A>
  );
}
```

- [ ] **Step 2: Implement Card.css**

Create `web/src/components/Card.css`:
```css
.card {
  display: block;
  width: 160px;
  cursor: pointer;
  transition: transform 0.15s ease;
  color: inherit;
}

.card:hover {
  transform: translateY(-2px);
}

.card:hover .card-poster {
  box-shadow: var(--shadow);
  outline: 2px solid var(--stroke);
}

.card-poster {
  aspect-ratio: 2 / 3;
  background: var(--bg-elevated);
  border-radius: var(--radius-sm);
  overflow: hidden;
  position: relative;
}

.card-poster img {
  width: 100%;
  height: 100%;
  object-fit: cover;
  display: block;
}

.card-poster-placeholder {
  width: 100%;
  height: 100%;
  display: grid;
  place-items: center;
  color: var(--text-muted);
  font-size: 32px;
  font-weight: 700;
}

.card-meta {
  padding: 6px 2px;
}

.card-title {
  font-size: 13px;
  color: var(--text);
  line-height: 1.2;
  overflow: hidden;
  text-overflow: ellipsis;
  display: -webkit-box;
  -webkit-line-clamp: 2;
  -webkit-box-orient: vertical;
}

.card-subtitle {
  font-size: 11px;
  color: var(--text-muted);
  margin-top: 2px;
}

.card-year {
  font-size: 11px;
  color: var(--text-muted);
}
```

- [ ] **Step 3: Build + commit**

```bash
cd web && npm run build
cd ..
git add web/src/components/Card.tsx web/src/components/Card.css
git commit -m "feat(web): Card component with lazy image proxy and placeholder"
```

---

## Task 17: Shelf component

**Files:**
- Create: `web/src/components/Shelf.tsx`
- Create: `web/src/components/Shelf.css`

**Context:** Spec §11.1–§11.2. Labelled row of cards that wraps up to N rows (default 3, configurable — Session 3). Collapsible via chevron. Session 2: in-memory collapse state only. Drag-to-reorder is Session 3 (persisted state). Keep the props surface wide enough that Session 3 can just pass extra props without rewriting the component.

- [ ] **Step 1: Implement Shelf.tsx**

Create `web/src/components/Shelf.tsx`:
```tsx
import { createSignal, For, JSX, Show } from "solid-js";
import "./Shelf.css";

export interface ShelfProps {
  id: string;
  title: string;
  rowsPerShelf?: number; // default 3
  children?: JSX.Element; // cards
  initialCollapsed?: boolean;
}

export default function Shelf(props: ShelfProps) {
  const [collapsed, setCollapsed] = createSignal(!!props.initialCollapsed);
  const rowsPerShelf = () => props.rowsPerShelf ?? 3;

  return (
    <section class="shelf" data-shelf-id={props.id}>
      <header class="shelf-header">
        <button
          class="shelf-collapse-btn"
          aria-expanded={!collapsed()}
          onClick={() => setCollapsed(!collapsed())}
        >
          <span class="caret">{collapsed() ? "▸" : "▾"}</span>
          <h2 class="shelf-title">{props.title}</h2>
        </button>
      </header>
      <Show when={!collapsed()}>
        <div
          class="shelf-cards"
          style={{ "--rows-per-shelf": rowsPerShelf() }}
        >
          {props.children}
        </div>
      </Show>
    </section>
  );
}
```

- [ ] **Step 2: Implement Shelf.css**

Create `web/src/components/Shelf.css`:
```css
.shelf {
  margin-bottom: 32px;
}

.shelf-header {
  margin-bottom: 12px;
}

.shelf-collapse-btn {
  display: flex;
  align-items: center;
  gap: 8px;
  color: var(--text);
}

.shelf-collapse-btn:hover {
  color: var(--text-muted);
}

.shelf-collapse-btn .caret {
  color: var(--text-muted);
}

.shelf-title {
  font-size: 18px;
  font-weight: 600;
  margin: 0;
}

/*
 * Cards wrap up to --rows-per-shelf rows without horizontal scroll (spec §11.1).
 * Default 3 rows (set via --rows-per-shelf on inline style).
 * After 3 rows, items are simply not rendered (pagination is a Session 5 concern).
 * For now we cap the container height instead so overflow just hides extras.
 */
.shelf-cards {
  display: grid;
  grid-template-columns: repeat(auto-fill, minmax(160px, 1fr));
  gap: 20px 16px;
  max-height: calc((240px + 60px) * var(--rows-per-shelf, 3));
  overflow: hidden;
}
```

- [ ] **Step 3: Build + commit**

```bash
cd web && npm run build
cd ..
git add web/src/components/Shelf.tsx web/src/components/Shelf.css
git commit -m "feat(web): Shelf component with collapse and multi-row wrap"
```

---

## Task 18: Group component

**Files:**
- Create: `web/src/components/Group.tsx`
- Create: `web/src/components/Group.css`

**Context:** Spec §11.3. Used on the Home page to wrap the Stargaze shelves and DKNZPLEX shelves into collapsible blocks. Single-level only — groups cannot contain groups.

- [ ] **Step 1: Implement Group.tsx + CSS**

Create `web/src/components/Group.tsx`:
```tsx
import { createSignal, JSX, Show } from "solid-js";
import "./Group.css";

export interface GroupProps {
  id: string;
  title: string;
  initialCollapsed?: boolean;
  children?: JSX.Element;
}

export default function Group(props: GroupProps) {
  const [collapsed, setCollapsed] = createSignal(!!props.initialCollapsed);
  return (
    <section class="group" data-group-id={props.id}>
      <button
        class="group-header"
        aria-expanded={!collapsed()}
        onClick={() => setCollapsed(!collapsed())}
      >
        <span class="caret">{collapsed() ? "▸" : "▾"}</span>
        <h1 class="group-title">{props.title}</h1>
      </button>
      <Show when={!collapsed()}>
        <div class="group-body">{props.children}</div>
      </Show>
    </section>
  );
}
```

Create `web/src/components/Group.css`:
```css
.group {
  margin-bottom: 48px;
}

.group-header {
  display: flex;
  align-items: center;
  gap: 12px;
  color: var(--text);
  margin-bottom: 24px;
  padding-bottom: 8px;
  border-bottom: 1px solid var(--border);
  width: 100%;
}

.group-header:hover { color: var(--text-muted); }

.group-title {
  font-size: 24px;
  font-weight: 700;
  margin: 0;
  letter-spacing: 0.5px;
}

.group-body {
  padding-left: 8px;
}
```

- [ ] **Step 2: Build + commit**

```bash
cd web && npm run build
cd ..
git add web/src/components/Group.tsx web/src/components/Group.css
git commit -m "feat(web): Group component for Home page server groupings"
```

---

## Task 19: Home page with all 14 shelves

**Files:**
- Modify: `web/src/pages/Home.tsx`
- Create: `web/src/pages/Home.css`

**Context:** Spec §12.1 shelf definitions:
- Continue Watching (pinned, merged across servers, from per-server onDeck)
- Stargaze group: 7 shelves (Trending Movies, Recently Released Movies, Recently Released Movies 4K, Trending TV Shows, Recently Released Episodes, Recently Released Episodes 4K, Recently Released Anime Episodes)
- DKNZPLEX group: 6 shelves (Recently Released Movies, Recently Released Movies 4K, Recently Released Anime Movies, Recently Released Episodes, Recently Released Episodes 4K, Recently Released Anime Episodes)

To fetch a shelf's contents, we call:
- "Recently Released" → `GET /api/servers/<id>/libraries/<libKey>/items?sort=addedAt:desc&size=20`
- "Trending Movies" / "Trending TV Shows" (Stargaze collections) → need a server-collection call; Session 2 scope does NOT include Plex collections, so we **stub these two shelves** with "coming soon" placeholders and flag for Session 5.
- "Continue Watching" → new `/api/ondeck` endpoint — we need to add it here.

### 19A: Add `/api/ondeck` endpoint

Before the Home page can render CW, the server needs to expose per-server onDeck. Spec §12.1 merges across servers, but the simpler implementation is: SPA fetches each server's onDeck via a new endpoint, interleaves by `lastViewedAt` on the client side.

- [ ] **Step A1: Add Plex client method**

Extend `internal/plex/libraries.go` (or create `internal/plex/ondeck.go`) — add:
```go
// GetOnDeck returns the server's in-progress items, matching Plex Web's Continue
// Watching row for that server. Used by spec §12.1's pinned shelf.
func (c *Client) GetOnDeck(s *Server) ([]Item, error) {
	mc, err := c.serverGet(s, "/library/onDeck", nil)
	if err != nil {
		return nil, err
	}
	return metadataSliceToItems(mc.MediaContainer.Metadata), nil
}
```

- [ ] **Step A2: Add server endpoint**

Modify `internal/server/api_servers.go` — extend `handleServerScoped`:
```go
	case len(parts) == 2 && parts[1] == "ondeck":
		s.handleOnDeck(w, r, srv)
```

Add the handler:
```go
func (s *Server) handleOnDeck(w http.ResponseWriter, r *http.Request, srv *config.Server) {
	items, err := s.plex.GetOnDeck(toPlexServer(srv))
	if err != nil {
		writeError(w, http.StatusBadGateway, err.Error())
		return
	}
	writeJSON(w, items)
}
```

- [ ] **Step A3: Add client helper in the SPA**

Modify `web/src/api/client.ts` — add to the `api` object:
```typescript
  onDeck: (serverID: string) =>
    getJSON<Item[]>(`/api/servers/${encodeURIComponent(serverID)}/ondeck`),
```

- [ ] **Step A4: Commit the groundwork**

```bash
go test ./...
cd web && npm run build && cd ..
git add internal/plex/libraries.go internal/plex/ondeck.go internal/server/api_servers.go web/src/api/client.ts
git commit -m "feat: /api/servers/:id/ondeck + plex.Client.GetOnDeck for Home's Continue Watching"
```

(Create `ondeck.go` if you chose that path; otherwise stage `libraries.go`.)

### 19B: Render Home

- [ ] **Step B1: Implement Home.tsx**

Replace `web/src/pages/Home.tsx`:
```tsx
import { createResource, For, Show } from "solid-js";
import { api } from "../api/client";
import type { Item, Library, Server } from "../api/types";
import Group from "../components/Group";
import Shelf from "../components/Shelf";
import Card from "../components/Card";
import "./Home.css";

// Shelf definitions — one entry per shelf on the Home page (spec §12.1).
// Session 2 stubs shelves that require Plex Collections (Trending Movies / Trending TV Shows).

type ShelfDef =
  | { kind: "ondeck-merged"; id: string; title: string }
  | { kind: "server-recent"; id: string; title: string; serverName: string; libraryName: string }
  | { kind: "stub"; id: string; title: string; reason: string };

const STARGAZE_SHELVES: ShelfDef[] = [
  { kind: "stub", id: "stargaze-trending-movies", title: "Trending Movies", reason: "Plex Collections — Session 5" },
  { kind: "server-recent", id: "stargaze-recent-movies", title: "Recently Released Movies", serverName: "Stargaze", libraryName: "Movies" },
  { kind: "server-recent", id: "stargaze-recent-movies-4k", title: "Recently Released Movies (4K)", serverName: "Stargaze", libraryName: "Movies - 4K" },
  { kind: "stub", id: "stargaze-trending-tv", title: "Trending TV Shows", reason: "Plex Collections — Session 5" },
  { kind: "server-recent", id: "stargaze-recent-episodes", title: "Recently Released Episodes", serverName: "Stargaze", libraryName: "TV Shows" },
  { kind: "server-recent", id: "stargaze-recent-episodes-4k", title: "Recently Released Episodes (4K)", serverName: "Stargaze", libraryName: "TV Shows - 4K" },
  { kind: "server-recent", id: "stargaze-recent-anime", title: "Recently Released Anime Episodes", serverName: "Stargaze", libraryName: "Anime" },
];

const DKNZPLEX_SHELVES: ShelfDef[] = [
  { kind: "server-recent", id: "dknzplex-recent-movies", title: "Recently Released Movies", serverName: "DKNZPLEX", libraryName: "Movies" },
  { kind: "server-recent", id: "dknzplex-recent-movies-4k", title: "Recently Released Movies (4K)", serverName: "DKNZPLEX", libraryName: "Movies - 4K UHD" },
  { kind: "server-recent", id: "dknzplex-recent-anime-movies", title: "Recently Released Anime Movies", serverName: "DKNZPLEX", libraryName: "Movies - Anime" },
  { kind: "server-recent", id: "dknzplex-recent-episodes", title: "Recently Released Episodes", serverName: "DKNZPLEX", libraryName: "TV Shows" },
  { kind: "server-recent", id: "dknzplex-recent-episodes-4k", title: "Recently Released Episodes (4K)", serverName: "DKNZPLEX", libraryName: "TV Shows - 4K HDR" },
  { kind: "server-recent", id: "dknzplex-recent-anime-episodes", title: "Recently Released Anime Episodes", serverName: "DKNZPLEX", libraryName: "TV Shows - Anime" },
];

export default function Home() {
  const [servers] = createResource(() => api.servers());
  return (
    <div class="home-page">
      <Show when={servers()}>
        {(srvs) => (
          <>
            <ContinueWatching servers={srvs() as Server[]} />
            {/* Stargaze group — resolve displayName match, not hardcoded */}
            <ServerGroup srvs={srvs() as Server[]} logicalName="Stargaze" shelves={STARGAZE_SHELVES} />
            <ServerGroup srvs={srvs() as Server[]} logicalName="DKNZPLEX" shelves={DKNZPLEX_SHELVES} />
          </>
        )}
      </Show>
    </div>
  );
}

function ContinueWatching(props: { servers: Server[] }) {
  const [decks] = createResource(
    () => props.servers.map((s) => s.machineIdentifier),
    async (ids) => {
      const results = await Promise.all(ids.map(async (id) => {
        try {
          return { id, items: await api.onDeck(id) };
        } catch {
          return { id, items: [] };
        }
      }));
      // Flatten, tagging each item with its originating serverID.
      return results.flatMap((r) => r.items.map((it) => ({ ...it, serverID: r.id })));
    }
  );
  return (
    <Shelf id="continue-watching" title="Continue Watching">
      <Show when={decks()}>
        {(items) => (
          <For each={items() as (Item & { serverID: string })[]}>
            {(it) => (
              <Card
                title={it.title}
                year={it.year}
                ratingKey={it.ratingKey}
                serverID={it.serverID}
              />
            )}
          </For>
        )}
      </Show>
    </Shelf>
  );
}

function ServerGroup(props: { srvs: Server[]; logicalName: string; shelves: ShelfDef[] }) {
  // Find the server whose displayName contains the logical name (case-insensitive).
  // This gracefully handles Stargaze's empty-name fallback to machineIdentifier.
  const matched = () =>
    props.srvs.find((s) =>
      s.displayName.toLowerCase().includes(props.logicalName.toLowerCase())
    ) ?? props.srvs.find((s) => s.name.toLowerCase() === props.logicalName.toLowerCase());

  return (
    <Group id={`group-${props.logicalName.toLowerCase()}`} title={props.logicalName}>
      <Show when={matched()} fallback={<div class="group-missing">{props.logicalName} not found in servers — run `lumen list`</div>}>
        {(srv) => (
          <For each={props.shelves}>
            {(def) => <ShelfLoader server={srv() as Server} def={def} />}
          </For>
        )}
      </Show>
    </Group>
  );
}

function ShelfLoader(props: { server: Server; def: ShelfDef }) {
  if (props.def.kind === "stub") {
    return (
      <Shelf id={props.def.id} title={props.def.title} initialCollapsed>
        <div class="shelf-stub">({props.def.reason})</div>
      </Shelf>
    );
  }
  if (props.def.kind === "ondeck-merged") {
    return null; // handled by <ContinueWatching />
  }
  // server-recent
  const [libs] = createResource(() => api.libraries(props.server.machineIdentifier));
  return (
    <Shelf id={props.def.id} title={props.def.title}>
      <Show when={libs()}>
        {(libList) => {
          const lib = (libList() as Library[]).find((l) => l.title === props.def.libraryName);
          if (!lib) {
            return <div class="shelf-stub">(library "{props.def.libraryName}" not found on {props.server.displayName})</div>;
          }
          return <LibraryCards server={props.server} libraryKey={lib.key} />;
        }}
      </Show>
    </Shelf>
  );
}

function LibraryCards(props: { server: Server; libraryKey: string }) {
  const [items] = createResource(() =>
    api.items(props.server.machineIdentifier, props.libraryKey, { sort: "addedAt:desc", size: 20 })
  );
  return (
    <Show when={items()} fallback={<div class="shelf-loading">Loading…</div>}>
      {(list) => (
        <For each={list() as Item[]}>
          {(it) => (
            <Card
              title={it.title}
              year={it.year}
              ratingKey={it.ratingKey}
              serverID={props.server.machineIdentifier}
            />
          )}
        </For>
      )}
    </Show>
  );
}
```

- [ ] **Step B2: Add Home.css**

Create `web/src/pages/Home.css`:
```css
.home-page {
  max-width: 1600px;
}

.shelf-stub, .group-missing, .shelf-loading {
  padding: 10px 4px;
  color: var(--text-muted);
  font-style: italic;
}
```

- [ ] **Step B3: Build + commit**

```bash
cd web && npm run build
cd ..
git add web/src/pages/Home.tsx web/src/pages/Home.css
git commit -m "feat(web): Home page with 14 shelves across Stargaze + DKNZPLEX groups"
```

---

## Task 20: Library grid page

**Files:**
- Modify: `web/src/pages/Library.tsx`
- Create: `web/src/pages/Library.css`

**Context:** Spec §12.5. Drill-down from the left menu. Grid of cards. Sticky sort/filter dropdowns above the grid (spec lists: Title / Year Added / Release Year / Rating / Unwatched / Last Viewed; filters: Unwatched only / Genre / Decade). Session 2 wires sort only — filters come in Session 5 once the Plex /library/sections/<id>/all query-param vocab is exercised against real data.

- [ ] **Step 1: Implement Library.tsx**

Replace `web/src/pages/Library.tsx`:
```tsx
import { useParams } from "@solidjs/router";
import { createResource, createSignal, For, Show } from "solid-js";
import { api } from "../api/client";
import type { Item } from "../api/types";
import Card from "../components/Card";
import "./Library.css";

const SORT_OPTIONS = [
  { value: "addedAt:desc", label: "Date Added (newest)" },
  { value: "titleSort:asc", label: "Title (A→Z)" },
  { value: "year:desc",    label: "Release Year (newest)" },
  { value: "rating:desc",  label: "Rating (highest)" },
  { value: "lastViewedAt:desc", label: "Last Viewed" },
];

export default function Library() {
  const params = useParams();
  const [sort, setSort] = createSignal(SORT_OPTIONS[0].value);

  const [items] = createResource(
    () => ({ server: params.serverID, lib: params.libraryID, sort: sort() }),
    ({ server, lib, sort }) => api.items(server, lib, { sort, size: 200 })
  );

  return (
    <div class="library-page">
      <header class="library-header">
        <label>
          Sort:
          <select value={sort()} onChange={(e) => setSort(e.currentTarget.value)}>
            <For each={SORT_OPTIONS}>
              {(o) => <option value={o.value}>{o.label}</option>}
            </For>
          </select>
        </label>
        <Show when={items()}>
          {(list) => <span class="library-count">{(list() as Item[]).length} items</span>}
        </Show>
      </header>
      <div class="library-grid">
        <Show when={items()} fallback={<div class="library-loading">Loading…</div>}>
          {(list) => (
            <For each={list() as Item[]}>
              {(it) => (
                <Card
                  title={it.title}
                  year={it.year}
                  ratingKey={it.ratingKey}
                  serverID={params.serverID}
                />
              )}
            </For>
          )}
        </Show>
      </div>
    </div>
  );
}
```

- [ ] **Step 2: Library.css**

Create `web/src/pages/Library.css`:
```css
.library-page {
  max-width: 1600px;
}

.library-header {
  position: sticky;
  top: 0;
  background: var(--bg);
  display: flex;
  align-items: center;
  gap: 20px;
  padding: 8px 0 16px;
  margin-bottom: 16px;
  border-bottom: 1px solid var(--border);
  z-index: 5;
}

.library-header label {
  display: flex;
  align-items: center;
  gap: 8px;
  color: var(--text-muted);
}

.library-header select {
  background: var(--bg-elevated);
  color: var(--text);
  border: 1px solid var(--border);
  border-radius: var(--radius-sm);
  padding: 6px 10px;
}

.library-count {
  margin-left: auto;
  color: var(--text-muted);
  font-size: 13px;
}

.library-grid {
  display: grid;
  grid-template-columns: repeat(auto-fill, minmax(160px, 1fr));
  gap: 24px 16px;
}

.library-loading {
  padding: 20px;
  color: var(--text-muted);
  font-style: italic;
}
```

- [ ] **Step 3: Build + commit**

```bash
cd web && npm run build
cd ..
git add web/src/pages/Library.tsx web/src/pages/Library.css
git commit -m "feat(web): Library page with sticky sort dropdown and card grid"
```

---

## Task 21: Item Detail page skeleton

**Files:**
- Modify: `web/src/pages/ItemDetail.tsx`
- Create: `web/src/pages/ItemDetail.css`

**Context:** Spec §12.6. Full page with hero banner, title block, action row, overview, episodes (shows — defer to Session 5), availability block, cast/crew (Session 5). Session 2 does **hero + title + action row (stubbed Play button) + overview + availability block**. Hover action for watchlist, mark-watched — all stubbed.

- [ ] **Step 1: Implement ItemDetail.tsx**

Replace `web/src/pages/ItemDetail.tsx`:
```tsx
import { useParams } from "@solidjs/router";
import { createResource, For, Show } from "solid-js";
import { api } from "../api/client";
import type { Item, Match } from "../api/types";
import "./ItemDetail.css";

export default function ItemDetail() {
  const params = useParams();
  const [item] = createResource(
    () => ({ server: params.serverID, rk: params.ratingKey }),
    ({ server, rk }) => api.item(server, rk)
  );
  const [availability] = createResource(
    () => item()?.guid,
    (guid) => (guid ? api.availability(guid) : Promise.resolve([] as Match[]))
  );

  return (
    <div class="item-detail">
      <Show when={item()} fallback={<div class="item-loading">Loading…</div>}>
        {(it) => (
          <>
            <Hero item={it() as Item} serverID={params.serverID} />
            <ActionRow item={it() as Item} serverID={params.serverID} />
            <section class="overview">
              <h3>Overview</h3>
              <p>{(it() as Item).summary ?? "No synopsis available."}</p>
            </section>
            <section class="availability">
              <h3>More Ways to Watch</h3>
              <Show when={availability()} fallback={<div class="availability-loading">Checking other servers…</div>}>
                {(matches) => (
                  <ul>
                    <For each={matches() as Match[]}>
                      {(m) => (
                        <li class="availability-row">
                          <strong>{m.serverName || m.machineIdentifier}</strong>
                          <span class="availability-lib">{m.libraryName}</span>
                          <span class="availability-quality">{m.resolution}p · {m.codec ?? m.container}</span>
                          <span class="availability-size">{formatBytes(m.size)}</span>
                        </li>
                      )}
                    </For>
                    <Show when={(matches() as Match[]).length === 0}>
                      <li class="availability-empty">Not available on any connected server.</li>
                    </Show>
                  </ul>
                )}
              </Show>
            </section>
          </>
        )}
      </Show>
    </div>
  );
}

function Hero(props: { item: Item; serverID: string }) {
  return (
    <header class="hero">
      <div class="hero-meta">
        <h1>{props.item.title}</h1>
        <div class="meta-pills">
          {props.item.year && <span class="pill">{props.item.year}</span>}
          {props.item.type && <span class="pill">{props.item.type}</span>}
          {/* IMDB rating pill lands Session 5 with OMDB integration */}
        </div>
      </div>
    </header>
  );
}

function ActionRow(props: { item: Item; serverID: string }) {
  return (
    <nav class="action-row">
      <button class="btn-primary" onClick={() => launchPlayback(props.item, props.serverID)}>
        ▶ Play
      </button>
      <select class="btn-subtitle" disabled>
        <option>Subtitle: Default</option>
        <option>Off</option>
      </select>
      <button class="btn" disabled title="Session 5">Play Trailer</button>
      <button class="btn" disabled title="Session 4">Mark as Watched</button>
      <button class="btn" disabled title="Session 4">Mark as Unwatched</button>
      <button class="btn" disabled title="Session 5">Add to Watchlist</button>
    </nav>
  );
}

async function launchPlayback(item: Item, serverID: string) {
  console.log("play stub — Session 4 will wire this", { item, serverID });
  try {
    const res = await fetch("/api/play", { method: "POST" });
    console.log("server said:", res.status);
  } catch (e) {
    console.error(e);
  }
}

function formatBytes(n: number): string {
  if (!n) return "";
  const units = ["B", "KB", "MB", "GB", "TB"];
  let u = 0;
  let v = n;
  while (v >= 1024 && u < units.length - 1) { v /= 1024; u++; }
  return `${v.toFixed(1)} ${units[u]}`;
}
```

- [ ] **Step 2: ItemDetail.css**

Create `web/src/pages/ItemDetail.css`:
```css
.item-detail { max-width: 1200px; }

.item-loading { padding: 20px; color: var(--text-muted); }

.hero {
  margin-bottom: 16px;
  padding-bottom: 24px;
  border-bottom: 1px solid var(--border);
}

.hero-meta h1 {
  font-size: 32px;
  margin: 0 0 10px;
  color: var(--text);
}

.meta-pills { display: flex; gap: 8px; }

.pill {
  background: var(--bg-elevated);
  color: var(--text);
  padding: 3px 10px;
  border-radius: var(--radius-pill);
  font-size: 12px;
}

.action-row {
  display: flex;
  flex-wrap: wrap;
  gap: 8px;
  margin: 16px 0 28px;
  align-items: center;
}

/* Secondary buttons — dark navy pill, white content. Matches Plezy's circular
   icon buttons next to the primary action. */
.btn, .btn-subtitle {
  background: var(--bg-elevated);
  color: var(--text);
  border: 1px solid transparent;
  border-radius: var(--radius-pill);
  padding: 8px 14px;
  font-size: 13px;
}

.btn:hover:not(:disabled), .btn-subtitle:hover:not(:disabled) {
  border-color: var(--stroke);
}

/* Primary action — inverse fill (white bg, black text) per Byron's design call. */
.btn-primary {
  background: var(--bg-inverse);
  color: var(--text-inverse);
  border: 1px solid var(--bg-inverse);
  border-radius: var(--radius-pill);
  padding: 8px 18px;
  font-size: 13px;
  font-weight: 600;
}

.btn-primary:hover { opacity: 0.92; }

.btn:disabled, .btn-subtitle:disabled {
  opacity: 0.5;
  cursor: not-allowed;
}

.overview {
  margin-bottom: 28px;
}

.overview h3, .availability h3 {
  font-size: 14px;
  color: var(--text-muted);
  letter-spacing: 1px;
  text-transform: uppercase;
  margin: 0 0 10px;
}

.overview p {
  line-height: 1.6;
  color: var(--text);
  max-width: 80ch;
}

.availability ul { list-style: none; padding: 0; margin: 0; }

.availability-row {
  display: grid;
  grid-template-columns: 160px 1fr 160px 120px;
  gap: 16px;
  padding: 10px 0;
  border-bottom: 1px solid var(--border);
  align-items: center;
}

.availability-row strong { color: var(--text); }
.availability-lib, .availability-quality, .availability-size {
  color: var(--text-muted);
  font-size: 13px;
}

.availability-empty {
  padding: 10px 0;
  color: var(--text-muted);
  font-style: italic;
}

.availability-loading {
  color: var(--text-muted);
  font-style: italic;
  padding: 6px 0;
}
```

- [ ] **Step 3: Build + commit**

```bash
cd web && npm run build
cd ..
git add web/src/pages/ItemDetail.tsx web/src/pages/ItemDetail.css
git commit -m "feat(web): ItemDetail page skeleton with hero, action row, overview, availability"
```

---

## Task 22: Placeholder routes for Watchlist / Recommended / Discover / Settings

**Files:**
- Modify: `web/src/main.tsx`
- Create: `web/src/pages/Placeholder.tsx`

**Context:** The left menu links to Watchlist/Recommended/Discover/Settings but those land in Sessions 3/5. Without routes, the links 404. Register placeholder routes that render a "Coming in Session N" notice.

- [ ] **Step 1: Implement Placeholder.tsx**

Create `web/src/pages/Placeholder.tsx`:
```tsx
import { useLocation } from "@solidjs/router";

export default function Placeholder(props: { name: string; session: string }) {
  const loc = useLocation();
  return (
    <div style={{ "padding": "40px" }}>
      <h1 style={{ "color": "var(--text)" }}>{props.name}</h1>
      <p style={{ "color": "var(--text-muted)", "max-width": "60ch" }}>
        This page lands in <strong>{props.session}</strong>. Current route: <code>{loc.pathname}</code>.
      </p>
    </div>
  );
}
```

- [ ] **Step 2: Wire the routes in main.tsx**

Replace the routes in `web/src/main.tsx`:
```tsx
import { render } from "solid-js/web";
import { Router, Route } from "@solidjs/router";
import App from "./App";
import Home from "./pages/Home";
import Library from "./pages/Library";
import ItemDetail from "./pages/ItemDetail";
import Placeholder from "./pages/Placeholder";
import "./theme.css";

render(() => (
  <Router root={App}>
    <Route path="/" component={Home} />
    <Route path="/library/:serverID/:libraryID" component={Library} />
    <Route path="/item/:serverID/:ratingKey" component={ItemDetail} />
    <Route path="/watchlist"   component={() => <Placeholder name="Watchlist"   session="Session 5" />} />
    <Route path="/recommended" component={() => <Placeholder name="Recommended" session="Session 5" />} />
    <Route path="/discover"    component={() => <Placeholder name="Discover"    session="Session 5" />} />
    <Route path="/settings"    component={() => <Placeholder name="Settings"    session="Session 3" />} />
  </Router>
), document.getElementById("root")!);
```

- [ ] **Step 3: Build + commit**

```bash
cd web && npm run build
cd ..
git add web/src/pages/Placeholder.tsx web/src/main.tsx
git commit -m "feat(web): placeholder routes for Watchlist/Recommended/Discover/Settings"
```

---

## Task 23: End-to-end verification

**Files:**
- No code changes.

**Context:** Session 2's exit criteria (spec §3): `lumen serve` launches; browser opens to `localhost:7832`; Byron navigates every left-menu item + drills into item detail; images render fast via the proxy; DevTools Network tab shows zero raw Plex URLs.

This task is **human-driven** (Byron on his machine). No commit. Deliverable: pasted observations back to Archie so findings can land in `docs/session-2-findings.md`.

- [ ] **Step 1: Full rebuild**

```bash
cd web && npm run build && cd ..
go build -o lumen.exe ./cmd/lumen
```
Expected: both succeed, no warnings.

- [ ] **Step 2: Run tests**

```bash
go test ./...
```
Expected: all tests pass across `internal/config`, `internal/plex`, `internal/server`.

- [ ] **Step 3: Launch the app**

```bash
./lumen.exe serve
```
Expected:
- Console prints `Lumen is serving at http://127.0.0.1:7832`.
- Default browser opens to the Lumen Home page.
- Home page shows two groups (Stargaze + DKNZPLEX) with shelves rendering posters.
- Continue Watching shelf at the top shows in-progress items merged across both servers.
- Trending Movies / Trending TV Shows shelves display "(Plex Collections — Session 5)" stub.

- [ ] **Step 4: Browser smoke checks**

- Click a library in the left menu → navigates to `/library/...` → grid of posters renders.
- Change the sort dropdown → grid re-orders.
- Click a card → navigates to `/item/...` → hero, action row, overview, availability block all render.
- Click "Play" on the action row → console should log `play stub — Session 4 will wire this` and server returns 501.
- Click Watchlist / Recommended / Discover / Settings in left menu → placeholder pages show the right "Session N" label.
- **Hit Ctrl+C in the terminal** → console prints `shutting down...`; process exits cleanly.

- [ ] **Step 5: CRITICAL security check — no raw Plex URLs in browser**

- Reopen Lumen (`./lumen.exe serve`).
- In the browser, open **DevTools** (F12) → **Network** tab.
- **Filter by "plex.direct" and "X-Plex-Token"** — neither string should appear in any request URL.
- Image requests should all be `/api/image-proxy?server=...&path=...` — no tokens visible.
- If you see ANY raw Plex URL or `X-Plex-Token=` in the browser, halt and tell Archie — the image proxy has a leak.

- [ ] **Step 6: Report back**

Paste:
- Did every left-menu item navigate without a crash? List any errors.
- Screenshot (or description) of the Home page.
- DevTools Network tab result — plex.direct/X-Plex-Token count.
- Any slow-loading paths, rendering glitches, or console errors.
- Ctrl+C shutdown clean?

Archie writes up `docs/session-2-findings.md` from the observations.

---

## Design notes for later sessions

Carried forward from Byron's 2026-04-24 design call — enforce when the referenced features land:

- **Session 5 Cast/Crew grid (§12.6):** actor names are primary white (`--text`); **character names are body-text grey** (`--text-muted`). Plezy's reference shows both in white — Byron explicitly wants the character name dropped to body-text to differentiate.
- **Session 5 Episode rows (§12.6 shows):** episode title white; **description, duration, air date are all body-text muted** (`--text-muted`). Matches the Plezy episode-list reference.
- **Session 3 theme variants:** when Dim / High Contrast / Custom themes land, preserve the inverse-primary-action pattern (white fill, black content) as the pattern — only the exact color values swap per theme.
- **Session 3 persistence:** zoom slider currently applies `document.documentElement.style.zoom` in-session only. Persist the last value to `config.json` under a new `ui.zoom` field, restore on `lumen serve` startup.
- **Session 3 per-server display-name override (§13.4):** Session 1's finding — Stargaze returns an empty `name` field. When Settings lands, wire the override so Stargaze's left-menu label and Home group heading read "Stargaze" not `4db54e45876c`.

---

## Self-review checklist (for the executing agent)

Before marking Session 2 done, confirm:

- [ ] Every Task 1–22 step checkbox is ticked.
- [ ] `go test ./...` passes across all packages from the repo root.
- [ ] `cd web && npm run build` succeeds and writes to `internal/server/web/dist/`.
- [ ] `go build ./cmd/lumen` succeeds; `./lumen.exe serve` binds and serves.
- [ ] `/api/image-proxy` never echoes the token in any response body (validated by Task 8 unit test).
- [ ] `//go:embed all:web/dist` in `internal/server/spa.go` compiles (requires `internal/server/web/dist/` exists at build time — placeholder `index.html` is committed).
- [ ] All 22 task-level commits are on `main` with conventional-commit messages.
- [ ] `probe/` and Session 0 findings untouched.
- [ ] Session 1's `lumen auth` / `lumen list` / `lumen probe-hubs` still work unchanged.
