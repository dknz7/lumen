# Session 0 — Pot Player IPC Findings

**Date:** 2026-04-24
**Pot Player version:** Mini 64-bit v260422 (1.7.22859)
**Test video:** `C:\Users\dicke\Downloads\xmen.mp4` — duration 1 min 47 s (107 s)
**Probe source:** `probe/main.go` at commit `024f423`
**Install path (not in registry, supplied via `--potplayer` flag):** `C:\Program Files\DAUM\PotPlayer\PotPlayerMini64.exe`

## Results

| Capability | Probe step | Result | Notes |
|---|---|---|---|
| Registry path lookup | 2 | **FAIL** | `HKCU\Software\DAUM\PotPlayerMini64\ProgramPath` does not exist on this install. Adjacent values (`MInfo1`, `LastUpdatePath`, etc.) are present — DAUM just doesn't write `ProgramPath` reliably. **Mitigation in probe:** added `--potplayer=<path>` override flag; Session 4 will do the same. |
| Launch subprocess | 3 | **PASS** | `exec.Command(exe, video).Start()` works; pid returned. Window visible within ~1 s. |
| FindWindowW on `PotPlayer64` | 4 | **PASS** | Window class confirmed as `PotPlayer64`. HWND returned within ~100 ms of launch. |
| Position read (WM_USER+0x5004) | 5 | **PASS** | **Units: milliseconds** (raw=1000 ≈ 1 s wall-clock). Monotonic, ~2 s cold-start before first non-zero reading. |
| Duration read (WM_USER+0x5002) | 6 | **PASS** | **Units: milliseconds.** raw=107345 for a 1 m 47 s file — exact match. Available after ~1.5 s poll. |
| State read (WM_USER+0x5006) | 7 | **PASS** | Values confirmed: `1=PAUSED`, `2=PLAYING`. **Cold-start caveat:** returns `-1` (reaches Go as `uint64 max = 18446744073709551615`) during the first ~2 s while media is loading. Session 4 must treat `-1` as "not ready, poll again". |
| WM_COMMAND pause toggle (0x4E5E) | 8 | **FAIL** | Both `SendMessage` (A) and `PostMessage` (B) variants silently ignored. Return code non-zero but no visible effect on playback. Either the documented ID changed in v260422 or Pot Player no longer routes menu IDs this way. |
| WM_COMMAND stop (0x4E67) | 8 | **FAIL** | Not independently tested because WM_COMMAND pause toggle failed — WM_APPCOMMAND path (below) is the confirmed route. |
| **WM_APPCOMMAND pause toggle (14)** | 8 | **PASS** | `SendMessage(hwnd, 0x0319, 0, 14<<16)` — playback paused on command. |
| **WM_APPCOMMAND stop (13)** | 8 | **PASS** | `SendMessage(hwnd, 0x0319, 0, 13<<16)` — playback stopped. Window did not auto-close (expected; stop ≠ quit). |
| Clean exit detect (IsWindow) | 9 | **PASS** | `IsWindow` flipped `1→0` within 1 s of the X-button click. Detection latency: ≤ 1 s. |
| Dirty exit detect (Task Manager kill) | 9 | **PASS** | Same behaviour — `IsWindow` flipped `1→0` within 1 s of `End Task`. Detection latency: ≤ 1 s. |

## Hard-gate capabilities (per spec §3)

- [x] Position read works reliably
- [x] Exit detect works reliably (clean AND dirty)

## Go / No-Go decision

**GO** — both hard gates clear with wide margin. All reads (position, duration, state) work via `WM_USER` offsets; writes work via `WM_APPCOMMAND` (not `WM_COMMAND` as the spec originally assumed); exit detection is sub-second and robust for both clean and dirty closes. Session 1 can proceed.

## Unit / encoding quirks to pin into `internal/potplayer/commands.go`

