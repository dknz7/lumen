# Lumen — Design Specification

**Date:** 2026-04-24
**Owner:** Byron (zkd)
**Status:** Approved for implementation planning
**Supersedes:** The original `Lumen — Project Brief` for any section where this document contradicts it.

---

## 1. Purpose

Lumen is a personal Windows-only Plex companion. A card-based, OLED-friendly library browser for Byron's two Plex servers — **Stargaze** (a shared-to-him remote library owned by someone else) and **DKNZPLEX** (his own managed VPS). Lumen hands playback off to Pot Player Mini 64-bit and syncs progress back to Plex so resume points, watched status, and Continue Watching behave as though a first-class Plex client were in use. It replaces the official Plex desktop app as Byron's daily driver.

### Non-goals for v1.0

DVR, Live TV, Watch Together, offline downloads, audio track UI, mobile support, Linux/macOS support, multi-user profiles, remote hosting, keyboard shortcuts configuration, rating/review workflows, mid-playback subtitle sync-back to Plex.

---

## 2. Users & Constraints

- Single user (Byron). Not shared with anyone — explicitly not going to his partner.
- Windows 10 or 11 only.
- Pot Player Mini 64-bit, version 260422 (1.7.22859) or compatible.
- Both Plex servers accessed over the public internet (no LAN presence on Byron's desktop). One is a shared library on someone else's server; one is Byron's own VPS.
- Single static Go binary with embedded SPA — one `lumen.exe` Byron double-clicks.
- Config at `%APPDATA%\Lumen\`.
- Plex account + per-server tokens encrypted at rest using Windows DPAPI.
- App binds to `localhost:7832` only.
- 1 Gbps fibre connection — direct-play over WAN is the expected norm.
- Owner of the Stargaze library has explicitly asked Byron not to cause transcoding, for performance reasons.

---

## 3. Session Plan

Six sessions. Session 0 is a go / no-go spike added during brainstorming because the original brief's assumption that Pot Player exposes an HTTP/JSON API was incorrect (that's MPC-HC, not Pot Player).

### Session 0 — Pot Player Control Spike *(half-session, derisking)*

**Goal:** prove that Go can reliably drive Pot Player 260422 via `WM_COPYDATA` before any production code is written.

**Scope:**

- Throwaway Go program under `probe/`.
- Uses `golang.org/x/sys/windows` to:
  - Launch Pot Player against a local test file.
  - Find Pot Player's HWND via `FindWindowW` on window class `PotPlayer64`.
  - Send the documented command IDs from the `rasvob/PotPlayerRemoteAPI` repo via `SendMessageW` with `WM_COPYDATA` and / or Pot Player's registered `0x6000+` command range.
  - Read back: current playback position, media duration, paused / playing state.
  - Detect clean exit and dirty exit (user force-closes Pot Player).

**Exit criteria:** a one-page findings document at `docs/session-0-findings.md` listing which of the four capabilities (position read, state read, duration read, exit detection) work reliably on Pot Player 260422.

**Decision gate:** if position-read and exit-detection both work reliably, the project proceeds to Session 1. If position-read fails, development halts and Byron and Archie discuss pivoting players or dropping the progress-sync feature pillar.

### Session 1 — Foundation & Plex API client

**Deliverables:**

- Go project scaffold:
  - `cmd/lumen/main.go`
  - `internal/plex/` — Plex API client
  - `internal/config/` — config load/save with DPAPI-encrypted fields
  - `internal/potplayer/` — Pot Player control (populated in Session 4; skeleton only here)
  - `web/` — placeholder directory for SPA (populated Session 2)
- Go version: **1.22+**. (To confirm toolchain availability on Byron's machine before starting.)
- Plex PIN-based OAuth flow via `https://plex.tv/api/v2/pins` — opens default browser, user links PIN, Lumen polls for the resulting token.
- DPAPI-encrypted storage of the account token in `%APPDATA%\Lumen\config.json`.
- Stable `X-Plex-Client-Identifier` generated on first run, stored in config, used across all Plex requests. Standard headers sent on every request: `X-Plex-Product: Lumen`, `X-Plex-Version: <semver>`, `X-Plex-Platform: Windows`, `X-Plex-Device: PC`.
- **Connection discovery via `plex.tv/api/v2/resources?includeHttps=1&includeRelay=1`** — returns each server's `machineIdentifier`, per-server access token, and a list of candidate connection URIs (plex.direct / local / relay).
- **Per-server connection picker** — for each server, probes candidate connections in preference order (plex.direct HTTPS → relay), caches the winner for the session, falls back on health-check failure. Last-good connection cached to `config.json` for faster startup.
- Plex client methods (each takes `*Server`, which carries base URL + per-server token):
  - `DiscoverServers(accountToken) []Server`
  - `PickConnection(*Server) Connection`
  - `GetLibraries(*Server) []Library`
  - `GetItems(*Server, libraryID, opts) []Item`
  - `GetItem(*Server, ratingKey) Item`
  - `Search(*Server, query) []Item`
  - `GetHub(namespace, slug string) []HubItem` — single method covering all plex.tv Discover hub calls. `namespace` is `"home"` or `"watchlist"`.
- CLI subcommand `lumen list` — prints both servers, their picked connection URLs, and all libraries on each.

**Verify:** `lumen auth` completes PIN flow end to end. `lumen list` prints Stargaze and DKNZPLEX with every library on each.

### Session 2 — Web app shell & library browsing

**Deliverables:**

- SolidJS project under `web/`, built with Vite, output embedded into the Go binary via `//go:embed`.
- Go HTTP server on `localhost:7832` serves the SPA and `/api/*` routes.
- **Image proxy** endpoint (`/api/image-proxy?path=...&server=...`) so the SPA never sees raw Plex URLs containing `X-Plex-Token=`. The backend holds tokens; the browser holds opaque proxy paths.
- **Full navigation shell** — top bar and left menu (full spec in §10).
- **Shelf-and-card system** — two-level group / shelf model (full spec in §11).
- Home page with all 15 shelves live (full spec in §12.1).
- Library grid views with sort & filter dropdowns.
- **Item detail page** skeleton with availability block (full spec in §12.6).
- Play button present but stubbed (logs to console; real launch lands in Session 4).
- Navigation: browser back / forward works; URL reflects state.

**Verify:** running `lumen` launches, opens default browser to `localhost:7832`, Byron navigates the full left menu and clicks through to item detail pages, images render fast via the proxy, dev-tools Network tab shows no raw Plex URLs.

### Session 3 — Theme system, OLED controls, Settings modal

**Deliverables:**

- Theme tokens as CSS custom properties on `:root`.
- Built-in themes: **Pure OLED** (true black, muted text), **Dim** (near-black), **High Contrast**, **Custom** (user-editable).
- Settings modal (full spec in §13) — overlay with a section nav on the left, detail pane on the right.
- Shelf reorder / collapse / hide state persisted per-page in `config.json`.
- Group collapse state (Stargaze / DKNZPLEX groups on Home) persisted likewise.
- Per-library show/hide toggles live inline in the Libraries section of the left menu (not in Settings).
- Pixel-shift layer: a root-level CSS transform that subtly shifts the viewport a few pixels on a timer. Off, subtle (2 px every 2 min), or aggressive (4 px every 30 s).

**Verify:** Byron can shape the Home page to his liking — reorder groups, collapse shelves, hide libraries — and the state survives restart. Pixel-shift is visibly active on the OLED but not distracting.

### Session 4 — Pot Player launch & progress sync

**Deliverables:**

- `internal/potplayer/` — Win32 IPC client (full spec in §7).
- Stream URL resolution with direct-play-first strategy (full spec in §8).
- Playback session manager with three goroutine tickers (full spec in §9).
- "Now Playing" strip in the top bar: title, thin progress bar, pause/resume toggle, stop button.
- "Mark as watched" action on item detail as the manual safety net if progress sync ever silently fails.

**Verify:**

- Launching a Stargaze episode via Lumen opens Pot Player and starts playback.
- Plex Web's Dashboard shows an active "Now Playing" session titled Lumen with correct progress.
- Closing Pot Player partway through stores an accurate resume point on the server; re-opening the same episode from Lumen resumes from that point.
- Watching past the watched-threshold advances "Continue Watching" to the next episode.

### Session 5 — Watchlist, Recommended, Discover, packaging, kiosk mode

**Deliverables:**

- **Watchlist page** (§12.2) — bookmark grid with filter / sort / count bar.
- **Recommended page** (§12.3) — five shelves, all sourced from watchlist-scoped Plex Discover hubs.
- **Discover page** (§12.4) — eight hand-picked shelves from the plex.tv home hubs.
- **Subtitle picker on item detail** (§12.6.2) — dropdown in the action row with external-sidecar download + `/sub=` flag launch.
- Cross-server search UI.
- System tray icon (Go library: `github.com/getlantern/systray` or `fyne.io/systray`): Open Lumen, Toggle Kiosk Mode, Quit.
- Windows auto-start via an opt-in registry entry in `HKCU\Software\Microsoft\Windows\CurrentVersion\Run`.
- **Kiosk mode** — detects Edge via registry, launches `msedge.exe --app=http://localhost:7832 --start-fullscreen --no-first-run --no-default-browser-check --disable-session-crashed-bubble --disable-features=TranslateUI`. Falls back to Chrome with equivalent flags if Edge is not installed. Tray toggle enters / exits kiosk on demand; config flag launches kiosk on boot.
- Error states: server-unreachable badge with retry-with-backoff; token-expired triggers the PIN flow inline.
- Build pipeline: `go build -ldflags="-H=windowsgui -s -w" -o lumen.exe ./cmd/lumen`.
- Packaged zip with README and changelog.

**Verify:** PC boot → tray icon within 10 s → Edge auto-opens in kiosk fullscreen to Lumen, no browser chrome visible. Force-stopping one of the servers still lets the other work with a clear offline badge. Expired token re-auth flow works without needing a restart.

---

## 4. Architecture Overview

```
Go backend (single lumen.exe)
├── HTTP server on loopback (127.0.0.1:7832)
├── Serves embedded SPA (SolidJS, //go:embed)
├── API routes
│   ├── /api/auth/*           PIN-based OAuth flow
│   ├── /api/servers          list both servers + connection status
│   ├── /api/servers/:id/*    libraries, items, search
│   ├── /api/items/:key       item detail (Plex metadata + enriched with OMDB)
│   ├── /api/watchlist        account-level watchlist
│   ├── /api/hubs/:ns/:slug   Discover hub passthrough (ns ∈ {home, watchlist})
│   ├── /api/availability     GUID → list of (server, ratingKey, quality)
│   ├── /api/play             launches Pot Player for a ratingKey on a chosen server
│   ├── /api/playback         current playback session state (for Now Playing strip)
│   ├── /api/image-proxy      proxies Plex & plex.tv images with token handling
│   ├── /api/imdb/:id         OMDB passthrough with caching
│   └── /api/settings         GET / PUT UI + behaviour config
├── Pot Player subprocess + WM_COPYDATA IPC client
├── Status poller goroutine (5 s tick, active while playing)
├── Plex timeline reporter goroutine (10 s tick, active while playing)
├── Transcode keep-alive goroutine (10 s tick, only when transcoding)
└── System tray (Windows shell integration)

SolidJS frontend (embedded via Go embed.FS)
├── Left menu: Home, Watchlist, Recommended, Discover, Libraries, Settings
├── Top bar: logo · search (60%) · kiosk toggle · zoom slider · close
├── Shelf-and-card system (groups, shelves, cards; drag & drop, collapse, hide)
├── Theme engine (CSS custom properties, live-editable)
├── Pixel-shift layer (root transform, timed)
└── Now Playing strip (top bar, visible while playback active)
```

---

## 5. Plex API Client

### 5.1 Connection discovery

Both servers are remote. Lumen must not assume any LAN reachability.

1. Call `GET https://plex.tv/api/v2/resources?includeHttps=1&includeRelay=1` with the account token.
2. For each returned server (filtered to `product == "Plex Media Server"`):
   - Record `name`, `clientIdentifier` (machine ID), per-server `accessToken`.
   - Enumerate `connections[]`. Each has `uri`, `address`, `port`, `local`, `relay`, `IPv6`, `protocol`.
3. Pick a connection per server using this preference order:
   - Non-relay HTTPS (plex.direct URLs)
   - Relay HTTPS (plex.tv relay)
   - IPv6 variants as lower-priority alternates within each tier
4. Probe the top candidate with a 2 s `HEAD /identity`. If it fails, drop to the next candidate. Cache the winner for the session and persist it under `config.servers[*].lastGoodConnection`.

Re-discovery is triggered on: manual "Refresh connections" in Settings, a 401 from a server request, or an explicit stale-age threshold (24 h).

### 5.2 Plex Discover hub client

All twelve cloud-sourced shelves across Recommended and Discover go through one endpoint family:

```
GET https://discover.provider.plex.tv/hubs/sections/<namespace>/<slug>
    ?contentDirectoryID=<namespace>
    X-Plex-Token: <accountToken>
```

Where `namespace ∈ {home, watchlist}`. Pinned slugs:

- **home/**: `coming-soon`, `recently-released-trailers`, `trending-trailers`, `top_watchlisted` (underscore), `trending-plex`, `blockbuster-trailers`, `highly-anticipated-movies`, `trend-apple-itunes`.
- **watchlist/**: `new-episodes`, `new-trailers`, `coming-soon`, `recently-added`, plus the Pick Up Again slug (to be confirmed during Session 1 — likely `continue-watching` or `on-deck`; position ~1–3 on the Recommended page).

Responses are cached in memory for 5 minutes.

### 5.3 Watchlist

- Source: `https://metadata.provider.plex.tv/library/sections/watchlist/all` with the account token.
- Returns plex.tv GUIDs plus metadata (title, year, poster, type).
- Lumen does not store a local copy beyond the 5-minute memory cache.

### 5.4 Availability resolution

For any plex.tv GUID (or a given server + ratingKey), Lumen determines which of Byron's servers holds a match.

1. For each server, call `GET {server.baseURL}/library/all?guid=<guid>` with the per-server token.
2. For each hit, fetch `/library/metadata/<ratingKey>?includeChildren=1` and extract `Media[0].Part[0]` fields: container, resolution, bitrate, size, codec.
3. Return `[]ServerMatch{ serverName, libraryName, ratingKey, resolution, codec, size }`.
4. Cache per GUID for 5 minutes.

---

## 6. OMDB Integration (IMDB ratings)

- Byron registers a free OMDB API key (`omdbapi.com/apikey.aspx`) and enters it in Settings.
- Stored plaintext in `config.json` (read-only public-facing key; not DPAPI-encrypted by default).
- Lumen calls `GET https://www.omdbapi.com/?i=<imdbId>&apikey=<key>` keyed off the `imdb://` GUID returned by Plex metadata.
- Response cached on disk under `%APPDATA%\Lumen\cache\omdb\<imdbId>.json` for 30 days.
- If the key is blank or OMDB returns an error, the IMDB pill shows "—" instead of a rating. No user-blocking errors.

---

## 7. Pot Player Control (internal/potplayer)

Pot Player does **not** expose an HTTP API. All control is via Win32 inter-process messages.

### 7.1 Client surface

```go
type Client struct { /* HWND, state */ }

