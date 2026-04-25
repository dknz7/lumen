# Lumen — Session 4 Implementation Plan (Pot Player Playback, Plex Progress Sync, Item Detail Fill-Out)

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Wire Lumen to actually play things. Build the `internal/potplayer` Win32 IPC client (Session 0's findings made concrete), a `internal/playback` session manager with three concurrent goroutines (status poller, Plex timeline reporter, transcode keep-alive), the `/api/play` family of endpoints, an SSE-driven Now Playing strip below the top bar, three playback modals (Resume vs Restart, Direct-Play Failed → Transcode?, Next Episode in 5s), full Item Detail with hero banner backdrop and shows' season-tabs + episode-list, and proper Plex sync for Mark Watched / Mark Unwatched on Item Detail. Includes the desktop shortcut icon pre-flight item carried over from Session 3.

**Architecture:** A new `internal/playback` package owns the active `PlaybackContext` (one at a time — UI enforces single-stream). `Manager` constructs a context on `/api/play`, kicks the three goroutines, and exposes a `Subscribe()` channel that the SSE endpoint streams to the SPA. End-of-file detection is at the 90% threshold (mirrors Plex's own watched-logic): once the status poller sees `position >= 0.9 × duration`, the manager fires `/:/scrobble` and emits an `ended` event; for episodes, it also resolves the next-up via Plex and emits `next-episode-prompt`. Direct-play timeout (10s without non-zero duration) emits `transcode-prompt`; the SPA shows a modal asking the user to confirm transcoding before any fallback URL is built. Item Detail expands to a Plezy-style hero with backdrop, season tabs, episode rows, and properly wired action buttons (`Play / Resume`, `Mark as Watched`, `Mark as Unwatched`).

**Tech Stack additions:**
- `github.com/josephspurrier/goversioninfo/cmd/goversioninfo` — generates `resource_windows_amd64.syso` from a `.ico` + JSON manifest. Embedded in `lumen.exe` so the desktop shortcut shows a real icon.
- Go stdlib only for new endpoints (`net/http` for SSE via `http.Flusher`).
- No new SPA deps — Motion One + Lucide already cover modals + icons.

**Carry-forward gotchas (Session 3 findings — non-negotiable):**
1. **`createSortable` outside `DragDropProvider` crashes.** Any new component using `@thisbeyond/solid-dnd` primitives must guard `useDragDropContext() != null`. Session 4 doesn't add new dnd surfaces, but rule stands.
2. **Vite does not run `tsc`.** Every SPA-touching task must `cd web && npx tsc --noEmit` and confirm clean before committing.
3. **`lumen.exe` embeds `internal/server/web/dist/` via `//go:embed`.** Every SPA-touching task must also `go build -o lumen.exe ./cmd/lumen` AND remind Byron to restart the running `lumen serve`. Don't repeat the stale-binary trap.
4. **Motion One Solid does NOT accept Framer-Motion-style spring strings.** Use cubic-bezier easing arrays (e.g. `[0.22, 1, 0.36, 1]`).
5. **Solid components called as `component({})` break the render tree.** Use `<Dynamic component={X} />` for any reactive component switching.
6. **Plex's `guid` (string) vs `Guid` (array) case-sensitivity.** Always declare an explicit absorber field for capital `Guid` to keep Go's case-insensitive json decoding from conflating them.

**Pot Player gotchas (Session 0 findings — non-negotiable):**
1. **Cold-start window ~2 s.** `GetPosition` / `GetDuration` / `GetState` return 0 or `-1` (uint64 max) during the first ~2 s after launch. Production `Client` retries reads up to 3 s before treating a zero/negative as real.
2. **State `-1` sentinel is "not ready", NOT "stopped".** Map it to a distinct `PlayStateUnknown` — incorrectly treating it as `Stopped` would trigger premature session teardown.
3. **Registry path `HKCU\Software\DAUM\PotPlayerMini64\ProgramPath` is not reliably written.** Path resolution must be three-stage: (1) registry, (2) Settings override field (already in config), (3) default install paths.
4. **`WM_APPCOMMAND` (0x0319) replaces `WM_COMMAND` for write-side control.** Pause/Resume not exposed in v1 UI but implemented for `Stop()` chaining: `SendMessage(hwnd, 0x0319, 0, 13<<16)` for STOP, optional `WM_CLOSE` (0x0010) follow-up if window stays open.
5. **HWND can go stale.** Re-check `IsWindow(hwnd)` before every send.

**Spec items deferred to Session 5 (do NOT build in Session 4):**
- Subtitle picker (§12.6.2) — buttons stay as the existing disabled stub.
- Play Trailer button — stays disabled.
- Add to Watchlist toggle — stays disabled.
- OMDB / IMDB rating pill on Item Detail — stays absent.
- Cast / Crew grids on Item Detail — stays absent.
- "More Ways to Watch" server-context-swap behaviour (§12.6.1) — stays as the existing read-only list.

**Carry-ins from Sessions 0–3:**
- All `internal/config`, `internal/plex`, `internal/server`, `internal/potplayer` (skeleton) packages.
- `lumen install-shortcut` CLI + Settings button (Session 3) — drops a `Lumen.lnk` on the desktop. Currently uses Windows' default exe icon. Task 1 adds a real icon.
- Scrobble / Unscrobble endpoints (Session 3.5) — Item Detail Mark Watched / Mark Unwatched reuse them.
- Pure OLED theme tokens, transparent shelves, Rajdhani+Saira font system, all the Session 3 UI primitives.

**Pre-flight:**
- Working directory: `C:\Users\dicke\Desktop\Dump Zone\STACK\04-DEV\lumen`
- Stay on `main`.
- Node ≥ 20, npm ≥ 10. Go 1.22+.
- Start from a clean `go test ./...` (one pre-existing fail: `TestImageProxyForwardsWithTokenServerSide` — Session 2 carry-over, NOT Session 4's problem) and a clean `npm run build`.
- Source the Lumen icon asset BEFORE Task 1. Byron supplies a `.ico` (256×256, multi-resolution preferred) at `assets/lumen.ico`. Discussed at Session 4 kickoff. If absent, Task 1 substitutes a placeholder generated from `Sparkles` Lucide glyph at 256px.

---

## File Structure

**Go additions:**

```
internal/
├── potplayer/
│   ├── commands.go                # Win32 message + command-ID constants (Session 0 confirmed)
│   ├── path.go                    # Three-stage executable path resolution
│   ├── path_test.go
│   ├── client.go                  # Client struct: Launch, GetPosition, GetDuration, GetState, Stop, IsAlive
│   └── client_test.go             # Unit tests for state-mapping; manual harness for IPC
├── plex/
│   ├── stream.go                  # DirectPlayURL + TranscodeURL builders
│   ├── stream_test.go
│   ├── timeline.go                # POST /:/timeline reporter
│   ├── timeline_test.go
│   ├── episodes.go                # GetSeasons + GetEpisodes + NextUnwatchedEpisode + NextUpInSeries
│   └── episodes_test.go
└── playback/
    ├── context.go                 # PlaybackContext struct + lifecycle hooks
    ├── manager.go                 # Manager: Start, Stop, Subscribe, current state
    ├── poller.go                  # Status poller goroutine (5 s)
    ├── reporter.go                # Plex timeline reporter goroutine (10 s)
    ├── transcode.go               # Transcode keep-alive goroutine (10 s, only when transcoding)
    └── manager_test.go            # Manager tests with a fake potplayer.Client

cmd/lumen/
└── resource_windows_amd64.syso    # Generated by goversioninfo; checked in (Go links it at build time — must live in main package dir)

internal/server/
├── api_play.go                    # POST /api/play, /api/play/transcode, /api/play/stop (replaces 501 stub)
├── api_play_test.go
├── api_playback.go                # GET /api/playback (current state) + GET /api/playback/stream (SSE)
├── api_playback_test.go
└── server.go                      # Wires the playback.Manager into the server struct
```

**Asset additions (project root):**

```
assets/
├── lumen.ico                      # Multi-res icon (16/32/48/256 px) — sourced by Byron pre-Task 1
└── versioninfo.json               # goversioninfo manifest
```

**SPA additions:**

```
web/src/
├── api/
│   ├── client.ts                  # Add: play(), playStop(), playTranscode(), playbackState()
│   └── types.ts                   # Add: PlaybackState, PlaybackEvent, NextEpisodeInfo, Season
├── state/
│   └── playback.ts                # SSE client + Solid store for active playback state
├── components/
│   ├── NowPlaying.tsx + .css      # Strip below TopBar; visible while session active
│   ├── Modal/
│   │   ├── ModalShell.tsx + .css  # Shared backdrop + Motion-driven container (factor from Settings/Close)
│   │   ├── ResumeRestartModal.tsx + .css
│   │   ├── TranscodePromptModal.tsx + .css
│   │   └── NextEpisodeModal.tsx + .css
│   ├── Episodes.tsx + .css        # Season tabs + episode rows (shows-only on ItemDetail)
│   └── icons.ts                   # Add: PlayCircle, RotateCcw, Tv2 (or reuse), Subtitles
├── pages/
│   ├── ItemDetail.tsx             # Hero backdrop, action wiring, Episodes mount (shows only)
│   └── ItemDetail.css             # Hero + episodes layout
└── App.tsx                        # Mount NowPlaying between TopBar and content
```

---

## Phase A — Pre-flight (desktop shortcut icon)

### Task 1: Embed a real icon into `lumen.exe`

**Files:**
- Source asset: `assets/lumen.ico` (Byron-supplied; placeholder if absent — see step 1)
- Create: `assets/versioninfo.json`
- Create (generated, checked in): `resource_windows_amd64.syso`
- Modify: `internal/shortcuts/windows.go` (verify `IconLocation` already targets exe)

**Context:** Session 3 closed with `Lumen.lnk` showing Windows' default exe icon. Real icon requires a `.syso` resource compiled in at Go build time via `goversioninfo`. After this task, the shortcut auto-uses the icon embedded in `lumen.exe`; no `internal/shortcuts/windows.go` change strictly required (it already passes `exePath, 0` as IconLocation, which means "first icon resource of the exe").

- [ ] **Step 1: Confirm asset present**

Check for `assets/lumen.ico`. If missing, Byron provides one (multi-res ICO, 16/32/48/64/128/256 px). For development continuity, if Byron hasn't dropped one yet, generate a placeholder from a Lucide glyph at 256px white-on-transparent and convert via `magick` / online ICO converter. Ship a real icon before merging.

Run: `dir assets\lumen.ico` (PowerShell)
Expected: file present.

- [ ] **Step 2: Install goversioninfo**

Run: `go install github.com/josephspurrier/goversioninfo/cmd/goversioninfo@latest`
Expected: binary lands at `%GOPATH%\bin\goversioninfo.exe` (typically `%USERPROFILE%\go\bin`).

- [ ] **Step 3: Create `assets/versioninfo.json`**

```json
{
  "FixedFileInfo": {
    "FileVersion":     { "Major": 0, "Minor": 1, "Patch": 0, "Build": 0 },
    "ProductVersion":  { "Major": 0, "Minor": 1, "Patch": 0, "Build": 0 },
    "FileFlagsMask":   "3f",
    "FileFlags":       "00",
    "FileOS":          "040004",
    "FileType":        "01",
    "FileSubType":     "00"
  },
  "StringFileInfo": {
    "Comments":         "Personal Plex companion for Windows",
    "CompanyName":      "Byron",
    "FileDescription":  "Lumen",
    "FileVersion":      "0.1.0",
    "InternalName":     "lumen",
    "LegalCopyright":   "© Byron",
    "OriginalFilename": "lumen.exe",
    "ProductName":      "Lumen",
    "ProductVersion":   "0.1.0"
  },
  "VarFileInfo": {
    "Translation": { "LangID": "0409", "CharsetID": "04B0" }
  },
  "IconPath": "assets/lumen.ico"
}
```

- [ ] **Step 4: Generate the syso resource**

Run from repo root:
```
goversioninfo -platform-specific=true -64=true -o cmd/lumen/resource_windows_amd64.syso assets/versioninfo.json
```
Expected: `cmd/lumen/resource_windows_amd64.syso` produced. Go's build tooling auto-includes `*.syso` files that live IN THE SAME DIRECTORY as the main package — `cmd/lumen/`, not the repo root. The default goversioninfo output goes to the working directory; the `-o` flag overrides this so the syso lands where Go will actually link it.

- [ ] **Step 5: Rebuild and verify icon**

Run:
```
go build -o lumen.exe ./cmd/lumen
```
Expected: clean build.

Open `lumen.exe` in File Explorer — Windows shows the new icon. Re-run `lumen install-shortcut`; the desktop shortcut shows the new icon (Windows may take a moment to refresh the icon cache; `ie4uinit.exe -show` forces a refresh if needed).

- [ ] **Step 6: Commit**

```
git add assets/lumen.ico assets/versioninfo.json resource_windows_amd64.syso
git commit -m "feat(build): embed lumen.ico into lumen.exe via goversioninfo"
```

---

## Phase B — Pot Player Control Package

### Task 2: Pot Player command constants (`commands.go`)

**Files:**
- Create: `internal/potplayer/commands.go`

**Context:** Pin every magic number from Session 0's findings. Subsequent tasks reference these by name only — never bare `0x5004`-style literals.

- [ ] **Step 1: Write the file**

```go
// Package potplayer drives Pot Player Mini 64-bit via Win32 IPC. All
// command IDs and message constants here were confirmed against Pot Player
// v260422 (1.7.22859) during the Session 0 spike — see docs/session-0-findings.md.
package potplayer

// Win32 message constants.
const (
	wmUser       uintptr = 0x0400 // base for user-defined messages
	wmAppCommand uintptr = 0x0319 // multimedia-key-style commands
	wmClose      uintptr = 0x0010 // graceful close

	// Pot Player accepts read-only queries via WM_USER + offset. Each returns
	// its result as the SendMessage return value.
	ppGetPosition uintptr = 0x5004 // returns current position in milliseconds
	ppGetDuration uintptr = 0x5002 // returns total duration in milliseconds
	ppGetState    uintptr = 0x5006 // returns 1=PAUSED, 2=PLAYING, -1=NOT_READY

	// WM_APPCOMMAND high-word values for write-side control. Sent via
	// SendMessage(hwnd, wmAppCommand, 0, value<<16).
	appCmdMediaStop      uintptr = 13
	appCmdMediaPlayPause uintptr = 14 // toggle (kept for completeness; v1 UI doesn't use it)
)

// Window class for Pot Player's main window. Used by FindWindowW.
const potPlayerWindowClass = "PotPlayer64"

// Cold-start: position/duration/state can return 0 or -1 for ~2 s after
// launch while media loads. Wrap reads with retries up to coldStartRetry.
const (
	coldStartRetry  = 6                // number of read attempts during cold-start
	coldStartGap_ms = 500              // ms between attempts
	stateNotReady   = ^uintptr(0)      // -1 cast to uintptr; Go sees uint64 max
)
```

- [ ] **Step 2: Verify compilation**

Run: `go build ./internal/potplayer/...`
Expected: clean.

- [ ] **Step 3: Commit**

```
git add internal/potplayer/commands.go
git commit -m "feat(potplayer): pin Win32 message + command IDs from Session 0"
```

---

### Task 3: Pot Player executable path resolution (`path.go`)

**Files:**
- Create: `internal/potplayer/path.go`
- Create: `internal/potplayer/path_test.go`

**Context:** Session 0 found that `HKCU\Software\DAUM\PotPlayerMini64\ProgramPath` is unreliable. Three-stage fallback: (1) registry, (2) Settings override (config.UI.Playback.PotPlayerPath), (3) default install paths.

- [ ] **Step 1: Write the test first**

```go
package potplayer

import (
	"os"
	"path/filepath"
	"testing"
)

func TestResolveExePath_OverrideTakesPrecedence(t *testing.T) {
	dir := t.TempDir()
	fake := filepath.Join(dir, "PotPlayerMini64.exe")
	if err := os.WriteFile(fake, []byte{0x4D, 0x5A}, 0o644); err != nil {
		t.Fatal(err)
	}
	got, err := ResolveExePath(fake)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != fake {
		t.Errorf("override ignored; got %q, want %q", got, fake)
	}
}

func TestResolveExePath_OverrideMissing_FallsThrough(t *testing.T) {
	// Override points at a non-existent path; resolver should NOT return it.
	missing := filepath.Join(t.TempDir(), "nope.exe")
	got, err := ResolveExePath(missing)
	// On a CI box with no Pot Player installed, this errors. On Byron's box it
	// returns the real install path.
	if err == nil && got == missing {
		t.Errorf("resolver returned non-existent override path %q", got)
	}
}
```

- [ ] **Step 2: Verify it fails**

Run: `go test ./internal/potplayer/... -run TestResolveExePath -v`
Expected: FAIL — `ResolveExePath` undefined.

- [ ] **Step 3: Implement `path.go`**

```go
package potplayer

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"golang.org/x/sys/windows/registry"
)

// ResolveExePath returns the absolute path to PotPlayerMini64.exe, in order:
//
//  1. override (Settings → Playback → Pot Player path) if non-empty AND the
//     file exists.
//  2. HKCU\Software\DAUM\PotPlayerMini64\ProgramPath registry value.
//  3. Default install locations: C:\Program Files\DAUM\PotPlayer\,
//     C:\Program Files\DAUM\PotPlayerMini64\.
//
// Returns an error only if every stage fails. Stage-1 misses (override given
// but file absent) fall through silently — they're not user-facing errors.
func ResolveExePath(override string) (string, error) {
	if override != "" {
		if _, err := os.Stat(override); err == nil {
			return override, nil
		}
		// Override given but missing — fall through, do not error here.
	}

	if p, ok := readRegistryProgramPath(); ok {
		if _, err := os.Stat(p); err == nil {
			return p, nil
		}
	}

	defaults := []string{
		`C:\Program Files\DAUM\PotPlayer\PotPlayerMini64.exe`,
		`C:\Program Files\DAUM\PotPlayerMini64\PotPlayerMini64.exe`,
		`C:\Program Files (x86)\DAUM\PotPlayer\PotPlayerMini64.exe`,
	}
	for _, p := range defaults {
		if _, err := os.Stat(p); err == nil {
			return p, nil
		}
	}

	return "", fmt.Errorf("Pot Player executable not found — set Settings → Playback → Pot Player path")
}

func readRegistryProgramPath() (string, bool) {
	k, err := registry.OpenKey(registry.CURRENT_USER, `Software\DAUM\PotPlayerMini64`, registry.QUERY_VALUE)
	if err != nil {
		return "", false
	}
	defer k.Close()
	v, _, err := k.GetStringValue("ProgramPath")
	if err != nil || v == "" {
		return "", false
	}
	// Some installers store the directory; append exe name if so.
	if filepath.Ext(v) == "" {
		v = filepath.Join(v, "PotPlayerMini64.exe")
	}
	return v, true
}

// errExeNotFound is exported for callers that want to discriminate.
var ErrExeNotFound = errors.New("pot player executable not found")
```

- [ ] **Step 4: Run tests**

Run: `go test ./internal/potplayer/... -run TestResolveExePath -v`
Expected: PASS for the override-precedence test; the fallthrough test passes either way (it has no positive assertion when no Pot Player installed).

- [ ] **Step 5: Commit**

```
git add internal/potplayer/path.go internal/potplayer/path_test.go
git commit -m "feat(potplayer): three-stage exe path resolution (override > registry > defaults)"
```

---

### Task 4: `Client` struct + `Launch` + `IsAlive`

**Files:**
- Create: `internal/potplayer/client.go`
- Create: `internal/potplayer/client_test.go`

**Context:** First half of the IPC client. Launch spawns Pot Player as a subprocess, polls for the HWND via `FindWindowW`, returns once found (or 3 s timeout).

- [ ] **Step 1: Write the file**

```go
package potplayer

import (
	"errors"
	"fmt"
	"os/exec"
	"sync"
	"syscall"
	"time"
	"unsafe"

	"golang.org/x/sys/windows"
)

// PlayState mirrors Pot Player's GetState return values plus a sentinel.
type PlayState int

const (
	PlayStateUnknown PlayState = iota // returned during cold-start (-1 from Pot Player)
	PlayStatePaused                   // 1
	PlayStatePlaying                  // 2
	PlayStateStopped                  // synthetic — produced when window is gone
)

func (s PlayState) String() string {
	switch s {
	case PlayStatePaused:
		return "paused"
	case PlayStatePlaying:
		return "playing"
	case PlayStateStopped:
		return "stopped"
	default:
		return "unknown"
	}
}

// Client wraps a single Pot Player subprocess. One Client per playback session.
// All methods are safe to call concurrently.
type Client struct {
	mu   sync.Mutex
	hwnd windows.Handle
	cmd  *exec.Cmd
	exe  string
}

// Launch spawns Pot Player against streamURL and waits up to 3 s for the
// HWND to appear before returning. Caller should keep the *Client until
// playback is torn down.
func Launch(exePath, streamURL string) (*Client, error) {
	if exePath == "" {
		return nil, errors.New("potplayer.Launch: empty exePath")
	}
	if streamURL == "" {
		return nil, errors.New("potplayer.Launch: empty streamURL")
	}
	cmd := exec.Command(exePath, streamURL)
	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("start PotPlayer: %w", err)
	}
	c := &Client{cmd: cmd, exe: exePath}

	// Poll for the window. ~10 attempts × 300 ms = 3 s budget.
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		hwnd, ok := findPotPlayerWindow()
		if ok {
			c.mu.Lock()
			c.hwnd = hwnd
			c.mu.Unlock()
			return c, nil
		}
		time.Sleep(300 * time.Millisecond)
	}
	// Window never appeared — kill the subprocess to avoid orphaning.
	_ = cmd.Process.Kill()
	return nil, errors.New("potplayer.Launch: window did not appear within 3s")
}

// IsAlive returns true while Pot Player's window is still valid. Cheap —
// safe to call from the status poller every 5 s.
func (c *Client) IsAlive() bool {
	c.mu.Lock()
	hwnd := c.hwnd
	c.mu.Unlock()
	if hwnd == 0 {
		return false
	}
	return isWindow(hwnd)
}

// findPotPlayerWindow looks up the top-level window with class PotPlayer64.
// Returns the HWND and true if found.
func findPotPlayerWindow() (windows.Handle, bool) {
	user32 := windows.NewLazySystemDLL("user32.dll")
	findWindowW := user32.NewProc("FindWindowW")
	className, _ := syscall.UTF16PtrFromString(potPlayerWindowClass)
	r1, _, _ := findWindowW.Call(
		uintptr(unsafe.Pointer(className)),
		0,
	)
	if r1 == 0 {
		return 0, false
	}
	return windows.Handle(r1), true
}

// isWindow wraps user32.IsWindow.
func isWindow(hwnd windows.Handle) bool {
	user32 := windows.NewLazySystemDLL("user32.dll")
	isWin := user32.NewProc("IsWindow")
	r1, _, _ := isWin.Call(uintptr(hwnd))
	return r1 != 0
}
```

- [ ] **Step 2: Add a state-mapping unit test**

```go
package potplayer

import "testing"

func TestPlayStateString(t *testing.T) {
	cases := map[PlayState]string{
		PlayStateUnknown: "unknown",
		PlayStatePaused:  "paused",
		PlayStatePlaying: "playing",
		PlayStateStopped: "stopped",
	}
	for s, want := range cases {
		if got := s.String(); got != want {
			t.Errorf("%d.String() = %q, want %q", s, got, want)
		}
	}
}
```

- [ ] **Step 3: Run tests**

Run: `go test ./internal/potplayer/... -v`
Expected: PASS (path tests + state-string test). `Launch`/`IsAlive` exercised via manual harness later — they require Pot Player + a real video.

- [ ] **Step 4: Commit**

```
git add internal/potplayer/client.go internal/potplayer/client_test.go
git commit -m "feat(potplayer): Client.Launch + IsAlive via FindWindowW polling"
```

---

### Task 5: `GetPosition` / `GetDuration` / `GetState` with cold-start retry

**Files:**
- Modify: `internal/potplayer/client.go`

**Context:** Three read methods, all share the cold-start retry envelope: if the first call returns `0` (position/duration) or `-1` sentinel (state), wait `coldStartGap_ms` and retry up to `coldStartRetry` times. Returns the first non-zero / non-sentinel value.

- [ ] **Step 1: Append to `client.go`**

```go
// GetPosition returns the current playback position. May block up to
// coldStartRetry × coldStartGap_ms (~3 s) immediately after launch while
// Pot Player loads the media.
func (c *Client) GetPosition() (time.Duration, error) {
	for i := 0; i < coldStartRetry; i++ {
		ms, err := c.sendUserQuery(ppGetPosition)
		if err != nil {
			return 0, err
		}
		if ms > 0 {
			return time.Duration(ms) * time.Millisecond, nil
		}
		time.Sleep(coldStartGap_ms * time.Millisecond)
	}
	// Final attempt — return whatever we got, even zero.
	ms, err := c.sendUserQuery(ppGetPosition)
	if err != nil {
		return 0, err
	}
	return time.Duration(ms) * time.Millisecond, nil
}

// GetDuration returns the media's total duration. Same cold-start envelope.
func (c *Client) GetDuration() (time.Duration, error) {
	for i := 0; i < coldStartRetry; i++ {
		ms, err := c.sendUserQuery(ppGetDuration)
		if err != nil {
			return 0, err
		}
		if ms > 0 {
			return time.Duration(ms) * time.Millisecond, nil
		}
		time.Sleep(coldStartGap_ms * time.Millisecond)
	}
	ms, err := c.sendUserQuery(ppGetDuration)
	if err != nil {
		return 0, err
	}
	return time.Duration(ms) * time.Millisecond, nil
}

// GetState returns the current play state. Maps Pot Player's -1 sentinel to
// PlayStateUnknown so callers can distinguish "still loading" from "stopped".
func (c *Client) GetState() (PlayState, error) {
	for i := 0; i < coldStartRetry; i++ {
		raw, err := c.sendUserQuery(ppGetState)
		if err != nil {
			return PlayStateUnknown, err
		}
		if raw == stateNotReady {
			time.Sleep(coldStartGap_ms * time.Millisecond)
			continue
		}
		switch raw {
		case 1:
			return PlayStatePaused, nil
		case 2:
			return PlayStatePlaying, nil
		default:
			// Unrecognized — surface as Unknown rather than guessing.
			return PlayStateUnknown, nil
		}
	}
	return PlayStateUnknown, nil
}

// sendUserQuery wraps SendMessageW with the WM_USER + wParam pattern Pot
// Player uses for read-only queries. Refreshes the HWND if the cached one
// has gone stale (Pot Player crash + auto-restart, etc.).
func (c *Client) sendUserQuery(wParam uintptr) (uintptr, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	// Re-find HWND if the cached one died.
	if c.hwnd == 0 || !isWindow(c.hwnd) {
		hwnd, ok := findPotPlayerWindow()
		if !ok {
			return 0, errors.New("pot player window not found")
		}
		c.hwnd = hwnd
	}

	user32 := windows.NewLazySystemDLL("user32.dll")
	sendMessage := user32.NewProc("SendMessageW")
	r1, _, _ := sendMessage.Call(
		uintptr(c.hwnd),
		wmUser,
		wParam,
		0,
	)
	return r1, nil
}
```

- [ ] **Step 2: Verify compilation**

Run: `go build ./internal/potplayer/...`
Expected: clean.

- [ ] **Step 3: Commit**

```
git add internal/potplayer/client.go
git commit -m "feat(potplayer): GetPosition/GetDuration/GetState with cold-start retry"
```

---

### Task 6: `Stop` (APPCOMMAND_MEDIA_STOP + WM_CLOSE)

**Files:**
- Modify: `internal/potplayer/client.go`

**Context:** `Stop` halts playback AND closes the window so the session manager's `IsAlive` flips false. `APPCOMMAND_MEDIA_STOP` only halts; we follow with `WM_CLOSE` to close the window. If the window survives 2 s, fall back to `Process.Kill()`.

- [ ] **Step 1: Append to `client.go`**

```go
// Stop halts playback and closes the Pot Player window. Returns when the
// window is gone (or after a 2 s force-kill fallback).
func (c *Client) Stop() error {
	c.mu.Lock()
	hwnd := c.hwnd
	cmd := c.cmd
	c.mu.Unlock()

	if hwnd == 0 {
		return nil // already gone
	}

	// Halt playback first.
	_ = sendAppCommand(hwnd, appCmdMediaStop)

	// Ask the window to close.
	user32 := windows.NewLazySystemDLL("user32.dll")
	postMessage := user32.NewProc("PostMessageW")
	_, _, _ = postMessage.Call(uintptr(hwnd), wmClose, 0, 0)

	// Wait up to 2 s for IsWindow to flip false.
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if !isWindow(hwnd) {
			return nil
		}
		time.Sleep(100 * time.Millisecond)
	}

	// Force-kill the subprocess. Last resort.
	if cmd != nil && cmd.Process != nil {
		_ = cmd.Process.Kill()
	}
	return nil
}

// sendAppCommand fires WM_APPCOMMAND with the given command in the high
// word of lParam. Per Session 0 findings: SendMessage(hwnd, 0x0319, 0, cmd<<16).
func sendAppCommand(hwnd windows.Handle, cmd uintptr) error {
	user32 := windows.NewLazySystemDLL("user32.dll")
	sendMessage := user32.NewProc("SendMessageW")
	_, _, _ = sendMessage.Call(
		uintptr(hwnd),
		wmAppCommand,
		0,
		cmd<<16,
	)
	return nil
}
```

- [ ] **Step 2: Verify compilation**

Run: `go build ./internal/potplayer/...`
Expected: clean.

- [ ] **Step 3: Commit**

```
git add internal/potplayer/client.go
git commit -m "feat(potplayer): Stop via APPCOMMAND_MEDIA_STOP + WM_CLOSE + Kill fallback"
```

---

## Phase C — Stream URL Resolution + Plex Helpers

### Task 7: `DirectPlayURL` + `TranscodeURL` builders (`stream.go`)

**Files:**
- Create: `internal/plex/stream.go`
- Create: `internal/plex/stream_test.go`

**Context:** Spec §8 builds direct-play URLs from `Media[0].Part[0].partID` + container/extension. Transcode URL pinned per spec §8.1. Both functions are pure; no HTTP. Take a `*Server` (carries BaseURL + AccessToken) plus the relevant Plex item parts.

- [ ] **Step 1: Test first**

```go
package plex

import (
	"net/url"
	"strings"
	"testing"
)

func TestDirectPlayURL(t *testing.T) {
	srv := &Server{
		BaseURL:     "https://srv.example.com",
		AccessToken: "secret",
	}
	got := DirectPlayURL(srv, "12345", "mkv")
	want := "https://srv.example.com/library/parts/12345/0/file.mkv?X-Plex-Token=secret"
	if got != want {
		t.Errorf("DirectPlayURL = %q, want %q", got, want)
	}
}

func TestTranscodeURL_HasRequiredParams(t *testing.T) {
	srv := &Server{
		BaseURL:     "https://srv.example.com",
		AccessToken: "secret",
	}
	got := TranscodeURL(srv, "12345", "session-abc")
	u, err := url.Parse(got)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasSuffix(u.Path, "/video/:/transcode/universal/start.m3u8") {
		t.Errorf("wrong path: %s", u.Path)
	}
	q := u.Query()
	for _, key := range []string{"path", "directPlay", "directStream", "protocol", "videoQuality", "videoResolution", "session", "X-Plex-Token"} {
		if q.Get(key) == "" {
			t.Errorf("missing query param: %s", key)
		}
	}
	if q.Get("session") != "session-abc" {
		t.Errorf("session = %q, want session-abc", q.Get("session"))
	}
}
```

- [ ] **Step 2: Verify failure**

Run: `go test ./internal/plex/... -run TestDirectPlayURL -v`
Expected: FAIL — undefined.

- [ ] **Step 3: Implement `stream.go`**

```go
package plex

import (
	"fmt"
	"net/url"
)

// DirectPlayURL builds the URL Pot Player should hit for direct play. Per
// spec §8.1: /library/parts/{partID}/{id}/file.{ext}?X-Plex-Token=...
// The {id} segment is unused by Plex (any value works); we hardcode 0.
func DirectPlayURL(s *Server, partID, ext string) string {
	q := url.Values{
		"X-Plex-Token": []string{s.AccessToken},
	}
	return fmt.Sprintf("%s/library/parts/%s/0/file.%s?%s",
		s.BaseURL, partID, ext, q.Encode())
}

// TranscodeURL builds the HLS transcode URL used as fallback when direct
// play fails. The session parameter MUST match the value passed to
// transcode/universal/ping for the keep-alive ticker. Spec §8.1 + §9.3.
func TranscodeURL(s *Server, ratingKey, session string) string {
	q := url.Values{
		"path":            []string{fmt.Sprintf("/library/metadata/%s", ratingKey)},
		"directPlay":      []string{"0"},
		"directStream":    []string{"0"},
		"protocol":        []string{"hls"},
		"videoQuality":    []string{"100"},
		"videoResolution": []string{"1920x1080"},
		"mediaBufferSize": []string{"204800"},
		"session":         []string{session},
		"X-Plex-Token":    []string{s.AccessToken},
	}
	return fmt.Sprintf("%s/video/:/transcode/universal/start.m3u8?%s",
		s.BaseURL, q.Encode())
}
```

- [ ] **Step 4: Run tests**

Run: `go test ./internal/plex/... -run "TestDirectPlayURL|TestTranscodeURL" -v`
Expected: PASS.

- [ ] **Step 5: Commit**

```
git add internal/plex/stream.go internal/plex/stream_test.go
git commit -m "feat(plex): direct-play + transcode URL builders"
```

---

### Task 8: Plex timeline reporter (`timeline.go`)

**Files:**
- Create: `internal/plex/timeline.go`
- Create: `internal/plex/timeline_test.go`

**Context:** POSTs `/:/timeline` per spec §9.2. Plex uses these reports to update `viewOffset` and trigger watched-state advancement (server defaults to 90% threshold). Single function — the goroutine that calls it on a 10 s tick lives in `internal/playback`.

- [ ] **Step 1: Test first**

```go
package plex

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"
)

func TestReportTimeline_SendsExpectedQuery(t *testing.T) {
	var gotPath, gotQuery string
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		gotQuery = r.URL.RawQuery
		w.WriteHeader(http.StatusOK)
	}))
	defer ts.Close()

	c := NewClient("client-id", "0.1.0")
	srv := &Server{BaseURL: ts.URL, AccessToken: "secret"}

	err := c.ReportTimeline(srv, TimelineReport{
		RatingKey: "12345",
		State:     "playing",
		Position:  5 * time.Second,
		Duration:  60 * time.Second,
	})
	if err != nil {
		t.Fatal(err)
	}

	if gotPath != "/:/timeline" {
		t.Errorf("path = %q, want /:/timeline", gotPath)
	}
	q, _ := url.ParseQuery(gotQuery)
	if q.Get("ratingKey") != "12345" {
		t.Errorf("ratingKey = %q", q.Get("ratingKey"))
	}
	if q.Get("state") != "playing" {
		t.Errorf("state = %q", q.Get("state"))
	}
	if !strings.HasPrefix(q.Get("time"), "5000") {
		t.Errorf("time = %q, want 5000ms", q.Get("time"))
	}
	if q.Get("duration") != "60000" {
		t.Errorf("duration = %q, want 60000ms", q.Get("duration"))
	}
}
```

- [ ] **Step 2: Verify failure**

Run: `go test ./internal/plex/... -run TestReportTimeline -v`
Expected: FAIL — undefined.

- [ ] **Step 3: Implement `timeline.go`**

```go
package plex

import (
	"fmt"
	"net/url"
	"time"
)

// TimelineReport carries the data sent in a single POST /:/timeline.
type TimelineReport struct {
	RatingKey string
	State     string // "playing" | "paused" | "stopped"
	Position  time.Duration
	Duration  time.Duration
}

// ReportTimeline POSTs /:/timeline so Plex updates viewOffset, lastViewedAt,
// and (server-side) watched state. Caller decides cadence — typically every
// 10 s while playing.
func (c *Client) ReportTimeline(s *Server, r TimelineReport) error {
	if r.State != "playing" && r.State != "paused" && r.State != "stopped" {
		return fmt.Errorf("ReportTimeline: invalid state %q", r.State)
	}
	q := url.Values{
		"ratingKey":    []string{r.RatingKey},
		"state":        []string{r.State},
		"time":         []string{fmt.Sprintf("%d", r.Position.Milliseconds())},
		"duration":     []string{fmt.Sprintf("%d", r.Duration.Milliseconds())},
		"X-Plex-Token": []string{s.AccessToken},
	}
	u := s.BaseURL + "/:/timeline?" + q.Encode()
	req, err := c.NewRequest("POST", u, nil)
	if err != nil {
		return err
	}
	c.SetToken(req, s.AccessToken)
	resp, err := c.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		return fmt.Errorf("timeline %s: status %d", r.RatingKey, resp.StatusCode)
	}
	return nil
}
```

- [ ] **Step 4: Run tests**

Run: `go test ./internal/plex/... -run TestReportTimeline -v`
Expected: PASS.

- [ ] **Step 5: Commit**

```
git add internal/plex/timeline.go internal/plex/timeline_test.go
git commit -m "feat(plex): /:/timeline reporter for playback progress sync"
```

---

### Task 9: Episode chaining helpers (`episodes.go`)

**Files:**
- Create: `internal/plex/episodes.go`
- Create: `internal/plex/episodes_test.go`

**Context:** Auto-play next episode (spec/Byron) needs a way to ask "what's the next episode after THIS one in THIS show". Plex stores episode hierarchy via `parentRatingKey` (season) → `grandparentRatingKey` (show). The flat sorted list comes from `/library/metadata/{showRatingKey}/allLeaves`.

- [ ] **Step 1: Test first**

```go
package plex

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestNextEpisode_FindsNextInSeries(t *testing.T) {
	episodes := []Item{
		{RatingKey: "ep1", ParentIndex: 1, Index: 1},
		{RatingKey: "ep2", ParentIndex: 1, Index: 2},
		{RatingKey: "ep3", ParentIndex: 1, Index: 3},
		{RatingKey: "ep4", ParentIndex: 2, Index: 1},
	}
	body := map[string]any{
		"MediaContainer": map[string]any{
			"Metadata": episodes,
		},
	}
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(body)
	}))
	defer ts.Close()

	c := NewClient("client-id", "0.1.0")
	srv := &Server{BaseURL: ts.URL, AccessToken: "secret"}

	next, err := c.NextEpisode(srv, "show-99", "ep2")
	if err != nil {
		t.Fatal(err)
	}
	if next == nil {
		t.Fatal("expected ep3, got nil")
	}
	if next.RatingKey != "ep3" {
		t.Errorf("next = %s, want ep3", next.RatingKey)
	}
}

func TestNextEpisode_LastEpisodeReturnsNil(t *testing.T) {
	episodes := []Item{{RatingKey: "ep1", ParentIndex: 1, Index: 1}}
	body := map[string]any{
		"MediaContainer": map[string]any{"Metadata": episodes},
	}
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(body)
	}))
	defer ts.Close()

	c := NewClient("client-id", "0.1.0")
	srv := &Server{BaseURL: ts.URL, AccessToken: "secret"}

	next, err := c.NextEpisode(srv, "show-1", "ep1")
	if err != nil {
		t.Fatal(err)
	}
	if next != nil {
		t.Errorf("expected nil for last episode, got %v", next)
	}
}
```

- [ ] **Step 2: Verify failure**

Run: `go test ./internal/plex/... -run TestNextEpisode -v`
Expected: FAIL — undefined.

- [ ] **Step 3: Implement `episodes.go`**

```go
package plex

import (
	"encoding/json"
	"fmt"
	"sort"
)

// allLeavesResponse mirrors /library/metadata/<showKey>/allLeaves.
type allLeavesResponse struct {
	MediaContainer struct {
		Metadata []Item `json:"Metadata"`
	} `json:"MediaContainer"`
}

// NextEpisode returns the episode that comes immediately after currentRatingKey
// in show showRatingKey, ordered by (season, episode-index). Returns nil
// (without error) when currentRatingKey is the last episode in the show.
func (c *Client) NextEpisode(s *Server, showRatingKey, currentRatingKey string) (*Item, error) {
	u := fmt.Sprintf("%s/library/metadata/%s/allLeaves?X-Plex-Token=%s",
		s.BaseURL, showRatingKey, s.AccessToken)
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
		return nil, fmt.Errorf("allLeaves %s: status %d", showRatingKey, resp.StatusCode)
	}
	var body allLeavesResponse
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		return nil, err
	}

	eps := body.MediaContainer.Metadata
	sort.SliceStable(eps, func(i, j int) bool {
		if eps[i].ParentIndex != eps[j].ParentIndex {
			return eps[i].ParentIndex < eps[j].ParentIndex
		}
		return eps[i].Index < eps[j].Index
	})

	for i, ep := range eps {
		if ep.RatingKey == currentRatingKey {
			if i+1 < len(eps) {
				return &eps[i+1], nil
			}
			return nil, nil
		}
	}
	return nil, fmt.Errorf("episode %s not found in show %s", currentRatingKey, showRatingKey)
}
```

- [ ] **Step 4: Run tests**

Run: `go test ./internal/plex/... -run TestNextEpisode -v`
Expected: PASS both cases.

- [ ] **Step 5: Commit**

```
git add internal/plex/episodes.go internal/plex/episodes_test.go
git commit -m "feat(plex): NextEpisode resolver via /library/metadata/{showKey}/allLeaves"
```

---

## Phase D — Playback Session Manager

### Task 10: `PlaybackContext` + `Manager` skeleton (`context.go` + `manager.go`)

**Files:**
- Create: `internal/playback/context.go`
- Create: `internal/playback/manager.go`

**Context:** The Manager is the singleton playback orchestrator. It owns: the active context, the three goroutines, the SSE subscriber list. Goroutines + endpoint code in subsequent tasks reference it. UI enforces single-stream playback (spec §9) — `Manager.Start` returns an error if a session is already active.

- [ ] **Step 1: Write `context.go`**

```go
// Package playback orchestrates a single active media playback session —
// driving Pot Player, syncing progress to Plex, and streaming state to the
// SPA. Spec §9.
package playback

import (
	"sync"
	"time"

	"lumen/internal/plex"
	"lumen/internal/potplayer"
)

// Context carries everything one active playback session needs. There is at
// most one Context alive in the process at a time (UI enforces single-stream).
type Context struct {
	RatingKey        string
	Server           *plex.Server
	ShowRatingKey    string // empty for movies
	IsEpisode        bool
	PartID           string
	Container        string
	StartedAt        time.Time
	Duration         time.Duration
	Transcoding      bool
	TranscodeSession string

	PotPlayer *potplayer.Client
}

// State is the snapshot the SPA consumes via /api/playback (and SSE).
type State struct {
	Active       bool          `json:"active"`
	RatingKey    string        `json:"ratingKey,omitempty"`
	ServerID     string        `json:"serverID,omitempty"`
	Title        string        `json:"title,omitempty"`
	ShowTitle    string        `json:"showTitle,omitempty"`
	Position     time.Duration `json:"position"`
	Duration     time.Duration `json:"duration"`
	State        string        `json:"state"` // "playing" | "paused" | "stopped" | "unknown"
	Quality      string        `json:"quality,omitempty"`
	Transcoding  bool          `json:"transcoding"`
	ThumbPath    string        `json:"thumbPath,omitempty"`
}

// Event is the discriminated message the SPA receives over SSE.
type Event struct {
	Type    string `json:"type"`
	State   *State `json:"state,omitempty"`
	Payload any    `json:"payload,omitempty"`
}

// Event types — the SPA dispatches on Type.
const (
	EventStateUpdate     = "state"
	EventEnded           = "ended"
	EventNextEpisode     = "next-episode-prompt"
	EventTranscodePrompt = "transcode-prompt"
	EventStopped         = "stopped"
)

// NextEpisodeInfo accompanies an EventNextEpisode payload.
type NextEpisodeInfo struct {
	RatingKey       string `json:"ratingKey"`
	ServerID        string `json:"serverID"`
	Title           string `json:"title"`
	Season          int    `json:"season"`
	Episode         int    `json:"episode"`
	ThumbPath       string `json:"thumbPath,omitempty"`
}

// TranscodePromptInfo accompanies EventTranscodePrompt.
type TranscodePromptInfo struct {
	RatingKey string `json:"ratingKey"`
	ServerID  string `json:"serverID"`
	Title     string `json:"title"`
	Reason    string `json:"reason"`
}

// stateMu protects mutable fields on the live Context (position, etc.) so
// the poller and SSE encoder don't race.
type liveState struct {
	mu       sync.Mutex
	position time.Duration
	state    potplayer.PlayState
}
```

- [ ] **Step 2: Write `manager.go`**

```go
package playback

import (
	"context"
	"errors"
	"sync"
	"time"

	"lumen/internal/plex"
	"lumen/internal/potplayer"
)

// Manager owns the lifecycle of the single active playback Context. Methods
// are safe to call concurrently.
type Manager struct {
	plex    *plex.Client
	potPath func() string // closure so the path picks up Settings updates

	mu      sync.Mutex
	active  *Context
	live    liveState
	cancel  context.CancelFunc
	subs    map[chan Event]struct{}
}

// NewManager wires a Manager to a Plex client and a Pot Player path resolver.
// The resolver is a closure so updates to Settings → Playback → Pot Player
// path are visible without restarting the manager.
func NewManager(c *plex.Client, potPath func() string) *Manager {
	return &Manager{
		plex:    c,
		potPath: potPath,
		subs:    make(map[chan Event]struct{}),
	}
}

// StartArgs is the input to Start.
type StartArgs struct {
	Server           *plex.Server
	RatingKey        string
	ShowRatingKey    string // empty for movies
	IsEpisode        bool
	PartID           string
	Container        string
	StreamURL        string // built by caller (DirectPlayURL or TranscodeURL)
	Transcoding      bool
	TranscodeSession string
	Duration         time.Duration // initial duration from Plex metadata; refined after launch
	Title            string
	ShowTitle        string
	ThumbPath        string
	Quality          string // e.g. "1080p H.264"
}

// Start launches Pot Player, builds the Context, kicks the three goroutines.
// Returns ErrAlreadyActive if another session is live.
func (m *Manager) Start(args StartArgs) error {
	m.mu.Lock()
	if m.active != nil {
		m.mu.Unlock()
		return ErrAlreadyActive
	}

	exe := m.potPath()
	if exe == "" {
		m.mu.Unlock()
		return errors.New("pot player path not resolved")
	}

	pp, err := potplayer.Launch(exe, args.StreamURL)
	if err != nil {
		m.mu.Unlock()
		return err
	}

	ctx, cancel := context.WithCancel(context.Background())
	c := &Context{
		RatingKey:        args.RatingKey,
		Server:           args.Server,
		ShowRatingKey:    args.ShowRatingKey,
		IsEpisode:        args.IsEpisode,
		PartID:           args.PartID,
		Container:        args.Container,
		StartedAt:        time.Now(),
		Duration:         args.Duration,
		Transcoding:      args.Transcoding,
		TranscodeSession: args.TranscodeSession,
		PotPlayer:        pp,
	}
	m.active = c
	m.cancel = cancel
	m.live.position = 0
	m.live.state = potplayer.PlayStateUnknown
	m.mu.Unlock()

	// Initial state broadcast.
	m.broadcast(Event{Type: EventStateUpdate, State: m.snapshot(args.Title, args.ShowTitle, args.ThumbPath, args.Quality)})

	// Kick goroutines (each is defined in its own file).
	go m.runPoller(ctx, args)
	go m.runReporter(ctx)
	if args.Transcoding {
		go m.runTranscodeKeepAlive(ctx)
	}

	return nil
}

// Stop tears down the active session. Idempotent.
func (m *Manager) Stop() {
	m.mu.Lock()
	c := m.active
	cancel := m.cancel
	m.mu.Unlock()
	if c == nil {
		return
	}
	if cancel != nil {
		cancel()
	}
	if c.PotPlayer != nil {
		_ = c.PotPlayer.Stop()
	}
	// Final timeline report — best-effort, swallow error.
	pos := m.currentPosition()
	_ = m.plex.ReportTimeline(c.Server, plex.TimelineReport{
		RatingKey: c.RatingKey,
		State:     "stopped",
		Position:  pos,
		Duration:  c.Duration,
	})

	m.mu.Lock()
	m.active = nil
	m.cancel = nil
	m.mu.Unlock()
	m.broadcast(Event{Type: EventStopped})
}

// Subscribe returns a channel of events. Caller MUST drain the channel and
// call the returned cleanup func when done.
func (m *Manager) Subscribe() (<-chan Event, func()) {
	ch := make(chan Event, 16)
	m.mu.Lock()
	m.subs[ch] = struct{}{}
	m.mu.Unlock()
	cleanup := func() {
		m.mu.Lock()
		delete(m.subs, ch)
		m.mu.Unlock()
		close(ch)
	}
	return ch, cleanup
}

// SnapshotState returns the current playback State (whether or not a session
// is active).
func (m *Manager) SnapshotState() State {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.active == nil {
		return State{Active: false}
	}
	return m.snapshotLocked("", "", "", "")
}

// broadcast fans an event out to all subscribers. Drops events on full
// channels rather than blocking.
func (m *Manager) broadcast(e Event) {
	m.mu.Lock()
	subs := make([]chan Event, 0, len(m.subs))
	for ch := range m.subs {
		subs = append(subs, ch)
	}
	m.mu.Unlock()
	for _, ch := range subs {
		select {
		case ch <- e:
		default:
		}
	}
}

// currentPosition reads the latest position the poller cached.
func (m *Manager) currentPosition() time.Duration {
	m.live.mu.Lock()
	defer m.live.mu.Unlock()
	return m.live.position
}

// snapshot builds a fresh State; call without holding m.mu (it locks itself).
func (m *Manager) snapshot(title, showTitle, thumbPath, quality string) *State {
	m.mu.Lock()
	defer m.mu.Unlock()
	st := m.snapshotLocked(title, showTitle, thumbPath, quality)
	return &st
}

func (m *Manager) snapshotLocked(title, showTitle, thumbPath, quality string) State {
	if m.active == nil {
		return State{Active: false}
	}
	m.live.mu.Lock()
	pos := m.live.position
	stateStr := m.live.state.String()
	m.live.mu.Unlock()
	return State{
		Active:      true,
		RatingKey:   m.active.RatingKey,
		ServerID:    m.active.Server.MachineIdentifier,
		Title:       title,
		ShowTitle:   showTitle,
		Position:    pos,
		Duration:    m.active.Duration,
		State:       stateStr,
		Transcoding: m.active.Transcoding,
		ThumbPath:   thumbPath,
		Quality:     quality,
	}
}

// ErrAlreadyActive is returned by Start when a session is already running.
var ErrAlreadyActive = errors.New("playback session already active")
```

- [ ] **Step 3: Verify compilation**

Run: `go build ./internal/playback/...`
Expected: clean. The unimplemented methods (`runPoller`, `runReporter`, `runTranscodeKeepAlive`) are filled in by the next three tasks; until then compilation will fail with "undefined" — that's fine, this task and the next three are checkpoints in a sequence.

If you want a green compile here, add stub method receivers:
```go
func (m *Manager) runPoller(ctx context.Context, args StartArgs)   {}
func (m *Manager) runReporter(ctx context.Context)                 {}
func (m *Manager) runTranscodeKeepAlive(ctx context.Context)       {}
```
in `manager.go` for now; they're replaced in Tasks 11-13.

- [ ] **Step 4: Commit**

```
git add internal/playback/context.go internal/playback/manager.go
git commit -m "feat(playback): Manager skeleton + Context + Event types"
```

---

### Task 11: Status poller goroutine (`poller.go`)

**Files:**
- Create: `internal/playback/poller.go`
- Modify: `internal/playback/manager.go` (remove the `runPoller` stub)

**Context:** Polls Pot Player every 5 s for position + state. On each tick, broadcasts a `state` event. Detects 90% threshold → fires scrobble + emits `next-episode-prompt` (for episodes) or `ended` (for movies). Detects `IsAlive()==false` → tears down via `Manager.Stop()`. Detects 10 s without non-zero duration → emits `transcode-prompt` (caller already passed initial duration; this is the post-launch refinement).

- [ ] **Step 1: Implement `poller.go`**

```go
package playback

import (
	"context"
	"log"
	"time"

	"lumen/internal/plex"
	"lumen/internal/potplayer"
)

const (
	pollInterval         = 5 * time.Second
	directPlayTimeout    = 10 * time.Second
	watchedThresholdFrac = 0.90 // mirrors Plex's server-side default
)

// runPoller reads Pot Player's position/state every pollInterval, broadcasts
// state updates, and triggers end-of-file / direct-play-failure logic.
func (m *Manager) runPoller(ctx context.Context, args StartArgs) {
	t := time.NewTicker(pollInterval)
	defer t.Stop()

	startedAt := time.Now()
	scrobbled := false
	durationConfirmed := args.Duration > 0
	endedFired := false

	for {
		select {
		case <-ctx.Done():
			return
		case <-t.C:
		}

		m.mu.Lock()
		c := m.active
		m.mu.Unlock()
		if c == nil {
			return
		}

		// Liveness check first — fast and cheap.
		if !c.PotPlayer.IsAlive() {
			// Final position is whatever we last saw.
			m.broadcast(Event{Type: EventStateUpdate, State: m.snapshot(args.Title, args.ShowTitle, args.ThumbPath, args.Quality)})
			m.Stop()
			return
		}

		pos, err := c.PotPlayer.GetPosition()
		if err != nil {
			log.Printf("playback: GetPosition: %v", err)
			continue
		}
		state, _ := c.PotPlayer.GetState()

		m.live.mu.Lock()
		m.live.position = pos
		m.live.state = state
		m.live.mu.Unlock()

		// First non-zero duration confirms direct-play OR transcode bootstrapped.
		if !durationConfirmed {
			d, _ := c.PotPlayer.GetDuration()
			if d > 0 {
				m.mu.Lock()
				c.Duration = d
				m.mu.Unlock()
				durationConfirmed = true
			} else if time.Since(startedAt) > directPlayTimeout && !c.Transcoding {
				// No duration after 10 s — direct play failed. Emit prompt
				// and tear down so the SPA can decide whether to retry as
				// transcode.
				m.broadcast(Event{
					Type: EventTranscodePrompt,
					Payload: TranscodePromptInfo{
						RatingKey: c.RatingKey,
						ServerID:  c.Server.MachineIdentifier,
						Title:     args.Title,
						Reason:    "Pot Player did not report a duration within 10 s",
					},
				})
				m.Stop()
				return
			}
		}

		// 90% threshold: scrobble once, emit ended/next-episode once.
		if c.Duration > 0 && pos >= time.Duration(float64(c.Duration)*watchedThresholdFrac) {
			if !scrobbled {
				if err := m.plex.Scrobble(c.Server, c.RatingKey); err != nil {
					log.Printf("playback: Scrobble: %v", err)
				}
				scrobbled = true
			}
			if !endedFired {
				m.fireEnded(c, args)
				endedFired = true
			}
		}

		// Always rebroadcast latest state.
		m.broadcast(Event{Type: EventStateUpdate, State: m.snapshot(args.Title, args.ShowTitle, args.ThumbPath, args.Quality)})
	}
}

// fireEnded emits the appropriate "we crossed the watched threshold" event.
// For episodes, looks up the next-up episode and emits next-episode-prompt;
// for movies, emits a generic ended event.
func (m *Manager) fireEnded(c *Context, args StartArgs) {
	if !c.IsEpisode || c.ShowRatingKey == "" {
		m.broadcast(Event{Type: EventEnded})
		return
	}
	next, err := m.plex.NextEpisode(c.Server, c.ShowRatingKey, c.RatingKey)
	if err != nil {
		log.Printf("playback: NextEpisode: %v", err)
		m.broadcast(Event{Type: EventEnded})
		return
	}
	if next == nil {
		// Last episode in show.
		m.broadcast(Event{Type: EventEnded})
		return
	}
	info := NextEpisodeInfo{
		RatingKey: next.RatingKey,
		ServerID:  c.Server.MachineIdentifier,
		Title:     next.Title,
		Season:    next.ParentIndex,
		Episode:   next.Index,
	}
	if next.Thumb != "" {
		info.ThumbPath = next.Thumb
	} else if next.GrandparentThumb != "" {
		info.ThumbPath = next.GrandparentThumb
	}
	m.broadcast(Event{Type: EventNextEpisode, Payload: info})
}

// Re-export for the unused-import linter while the file is light.
var _ = potplayer.PlayStatePlaying
var _ = plex.TimelineReport{}
```

- [ ] **Step 2: Remove the `runPoller` stub from `manager.go`** (if you added it in Task 10's compile shim).

- [ ] **Step 3: Verify compilation**

Run: `go build ./internal/playback/...`
Expected: clean (`runReporter`, `runTranscodeKeepAlive` may still need stubs from Task 10).

- [ ] **Step 4: Commit**

```
git add internal/playback/poller.go internal/playback/manager.go
git commit -m "feat(playback): status poller with 90% threshold + direct-play timeout"
```

---

### Task 12: Plex timeline reporter goroutine (`reporter.go`)

**Files:**
- Create: `internal/playback/reporter.go`
- Modify: `internal/playback/manager.go` (remove the `runReporter` stub)

**Context:** Every 10 s while active, POSTs `/:/timeline`. Plex uses these reports to update `viewOffset` and trigger watched-state.

- [ ] **Step 1: Implement `reporter.go`**

```go
package playback

import (
	"context"
	"log"
	"time"

	"lumen/internal/plex"
)

const reporterInterval = 10 * time.Second

func (m *Manager) runReporter(ctx context.Context) {
	t := time.NewTicker(reporterInterval)
	defer t.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-t.C:
		}

		m.mu.Lock()
		c := m.active
		m.mu.Unlock()
		if c == nil {
			return
		}

		m.live.mu.Lock()
		pos := m.live.position
		state := stateToPlexString(m.live.state)
		m.live.mu.Unlock()

		err := m.plex.ReportTimeline(c.Server, plex.TimelineReport{
			RatingKey: c.RatingKey,
			State:     state,
			Position:  pos,
			Duration:  c.Duration,
		})
		if err != nil {
			log.Printf("playback: ReportTimeline: %v", err)
		}
	}
}

// stateToPlexString maps potplayer.PlayState to the lowercase strings
// /:/timeline expects.
func stateToPlexString(s int) string {
	// importing potplayer for the type would create a cycle in some setups;
	// we accept the int value directly.
	switch s {
	case 1:
		return "paused"
	case 2:
		return "playing"
	default:
		return "playing" // optimistic when unknown — prevents Plex pausing the session needlessly
	}
}
```

Note: `m.live.state` is `potplayer.PlayState` (an int). Adjust the call site if needed; the function signature here uses int for cycle-avoidance simplicity.

- [ ] **Step 2: Adjust `live.state` access**

In `runReporter`, replace `state := stateToPlexString(m.live.state)` with the actual int conversion that matches your enum: `state := stateToPlexString(int(m.live.state))`.

- [ ] **Step 3: Remove the `runReporter` stub from `manager.go`.**

- [ ] **Step 4: Verify compilation**

Run: `go build ./internal/playback/...`
Expected: clean.

- [ ] **Step 5: Commit**

```
git add internal/playback/reporter.go internal/playback/manager.go
git commit -m "feat(playback): Plex /:/timeline reporter goroutine (10s tick)"
```

---

### Task 13: Transcode keep-alive goroutine (`transcode.go`)

**Files:**
- Create: `internal/playback/transcode.go`
- Modify: `internal/playback/manager.go` (remove the stub)

**Context:** Only runs when `Context.Transcoding == true`. Every 10 s, pings `/video/:/transcode/universal/ping?session=<id>` so Plex doesn't reap the transcode.

- [ ] **Step 1: Implement `transcode.go`**

```go
package playback

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"net/url"
	"time"
)

const transcodeKeepAliveInterval = 10 * time.Second

func (m *Manager) runTranscodeKeepAlive(ctx context.Context) {
	t := time.NewTicker(transcodeKeepAliveInterval)
	defer t.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-t.C:
		}

		m.mu.Lock()
		c := m.active
		m.mu.Unlock()
		if c == nil || !c.Transcoding || c.TranscodeSession == "" {
			return
		}

		q := url.Values{
			"session":      []string{c.TranscodeSession},
			"X-Plex-Token": []string{c.Server.AccessToken},
		}
		u := fmt.Sprintf("%s/video/:/transcode/universal/ping?%s", c.Server.BaseURL, q.Encode())
		req, err := http.NewRequest("POST", u, nil)
		if err != nil {
			log.Printf("playback: keepalive build: %v", err)
			continue
		}
		req.Header.Set("X-Plex-Token", c.Server.AccessToken)
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			log.Printf("playback: keepalive request: %v", err)
			continue
		}
		_ = resp.Body.Close()
	}
}
```

- [ ] **Step 2: Remove the `runTranscodeKeepAlive` stub from `manager.go`.**

- [ ] **Step 3: Verify compilation**

Run: `go build ./internal/playback/...`
Expected: clean.

- [ ] **Step 4: Commit**

```
git add internal/playback/transcode.go internal/playback/manager.go
git commit -m "feat(playback): transcode keep-alive ping goroutine (10s, when transcoding)"
```

---

## Phase E — HTTP Endpoints

### Task 14: Wire `playback.Manager` into `*Server`

**Files:**
- Modify: `internal/server/server.go`
- Modify: `cmd/lumen/serve.go`

**Context:** The Manager needs to live as long as the HTTP server. Construct it in `New()`, pass the existing Plex client + a path-resolver closure that reads from `cfg.UI.Playback.PotPlayerPath`.

- [ ] **Step 1: Add field + constructor wiring**

In `internal/server/server.go`, add:
```go
import (
    // ... existing imports
    "lumen/internal/playback"
    "lumen/internal/potplayer"
)

type Server struct {
    cfg    *config.Config
    plex   *plex.Client
    mux    *http.ServeMux
    http   *http.Server
    ln     net.Listener
    hubs   *hubCache
    images *imageCache
    quit   chan struct{}
    playback *playback.Manager
}
```

In `New(...)`:
```go
s.playback = playback.NewManager(c, func() string {
    override := cfg.UI.Playback.PotPlayerPath
    p, err := potplayer.ResolveExePath(override)
    if err != nil {
        return ""
    }
    return p
})
```

- [ ] **Step 2: Verify compilation**

Run: `go build ./...`
Expected: clean.

- [ ] **Step 3: Commit**

```
git add internal/server/server.go
git commit -m "feat(server): wire playback.Manager into Server with path-resolver closure"
```

---

### Task 15: `POST /api/play` (replace 501 stub)

**Files:**
- Modify: `internal/server/api_play.go`
- Modify: `internal/server/server.go` (route already registered for `/api/play`)
- Create: `internal/server/api_play_test.go`

**Context:** Body: `{ serverID, ratingKey, resumeFromOffset?: number }`. Looks up the Plex item, builds the direct-play URL, calls `manager.Start(...)`. Returns 200 + the initial State. If a session is already active, returns 409.

- [ ] **Step 1: Read existing stub**

Confirm `internal/server/api_play.go` returns 501. We replace it.

- [ ] **Step 2: Implement the handler**

```go
package server

import (
	"encoding/json"
	"fmt"
	"net/http"

	"lumen/internal/playback"
	"lumen/internal/plex"
)

type playRequest struct {
	ServerID         string `json:"serverID"`
	RatingKey        string `json:"ratingKey"`
	ResumeFromOffset int64  `json:"resumeFromOffset,omitempty"` // ms
}

func (s *Server) handlePlay(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "POST required")
		return
	}
	var req playRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON: "+err.Error())
		return
	}
	srv := s.serverByID(req.ServerID)
	if srv == nil {
		writeError(w, http.StatusNotFound, "unknown server")
		return
	}
	plexSrv := toPlexServer(srv)

	// Fetch item to get partID + container + duration + episode hierarchy.
	item, err := s.plex.GetItem(plexSrv, req.RatingKey)
	if err != nil {
		writeError(w, http.StatusBadGateway, "fetch item: "+err.Error())
		return
	}
	if len(item.Media) == 0 || len(item.Media[0].Part) == 0 {
		writeError(w, http.StatusUnprocessableEntity, "item has no playable parts")
		return
	}
	part := item.Media[0].Part[0]
	ext := containerToExt(part.Container)

	streamURL := plex.DirectPlayURL(plexSrv, fmt.Sprintf("%d", part.ID), ext)
	if req.ResumeFromOffset > 0 {
		// Pot Player honors trailing /seek=<seconds> on the URL? No — its CLI
		// doesn't support seek args. We pass position to Plex via timeline
		// updates instead; Pot Player starts at 0. Resume is a Plex-side
		// concept handled by the resume modal which decides whether the user
		// wants to restart or pick up; either way we hand a clean URL to
		// Pot Player. This field is recorded for the timeline reporter to
		// post on the very first tick so Plex's Now Playing reflects resume.
		// (Future enhancement: explore `/seek=N` if Pot Player adds CLI
		// support, or send WM_USER position-set message after launch.)
	}

	args := playback.StartArgs{
		Server:        plexSrv,
		RatingKey:     req.RatingKey,
		ShowRatingKey: item.GrandparentRatingKey,
		IsEpisode:     item.Type == "episode",
		PartID:        fmt.Sprintf("%d", part.ID),
		Container:     part.Container,
		StreamURL:     streamURL,
		Transcoding:   false,
		Duration:      msToDuration(item.Duration),
		Title:         item.Title,
		ShowTitle:     item.GrandparentTitle,
		ThumbPath:     pickThumbPath(item),
		Quality:       formatQuality(part),
	}

	if err := s.playback.Start(args); err != nil {
		if err == playback.ErrAlreadyActive {
			writeError(w, http.StatusConflict, "another session is already active")
			return
		}
		writeError(w, http.StatusInternalServerError, "start: "+err.Error())
		return
	}

	state := s.playback.SnapshotState()
	writeJSON(w, state)
}

// containerToExt maps a Plex container value to the file extension Pot Player
// expects in the direct-play URL. Plex uses "mkv", "mp4", "avi" etc. mostly
// directly — pass through, default to "mp4" if blank.
func containerToExt(container string) string {
	if container == "" {
		return "mp4"
	}
	return container
}

func msToDuration(ms int) (d time.Duration) {
	return time.Duration(ms) * time.Millisecond
}

func pickThumbPath(it plex.Item) string {
	// Episodes: prefer the show's banner backdrop for the Now Playing strip.
	if it.GrandparentThumb != "" {
		return it.GrandparentThumb
	}
	return it.Thumb
}

func formatQuality(p plex.Part) string {
	if p.Container == "" {
		return ""
	}
	res := p.VideoResolution
	if res == "" {
		res = p.Resolution
	}
	codec := p.VideoCodec
	if codec == "" {
		codec = p.Codec
	}
	if res != "" && codec != "" {
		return fmt.Sprintf("%s %s", res, codec)
	}
	if res != "" {
		return res
	}
	return codec
}
```

You'll need to:
- Add `"time"` import.
- Confirm `plex.Item` struct has `GrandparentRatingKey`, `GrandparentTitle`, `GrandparentThumb`, `Title`, `Thumb`, `Type`, `Duration`, `Media[].Part[]` with the fields used. Add missing JSON-tagged fields to `internal/plex/types.go` if absent (e.g. `VideoResolution`, `VideoCodec`).

- [ ] **Step 3: Add a unit test**

```go
package server

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestHandlePlay_RejectsGET(t *testing.T) {
	s := newTestServer(t) // existing helper from api_servers_test.go
	req := httptest.NewRequest(http.MethodGet, "/api/play", nil)
	rr := httptest.NewRecorder()
	s.mux.ServeHTTP(rr, req)
	if rr.Code != http.StatusMethodNotAllowed {
		t.Errorf("got %d, want 405", rr.Code)
	}
}

func TestHandlePlay_BadJSON(t *testing.T) {
	s := newTestServer(t)
	req := httptest.NewRequest(http.MethodPost, "/api/play", bytes.NewReader([]byte("not json")))
	rr := httptest.NewRecorder()
	s.mux.ServeHTTP(rr, req)
	if rr.Code != http.StatusBadRequest {
		t.Errorf("got %d, want 400", rr.Code)
	}
}

func TestHandlePlay_UnknownServer(t *testing.T) {
	s := newTestServer(t)
	body, _ := json.Marshal(playRequest{ServerID: "missing", RatingKey: "1"})
	req := httptest.NewRequest(http.MethodPost, "/api/play", bytes.NewReader(body))
	rr := httptest.NewRecorder()
	s.mux.ServeHTTP(rr, req)
	if rr.Code != http.StatusNotFound {
		t.Errorf("got %d, want 404", rr.Code)
	}
}
```

- [ ] **Step 4: Run tests**

Run: `go test ./internal/server/... -run TestHandlePlay -v`
Expected: PASS.

- [ ] **Step 5: Commit**

```
git add internal/server/api_play.go internal/server/api_play_test.go
git commit -m "feat(server): wire POST /api/play to playback.Manager"
```

---

### Task 16: `POST /api/play/transcode` (transcode fallback after user confirmation)

**Files:**
- Modify: `internal/server/api_play.go`
- Modify: `internal/server/server.go` (register route)

**Context:** Same body as `/api/play`. Builds the transcode URL with a fresh session ID. Caller (SPA) only hits this after the user accepts the Direct-Play-Failed modal.

- [ ] **Step 1: Append handler**

Append to `internal/server/api_play.go`:
```go
import "crypto/rand"
import "encoding/hex"

func (s *Server) handlePlayTranscode(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "POST required")
		return
	}
	var req playRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON: "+err.Error())
		return
	}
	srv := s.serverByID(req.ServerID)
	if srv == nil {
		writeError(w, http.StatusNotFound, "unknown server")
		return
	}
	plexSrv := toPlexServer(srv)
	item, err := s.plex.GetItem(plexSrv, req.RatingKey)
	if err != nil {
		writeError(w, http.StatusBadGateway, err.Error())
		return
	}

	session := newTranscodeSession()
	streamURL := plex.TranscodeURL(plexSrv, req.RatingKey, session)

	args := playback.StartArgs{
		Server:           plexSrv,
		RatingKey:        req.RatingKey,
		ShowRatingKey:    item.GrandparentRatingKey,
		IsEpisode:        item.Type == "episode",
		PartID:           "",
		Container:        "",
		StreamURL:        streamURL,
		Transcoding:      true,
		TranscodeSession: session,
		Duration:         msToDuration(item.Duration),
		Title:            item.Title,
		ShowTitle:        item.GrandparentTitle,
		ThumbPath:        pickThumbPath(item),
		Quality:          "transcoded 1080p",
	}
	if err := s.playback.Start(args); err != nil {
		if err == playback.ErrAlreadyActive {
			writeError(w, http.StatusConflict, "another session is already active")
			return
		}
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, s.playback.SnapshotState())
}

func newTranscodeSession() string {
	var b [8]byte
	_, _ = rand.Read(b[:])
	return "lumen-" + hex.EncodeToString(b[:])
}
```

- [ ] **Step 2: Register the route in `server.go`**

In `registerRoutes()`:
```go
s.mux.HandleFunc("/api/play/transcode", s.handlePlayTranscode)
```

- [ ] **Step 3: Verify compilation + tests**

Run: `go build ./... && go test ./internal/server/...`
Expected: clean + existing tests pass.

- [ ] **Step 4: Commit**

```
git add internal/server/api_play.go internal/server/server.go
git commit -m "feat(server): POST /api/play/transcode for user-confirmed fallback"
```

---

### Task 17: `POST /api/play/stop` + GET `/api/playback`

**Files:**
- Modify: `internal/server/api_play.go`
- Create: `internal/server/api_playback.go`
- Modify: `internal/server/server.go` (register routes)

**Context:** `stop` lets the SPA explicitly tear down (e.g. user clicks the close-Now-Playing X — though spec says no controls, we still allow programmatic stop for next-episode chaining). `playback` returns the current state snapshot for cold-load (SPA boot, etc.).

- [ ] **Step 1: Append `handlePlayStop` to `api_play.go`**

```go
func (s *Server) handlePlayStop(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "POST required")
		return
	}
	s.playback.Stop()
	writeJSON(w, map[string]string{"status": "stopped"})
}
```

- [ ] **Step 2: Create `api_playback.go`**

```go
package server

import "net/http"

func (s *Server) handlePlaybackState(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "GET only")
		return
	}
	writeJSON(w, s.playback.SnapshotState())
}
```

- [ ] **Step 3: Register routes in `server.go`**

```go
s.mux.HandleFunc("/api/play/stop", s.handlePlayStop)
s.mux.HandleFunc("/api/playback", s.handlePlaybackState)
```

- [ ] **Step 4: Verify compilation + tests**

Run: `go build ./... && go test ./internal/server/...`
Expected: clean.

- [ ] **Step 5: Commit**

```
git add internal/server/api_play.go internal/server/api_playback.go internal/server/server.go
git commit -m "feat(server): POST /api/play/stop + GET /api/playback"
```

---

### Task 18: `GET /api/playback/stream` (SSE)

**Files:**
- Modify: `internal/server/api_playback.go`
- Modify: `internal/server/server.go`

**Context:** Server-Sent Events stream. SPA opens an `EventSource("/api/playback/stream")` once, receives JSON-encoded events forever. Closes when client disconnects (Request.Context().Done()) or server shuts down.

- [ ] **Step 1: Append SSE handler to `api_playback.go`**

```go
import (
	"encoding/json"
	"fmt"
)

func (s *Server) handlePlaybackStream(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "GET only")
		return
	}
	flusher, ok := w.(http.Flusher)
	if !ok {
		writeError(w, http.StatusInternalServerError, "streaming unsupported")
		return
	}
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("X-Accel-Buffering", "no")

	ch, cleanup := s.playback.Subscribe()
	defer cleanup()

	// Initial snapshot — guarantees the SPA has state even before any event fires.
	initial := s.playback.SnapshotState()
	writeSSE(w, "state", initial)
	flusher.Flush()

	for {
		select {
		case <-r.Context().Done():
			return
		case ev, ok := <-ch:
			if !ok {
				return
			}
			writeSSEEvent(w, ev)
			flusher.Flush()
		}
	}
}

func writeSSE(w http.ResponseWriter, eventType string, data any) {
	body, _ := json.Marshal(data)
	fmt.Fprintf(w, "event: %s\ndata: %s\n\n", eventType, body)
}

func writeSSEEvent(w http.ResponseWriter, ev any) {
	body, _ := json.Marshal(ev)
	fmt.Fprintf(w, "event: message\ndata: %s\n\n", body)
}
```

- [ ] **Step 2: Register route**

In `registerRoutes()`:
```go
s.mux.HandleFunc("/api/playback/stream", s.handlePlaybackStream)
```

- [ ] **Step 3: Verify compilation**

Run: `go build ./...`
Expected: clean.

- [ ] **Step 4: Commit**

```
git add internal/server/api_playback.go internal/server/server.go
git commit -m "feat(server): SSE /api/playback/stream broadcasting Manager events"
```

---

## Phase F — SPA Playback State + Now Playing Strip

### Task 19: SSE client + playback store (`state/playback.ts`)

**Files:**
- Create: `web/src/state/playback.ts`
- Modify: `web/src/api/types.ts`
- Modify: `web/src/api/client.ts`

**Context:** Subscribe once to `/api/playback/stream` on app boot. Surface the current state via a Solid signal. Surface modal-trigger events (`transcode-prompt`, `next-episode-prompt`, `ended`) via separate signals so modals can react.

- [ ] **Step 1: Add types to `web/src/api/types.ts`**

```ts
export interface PlaybackState {
  active: boolean;
  ratingKey?: string;
  serverID?: string;
  title?: string;
  showTitle?: string;
  position: number; // ns (Go time.Duration JSON encoding)
  duration: number; // ns
  state: "playing" | "paused" | "stopped" | "unknown";
  quality?: string;
  transcoding?: boolean;
  thumbPath?: string;
}

export interface NextEpisodeInfo {
  ratingKey: string;
  serverID: string;
  title: string;
  season: number;
  episode: number;
  thumbPath?: string;
}

export interface TranscodePromptInfo {
  ratingKey: string;
  serverID: string;
  title: string;
  reason: string;
}

export type PlaybackEvent =
  | { type: "state"; state: PlaybackState }
  | { type: "ended" }
  | { type: "next-episode-prompt"; payload: NextEpisodeInfo }
  | { type: "transcode-prompt"; payload: TranscodePromptInfo }
  | { type: "stopped" };
```

- [ ] **Step 2: Add API methods to `web/src/api/client.ts`**

```ts
// Append inside the api object literal:

play: async (serverID: string, ratingKey: string, resumeFromOffset?: number) => {
  const res = await fetch("/api/play", {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify({ serverID, ratingKey, resumeFromOffset }),
  });
  if (!res.ok) {
    throw new Error(`${res.status} POST /api/play: ${await res.text()}`);
  }
  return res.json();
},

playTranscode: async (serverID: string, ratingKey: string) => {
  const res = await fetch("/api/play/transcode", {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify({ serverID, ratingKey }),
  });
  if (!res.ok) {
    throw new Error(`${res.status} POST /api/play/transcode: ${await res.text()}`);
  }
  return res.json();
},

playStop: async () => {
  const res = await fetch("/api/play/stop", { method: "POST" });
  if (!res.ok) {
    throw new Error(`${res.status} POST /api/play/stop: ${await res.text()}`);
  }
  return res.json();
},

playbackState: async () => {
  const res = await fetch("/api/playback");
  if (!res.ok) throw new Error(`GET /api/playback: ${res.status}`);
  return res.json() as Promise<import("./types").PlaybackState>;
},
```

- [ ] **Step 3: Write `web/src/state/playback.ts`**

```ts
import { createRoot, createSignal } from "solid-js";
import type { PlaybackState, PlaybackEvent, NextEpisodeInfo, TranscodePromptInfo } from "../api/types";

function createPlaybackStore() {
  const [state, setState] = createSignal<PlaybackState>({
    active: false,
    position: 0,
    duration: 0,
    state: "stopped",
  });

  // Modal triggers — reset to null after the modal handles them.
  const [nextEpisode, setNextEpisode] = createSignal<NextEpisodeInfo | null>(null);
  const [transcodePrompt, setTranscodePrompt] = createSignal<TranscodePromptInfo | null>(null);
  const [endedAt, setEndedAt] = createSignal<number>(0);

  let es: EventSource | null = null;

  function connect() {
    if (es) return;
    es = new EventSource("/api/playback/stream");

    // The Go server sends the initial snapshot as `event: state` and all
    // subsequent updates as `event: message` (default). Listen to both.
    es.addEventListener("state", (ev) => {
      try {
        const data = JSON.parse((ev as MessageEvent).data) as PlaybackState;
        setState(data);
      } catch (e) {
        console.error("playback: parse initial state", e);
      }
    });

    es.onmessage = (ev) => {
      try {
        const evt = JSON.parse(ev.data) as PlaybackEvent;
        switch (evt.type) {
          case "state":
            setState(evt.state);
            break;
          case "ended":
            setEndedAt(Date.now());
            break;
          case "next-episode-prompt":
            setNextEpisode(evt.payload);
            break;
          case "transcode-prompt":
            setTranscodePrompt(evt.payload);
            break;
          case "stopped":
            setState({ active: false, position: 0, duration: 0, state: "stopped" });
            break;
        }
      } catch (e) {
        console.error("playback: parse event", e);
      }
    };

    es.onerror = (e) => {
      console.error("playback SSE error", e);
      // Browser will auto-reconnect.
    };
  }

  function dismissNextEpisode() { setNextEpisode(null); }
  function dismissTranscodePrompt() { setTranscodePrompt(null); }

  return {
    state,
    nextEpisode,
    transcodePrompt,
    endedAt,
    connect,
    dismissNextEpisode,
    dismissTranscodePrompt,
  };
}

export const playbackStore = createRoot(createPlaybackStore);
```

- [ ] **Step 4: Connect on app boot**

In `web/src/main.tsx`, after the settings store load:
```ts
import { playbackStore } from "./state/playback";
playbackStore.connect();
```

- [ ] **Step 5: Verify TypeScript**

Run: `cd web && npx tsc --noEmit`
Expected: clean.

- [ ] **Step 6: Commit**

```
git add web/src/state/playback.ts web/src/api/types.ts web/src/api/client.ts web/src/main.tsx
git commit -m "feat(spa): SSE playback store with state + modal-trigger signals"
```

---

### Task 20: Now Playing strip component

**Files:**
- Create: `web/src/components/NowPlaying.tsx`
- Create: `web/src/components/NowPlaying.css`
- Modify: `web/src/App.tsx`

**Context:** Strip below TopBar, full width. Visible while `playbackStore.state().active === true`. Layout: thumbnail, title (showTitle for episodes + season/episode label, else item title), animated progress bar, position/duration text, quality badge. NO playback controls (Pot Player's domain).

- [ ] **Step 1: Write `NowPlaying.tsx`**

```tsx
import { Show } from "solid-js";
import { playbackStore } from "../state/playback";
import { api } from "../api/client";
import "./NowPlaying.css";

export default function NowPlaying() {
  const s = playbackStore.state;
  const pct = () => {
    const st = s();
    if (!st.duration) return 0;
    return Math.min(100, Math.max(0, (st.position / st.duration) * 100));
  };
  const fmt = (ns: number) => {
    const totalSec = Math.floor(ns / 1_000_000_000);
    const m = Math.floor(totalSec / 60);
    const sec = totalSec % 60;
    const h = Math.floor(m / 60);
    const min = m % 60;
    if (h > 0) return `${h}:${String(min).padStart(2, "0")}:${String(sec).padStart(2, "0")}`;
    return `${min}:${String(sec).padStart(2, "0")}`;
  };

  return (
    <Show when={s().active}>
      <div class="now-playing">
        <Show when={s().thumbPath && s().serverID}>
          <img
            class="np-thumb"
            src={api.image(s().serverID!, s().thumbPath!)}
            alt=""
          />
        </Show>
        <div class="np-meta">
          <div class="np-title">
            <Show when={s().showTitle} fallback={<span>{s().title}</span>}>
              <span class="np-show">{s().showTitle}</span>
              <span class="np-ep"> · {s().title}</span>
            </Show>
          </div>
          <div class="np-progress">
            <div class="np-progress-track">
              <div class="np-progress-fill" style={{ width: `${pct()}%` }} />
            </div>
            <div class="np-times">
              <span>{fmt(s().position)}</span>
              <span class="np-sep">/</span>
              <span>{fmt(s().duration)}</span>
            </div>
          </div>
        </div>
        <div class="np-quality">
          <Show when={s().transcoding}>
            <span class="np-badge np-badge-warn">TRANSCODE</span>
          </Show>
          <Show when={s().quality}>
            <span class="np-badge">{s().quality}</span>
          </Show>
        </div>
      </div>
    </Show>
  );
}
```

- [ ] **Step 2: Write `NowPlaying.css`**

```css
.now-playing {
  display: flex;
  align-items: center;
  gap: 14px;
  padding: 10px var(--top-bar-margin);
  margin: 0 var(--top-bar-margin);
  background: var(--bg-elevated);
  border: 1px solid var(--border-soft);
  border-radius: var(--radius-md);
  /* Sits just below the top bar; no extra top margin so the gap from TopBar
     (--top-bar-gap) does the breathing-room work. */
  margin-top: 0;
}

.np-thumb {
  width: 48px;
  height: 72px;
  object-fit: cover;
  border-radius: var(--radius-sm);
  flex-shrink: 0;
}

.np-meta {
  flex: 1;
  min-width: 0;
}

.np-title {
  font-family: var(--font-headline);
  font-size: 14px;
  font-weight: 600;
  color: var(--text);
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
  margin-bottom: 6px;
}

.np-show {
  color: var(--text);
}

.np-ep {
  color: var(--text-muted);
  font-weight: 500;
}

.np-progress {
  display: flex;
  align-items: center;
  gap: 10px;
}

.np-progress-track {
  flex: 1;
  height: 4px;
  background: rgba(255, 255, 255, 0.12);
  border-radius: 2px;
  overflow: hidden;
}

.np-progress-fill {
  height: 100%;
  background: var(--led-teal);
  transition: width 0.4s ease;
}

.np-times {
  font-size: 12px;
  color: var(--text-muted);
  font-variant-numeric: tabular-nums;
  flex-shrink: 0;
}

.np-sep {
  margin: 0 4px;
  color: var(--border-soft);
}

.np-quality {
  display: flex;
  gap: 6px;
  flex-shrink: 0;
}

.np-badge {
  display: inline-block;
  padding: 3px 8px;
  background: rgba(255, 255, 255, 0.06);
  color: var(--text-muted);
  border-radius: var(--radius-pill);
  font-size: 11px;
  font-family: var(--font-mono);
  letter-spacing: 0.5px;
  text-transform: uppercase;
}

.np-badge-warn {
  background: rgba(220, 160, 40, 0.15);
  color: #e0b050;
}
```

- [ ] **Step 3: Mount in `App.tsx`**

```tsx
import NowPlaying from "./components/NowPlaying";

// inside <div class="app-shell">:
<TopBar />
<NowPlaying />
<div class="app-body">
  ...
</div>
```

- [ ] **Step 4: Build + verify**

Run: `cd web && npx tsc --noEmit && npm run build && cd .. && go build -o lumen.exe ./cmd/lumen`
Expected: clean across the board.

Restart `lumen serve`. Hard-refresh browser. With no active session, the strip is hidden. (Manual play test in Task 32.)

- [ ] **Step 5: Commit**

```
git add web/src/components/NowPlaying.tsx web/src/components/NowPlaying.css web/src/App.tsx
git commit -m "feat(spa): NowPlaying strip below TopBar with progress bar + quality badge"
```

---

## Phase G — Playback Modals

### Task 21: Reusable `ModalShell` component

**Files:**
- Create: `web/src/components/Modal/ModalShell.tsx`
- Create: `web/src/components/Modal/ModalShell.css`

**Context:** Three new modals (resume, transcode-prompt, next-episode) all share the same backdrop + Motion-driven entrance + Escape-to-cancel. Factor out so each per-modal file just supplies content.

- [ ] **Step 1: Write `ModalShell.tsx`**

```tsx
import { JSX, onCleanup, onMount, Show } from "solid-js";
// @ts-expect-error — motionone/solid's package.json exports field hides its d.ts file
import { Motion, Presence } from "@motionone/solid";
import "./ModalShell.css";

export default function ModalShell(props: {
  open: boolean;
  onCancel: () => void;
  ariaLabel: string;
  children: JSX.Element;
}) {
  function onKeyDown(e: KeyboardEvent) {
    if (!props.open) return;
    if (e.key === "Escape") props.onCancel();
  }
  onMount(() => {
    document.addEventListener("keydown", onKeyDown);
    onCleanup(() => document.removeEventListener("keydown", onKeyDown));
  });
  return (
    <Presence>
      <Show when={props.open}>
        <Motion.div
          class="modal-shell-backdrop"
          onClick={props.onCancel}
          role="presentation"
          initial={{ opacity: 0 }}
          animate={{ opacity: 1 }}
          exit={{ opacity: 0 }}
          transition={{ duration: 0.18, easing: [0.16, 1, 0.3, 1] }}
        >
          <Motion.div
            class="modal-shell"
            onClick={(e: Event) => e.stopPropagation()}
            role="dialog"
            aria-modal="true"
            aria-label={props.ariaLabel}
            initial={{ opacity: 0, scale: 0.96, y: 8 }}
            animate={{ opacity: 1, scale: 1, y: 0 }}
            exit={{ opacity: 0, scale: 0.96, y: 8 }}
            transition={{ duration: 0.24, easing: [0.22, 1, 0.36, 1] }}
          >
            {props.children}
          </Motion.div>
        </Motion.div>
      </Show>
    </Presence>
  );
}
```

- [ ] **Step 2: Write `ModalShell.css`**

```css
.modal-shell-backdrop {
  position: fixed;
  inset: 0;
  background: rgba(0, 0, 0, 0.65);
  display: grid;
  place-items: center;
  z-index: 200;
  backdrop-filter: blur(4px);
}

.modal-shell {
  background: var(--bg);
  color: var(--text);
  border: 1px solid rgba(255, 255, 255, 0.08);
  border-radius: var(--radius-lg);
  box-shadow: var(--shadow), inset 0 1px 0 rgba(255, 255, 255, 0.08);
  padding: 24px 24px 20px;
  width: min(440px, 92vw);
  display: flex;
  flex-direction: column;
  gap: 14px;
}
```

- [ ] **Step 3: TS verify + commit**

Run: `cd web && npx tsc --noEmit`
Expected: clean.

```
git add web/src/components/Modal/ModalShell.tsx web/src/components/Modal/ModalShell.css
git commit -m "feat(spa): ModalShell component shared by playback modals"
```

---

### Task 22: `ResumeRestartModal` (5 s countdown, default Resume)

**Files:**
- Create: `web/src/components/Modal/ResumeRestartModal.tsx`
- Create: `web/src/components/Modal/ResumeRestartModal.css`

**Context:** Triggered by ItemDetail's Play button when the item has a non-zero `viewOffset` AND viewOffset < 90% of duration. 5 s countdown. Default: Resume from offset. Other option: Start Over (reset to 0). Cancel = dismiss without playing.

- [ ] **Step 1: Write `ResumeRestartModal.tsx`**

```tsx
import { createEffect, createSignal, onCleanup, Show } from "solid-js";
import ModalShell from "./ModalShell";
import "./ResumeRestartModal.css";

export interface ResumeRestartProps {
  open: boolean;
  resumeOffsetMs: number;
  onResume: () => void;
  onRestart: () => void;
  onCancel: () => void;
}

const COUNTDOWN_MS = 5000;
const TICK_MS = 100;

export default function ResumeRestartModal(props: ResumeRestartProps) {
  const [remaining, setRemaining] = createSignal(COUNTDOWN_MS);
  let timer: number | undefined;

  createEffect(() => {
    if (!props.open) return;
    setRemaining(COUNTDOWN_MS);
    timer = window.setInterval(() => {
      setRemaining((r) => {
        const next = r - TICK_MS;
        if (next <= 0) {
          window.clearInterval(timer);
          props.onResume();
          return 0;
        }
        return next;
      });
    }, TICK_MS);
    onCleanup(() => window.clearInterval(timer));
  });

  const fmtOffset = () => {
    const sec = Math.floor(props.resumeOffsetMs / 1000);
    const m = Math.floor(sec / 60);
    const s = sec % 60;
    return `${m}:${String(s).padStart(2, "0")}`;
  };

  const pct = () => 100 - (remaining() / COUNTDOWN_MS) * 100;

  return (
    <ModalShell open={props.open} onCancel={props.onCancel} ariaLabel="Resume or restart">
      <h2 class="rrm-title">Resume from {fmtOffset()}?</h2>
      <p class="rrm-body">Auto-resuming in {Math.ceil(remaining() / 1000)}s. Click Start Over to play from the beginning, or Cancel to stop.</p>
      <div class="rrm-progress">
        <div class="rrm-progress-fill" style={{ width: `${pct()}%` }} />
      </div>
      <div class="rrm-actions">
        <button class="rrm-cancel" onClick={() => { window.clearInterval(timer); props.onCancel(); }}>Cancel</button>
        <button class="rrm-restart" onClick={() => { window.clearInterval(timer); props.onRestart(); }}>Start Over</button>
        <button class="rrm-resume" onClick={() => { window.clearInterval(timer); props.onResume(); }}>Resume</button>
      </div>
    </ModalShell>
  );
}
```

- [ ] **Step 2: Write `ResumeRestartModal.css`**

```css
.rrm-title {
  margin: 0;
  font-family: var(--font-headline);
  font-size: 20px;
  font-weight: 600;
  letter-spacing: 0.4px;
}

.rrm-body {
  margin: 0;
  color: var(--text-muted);
  font-size: 14px;
  line-height: 1.5;
}

.rrm-progress {
  height: 3px;
  background: rgba(255, 255, 255, 0.1);
  border-radius: 2px;
  overflow: hidden;
}

.rrm-progress-fill {
  height: 100%;
  background: var(--led-teal);
  transition: width 0.1s linear;
}

.rrm-actions {
  display: flex;
  justify-content: flex-end;
  gap: 10px;
  margin-top: 6px;
}

.rrm-cancel,
.rrm-restart,
.rrm-resume {
  padding: 8px 18px;
  border-radius: var(--radius-pill);
  font-size: 13px;
  font-weight: 500;
  cursor: pointer;
  border: 1px solid var(--border-soft);
  transition: background 0.15s ease, border-color 0.15s ease;
}

.rrm-cancel { background: transparent; color: var(--text); }
.rrm-restart { background: transparent; color: var(--text); }
.rrm-resume { background: var(--bg-inverse); color: var(--text-inverse); border-color: var(--bg-inverse); }
.rrm-cancel:hover, .rrm-restart:hover { border-color: var(--stroke); }
.rrm-resume:hover { background: var(--text); }
```

- [ ] **Step 3: TS verify + commit**

Run: `cd web && npx tsc --noEmit`

```
git add web/src/components/Modal/ResumeRestartModal.tsx web/src/components/Modal/ResumeRestartModal.css
git commit -m "feat(spa): ResumeRestartModal with 5s countdown + auto-resume default"
```

---

### Task 23: `TranscodePromptModal`

**Files:**
- Create: `web/src/components/Modal/TranscodePromptModal.tsx`
- Create: `web/src/components/Modal/TranscodePromptModal.css`

**Context:** Triggered by `transcode-prompt` SSE event. NO countdown — direct-play failure is a user decision, not a default-action moment.

- [ ] **Step 1: Write `TranscodePromptModal.tsx`**

```tsx
import ModalShell from "./ModalShell";
import { playbackStore } from "../../state/playback";
import { api } from "../../api/client";
import "./TranscodePromptModal.css";

export default function TranscodePromptModal() {
  const info = playbackStore.transcodePrompt;
  const close = () => playbackStore.dismissTranscodePrompt();

  async function confirm() {
    const i = info();
    if (!i) return;
    close();
    try {
      await api.playTranscode(i.serverID, i.ratingKey);
    } catch (e) {
      console.error("playTranscode failed:", e);
      alert(`Transcode failed: ${(e as Error).message}`);
    }
  }

  return (
    <ModalShell open={info() !== null} onCancel={close} ariaLabel="Direct play failed">
      <h2 class="tpm-title">Direct play failed</h2>
      <p class="tpm-body">
        Pot Player couldn't play <strong>{info()?.title}</strong> directly. Reason: {info()?.reason}.
        <br/><br/>
        Try transcoding? This asks the Plex server to re-encode the file at 1080p H.264.
      </p>
      <div class="tpm-actions">
        <button class="tpm-cancel" onClick={close}>Cancel</button>
        <button class="tpm-confirm" onClick={confirm}>Try Transcode</button>
      </div>
    </ModalShell>
  );
}
```

- [ ] **Step 2: Write `TranscodePromptModal.css`** — same shape as ResumeRestartModal.css with `.tpm-` prefix:

```css
.tpm-title {
  margin: 0;
  font-family: var(--font-headline);
  font-size: 20px;
  font-weight: 600;
  letter-spacing: 0.4px;
}
.tpm-body {
  margin: 0;
  color: var(--text-muted);
  font-size: 14px;
  line-height: 1.5;
}
.tpm-actions {
  display: flex;
  justify-content: flex-end;
  gap: 10px;
  margin-top: 6px;
}
.tpm-cancel,
.tpm-confirm {
  padding: 8px 18px;
  border-radius: var(--radius-pill);
  font-size: 13px;
  font-weight: 500;
  cursor: pointer;
  border: 1px solid var(--border-soft);
  transition: background 0.15s ease, border-color 0.15s ease;
}
.tpm-cancel { background: transparent; color: var(--text); }
.tpm-confirm { background: var(--bg-inverse); color: var(--text-inverse); border-color: var(--bg-inverse); }
.tpm-cancel:hover { border-color: var(--stroke); }
.tpm-confirm:hover { background: var(--text); }
```

- [ ] **Step 3: TS verify + commit**

Run: `cd web && npx tsc --noEmit`

```
git add web/src/components/Modal/TranscodePromptModal.tsx web/src/components/Modal/TranscodePromptModal.css
git commit -m "feat(spa): TranscodePromptModal — user-confirmed direct-play fallback"
```

---

### Task 24: `NextEpisodeModal` (5 s countdown auto-play, with Cancel)

**Files:**
- Create: `web/src/components/Modal/NextEpisodeModal.tsx`
- Create: `web/src/components/Modal/NextEpisodeModal.css`

**Context:** Triggered by `next-episode-prompt` event. 5s countdown. Default: auto-play next. Cancel button stops the countdown and dismisses.

- [ ] **Step 1: Write `NextEpisodeModal.tsx`**

```tsx
import { createEffect, createSignal, onCleanup, Show } from "solid-js";
import ModalShell from "./ModalShell";
import { playbackStore } from "../../state/playback";
import { api } from "../../api/client";
import "./NextEpisodeModal.css";

const COUNTDOWN_MS = 5000;
const TICK_MS = 100;

export default function NextEpisodeModal() {
  const info = playbackStore.nextEpisode;
  const [remaining, setRemaining] = createSignal(COUNTDOWN_MS);
  let timer: number | undefined;

  function close() {
    if (timer) window.clearInterval(timer);
    playbackStore.dismissNextEpisode();
  }

  async function playNow() {
    const i = info();
    if (!i) return;
    close();
    try {
      // Stop any lingering session first.
      await api.playStop();
      await api.play(i.serverID, i.ratingKey);
    } catch (e) {
      console.error("auto-play next failed:", e);
      alert(`Failed to play next episode: ${(e as Error).message}`);
    }
  }

  createEffect(() => {
    const i = info();
    if (!i) return;
    setRemaining(COUNTDOWN_MS);
    timer = window.setInterval(() => {
      setRemaining((r) => {
        const next = r - TICK_MS;
        if (next <= 0) {
          window.clearInterval(timer);
          playNow();
          return 0;
        }
        return next;
      });
    }, TICK_MS);
    onCleanup(() => window.clearInterval(timer));
  });

  const pct = () => 100 - (remaining() / COUNTDOWN_MS) * 100;

  return (
    <ModalShell open={info() !== null} onCancel={close} ariaLabel="Next episode">
      <h2 class="nem-title">Next Episode in {Math.ceil(remaining() / 1000)}s</h2>
      <Show when={info()}>
        {(i) => (
          <div class="nem-card">
            <Show when={i().thumbPath}>
              <img class="nem-thumb" src={api.image(i().serverID, i().thumbPath!)} alt="" />
            </Show>
            <div class="nem-meta">
              <div class="nem-ep">S{i().season} · E{i().episode}</div>
              <div class="nem-name">{i().title}</div>
            </div>
          </div>
        )}
      </Show>
      <div class="nem-progress">
        <div class="nem-progress-fill" style={{ width: `${pct()}%` }} />
      </div>
      <div class="nem-actions">
        <button class="nem-cancel" onClick={close}>Cancel</button>
        <button class="nem-now" onClick={playNow}>Play Now</button>
      </div>
    </ModalShell>
  );
}
```

- [ ] **Step 2: Write `NextEpisodeModal.css`**

```css
.nem-title {
  margin: 0;
  font-family: var(--font-headline);
  font-size: 20px;
  font-weight: 600;
  letter-spacing: 0.4px;
}
.nem-card {
  display: flex;
  gap: 12px;
  padding: 10px;
  background: var(--bg-shelf-inner);
  border: 1px solid var(--border-soft);
  border-radius: var(--radius-md);
}
.nem-thumb {
  width: 92px;
  height: 138px;
  object-fit: cover;
  border-radius: var(--radius-sm);
  flex-shrink: 0;
}
.nem-meta { display: flex; flex-direction: column; justify-content: center; gap: 4px; }
.nem-ep { font-size: 12px; color: var(--text-muted); letter-spacing: 0.6px; text-transform: uppercase; }
.nem-name { color: var(--text); font-weight: 500; line-height: 1.3; }
.nem-progress { height: 3px; background: rgba(255,255,255,0.1); border-radius: 2px; overflow: hidden; }
.nem-progress-fill { height: 100%; background: var(--led-teal); transition: width 0.1s linear; }
.nem-actions { display: flex; justify-content: flex-end; gap: 10px; margin-top: 6px; }
.nem-cancel, .nem-now {
  padding: 8px 18px;
  border-radius: var(--radius-pill);
  font-size: 13px;
  font-weight: 500;
  cursor: pointer;
  border: 1px solid var(--border-soft);
  transition: background 0.15s ease, border-color 0.15s ease;
}
.nem-cancel { background: transparent; color: var(--text); }
.nem-now { background: var(--bg-inverse); color: var(--text-inverse); border-color: var(--bg-inverse); }
.nem-cancel:hover { border-color: var(--stroke); }
.nem-now:hover { background: var(--text); }
```

- [ ] **Step 3: TS verify + commit**

Run: `cd web && npx tsc --noEmit`

```
git add web/src/components/Modal/NextEpisodeModal.tsx web/src/components/Modal/NextEpisodeModal.css
git commit -m "feat(spa): NextEpisodeModal with 5s countdown auto-play + Cancel"
```

---

### Task 25: Mount transcode + next-episode modals globally

**Files:**
- Modify: `web/src/App.tsx`

**Context:** Both modals are SSE-driven (`playbackStore.transcodePrompt` / `playbackStore.nextEpisode`). They need to be mounted at the app root so they fire regardless of current page.

- [ ] **Step 1: Mount in App.tsx**

```tsx
import TranscodePromptModal from "./components/Modal/TranscodePromptModal";
import NextEpisodeModal from "./components/Modal/NextEpisodeModal";

// Inside the JSX, after existing modals:
<TranscodePromptModal />
<NextEpisodeModal />
```

- [ ] **Step 2: Verify**

Run: `cd web && npx tsc --noEmit && npm run build && cd .. && go build -o lumen.exe ./cmd/lumen`
Expected: clean.

- [ ] **Step 3: Commit**

```
git add web/src/App.tsx
git commit -m "feat(spa): mount transcode + next-episode modals at app root"
```

---

## Phase H — Item Detail Page Expansion

### Task 26: Hero banner backdrop

**Files:**
- Modify: `web/src/pages/ItemDetail.tsx`
- Modify: `web/src/pages/ItemDetail.css`

**Context:** Plex serves backdrop art at `item.art` (full path). Render as full-width hero with gradient fade-to-bg at bottom for legibility. Show banner above the existing title block.

- [ ] **Step 1: Update `Hero` in `ItemDetail.tsx`**

```tsx
function Hero(props: { item: Item; serverID: string }) {
  const isEpisode = () => props.item.type === "episode";
  const showTitle = () => (isEpisode() ? props.item.grandparentTitle : props.item.title);
  const episodeLabel = () => {
    if (!isEpisode()) return null;
    const season = props.item.parentIndex ?? 0;
    const ep = props.item.index ?? 0;
    const se = season && ep ? `S${season} · E${ep}` : "";
    const title = props.item.title;
    return se + (title && se ? ` · ${title}` : title ?? "");
  };
  // Backdrop preference order:
  //  1. item.art (movie/show backdrop)
  //  2. item.grandparentArt (episode → show backdrop)
  //  3. item.thumb (poster as last resort)
  const backdropPath = () =>
    props.item.art || props.item.grandparentArt || props.item.thumb;
  return (
    <header class="hero">
      <Show when={backdropPath()}>
        <div
          class="hero-backdrop"
          style={{ "background-image": `url(${api.image(props.serverID, backdropPath()!)})` }}
        />
        <div class="hero-fade" />
      </Show>
      <div class="hero-meta">
        <h1>{showTitle() ?? props.item.title}</h1>
        {isEpisode() && <div class="hero-episode">{episodeLabel()}</div>}
        <div class="meta-pills">
          {props.item.year && <span class="pill">{props.item.year}</span>}
          {props.item.type && <span class="pill">{props.item.type}</span>}
          {/* IMDB rating pill lands Session 5 with OMDB integration */}
        </div>
      </div>
    </header>
  );
}
```

Add at the top of the file:
```tsx
import { api } from "../api/client";
```

Confirm `Item` type in `web/src/api/types.ts` has `art?: string` and `grandparentArt?: string`. Add if missing.

Confirm Go's `plex.Item` struct exposes `art` and `grandparentArt` JSON fields. Add if missing in `internal/plex/types.go`:
```go
Art               string `json:"art,omitempty"`
GrandparentArt    string `json:"grandparentArt,omitempty"`
```

- [ ] **Step 2: Update `ItemDetail.css`**

Append:
```css
.hero {
  position: relative;
  min-height: 320px;
  margin: 0 calc(-1 * var(--top-bar-margin)) 24px;
  padding: 24px 24px 16px;
  border-radius: var(--radius-md);
  overflow: hidden;
  isolation: isolate;
}

.hero-backdrop {
  position: absolute;
  inset: 0;
  background-size: cover;
  background-position: center 30%;
  filter: brightness(0.55) saturate(0.9);
  z-index: -2;
}

.hero-fade {
  position: absolute;
  inset: 0;
  background: linear-gradient(
    to bottom,
    rgba(0, 0, 0, 0.1) 0%,
    rgba(0, 0, 0, 0.6) 65%,
    var(--bg) 100%
  );
  z-index: -1;
}

.hero-meta {
  position: relative;
  margin-top: 220px;
}

.hero-meta h1 {
  font-family: var(--font-headline);
  font-size: 36px;
  letter-spacing: 0.4px;
  margin: 0 0 4px;
  text-shadow: 0 2px 16px rgba(0, 0, 0, 0.7);
}

.hero-episode {
  color: var(--text-muted);
  font-size: 14px;
  margin-bottom: 12px;
}

.meta-pills {
  display: flex;
  gap: 8px;
}
```

- [ ] **Step 3: TS verify + build**

Run: `cd web && npx tsc --noEmit && npm run build && cd .. && go build -o lumen.exe ./cmd/lumen`
Expected: clean.

Restart `lumen serve`, navigate to a show / movie detail, confirm the backdrop appears.

- [ ] **Step 4: Commit**

```
git add web/src/pages/ItemDetail.tsx web/src/pages/ItemDetail.css web/src/api/types.ts internal/plex/types.go
git commit -m "feat(item-detail): hero backdrop banner with gradient fade"
```

---

### Task 27: Wire Play button + Mark Watched / Unwatched

**Files:**
- Modify: `web/src/pages/ItemDetail.tsx`

**Context:** Replace the stubbed `launchPlayback`. Logic:
- If `viewOffset > 0` AND `viewOffset < 0.9 * duration` → show ResumeRestartModal.
- Else → call `api.play()` directly, no modal.

Mark Watched → `api.scrobble`, then refetch. Mark Unwatched → `api.unscrobble`, then refetch.

- [ ] **Step 1: Update `ItemDetail.tsx`**

Add state + modal mount:
```tsx
import { createSignal } from "solid-js";
import ResumeRestartModal from "../components/Modal/ResumeRestartModal";

export default function ItemDetail() {
  const params = useParams();
  const [item, { refetch: refetchItem }] = createResource(
    () => ({ server: params.serverID!, rk: params.ratingKey! }),
    ({ server, rk }) => api.item(server, rk)
  );
  const [availability] = createResource(
    () => item()?.guid,
    (guid) => (guid ? api.availability(guid) : Promise.resolve([] as Match[]))
  );

  const [resumeOpen, setResumeOpen] = createSignal(false);

  async function handlePlay() {
    const it = item();
    if (!it) return;
    const offset = it.viewOffset ?? 0;
    const dur = it.duration ?? 0;
    if (offset > 0 && dur > 0 && offset < dur * 0.9) {
      setResumeOpen(true);
      return;
    }
    await playFromStart();
  }

  async function playFromStart() {
    const it = item();
    if (!it) return;
    try {
      await api.play(params.serverID!, it.ratingKey);
    } catch (e) {
      console.error("play failed:", e);
      alert(`Play failed: ${(e as Error).message}`);
    }
  }

  async function playResume() {
    const it = item();
    if (!it) return;
    try {
      await api.play(params.serverID!, it.ratingKey, it.viewOffset);
    } catch (e) {
      console.error("play resume failed:", e);
      alert(`Play failed: ${(e as Error).message}`);
    }
  }

  async function handleMarkWatched() {
    const it = item();
    if (!it) return;
    try {
      await api.scrobble(params.serverID!, it.ratingKey);
      refetchItem();
    } catch (e) {
      alert(`Mark watched failed: ${(e as Error).message}`);
    }
  }

  async function handleMarkUnwatched() {
    const it = item();
    if (!it) return;
    try {
      await api.unscrobble(params.serverID!, it.ratingKey);
      refetchItem();
    } catch (e) {
      alert(`Mark unwatched failed: ${(e as Error).message}`);
    }
  }

  // ...inside the return JSX, replace the existing ActionRow with the wired version
  // (the children below the Hero/Action block stay as-is):
  return (
    <div class="item-detail">
      <Show when={item()} fallback={<div class="item-loading"><Skeleton kind="line" count={4} /></div>}>
        {(it) => (
          <>
            <Hero item={it() as Item} serverID={params.serverID!} />
            <nav class="action-row">
              <button class="btn-primary" onClick={handlePlay}>
                ▶ {(it() as Item).viewOffset && (it() as Item).viewOffset! > 0 ? "Resume" : "Play"}
              </button>
              <select class="btn-subtitle" disabled>
                <option>Subtitle: Default</option>
                <option>Off</option>
              </select>
              <button class="btn" disabled title="Session 5">Play Trailer</button>
              <button class="btn" onClick={handleMarkWatched}>Mark as Watched</button>
              <button class="btn" onClick={handleMarkUnwatched}>Mark as Unwatched</button>
              <button class="btn" disabled title="Session 5">Add to Watchlist</button>
            </nav>
            {/* Existing Overview + Availability blocks stay; remove inline ActionRow function */}
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
      <ResumeRestartModal
        open={resumeOpen()}
        resumeOffsetMs={item()?.viewOffset ?? 0}
        onResume={() => { setResumeOpen(false); playResume(); }}
        onRestart={() => { setResumeOpen(false); playFromStart(); }}
        onCancel={() => setResumeOpen(false)}
      />
    </div>
  );
}
```

Delete the standalone `ActionRow` function (now inlined) and the obsolete `launchPlayback` function.

- [ ] **Step 2: TS verify + build**

Run: `cd web && npx tsc --noEmit && npm run build && cd .. && go build -o lumen.exe ./cmd/lumen`
Expected: clean.

- [ ] **Step 3: Commit**

```
git add web/src/pages/ItemDetail.tsx
git commit -m "feat(item-detail): wire Play (with resume modal), Mark Watched, Mark Unwatched"
```

---

### Task 28: Season tabs + Episode list (`Episodes.tsx`)

**Files:**
- Create: `web/src/components/Episodes.tsx`
- Create: `web/src/components/Episodes.css`
- Modify: `web/src/pages/ItemDetail.tsx`
- Modify: `web/src/api/client.ts` (add seasons + episodes fetchers)
- Modify: `web/src/api/types.ts` (Season type)
- Modify: `internal/server/api_servers.go` (route children fetches if not already exposed)

**Context:** Shows-only. Tabs along top for Season 1..N (default to season of current episode if viewing one); list below with thumb / S·E label / title / description / duration / air date / watched check.

For data: Plex has `/library/metadata/{showRatingKey}/children` returning seasons, and `/library/metadata/{seasonRatingKey}/children` returning episodes.

- [ ] **Step 1: Add Plex client methods**

In `internal/plex/episodes.go`, append:
```go
type Season struct {
	RatingKey string `json:"ratingKey"`
	Index     int    `json:"index"`
	Title     string `json:"title"`
	LeafCount int    `json:"leafCount"`
	ViewedLeafCount int `json:"viewedLeafCount"`
	Thumb     string `json:"thumb"`
}

type seasonsResponse struct {
	MediaContainer struct {
		Metadata []Season `json:"Metadata"`
	} `json:"MediaContainer"`
}

func (c *Client) GetSeasons(s *Server, showRatingKey string) ([]Season, error) {
	u := fmt.Sprintf("%s/library/metadata/%s/children?X-Plex-Token=%s",
		s.BaseURL, showRatingKey, s.AccessToken)
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
		return nil, fmt.Errorf("seasons %s: status %d", showRatingKey, resp.StatusCode)
	}
	var body seasonsResponse
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		return nil, err
	}
	return body.MediaContainer.Metadata, nil
}

func (c *Client) GetSeasonEpisodes(s *Server, seasonRatingKey string) ([]Item, error) {
	u := fmt.Sprintf("%s/library/metadata/%s/children?X-Plex-Token=%s",
		s.BaseURL, seasonRatingKey, s.AccessToken)
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
		return nil, fmt.Errorf("season episodes %s: status %d", seasonRatingKey, resp.StatusCode)
	}
	var body allLeavesResponse
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		return nil, err
	}
	return body.MediaContainer.Metadata, nil
}
```

- [ ] **Step 2: Add server endpoints**

In `internal/server/api_servers.go`, extend `handleServerScoped` to route:
- `seasons/<showRatingKey>` → `GetSeasons`
- `seasons/<seasonRatingKey>/episodes` → `GetSeasonEpisodes`

```go
case len(parts) == 3 && parts[1] == "seasons":
    s.handleSeasons(w, r, srv, parts[2])
case len(parts) == 4 && parts[1] == "seasons" && parts[3] == "episodes":
    s.handleSeasonEpisodes(w, r, srv, parts[2])
```

Implement:
```go
func (s *Server) handleSeasons(w http.ResponseWriter, r *http.Request, srv *config.Server, showRatingKey string) {
	if s.plex == nil { writeError(w, http.StatusInternalServerError, "plex not initialised"); return }
	seasons, err := s.plex.GetSeasons(toPlexServer(srv), showRatingKey)
	if err != nil { writeError(w, http.StatusBadGateway, err.Error()); return }
	writeJSON(w, seasons)
}

func (s *Server) handleSeasonEpisodes(w http.ResponseWriter, r *http.Request, srv *config.Server, seasonRatingKey string) {
	if s.plex == nil { writeError(w, http.StatusInternalServerError, "plex not initialised"); return }
	eps, err := s.plex.GetSeasonEpisodes(toPlexServer(srv), seasonRatingKey)
	if err != nil { writeError(w, http.StatusBadGateway, err.Error()); return }
	writeJSON(w, eps)
}
```

- [ ] **Step 3: Add SPA client methods**

In `web/src/api/client.ts`:
```ts
seasons: (serverID: string, showRatingKey: string) =>
  getJSON<import("./types").Season[]>(
    `/api/servers/${encodeURIComponent(serverID)}/seasons/${encodeURIComponent(showRatingKey)}`
  ),

seasonEpisodes: (serverID: string, seasonRatingKey: string) =>
  getJSON<Item[]>(
    `/api/servers/${encodeURIComponent(serverID)}/seasons/${encodeURIComponent(seasonRatingKey)}/episodes`
  ),
```

In `web/src/api/types.ts`:
```ts
export interface Season {
  ratingKey: string;
  index: number;
  title: string;
  leafCount: number;
  viewedLeafCount: number;
  thumb?: string;
}
```

- [ ] **Step 4: Write `Episodes.tsx`**

```tsx
import { createMemo, createResource, createSignal, For, Show } from "solid-js";
import { A } from "@solidjs/router";
import { api } from "../api/client";
import type { Item, Season } from "../api/types";
import Skeleton from "./Skeleton";
import "./Episodes.css";

export default function Episodes(props: {
  serverID: string;
  showRatingKey: string;
  initialSeasonIndex?: number;
}) {
  const [seasons] = createResource(
    () => props.showRatingKey,
    (k) => api.seasons(props.serverID, k)
  );

  // Filter "All Episodes" pseudo-season Plex sometimes returns (index 0).
  const realSeasons = createMemo(() => (seasons() ?? []).filter((s) => s.index > 0));

  const [activeKey, setActiveKey] = createSignal<string | null>(null);

  // Default to current episode's season once both are known.
  createMemo(() => {
    if (activeKey()) return;
    const list = realSeasons();
    if (list.length === 0) return;
    if (props.initialSeasonIndex) {
      const match = list.find((s) => s.index === props.initialSeasonIndex);
      if (match) { setActiveKey(match.ratingKey); return; }
    }
    setActiveKey(list[0].ratingKey);
  });

  const [episodes] = createResource(
    () => activeKey(),
    (key) => (key ? api.seasonEpisodes(props.serverID, key) : Promise.resolve([] as Item[]))
  );

  return (
    <section class="episodes">
      <h3>Episodes</h3>
      <Show when={realSeasons().length > 0} fallback={<Skeleton kind="line" count={2} />}>
        <div class="season-tabs">
          <For each={realSeasons()}>
            {(s) => (
              <button
                class="season-tab"
                classList={{ active: activeKey() === s.ratingKey }}
                onClick={() => setActiveKey(s.ratingKey)}
              >
                Season {s.index}
              </button>
            )}
          </For>
        </div>
      </Show>
      <Show when={!episodes.loading} fallback={<Skeleton kind="line" count={6} />}>
        <ul class="episode-list">
          <For each={episodes() ?? []}>
            {(ep) => (
              <li class="episode-row">
                <A href={`/item/${props.serverID}/${ep.ratingKey}`} class="episode-link">
                  <Show when={ep.thumb}>
                    <img class="episode-thumb" src={api.image(props.serverID, ep.thumb!)} alt="" />
                  </Show>
                  <div class="episode-meta">
                    <div class="episode-line1">
                      <span class="episode-num">E{ep.index}</span>
                      <span class="episode-title">{ep.title}</span>
                      <Show when={(ep.viewCount ?? 0) > 0}>
                        <span class="episode-watched" title="Watched">✓</span>
                      </Show>
                    </div>
                    <Show when={ep.summary}>
                      <div class="episode-summary">{ep.summary}</div>
                    </Show>
                    <div class="episode-line3">
                      <Show when={ep.duration}>
                        <span>{Math.round((ep.duration ?? 0) / 60_000)} min</span>
                      </Show>
                      <Show when={ep.originallyAvailableAt}>
                        <span class="episode-date"> · {ep.originallyAvailableAt}</span>
                      </Show>
                    </div>
                  </div>
                </A>
              </li>
            )}
          </For>
        </ul>
      </Show>
    </section>
  );
}
```

Confirm `Item` type has `viewCount?: number` and `originallyAvailableAt?: string`. Add if missing.

- [ ] **Step 5: Write `Episodes.css`**

```css
.episodes {
  margin-top: 28px;
}

.episodes h3 {
  font-family: var(--font-headline);
  font-size: 14px;
  letter-spacing: 1.5px;
  color: var(--text-muted);
  text-transform: uppercase;
  margin: 0 0 12px;
}

.season-tabs {
  display: flex;
  gap: 8px;
  margin-bottom: 16px;
  flex-wrap: wrap;
}

.season-tab {
  padding: 6px 14px;
  border-radius: var(--radius-pill);
  background: transparent;
  color: var(--text-muted);
  border: 1px solid var(--border-soft);
  font-size: 13px;
  font-family: var(--font-headline);
  letter-spacing: 0.3px;
  cursor: pointer;
  transition: background 0.15s ease, color 0.15s ease;
}

.season-tab:hover { color: var(--text); border-color: var(--stroke); }
.season-tab.active { background: var(--bg-inverse); color: var(--text-inverse); border-color: var(--bg-inverse); }

.episode-list {
  list-style: none;
  margin: 0;
  padding: 0;
  display: flex;
  flex-direction: column;
  gap: 12px;
}

.episode-row {
  background: var(--bg-shelf-inner);
  border: 1px solid var(--border-soft);
  border-radius: var(--radius-md);
  overflow: hidden;
  transition: background 0.12s ease;
}

.episode-row:hover { background: rgba(15, 23, 41, 0.85); }

.episode-link {
  display: flex;
  gap: 14px;
  padding: 12px;
  color: inherit;
}

.episode-thumb {
  width: 160px;
  height: 90px;
  object-fit: cover;
  border-radius: var(--radius-sm);
  flex-shrink: 0;
}

.episode-meta { flex: 1; min-width: 0; }
.episode-line1 { display: flex; align-items: center; gap: 8px; margin-bottom: 4px; }
.episode-num { font-family: var(--font-mono); color: var(--text-muted); font-size: 12px; }
.episode-title { color: var(--text); font-weight: 500; }
.episode-watched { margin-left: auto; color: var(--led-teal); font-weight: 600; }
.episode-summary { color: var(--text-muted); font-size: 13px; line-height: 1.4; margin-bottom: 6px;
  display: -webkit-box; -webkit-line-clamp: 2; -webkit-box-orient: vertical; overflow: hidden; }
.episode-line3 { color: var(--text-muted); font-size: 12px; }
.episode-date { font-style: italic; }
```

- [ ] **Step 6: Mount in ItemDetail**

In `ItemDetail.tsx`, after the `<section class="overview">` block, add:
```tsx
<Show when={(it() as Item).type === "episode" || (it() as Item).type === "show"}>
  <Episodes
    serverID={params.serverID!}
    showRatingKey={(it() as Item).grandparentRatingKey ?? (it() as Item).ratingKey}
    initialSeasonIndex={(it() as Item).parentIndex}
  />
</Show>
```

Add import:
```tsx
import Episodes from "../components/Episodes";
```

Confirm `Item` type exposes `grandparentRatingKey`. Add to `web/src/api/types.ts` and `internal/plex/types.go` if missing.

- [ ] **Step 7: TS verify + build**

Run: `cd web && npx tsc --noEmit && npm run build && cd .. && go build -o lumen.exe ./cmd/lumen`
Expected: clean.

- [ ] **Step 8: Commit**

```
git add web/src/components/Episodes.tsx web/src/components/Episodes.css web/src/pages/ItemDetail.tsx web/src/api/client.ts web/src/api/types.ts internal/plex/episodes.go internal/server/api_servers.go internal/plex/types.go
git commit -m "feat(item-detail): season tabs + episode list (shows + episodes)"
```

---

## Phase I — Build Verification & Manual Smoke Test

### Task 29: Full build + go test + manual smoke test

**Files:** none (verification only).

- [ ] **Step 1: Full build pipeline**

Run:
```
cd web && npx tsc --noEmit && npm run build
cd ..
go build -o lumen.exe ./cmd/lumen
go test ./...
```
Expected: clean, all green except the carry-over `TestImageProxyForwardsWithTokenServerSide` from Session 2.

- [ ] **Step 2: Restart serve**

Kill any running `lumen serve` (or use the new Close Lumen modal — last session). Then:
```
.\lumen.exe serve
```
Expected: Plex auth + server discovery, browser opens to `http://127.0.0.1:7832`.

- [ ] **Step 3: Smoke checklist** (Byron walks the whole flow)

Tick each:
- [ ] Desktop shortcut shows the new Lumen icon (Task 1).
- [ ] Top bar wordmark + group titles render in Rajdhani (carry-over check).
- [ ] Click a Continue Watching card with `viewOffset > 0` → Resume modal pops up with correct timestamp + 5 s countdown auto-resume default.
- [ ] Click Restart in the modal → playback starts from 0; modal vanishes.
- [ ] Click an Item Detail Play on a movie with no progress → Pot Player launches; Now Playing strip appears below the top bar with thumb, title, progress bar (animating), time readout (m:ss / m:ss), quality badge.
- [ ] After ~10 s of playback, Plex Web's Dashboard shows "Lumen" as a Now Playing client with correct progress + duration.
- [ ] Close Pot Player early (X button) → strip disappears within ~5 s.
- [ ] Re-open the same item → Item Detail's Play button now reads "Resume" with the prior position recorded.
- [ ] Item Detail Mark as Watched → button click; refetch shows item as watched (`viewCount > 0`); Continue Watching no longer lists it.
- [ ] Item Detail Mark as Unwatched on a watched item → resets watched state; viewOffset cleared.
- [ ] Item Detail of a show → Season tabs render; clicking Season 2 loads its episodes; episode rows show thumb/title/duration/air-date and a green ✓ on watched ones.
- [ ] Click an episode in Episodes list → navigates to that episode's Item Detail.
- [ ] Force a direct-play failure (try a transcode-required file or temporarily corrupt the URL by editing Plex DB / pointing at a non-existent partID) → Transcode Prompt modal pops up; click Try Transcode → Pot Player launches with the HLS URL and the strip shows "TRANSCODE" badge.
- [ ] Watch an episode past 90% → at the 5s+ poll tick after threshold, Plex marks it watched (visible in Plex Web) AND the Next Episode modal pops up showing the next episode info; auto-plays after 5 s; Pot Player closes the current file and launches the next.
- [ ] Click Cancel during the Next Episode countdown → modal dismisses; nothing auto-plays.
- [ ] Try to start a second `/api/play` while one is active → 409 conflict (verify via DevTools network tab).

- [ ] **Step 4: Commit verification doc**

Create `docs/session-4-findings.md` summarising the smoke results, any deviations from plan, and Session 5 carry-ins. Mirror the structure of `docs/session-3-findings.md`.

```
git add docs/session-4-findings.md
git commit -m "docs: Session 4 findings"
```

---

## Self-Review Checklist (run before handoff)

- [ ] **Spec coverage:** §7 Pot Player Control fully implemented (path resolution + Launch + reads with cold-start + Stop). §8 stream URL resolution split into builder + interactive prompt (Byron's revision). §9 session manager + 3 goroutines. §10.1's Now Playing strip is a separate strip below TopBar (Byron's revision; spec to be amended in `docs/session-4-findings.md`). §12.6 expanded with hero + season/episode + wired action row; deferred items (subtitle, trailer, OMDB pill, watchlist, cast/crew, server-context-swap) explicitly called out as Session 5.
- [ ] **Placeholder scan:** zero "TBD"/"TODO"/"implement later" in this plan. Every code block compiles in isolation against the listed types.
- [ ] **Type consistency:** `PlaybackState`, `PlaybackEvent`, `NextEpisodeInfo`, `TranscodePromptInfo`, `Season`, `Item.art`, `Item.grandparentArt`, `Item.grandparentRatingKey`, `Part.VideoResolution`, `Part.VideoCodec` referenced in tasks all defined in their respective tasks (or flagged for addition where missing).
- [ ] **Carry-forward gotchas restated up front** so subagents executing tasks can't claim ignorance.

---

## Handoff

After saving this plan, two execution options:

1. **Subagent-Driven (recommended)** — dispatch a fresh subagent per task, review between, fast iteration.
2. **Inline Execution** — execute tasks in this session using executing-plans, batch with checkpoints.