- **Position and duration are milliseconds, not seconds.** Convert via `time.Duration(raw) * time.Millisecond`. (Spec §7.1 doesn't specify units — this locks them in.)
- **Cold-start window is ~2 s.** Position, duration, and state all return `0` or `-1` during this window. The production `Client` should retry reads up to 3 s after launch before treating a zero/negative as real (aligns with spec §7.2's 3 s HWND wait).
- **State `-1` sentinel.** `GetState()` must map `-1` (Go sees it as `^uint64(0)`) to a distinct "not ready" state, not to `Stopped`. Incorrectly treating `-1` as `Stopped` would trigger premature session teardown.
- **Pot Player install path NOT in registry on this machine.** Production `internal/potplayer` must: (1) try `HKCU\Software\DAUM\PotPlayerMini64\ProgramPath`, (2) fall back to a Settings-stored override, (3) last-resort check default install locations (`C:\Program Files\DAUM\PotPlayer\`, `C:\Program Files\DAUM\PotPlayerMini64\`). Settings modal §13.5 already promises an override field — its presence is load-bearing, not nice-to-have.
- **`WM_APPCOMMAND` replaces `WM_COMMAND` for write-side control.** The production client sends `SendMessage(hwnd, 0x0319, 0, appcmd<<16)`. No cross-process menu-ID trickery needed.

## Command IDs confirmed for v260422

| Name | Value | Confirmed? |
|---|---|---|
| WM_USER base | 0x0400 | **Yes** |
| PP_GET_POSITION (WM_USER wParam) | 0x5004 | **Yes** — returns ms |
| PP_GET_DURATION (WM_USER wParam) | 0x5002 | **Yes** — returns ms |
| PP_GET_STATE (WM_USER wParam) | 0x5006 | **Yes** — returns 1/2 (and `-1` during cold-start) |
| PP_CMD_PAUSE_TOGGLE (WM_COMMAND wParam) | 0x4E5E | **No** — ignored on v260422, abandoned |
| PP_CMD_STOP (WM_COMMAND wParam) | 0x4E67 | **No** — not exercised, abandoned |
| WM_APPCOMMAND message | 0x0319 | **Yes** |
| APPCOMMAND_MEDIA_PLAY_PAUSE (lParam high word) | 14 | **Yes** — toggles pause/resume |
| APPCOMMAND_MEDIA_STOP (lParam high word) | 13 | **Yes** — stops playback (does not close window) |

## Follow-ups for Session 4

- **Pause/Resume determinism.** `APPCOMMAND_MEDIA_PLAY_PAUSE` (14) is a toggle. For deterministic `Pause()` / `Resume()` methods on the production `Client`, either:
  - (a) Check `GetState()` first, send toggle only when state differs from desired — simple but has a race window.
  - (b) Probe `APPCOMMAND_MEDIA_PAUSE` (47) and `APPCOMMAND_MEDIA_PLAY` (46) as separate commands early in Session 4. If they work, prefer them. (Not tested in Session 0 because the toggle covers the v1.0 Now Playing strip's pause/resume button which is itself a toggle — but separate commands are cleaner for timeline-reporter-driven resumes.)
- **Stop vs. quit.** `APPCOMMAND_MEDIA_STOP` halts playback but leaves the window open. Spec §7.1 `Stop()` implies window closure for the session manager to fire its final timeline update. Options: (a) send `WM_CLOSE` (0x0010) after stop, (b) fall back to `TerminateProcess` (already specified in §7.2 as the 2 s timeout fallback). Session 4 decides.
- **Path resolution order.** Implement the three-stage fallback above (registry → Settings override → default paths). The Settings override field becomes mandatory for this install.
- **Cold-start retry.** Build a 3 s retry loop into `GetPosition()`, `GetDuration()`, `GetState()` that tolerates `0` / `-1` returns before the media is loaded.
- **`HWND` staleness.** The probe always found a fresh HWND right after launch. In Session 4, the `Client` should re-query via `IsWindow(hwnd)` before every send and re-find via `FindWindowW` if the handle went invalid — guards against Pot Player crash + auto-restart (if that ever happens).
- **rasvob repo's command IDs are documentation-only.** The repo's `WM_COMMAND` menu-ID approach does not work on v260422. Link it in code comments as historical reference but do not rely on any ID from it beyond the `WM_USER` query codes which DO work.

## Probe binary

`probe/probe.exe` remains in the repo at commit `024f423` for future reference. Not imported by production code.