func Launch(streamURL string) (*Client, error)     // spawns PotPlayerMini64.exe, waits for HWND
func (c *Client) GetPosition() (time.Duration, error)
func (c *Client) GetDuration() (time.Duration, error)
func (c *Client) GetState() (PlayState, error)     // Playing | Paused | Stopped
func (c *Client) Pause() error
func (c *Client) Resume() error
func (c *Client) Stop() error
func (c *Client) IsAlive() bool                    // IsWindow(hwnd)
```

### 7.2 Implementation details

- Uses `golang.org/x/sys/windows` for `FindWindowW`, `SendMessageW`, `WM_COPYDATA`.
- Window class to match: `PotPlayer64`.
- Command IDs referenced from the `rasvob/PotPlayerRemoteAPI` repo (exact set confirmed in Session 0).
- `Launch` waits up to 3 s for the HWND to appear before returning; times out otherwise.
- `Stop` sends the clean stop command; if `IsAlive()` is still true after 2 s, falls back to `TerminateProcess`.
- Pot Player's install path is auto-detected from `HKCU\Software\DAUM\PotPlayerMini64\ProgramPath` with a Settings override for non-standard installs.

---

## 8. Stream URL Resolution (direct-play-first)

Policy: **always try direct play first**, both servers, no exceptions. Transcoding is an emergency fallback only, never a default. This aligns with Stargaze's owner's performance request and works fine over Byron's 1 Gbps fibre.

### 8.1 Flow

1. Fetch item metadata; extract `Media[0].Part[0]` → `partID`, `container`, file extension.
2. Construct direct-play URL:
   ```
   {server.baseURL}/library/parts/{partID}/{id}/file.{ext}?X-Plex-Token={serverToken}
   ```
3. Launch Pot Player against that URL.
4. Wait up to **10 seconds** for Pot Player to report a non-zero duration via `GetDuration()`.
5. If duration comes back non-zero → direct play succeeded; proceed to Session Manager setup.
6. If Pot Player exits within 10 s without reporting duration, or duration is zero → direct-play failure. Fall back to:
   ```
   {server.baseURL}/video/:/transcode/universal/start.m3u8?...&directPlay=0&directStream=0
     &protocol=hls&videoQuality=100&videoResolution=1920x1080&mediaBufferSize=204800
     &X-Plex-Token={serverToken}&session={random-session-id}
   ```
   with a conservative h264 / AAC profile. The URL's `session` parameter is also used for the transcode keep-alive ticker (§9.3). UI surfaces a "Transcoding" badge so Byron can see when fallback triggered.
7. If the transcode fallback also fails, surface an error toast and halt the session.

---

## 9. Playback Session Manager

A single in-process `PlaybackContext` is active at a time. The UI enforces this — there is no multi-window playback in v1.0.

### 9.1 Context

```go
type PlaybackContext struct {
    RatingKey     string
    Server        *Server
    PartID        string
    Duration      time.Duration
    StartedAt     time.Time
    Transcoding   bool
    TranscodeSession string   // populated only when Transcoding == true
    PotPlayer     *potplayer.Client
}
```

### 9.2 Three concurrent goroutines while active

1. **Status poller** — every 5 s, reads `GetPosition()` and `GetState()` from Pot Player; updates in-memory state; broadcasts to the SPA via Server-Sent Events on `/api/playback/stream`.
2. **Plex timeline reporter** — every 10 s:
   ```
   POST {server.baseURL}/:/timeline?ratingKey=<key>&state=<playing|paused|stopped>
     &time=<positionMs>&duration=<durationMs>
     &X-Plex-Token=<serverToken>
   ```
   Uses the stable `X-Plex-Client-Identifier` from Session 1 so Plex treats every Lumen session as the same device.
3. **Transcode keep-alive** — only active when `Transcoding == true`; every 10 s:
   ```
   POST {server.baseURL}/video/:/transcode/universal/ping?session=<sessionId>
     &X-Plex-Token=<serverToken>
   ```
   Stops Plex from reaping the transcode session.

### 9.3 Exit handling

- On `IsAlive() == false` (detected by status poller): fire one final timeline with `state=stopped` and the last known position, tear down all three goroutines, clear `PlaybackContext`, broadcast "stopped" to the SPA.
- If Pot Player is force-closed dirtily, same flow — the last known position from the previous poll is good enough for Plex to store as the resume point.

---

## 10. Navigation Shell

### 10.1 Top bar (sticky, full width)

| Zone | Contents |
|---|---|
| Left | Lumen logo + wordmark |
| Centre (60%) | Search bar — cross-server + Discover, results split by origin |
| Right | Kiosk toggle · Viewport zoom slider (persisted) · Close Lumen button |

The Now Playing strip overlays the top bar when a session is active, with title, progress bar, pause/resume toggle, stop button.

### 10.2 Left menu (flat — no nesting)

1. Home
2. Watchlist
3. Recommended
4. Discover
5. **Libraries** — scrollable sub-section within the menu; grouped under collapsible **Stargaze** and **DKNZPLEX** headers; each library row has an inline toggle to hide/show that library across Lumen
6. *(spacer)*
7. **Settings** — pinned to the bottom, always visible

---

## 11. Shelf-and-Card System

A **shelf** is a labelled group of posters. A **group** is an optional labelled container holding an ordered list of shelves. Home uses groups to nest shelves under the Stargaze / DKNZPLEX headers; all other pages use ungrouped shelves.

### 11.1 Card behaviour

- Posters wrap up to **N rows per shelf** before paginating (N configurable 1–4 in Settings, default 3).
- This is the key departure from Plex Web's single-row horizontal scroll — Lumen shows more per shelf without side-scrolling.

### 11.2 Shelf behaviour

- **Collapsible** — whole shelf hides to a single-line header with a chevron.
- **Draggable** — reorder within its parent (page or group).
- **Hideable** — remove from view entirely via context menu; reachable in Settings.
- Order + collapsed + hidden state persists per-page in `config.json`.

### 11.3 Group behaviour

- Groups collapse / expand as a unit; their internal shelf order is still draggable.
- Groups cannot be nested inside groups. Single level.

### 11.4 Shelf definition shape

```jsonc
{
  "id": "stargaze-recent-movies-4k",
  "title": "Recently Released Movies (4K)",
  "source": "server_section_recent",
  "params": {
    "server": "Stargaze",
    "libraryName": "Movies - 4K"
  },
  "defaultCollapsed": false,
  "defaultHidden": false
}
```

`source` values:

- `server_hub` — merges a named hub across one or more servers.
- `server_section_recent` — `/library/sections/<id>/recentlyAdded` on a given server + library.
- `server_collection` — `/library/collections/<id>/children` on a given server.
- `plex_discover` — `GetHub(namespace, slug)` passthrough.
- `plex_watchlist` — the flat watchlist endpoint.

---

## 12. Pages

### 12.1 Home

Default order (all reorderable except Continue Watching, which is pinned top):

1. **Continue Watching** *(pinned, ungrouped)* — `server_hub`, merged across Stargaze + DKNZPLEX, sorted by `lastViewedAt` desc. Click resumes playback at the stored position.
2. **Stargaze** *(group, collapsible)*
   1. Trending Movies — `server_collection` on Stargaze named "Trending Movies"
   2. Recently Released Movies — library "Movies"
   3. Recently Released Movies (4K) — library "Movies - 4K"
   4. Trending TV Shows — `server_collection` on Stargaze named "Trending TV Shows"
   5. Recently Released Episodes — library "TV Shows"
   6. Recently Released Episodes (4K) — library "TV Shows - 4K"
   7. Recently Released Anime Episodes — library "Anime"
3. **DKNZPLEX** *(group, collapsible)*
   1. Recently Released Movies — library "Movies"
   2. Recently Released Movies (4K) — library "Movies - 4K UHD"
   3. Recently Released Anime Movies — library "Movies - Anime"
   4. Recently Released Episodes — library "TV Shows"
   5. Recently Released Episodes (4K) — library "TV Shows - 4K HDR"
   6. Recently Released Anime Episodes — library "TV Shows - Anime"

"Recently Released" labels use Plex's native `recentlyAdded` ordering (date added to library) — matches the stock Plex Web behaviour.

Explicitly dropped from Plex Web's Home: What's On Now, Today's Live TV For You, Spring Reawakenings, Binge-Worthy Shows, Recommended For You (On Plex), Trending on Apple TV (kept only on Discover).

### 12.2 Watchlist

- Dedicated left-menu item, static page (not nested under anything).
- Top bar above the grid, left → right:
  1. **Type filter dropdown** — All / Movies / TV Shows
  2. **Sort dropdown** — Date Added (to watchlist) / Title / Release Date
  3. *(right-aligned)* **Total count** — e.g. "483 items"
- Grid below: plex.tv-sourced bookmarks, poster cards.
- Card labelling when filter is "All": `TV Show — <Title> — N seasons`, `TV Show — <Title> — N episodes`, `Movie — <Release Year>`.
- **Hover action:** bin icon on each card removes the item from Watchlist.
- **Remove UX:** silent removal + bottom-left toast with 5 s Undo button. No confirm dialog.
- Click → item detail page.

### 12.3 Recommended

- Dedicated left-menu item, separate from Watchlist.
- **All shelves sourced exclusively from watchlist-scoped Plex Discover hubs.** No generic algorithmic recommendations.
- Shelves (top → bottom):

| # | Shelf | Source |
|---|---|---|
| 1 | **Pick Up Again** | `watchlist/<slug>` (slug pinned Session 1). Bookmark-only — click opens item detail, does **not** auto-resume playback (that's Home's job). |
| 2 | Recently Aired Episodes | `watchlist/new-episodes` |
| 3 | Coming Soon | `watchlist/coming-soon` |
| 4 | New Trailers from Your Watchlist | `watchlist/new-trailers` |
| 5 | Recently Added | `watchlist/recently-added` (shows you've recently added to your Watchlist, series-level) |

### 12.4 Discover

- Dedicated left-menu item, separate from Recommended.
- All shelves sourced from plex.tv's home-namespace Discover hubs. Byron-curated subset, not the full native Plex Discover dump.
- Shelves (in Byron's specified order):

| # | Shelf | `home/` slug |
|---|---|---|
| 1 | Coming Soon | `coming-soon` |
| 2 | New Trailers | `recently-released-trailers` |
| 3 | Trending Trailers | `trending-trailers` |
| 4 | Most Watchlisted This Week | `top_watchlisted` |
| 5 | Trending on Plex | `trending-plex` |
| 6 | Upcoming Blockbusters | `blockbuster-trailers` |
| 7 | Highly Anticipated | `highly-anticipated-movies` |
| 8 | Trending on Apple TV | `trend-apple-itunes` |

- Clicking any Discover card opens item detail. Availability block will usually say "Not available on Stargaze or DKNZPLEX"; primary action becomes **Add to Watchlist**.

### 12.5 Libraries

- Expandable within the left menu itself; no full-page view.
- Selecting a library navigates to a grid view styled like Plex Web's library page.
- Library view sort controls (sticky dropdown above the grid): Title, Year Added, Release Year, Rating, Unwatched, Last Viewed.
- Library view filter controls: Unwatched only, Genre picker, Decade.

### 12.6 Item Detail Page

Visual reference: Plezy's detail page (hero banner + metadata + episodes + cast).

**Layout (top → bottom):**

1. **Hero banner** — full-width landscape backdrop image (proxied). Poster inset on the left for movies; centred banner for shows.
2. **Title block** — title, then a row of metadata pills: `<Year>` · `<MPAA/TV rating>` · `<Duration>` · `IMDB <rating>/10` (icon from IMDB brand, value from OMDB).
3. **Action row** (in order):
   - ▶ **Play / Resume** (primary, filled) — for shows, label reads `S<n>E<n>` (next unwatched episode + its stored resume position); for movies, reads `Resume <mm:ss>` or `Play`.
   - **Subtitle** dropdown (see §12.6.2).
   - **Play Trailer** — uses Plex's trailer extra if present; falls back to "No trailer available" toast.
   - **Mark as Watched**
   - **Mark as Unwatched**
   - **Add to Watchlist** (toggles to "Remove from Watchlist" when already in)
4. **Overview** — synopsis paragraph.
5. **Episodes** (shows only) — season tabs above, vertical list of episode rows below (thumbnail, `E<n>`, title, description, duration, air date, watched checkmark).
6. **More Ways to Watch** — availability block listing every server + library that has this item, each with resolution / codec / size. Clicking an entry **swaps the current detail page's context** to that server's copy (see §12.6.1).
7. **Cast** — grid of actor thumbnails with name and character.
8. **Crew** — grid of crew thumbnails with name and role (directors, writers, etc.).

**Primary Play button logic (shows):**

- If there is a next unwatched episode with non-zero `viewOffset` → "Resume S<n>E<n>" from that offset.
- Else if there is a next unwatched episode with zero `viewOffset` → "Play S<n>E<n>" from start.
- Else (nothing watched or all watched) → "Play from beginning" (S1E1).

**Primary Play button logic (movies):**

- If `viewOffset > 0` and `viewOffset < 90% of duration` → "Resume <mm:ss>".
- Else → "Play".

#### 12.6.1 "More Ways to Watch" — server context swap

Clicking an entry in this block switches the page's **entire context** to that server's copy of the item. The page URL, watched states, resume positions, episode progress all flip to reflect that server's data. No page navigation; same-page re-render.

**Watched-state divergence caveat:** Plex does not sync watched state across servers for the same title. If Byron starts S5E3 on Stargaze and then swaps to DKNZPLEX, DKNZPLEX will show that episode as unwatched (because it is, on that server). The More Ways to Watch row shows a subtle "⚠ progress differs" indicator next to a server entry when its watched-state for the current title disagrees with the currently-viewed server. Tooltipped with a short explanation.

#### 12.6.2 Subtitle picker

Dropdown component sitting second in the action row, immediately after Play/Resume. Populated from the item's Plex metadata (`Media[0].Part[0].Stream[]` filtered to `streamType == 3`).

**Dropdown options (in order):**

1. **Default** *(selected by default)* — launches Pot Player with no `/sub=` flag. Pot Player's global **Preferred Subtitle Language** preference (set to English in Byron's config) picks the embedded track automatically.
2. **Off** — launches Pot Player with subtitles disabled.
3. **External sidecar tracks** — one entry per external `.srt` / `.ass` / `.vtt` subtitle, labelled by language and Plex's title field (e.g. "English", "English (SDH)", "Spanish Forced").

**External track selection flow:**

1. Lumen downloads the chosen subtitle via `GET {server.baseURL}/library/streams/<streamId>?X-Plex-Token=...` to `%TEMP%\lumen\current-<hash>.srt`.
2. Launches Pot Player with `PotPlayerMini64.exe "<streamURL>" /sub="<subPath>"`.
3. Cleans up the sidecar file on session teardown.

**Embedded tracks are intentionally excluded from the dropdown.** Pot Player's CLI has no reliable per-track selector flag, and forcing a specific embedded index would require a `WM_COPYDATA` command that v1.0 doesn't build. If Byron needs a non-default embedded track, he uses Pot Player's native `C` hotkey mid-playback. Listing embedded tracks in the Lumen dropdown would be a misleading no-op, so they're hidden.

**Selection is not synced back to Plex.** Byron's pick applies to this launch only. Plex's stored "selected subtitle" for the item is untouched. This keeps Lumen's subtitle logic one-way and simple.

---

## 13. Settings Modal

Triggered from the bottom-pinned ⚙ Settings item in the left menu. Overlay modal with a vertical section list on the left, detail pane on the right. No route deep-linking.

**1. Appearance**

- Theme: Pure OLED · Dim · High Contrast · Custom
- Card size: S · M · L · XL
- Card density (grid gap slider)
- Default rows per shelf: 1 · 2 · 3 · 4
- Font size (base rem slider)
- Card layout: Poster · Landscape

**2. OLED Protection**

- Pixel-shift: Off · Subtle (2 px / 2 min) · Aggressive (4 px / 30 s)
- Auto-hide chrome when idle: Off · 5 s · 15 s · 30 s · 60 s
- Show/hide nav bar (toggle)
- Show/hide server name in header (toggle)

**3. Startup & Kiosk**

- Launch Lumen on Windows startup (toggle — writes/removes `HKCU\...\Run\Lumen`)
- Launch in kiosk mode on startup (toggle)
- Kiosk browser preference: Edge · Chrome · System default

**4. Accounts & Servers**

- Plex account: show logged-in username · **Re-authenticate** button (runs the PIN flow inline)
- Servers block:
  - **Stargaze** — connection status (plex.direct / relay / offline), last good connection URL
  - **DKNZPLEX** — same
  - **Refresh connections** button (re-runs `plex.tv/api/v2/resources` + probes)
- **OMDB API key** — masked text input, with a "Get a free key" link that opens `omdbapi.com/apikey.aspx` in the default browser. If blank, IMDB pills show "—".

**5. Playback**

- Pot Player executable path — auto-detected from `HKCU\Software\DAUM\PotPlayerMini64\ProgramPath`; override field for non-standard installs
- Direct-play timeout before transcode fallback: **10 s** (read-only display, reflects the fixed policy)

**6. Data & Cache**

- **Clear image cache** button — wipes `%APPDATA%\Lumen\cache\images\`
- **Clear metadata cache** button — wipes `%APPDATA%\Lumen\cache\omdb\` + in-memory hub response cache
- **Clear all cache** button

**7. About**

- Lumen version
- Config file path (click to open parent folder in Explorer)
- Logs folder (click to open in Explorer)
- Link to project repo (when published)

**Deliberately excluded from v1.0:** logout button, keyboard shortcuts configuration, per-library default sort/filter, multi-user switcher.

**Fencing — not Lumen's responsibility (Pot Player's domain):** hardware-accelerated decoding / MadVR renderer config, Windows 11 Auto HDR, subtitle rendering (font, size, positioning), mid-playback subtitle track switching. All configured inside Pot Player itself and passed through untouched. Lumen does pre-select a specific external subtitle for Pot Player at launch time (§12.6.2), but renderer and hotkey behaviour remain Pot Player's.

---

## 14. Theme & OLED Strategy

- All theme tokens live as CSS custom properties on `:root` so a theme swap is a single style recalc.
- **Pure OLED** sets `--bg: #000000`, mutes text to around `#c9c9c9`, eliminates pure white.
- **Pixel-shift** is implemented as a single root-level `transform: translate(x, y)` that animates on a timer. No layout reflow. Does not affect click targets because the whole viewport shifts uniformly.
- **Auto-hide chrome** fades the top bar and left menu to `opacity: 0` + `pointer-events: none` after the configured idle duration; any mouse movement restores them.

