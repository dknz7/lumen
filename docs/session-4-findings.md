# Session 4 — Pot Player Playback + Plex Sync + Item Detail Expansion Findings

**Date:** 2026-04-25
**Status:** Session 4 complete (Phases A-I). Manual smoke test pending. Session 5 unblocked once smoke verifies.
**Commits:** 26 commits since Session 3 close (`3bf84af`).

## Phase status

| Phase | Tasks | Status |
|---|---|---|
| A — Pre-flight (icon embed) | 1 | ✅ DONE |
| B — Pot Player IPC package | 2-6 | ✅ DONE |
| C — Plex playback helpers | 7-9 | ✅ DONE |
| D — Playback Manager (Go) | 10-13 | ✅ DONE |
| E — HTTP playback endpoints | 14-18 | ✅ DONE |
| F — SPA SSE store + Now Playing | 19-20 | ✅ DONE |
| G — Playback modals | 21-25 | ✅ DONE |
| H — Item Detail expansion | 26-28 | ✅ DONE |
| I — Verification (autonomous sweep) | 29a | ✅ DONE |
| I — Verification (manual smoke) | 29b | ⏸ PENDING |

## Verification results

| Gate | Result |
|---|---|
| `go build ./...` | **CLEAN** |
| `go vet ./...` | **CLEAN** |
| `gofmt -l ./...` | 5 files flagged — **all pre-Session-4** (see Known Issues #3) |
| `go test ./...` | 1 pre-existing failure (`TestImageProxyForwardsWithTokenServerSide`, Session 2 carry-over). All Session 4 packages pass (`internal/playback`, `internal/plex`, `internal/potplayer`, `internal/server`). |
| `npx tsc --noEmit` | **CLEAN** |
| `npm run build` | **CLEAN** — JS 136.12 kB (gzip 45.45 kB), CSS 34.48 kB (gzip 6.33 kB), 5.14s |
| `go build -o lumen.exe ./cmd/lumen` | **CLEAN** — 11.7 MB binary |

## What's new this session — phase highlights

### Phase A — Pre-flight (Task 1, pre-session)
`goversioninfo` icon embed: `assets/lumen.ico` placeholder + `versioninfo.json` + `cmd/lumen/resource_windows_amd64.syso`. Closes the Session 3 carry-over for desktop-shortcut icon.

### Phase B — Pot Player IPC (Tasks 2-6, pre-session)
Full `internal/potplayer/` package — `Launch`, `IsAlive`, `GetPosition`, `GetDuration`, `GetState` (with cold-start retry), `Stop`. Win32 procs hoisted to package-level vars. `ErrExeNotFound` + `ErrWindowNotFound` sentinels. Implements the Session 0 spec (units = ms, reads via `WM_USER+0x500X`, writes via `WM_APPCOMMAND` codes 13/14).

### Phase C — Plex helpers (Tasks 7-9, pre-session)
- `internal/plex/stream.go` — `DirectPlayURL` + `TranscodeURL` builders.
- `internal/plex/timeline.go` — `/:/timeline` reporter.
- `internal/plex/episodes.go` — `NextEpisode` resolver via `/library/metadata/{showKey}/allLeaves`.

### Phase D — Playback Manager (Tasks 10-13)
`internal/playback/` package. Manager + Context + Event types + 3 goroutines:
- **`runPoller`** (5s tick): reads Pot Player position/state, detects 90% threshold (scrobble + emit `next-episode-prompt` for episodes / `ended` for movies), detects `IsAlive == false` → tear-down via `Stop`, detects 10s direct-play timeout → emit `transcode-prompt`.
- **`runReporter`** (10s tick): POSTs `/:/timeline` with current position/state.
- **`runTranscodeKeepAlive`** (10s tick, only when `Transcoding == true`): pings `/video/:/transcode/universal/ping?session=<id>`.

Subscriber/SSE event fan-out infra ready for Phase E.

### Phase E — HTTP endpoints (Tasks 14-18)
- Manager wired into `*Server` via closure-based path resolver (Settings → `PotPlayerPath` updates propagate live without Manager restart).
- `POST /api/play` (replaces 501 stub) — fetches Plex metadata, builds `DirectPlayURL`, calls `Manager.Start`.
- `POST /api/play/transcode` — fallback path (user-confirmed transcode).
- `POST /api/play/stop` — programmatic teardown.
- `GET /api/playback` — state snapshot for cold-load.
- `GET /api/playback/stream` — SSE broadcasting Manager events. Disables per-connection write deadline so long-lived connections don't get culled by `Server.WriteTimeout=30s`.

### Phase F — SPA SSE store + Now Playing strip (Tasks 19-20)
- `web/src/state/playback.ts` — singleton store with `state`, `nextEpisode`, `transcodePrompt`, `endedAt` signals + `connect`/`dismiss*` methods. EventSource subscribes to all 5 named SSE event types.
- `NowPlaying.tsx` strip below TopBar — shows thumb / title / progress bar / position-duration / quality badge while session active. **No** playback controls (Pot Player's domain).

### Phase G — Playback modals (Tasks 21-25)
- `ModalShell` — reusable backdrop + Motion-driven entrance + Escape-cancel.
- `ResumeRestartModal` — 5s countdown, default Resume (parametric, mounted in ItemDetail by Task 27).
- `TranscodePromptModal` — no countdown (user decision), reads from `playbackStore.transcodePrompt`.
- `NextEpisodeModal` — 5s countdown auto-play with Cancel-during-autoplay race fix (try/finally restructure).
- TranscodePrompt + NextEpisode mounted as SSE-driven singletons at App root.

### Phase H — Item Detail expansion (Tasks 26-28)
- Hero backdrop banner (full-width art with gradient fade-to-bg).
- Wired Play (with `ResumeRestartModal` logic — modal opens when `viewOffset > 0 AND < 90% duration`), Mark Watched / Unwatched (via existing `/api/servers/<id>/scrobble` endpoints, with refetch).
- Season tabs + Episode list for shows (`Episodes.tsx` + new Plex `GetSeasons` / `GetSeasonEpisodes` + new server routes `seasons/<key>` and `seasons/<key>/episodes`).

### Phase I — Verification (Task 29)
- **29a (this autonomous pass):** consistency sweep + plan amends + full build verification. **DONE.**
- **29b (Byron's manual smoke):** end-to-end Pot Player + Plex walkthrough. **PENDING.**

## Critical gotchas discovered (must carry forward)

### 1. Subscribe-vs-broadcast race on channel close (Task 10 fold-in I1)
`Subscribe` cleanup originally called `close(ch)`. But `broadcast` snapshots subscribers under lock then sends *outside* it. Sequence: broadcast snapshots → cleanup runs (deletes from map + closes channel) → broadcast sends to closed channel → panic. Fix: don't close the channel in cleanup. SSE handlers exit on `ctx.Done()` naturally. Reference: `internal/playback/manager.go:151`.

### 2. `Stop`'s TOCTOU race causing double timeline POST (Task 10 fold-in I2)
Two concurrent `Stop()` calls both pass the `c == nil` check before either nils `m.active`. Result: double-cancel (safe), double-PotPlayer-Stop, AND double `ReportTimeline(state="stopped")` POST → Plex confusion. Fix: capture-and-clear `m.active` AND `m.cancel` in the same critical section. Reference: `internal/playback/manager.go:120-127`.

### 3. `c.Duration` race between poller refinement and `Stop`'s final report (Task 11 fold-in I1)
Poller writes `c.Duration` under `m.mu` when it confirms duration. `Stop` reads `c.Duration` outside the lock. Race detector catches it in CI. Fix: capture `c.Duration` inside the same critical section that captures `c`. Same pattern applied to reporter goroutine (Task 12). Reference: `internal/playback/manager.go:124-133`.

### 4. SSE event-name convention drift (Task 18 deviation 1, Task 19 critical fix)
Plan assumed Go server sends initial state as `event: state` and subsequent events as default (`event: message`). Wrong. Task 18 deviation has the server send EVERY event with its named type (`event: state`, `event: ended`, etc.). Task 19's plan code used `es.onmessage` for subsequent events — would have silently dropped EVERY event after the initial. Fix: register `addEventListener` for each of the 5 named event types. Reference: `web/src/state/playback.ts`.

### 5. Long-lived SSE connections vs `Server.WriteTimeout` (Task 18 fold-in I1)
`Server.WriteTimeout = 30s` is right for short-lived JSON handlers, but SSE streams live for hours. Without per-connection deadline disable, every 30s of idle would trigger a write error mid-event and force the browser to reconnect. Fix: `http.NewResponseController(w).SetWriteDeadline(time.Time{})` per-request. Reference: `internal/server/api_playback.go:30-35`.

### 6. Plex auth is HEADER-ONLY, NEVER URL query (re-pinned in Tasks 13, 28)
`X-Plex-Token` in URL query violates the convention pinned by `internal/plex/scrobble.go` (Session 3.5) and re-confirmed in Tasks 8, 9, 13, 28. Use `c.SetToken(req, accessToken)`. Plan code authored before Phase D had this drift in Task 13's transcode keep-alive AND Task 28's `GetSeasons`/`GetSeasonEpisodes` — both caught and corrected before commit.

### 7. `metadataSliceToItems` is field-by-field copy, NOT direct unmarshal (Tasks 15 + 28)
Adding a field to `plex.Item` alone is not enough — the wire-to-Item converter copies fields explicitly. New fields are silently dropped during JSON decoding unless the converter is also extended. Tasks 15 (`Media`) and 28 (`viewCount`/`originallyAvailableAt`) both required dual updates. Reference: `internal/plex/libraries.go` `metadataWire` + `metadataSliceToItems`.

### 8. Cancel-during-autoplay race in countdown modals (Task 24 fold-in I2)
NextEpisodeModal's `playNow()` originally called `close()` (which dismissed the modal) BEFORE awaiting `playStop` then `play`. User clicks Cancel within ~50ms of countdown reaching 0 → modal vanishes → user thinks they cancelled → 300-800ms later next episode plays anyway. Real "I cancelled, why is it playing?" mental-model break. Fix: `try/finally` keeps modal visible during the awaits, dismiss happens in `finally`. Reference: `web/src/components/Modal/NextEpisodeModal.tsx`.

### 9. `setInterval` returning `0` is technically valid (Task 24 fold-in I1)
`if (timer) clearInterval(timer)` is falsy-buggy when timer is `0`. `clearInterval` is documented as a no-op for invalid IDs, so unconditional `window.clearInterval(timer)` is cleaner. Pattern fixed in NextEpisodeModal; ResumeRestartModal already used unconditional clears.

### 10. CSS append-vs-replace orphans (Task 26 cleanup fold-in)
Task 26 said "append" the new hero rules — but Session 2's original `.hero` block had a `border-bottom` that wasn't overridden by the new rule, leaving a stale hairline below the gradient fade. Per Karpathy Guideline 3 ("remove orphans your changes created"), the obsolete original blocks were removed in a sibling cleanup commit. Lesson: when "appending" replaces a same-class block, scan for orphan properties that don't get overridden.

## New features beyond plan

- **Stop captures Duration under lock** (Task 11 fold-in propagated to manager.go) — race fix.
- **`ResumeRestartModal` props.cancelled-flag avoidance via try/finally** (Task 24 fold-in) — cleaner than the originally-considered cancelled-flag approach.
- **Plan amend workflow** — every fix-fold-in commit also updates the plan code blocks to reflect as-shipped shape, so future replays don't reintroduce the same nits. Commits `3bf84af`, `3e488af`, `b1669da` are pure plan-alignment.
- **Closure-based path resolver for PotPlayer** (Task 14) — Manager doesn't take a static path; it takes a `func() string` that re-reads from settings on each launch. Settings → PotPlayerPath updates propagate live, no Manager restart needed.

## Known issues carried forward to Session 5+

### 1. Resume offset is cosmetic-only for v1.0 (plan-acknowledged)
`/api/play` handler currently DISCARDS `req.ResumeFromOffset` (`_ = req.ResumeFromOffset` with comment in `api_play.go`). Frontend `ResumeRestartModal` wiring is contractually correct, but Pot Player starts at position 0 regardless of Resume vs Start Over. Fix options: (a) explore Pot Player CLI seek arg, (b) `WM_USER` position-set after launch, (c) extend Manager to forward offset to timeline reporter for cross-device resume reflection. **Defer to Session 4.5 or 5.**

**Resolved in Session 4.5 polish PR (E):** wired `req.ResumeFromOffset` through `playback.StartArgs.ResumeOffsetMs` -> `potplayer.Launch` -> Pot Player's `/seek=hh:mm:ss` CLI arg (option a). Applied to both `handlePlay` and `handlePlayTranscode`. Smoke verification pending.

### 2. `TestImageProxyForwardsWithTokenServerSide` failure carries from Session 2
Pre-existing test in `internal/server` — token isn't being forwarded server-side per the test expectation. Session 3 findings already document this. **Not introduced by Session 4.**

### 3. Five pre-existing files flagged by `gofmt -l`
None touched by Session 4 — all pre-existing. Suggests a pre-Session-4 gofmt sweep missed these:
- `internal/config/config.go`
- `internal/plex/availability.go`
- `internal/server/api_servers_test.go`
- `internal/server/image_cache.go`
- `cmd/lumen/probe_hubs.go`

Worth a one-line sweep PR (`gofmt -w` on each) at any point.

### 4. Cross-cutting concurrency / observability concerns deferred to Session 4.5

(a) **Stop-vs-tick ordering window** — both poller and reporter goroutines can fire one stale POST after `Stop` nilled `m.active` (modal acquires `c` before Stop completes, posts after Stop). Reporter case is more harmful (wrong viewOffset), keep-alive case benign (refreshes a TTL on a session about to be reaped). Fix: `select { case <-ctx.Done(): return; default: }` guard immediately before each network call. Cross-cutting one-liner across `poller.go`, `reporter.go`, `transcode.go`.

(b) **Silent 4xx blindness** — `m.plex.Do` errors are logged but `resp.StatusCode >= 400` is not. Affects poller (Scrobble), reporter (ReportTimeline), transcode keep-alive. Worth a shared `doWithStatusCheck` helper.

(c) **Outage log spam** — 5s poller + 10s reporter + 10s keep-alive = up to 25 log lines/min during a Plex outage. Per-error-type dedup ("logged 18 identical errors in last 3 min") would dramatically improve signal.

### 5. Phase G visual polish observations (defer-but-noted)
- `▶` Unicode glyph in Play button (ItemDetail) — strictly not an emoji but spirit of "all glyphs from `lucide-solid`" suggests swap to `<Play size={16} />`.
- `Quality: "transcoded 1080p"` is hardcoded stub on transcode handler — `// TODO(phase-g+)` marker added in Task 29a sweep.
- No Enter-to-confirm autofocus on TranscodePromptModal's "Try Transcode" button — autofocus added to NextEpisodeModal's Play Now in Task 24 fold-in; same one-line fix applies here.

### 6. Mark Watched / Unwatched no optimistic update
Sequential `await api.scrobble()` then `refetchItem()`. Network round-trip × 2 before UI reflects new state. Acceptable v1.0; consider optimistic mutation later.

### 7. SPA error UX is `alert()`-based
Established pattern across all Phase G modals + Phase H buttons. Toast/snackbar infra would be a nice cross-cutting upgrade.

### 8. `ItemDetail` season auto-default doesn't react to prop changes
Navigating between two episodes of the same show won't auto-jump tabs because `activeKey` is already set. Pre-existing in plan. Real-usage smoke will reveal if it matters.

## Plan deviations codified

| Task | Deviation | Commit |
|---|---|---|
| 10 | 7 fold-in fixes (concurrency + cached display metadata) | `a131cd7` |
| 11 | 3 fold-in fixes (poller scrobble retry + Stop captures Duration) | `46445f2` |
| 12 | typed-enum `stateToPlexString` (no faux import-cycle workaround) + Duration race fix | `1b61d6c` |
| 13 | header-only auth + `plex.Client` wrappers (not stdlib http) | `db23ef9` |
| 14 | `serve.go` listed in plan files but not actually needed | `26f1ae9` |
| 15 | `msToDuration int → int64`, `formatQuality(item)` instead of `(Part)`, `newTestServer` doesn't exist (inline setup), Plex Media/Part scope-add + `metadataSliceToItems` extension. Plan amended. | `0e28c3f` / `b1669da` |
| 16 | 3 consistency fixes folded into Task 29a sweep | `fbefb4e` |
| 18 | 5 deviations (single `writeSSE` helper, Event envelope for initial, race comment, dead-branch removal, no-heartbeat comment) + WriteTimeout fold-in | `31c1577` / `fd2e582` |
| 19 | critical SSE listener rewrite (5 named-event listeners, no `onmessage`) | `df1e1e1` |
| 22 | dropped unused `Show` import | `5afc54e` |
| 24 | 3 fold-in fixes (race + clearInterval safety + autofocus) | `6a799f5` |
| 26 | CSS cleanup fold-in for orphan rules | `a2e70cd` |
| 28 | header-only auth on GetSeasons/GetSeasonEpisodes + scope-add of `viewCount`/`originallyAvailableAt` | `e43e093` |

## Build & test state at close

- `go build ./...` → **clean.**
- `go vet ./...` → **clean.**
- `gofmt -l ./...` → 5 pre-existing files (see Known Issues #3).
- `go test ./...` → all Session 4 packages pass; one pre-Session-2 carry-over (`TestImageProxyForwardsWithTokenServerSide`).
- `cd web && npx tsc --noEmit` → **clean.**
- `cd web && npm run build` → **clean** — JS 136.12 kB / gzip 45.45 kB; CSS 34.48 kB / gzip 6.33 kB; built in 5.14s.
- `go build -o lumen.exe ./cmd/lumen` → **clean** — 11.7 MB binary.
- `main` branch state: 26 commits added since Session 3 close (`3bf84af`). Final commit: `b1669da`.

## Pre-flight checklist for Session 5+

- [ ] Manual smoke (Task 29b) verifies end-to-end playback flow against real Plex servers.
- [ ] Resolve resume-offset gap (one of the 3 documented options in Known Issues #1).
- [ ] Cross-cutting concurrency / observability cleanup (Stop-vs-tick race, silent 4xx, log spam) — Session 4.5 PR.
- [ ] `gofmt -w` sweep on the 5 pre-existing flagged files.
- [ ] Optional Phase G visual polish (lucide `Play` icon, autofocus on TranscodePromptModal, optimistic Mark Watched).
- [ ] Carry forward all Session 3 pre-flight rules (`useDragDropContext != null` guard, `<Dynamic>` for reactive switching, anti-stale-binary `go build` after every web change, `npx tsc --noEmit` in every SPA-touching task).

## Design notes carried forward (live for Session 5+)

Reaffirming Session 3's notes plus Session 4 additions:

- Pure `#000000` retained.
- No emojis in any UI surface — all glyphs from `lucide-solid`. **NOTE:** Phase G/H introduced two Unicode glyphs (`▶` in Play button, `✓` in Episodes watched check) — both technically not emojis per Unicode classification but spirit-violations of the lucide convention; flagged for polish.
- Saira (body) + Rajdhani (headlines) replaces Geist Sans this session.
- `--led-teal` (`#2dd4bf`) used as the accent for progress bars (Now Playing, modals, episode watched check).
- Skeleton loaders for loading states.
- `:focus-visible` rings on every interactive element.
- Cubic-bezier easing arrays (`[0.22, 1, 0.36, 1]` and `[0.16, 1, 0.3, 1]`) — never spring strings (Session 3 critical gotcha #4).
- `<Dynamic component={...} />` for reactive Solid component switching (Session 3 critical gotcha #5).
- `useDragDropContext != null` guard for any new component that might render outside DragDropProvider (Session 3 critical gotcha #1) — Phase F-H didn't add new dnd surfaces.
- `ModalShell` pattern shared by Phase G modals; CloseConfirmModal stayed standalone (different ARIA surface).
- Singleton-from-store SPA pattern (TranscodePromptModal, NextEpisodeModal — read from `playbackStore` directly, no props).
- Parametric SPA pattern (ResumeRestartModal — controlled by ItemDetail local state, props-driven).

## Manual smoke test results (Task 29b)

**Date:** 2026-04-26
**Tester:** Byron

Direct play, Plex Activity sync, Now Playing strip, Pot Player tear-down, scrobble at 95%, Mark Watched/Unwatched roundtrip, season tabs, episode navigation, Resume modal, SSE auto-reconnect — all confirmed working. No Go-side panics, no browser console errors (one Firefox extension warning unrelated to Lumen).

**Bugs / gaps surfaced (driving the Session 4.5 polish PR):**

1. **Pagination regression (Library page).** Page 2 click flashes "100-200 Page 2" briefly then snaps back to Page 1. Library navigation effectively stuck at Page 1.
2. **No watched indicator on cards.** Plex flagged a movie as watched but Lumen UI gave no visual cue.
3. **No watched indicator on Mark Watched button.** Already-watched items show the same button label as unwatched.
4. **No "added" date under cards.** Crucial freshness info missing.
5. **Stargaze movie thumbnails 404.** Episodes load fine, but movie poster requests fail. DKNZPLEX no longer affected (upstream Plex bug appears resolved). **DEFERRED — see Known Issues #X.**
6. **No current/remaining duration on Item Detail page** when item is resumable. Resume button label correct but no context on how far in.
7. **Hero banner too small.** Cuts off most of the backdrop art on a 4K display.
8. **No Show-title navigation from episode/season hero.** Should jump to overall show view.
9. **90% threshold + 5s countdown for next-episode prompt is aggressive.** Cuts into shorter shows. Bumping to 95% + 10s.
10. **Now Playing strip metadata is sparse.** Show/episode title, S+E numbers, release date, added date, codec, duration all need to be visible.

Tests 27-30 (Transcode prompt) deferred — Byron has no backend file access to corrupt media for forced-direct-play-failure testing. Will verify if/when it surfaces in normal use.

## Known Issues (Stargaze movie thumbnails — deferred)

**Symptom:** Stargaze server's movie poster thumbnails return 404 through Lumen's image proxy. Episode posters from the same server load correctly. DKNZPLEX is unaffected (its previously intermittent 404s — Session 2/3 finding — appear resolved upstream).

**Hypothesis:** Stargaze's Plex API returns `thumb` paths in a different format for movies (`type=1`) vs episodes (`type=4`), OR the image-proxy handler's URL-rewrite path special-cases episode `grandparentThumb` fallback in a way that doesn't fire for movie `thumb`.

**Investigation plan (deferred to Session 4.5+ when Byron has time):**
1. Open DevTools Network tab on the Library page for a Stargaze movie library.
2. Capture the full URL of a failing thumb request (the `/api/image-proxy?server=...&path=...` URL).
3. Compare against a working DKNZPLEX movie thumb URL.
4. Compare against a working Stargaze episode thumb URL.
5. Determine whether the failure is Lumen's URL construction, Plex's response format, or the upstream CDN.

A `// TODO` marker is in `internal/server/api_image_proxy.go` flagging this for the next pass.

## Session 4.5 polish PR scope

Driven by the smoke test results above. Phased execution:

- **Phase 4 (urgent bug fix):** Pagination regression — Library page navigation broken.
- **Phase 1 (quick wins):** Episode-list watched check verify, Card "Added" timestamp, ItemDetail duration subtitle, clickable show title in hero, 95% threshold + 10s countdown.
- **Phase 2 (visual polish):** Card top-right corner ribbon "WATCHED" rotated 45°, Mark Watched button label `✓ Watched` when watched, Hero `min-height: 65vh`.
- **Phase 3 (cross-stack):** Now Playing State expansion (`EpisodeIndex`, `SeasonIndex`, `AddedAt`, `OriginallyAvailableAt` as optional fields on `playback.State`, populated by both `/api/play` and `/api/play/transcode`); SPA renders horizontal pill row above timeline with quality/codec moved into the row.