---

## 15. Config & Storage

`%APPDATA%\Lumen\` layout:

```
config.json                      # user settings + encrypted tokens
cache/
  images/                        # proxied poster/backdrop cache
  omdb/                          # OMDB rating cache (30 day TTL)
logs/
  lumen.log                      # rotated daily, 7 day retention
```

Scratch/temp under `%TEMP%\lumen\` (transcode session IDs, one-shot downloads). Cleaned on graceful shutdown.

**Encrypted fields in `config.json`** (DPAPI via `golang.org/x/sys/windows`):

- `plex.accountToken`
- `plex.servers[*].accessToken`
- `plex.servers[*].lastGoodConnection` *(not sensitive but encrypted for consistency)*

All other fields plaintext.

---

## 16. Security Boundaries

- Binds to `127.0.0.1:7832` only. No LAN binding.
- SPA never receives raw Plex URLs. Every poster, backdrop, and metadata request goes through `/api/image-proxy` which strips tokens and proxies server-side.
- DPAPI is user-scoped — `config.json` is not portable between Windows users. Documented in the README.
- OMDB key is plaintext (public-facing key, not sensitive).
- No analytics, telemetry, or external metrics. Lumen talks to: Plex (`plex.tv`, both server URLs), OMDB (`omdbapi.com`), and nothing else.

---

## 17. Build & Packaging

- Go 1.22+.
- `go build -ldflags="-H=windowsgui -s -w" -o lumen.exe ./cmd/lumen` produces a single `lumen.exe` with no console window.
- SPA built via `cd web && npm run build` before the Go build, output embedded via `//go:embed`.
- Session 5 produces a zipped release with `lumen.exe`, `README.md`, `CHANGELOG.md`.

---

## 18. Known Gotchas

1. **Pot Player 260422 has no HTTP API.** All control is Win32 IPC. Session 0 exists to derisk this.
2. **Plex transcode decision is non-trivial.** Direct-play is default; transcode fallback is exercised in Session 4 with both paths tested.
3. **`X-Plex-Client-Identifier` must be stable across restarts.** Generated once on first run in Session 1, persisted in `config.json`.
4. **DPAPI is user-scoped.** Backing up `config.json` to a different Windows user will fail to decrypt. Documented in README.
5. **Image proxy must not leak tokens.** Raw Plex URLs containing `X-Plex-Token=` must never appear in the SPA's view of the world.
6. **Kiosk mode Edge flags vary slightly by Edge version.** Session 5 tests against Byron's actual installed Edge build.
7. **Plex does not sync watched state across servers for the same title.** Handled via the "⚠ progress differs" indicator in More Ways to Watch (§12.6.1).
8. **Plex's `recentlyAdded` is by date-added-to-library, not real-world release date.** Labels say "Recently Released" for UX familiarity but the data sort is date-added. Matches stock Plex Web.
9. **OMDB daily quota (1,000 req/day) is more than enough for one user** with 30-day per-item caching.

---

## 19. Post-v1.0 Parking Lot

Deliberately deferred. Byron commits to using Lumen for at least two weeks before adding any of these:

- Embedded subtitle track selection (via `WM_COPYDATA` command)
- Mid-playback subtitle choice sync-back to Plex
- Audio track picker in launch dialog
- Collections browsing as first-class feature
- Keyboard shortcuts
- Remote control from phone browser
- Scrubber preview thumbnails
- Chromecast / DLNA target selection
- Rating / review workflows
- Multi-user profile support
- Shuffle play on item detail

---

## 20. Open Items (to resolve during implementation, not blocking)

- **Pick Up Again slug** — exact `watchlist/<slug>` string (likely `continue-watching` or `on-deck`). Resolved in Session 1 via a quick API probe; pinned in code.
- **Pot Player command IDs** — the exact `0x6000+` offsets for position-read, duration-read, state-read. Confirmed against Pot Player 260422 during Session 0; pinned in `internal/potplayer/commands.go`.
- **Plex "watched threshold" percentage** — Plex's server-side default is 90%; Lumen sends timeline updates and lets the server decide. No client-side threshold needed.
- **Session 0 test video** — Byron to nominate a small local file (MP4, 30 s – 2 min) for the spike. Not a blocker; any file works.

---

## 21. Name, Ports, Paths (locked)

| Setting | Value |
|---|---|
| Product name | **Lumen** |
| Binary | `lumen.exe` |
| Local port | `7832` |
| Config dir | `%APPDATA%\Lumen\` |
| Config file | `%APPDATA%\Lumen\config.json` |
| Cache dir | `%APPDATA%\Lumen\cache\` |
| Logs dir | `%APPDATA%\Lumen\logs\` |
| Scratch dir | `%TEMP%\lumen\` |
| Client identifier | Generated on first run, stored in config |
| Kiosk browser | Microsoft Edge (`--app=` mode); Chrome fallback |
| Pot Player version | 260422 (1.7.22859) or compatible |
| Go version | 1.22+ |
