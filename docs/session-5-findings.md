# Session 5 — Watchlist, Recommended, Discover, Item Detail Enrichment, Inline Re-Auth Findings

**Date:** 2026-04-26
**Status:** Phase 1 complete (Tasks 1-9). Phase 2 in progress.
**Plan:** [docs/superpowers/plans/2026-04-26-lumen-session-5.md](superpowers/plans/2026-04-26-lumen-session-5.md)

## Phase 1 — Stream B + C (Item Detail enrichment + inline re-auth)

### Commits (since Session 4.5 cleanup base `7b988d9`)

```
018976a feat(settings): inline PIN re-auth modal
c39dcbb feat(auth): inline PIN re-auth endpoints
4e6ad64 feat(item-detail): Play Trailer button via YouTube embed
a2214d6 feat(item-detail): Cast & Crew section
9216b9d feat(item-detail): IMDB rating pill via OMDB
4b2d1b4 feat(omdb): client + /api/imdb/<id> with 30-day disk cache
814bd66 style(plex): gofmt formatting pass on Task 2 additions
ce9c530 feat(plex): extend Item with IMDB ID, cast, crew, trailer
dd623d5 refactor(item-detail): drop disabled Subtitle picker — PotPlayer's domain
```

9 commits across 8 tasks (Task 2 fanned into feat + gofmt — implementer noted; would be a single commit going forward).

### Verification results

| Gate | Result |
|---|---|
| `go build ./...` | **CLEAN** |
| `go vet ./...` | **CLEAN** |
| `gofmt -l cmd internal probe` | **CLEAN** |
| `go test ./internal/plex/...` | All PASS (incl. new `TestExtractIMDBId` 4 sub-cases) |
| `go test ./internal/server/...` | New tests PASS (`TestIMDB*`, `TestAuth*`); pre-existing `TestImageProxyForwardsWithTokenServerSide` still failing per Session 2 carry-over |
| `npx tsc --noEmit` | **CLEAN** |
| `npm run build` | **CLEAN** — JS 147.09 kB (gzip 48.60 kB), CSS 38.87 kB (gzip 7.08 kB), 5.89s |
| `go build -o lumen.exe ./cmd/lumen` | **CLEAN** |

Bundle deltas vs Session 4.5 close (140.37 / 46.53 / 36.71 / 6.73): JS +6.72 kB / +2.07 kB gzipped, CSS +2.16 kB / +0.35 kB gzipped — proportional to 5 new SPA components (IMDBPill, CastCrew + PersonCard, TrailerModal, ReAuthModal) plus extended Item type and Person/TrailerInfo/OMDBRating wire types.

### Highlights

- **Subtitle picker DELETED** ([dd623d5](dd623d5)) — PotPlayer's domain. Spec §12.6.2 explicitly superseded.
- **IMDB rating pill** via 30-day on-disk OMDB cache. 503 when no key, '—' on miss. Brand yellow (`#f5c518`).
- **Cast & Crew** reads Plex's `Role`/`Director`/`Writer` arrays. Director + Writer merged into one Crew grid with role tags. Generic SVG silhouette fallback when no thumb.
- **Play Trailer** via YouTube embed (Plex `Extras` only); Plex-hosted `.flv` trailers explicitly out of v1.0 scope. Modal stops playback on close by clearing `iframe.src`.
- **Inline PIN re-auth** replaces the `alert()` stub in Settings → Accounts & Servers. 5-state machine (idle/polling/linked/expired/error) backed by `POST /api/auth/start` + `POST /api/auth/poll` (single-shot 2s polls). 15-min PIN TTL. Cancel safely tears down the poll timer.

### New patterns introduced this phase

1. **Plex `metadataWire` scope-add chain (Task 2)** — adding cast/crew/trailer fields required extending `metadataWire` AND `metadataSliceToItems` AND adding new `Person`/`TrailerInfo` types AND adding bridge helpers (`personsFromRole`, `personsFromCrew`, `trailerFromExtras`, `extractIMDBId`, `toIDOnly`). The Session 4 critical gotcha #7 about field-by-field copying is now codified across 5 new fields.

2. **Discrete on-disk JSON cache via mtime TTL (Task 3)** — `readFreshOMDBCache` reads `os.Stat().ModTime()` and rejects > 30d. Lazily creates `%APPDATA%\Lumen\cache\omdb\` on first miss. Pattern reusable for any future similar cache (TVDB, TMDB if we ever add them).

3. **`OMDBClient` as a separate type from `plex.Client`** (Task 3) — OMDB doesn't speak Plex identity headers. Distinct constructor + zero shared state with `plex.Client`. Pattern to follow for any future non-Plex external API client.

4. **Modal wrapping pattern: TrailerModal + ReAuthModal both wrap `ModalShell`** (Tasks 6 + 8) — backdrop / Escape-cancel / Motion entrance handled centrally by ModalShell. Each new modal is just a thin content shell + props. ModalShell's required `ariaLabel` prop is non-optional (the JSX spec requires it; tsc enforces).

5. **State-machine modal with `<Show when={state() === ...}>` blocks (Task 8)** — Solid signal flips between distinct UIs without manual narrowing. Cleaner than `<Switch>` for small finite-state UIs (5 states, no nesting). Re-applicable to any future modal with a flow.

### Spec deviations

- **§12.6.2 Subtitle picker — DROPPED from v1.0.** PotPlayer's native sub-switcher already handles selection well; Lumen's picker would just duplicate (and badly — no runtime switching without WM_USER probing we haven't done). Disabled UI removed in [dd623d5](dd623d5).

### Reviewer cadence (Phase 1)

Combined-review per task per Byron's directive — single reviewer pass checking spec compliance + code quality together. All 8 tasks APPROVED on first review with non-blocking observations only:
- Task 2: implementer landed a follow-up gofmt commit; spec called for one. Single-commit discipline reinforced for subsequent tasks.
- Task 4: inline `import("../api/types").OMDBRating` cast inside JSX is functional but slightly unconventional — kept for v1.0.
- Task 5: bundle delta marginally above projection (justified by component count).
- Task 6: `allowfullscreen` attribute cosmetic note (works but might warn under stricter solid-js types in future).
- Task 7: in-flight TOCTOU race between `/api/auth/start` overlapping a running poll — bounded to a single-user app, acceptable for v1.0.
- Task 8: `start()` flips status to `polling` before the API call resolves — brief blank UI moment. Cosmetic; suggestion-tier.

### Manual smoke test pending

Byron to walk through (Task 9 step 2 from the plan):

1. Open an Item Detail for a movie with a known IMDB ID — verify yellow IMDB pill renders the correct rating.
2. Open Item Detail with cast/crew metadata — verify Cast and Crew grids render below MORE WAYS TO WATCH.
3. Open Item Detail for a movie with a YouTube trailer Extra — Play Trailer enabled, click opens modal, video plays. Close clears iframe.
4. Open Item Detail with no trailer — button disabled, tooltip says "No trailer available".
5. Settings → Accounts & Servers → Re-authenticate — modal opens, click Start, browser opens to plex.tv/link with code; complete the link, modal flips to "Linked successfully".

### Phase 2 pre-flight

Phase 2 (Tasks 10-18) covers the Plex Discover stack (Watchlist + Add/Remove, Recommended page, Discover page) under 2-stage review per task per Byron's directive. Two DevTools captures required during Phase 2:

1. **Task 13 — Watchlist Add/Remove endpoint shape** (likely `PUT discover.provider.plex.tv/actions/{addToWatchlist,removeFromWatchlist}?ratingKey=<plexTvRk>` per the plan, but worth live-confirming).
2. **Task 16 — "Pick Up Again" hub slug** (spec §5.2 left this pinned to be confirmed — likely `continue-watching` but DevTools is ground truth).

## Phase 2 — Stream A (Plex Discover stack)

### Commits (since Phase 1 findings `52ede4d`)

```
cdf3193 feat(spa): Discover page (8 home-namespace shelves)
c23e0ac feat(spa): Recommended page (4 watchlist-namespace shelves)
852adbc feat(item-detail): Add to Watchlist button with optimistic toggle
b6b78b8 feat(server): /api/watchlist/{add,remove} with cache invalidation
a496472 feat(plex): AddToWatchlist + RemoveFromWatchlist
611268a fix(watchlist): render plex.tv thumbs directly + skeleton grid wrapper
36c0a01 feat(spa): Watchlist page (read-only, type filter + sort + count)
9484fc4 feat(server): GET /api/watchlist with 5-min cache
70d66d5 fix(plex): add Guid array absorber to watchlistWire
a6ac6d3 feat(plex): GetWatchlist via metadata.provider.plex.tv
```

10 commits across 9 tasks. Two are post-implementation fixes flagged by stage 2 quality reviewers (the watchlistWire `Guid` absorber on Task 10, and the Watchlist page thumb URL + skeleton-in-grid fix on Task 12) — both demonstrate the value of the 2-stage cadence catching real issues that combined-review would have shipped. Task 13 (Watchlist Add) DevTools-confirmed via Byron's live capture; Remove inferred symmetric.

### Verification results

| Gate | Result |
|---|---|
| `go build ./...` | **CLEAN** |
| `go vet ./...` | **CLEAN** |
| `gofmt -l cmd internal probe` | **CLEAN** |
| `go test ./internal/plex/...` | All PASS (4 new tests: `TestGetWatchlistHeaderOnlyAuthAndShape`, `TestAddRemoveWatchlistShape`, plus the existing suite) |
| `go test ./internal/server/...` | All new tests PASS (`TestWatchlistRequiresAccountToken`, `TestWatchlistAddRequiresPOST`, `TestWatchlistAddRequiresRatingKey`, `TestWatchlistRemoveRequiresPOST`); pre-existing `TestImageProxyForwardsWithTokenServerSide` still failing per Session 2 carry-over |
| `npx tsc --noEmit` | **CLEAN** |
| `npm run build` | **CLEAN** — JS 154.52 kB (gzip 50.29 kB), CSS 41.73 kB (gzip 7.37 kB), 6.25s |
| `go build -o lumen.exe ./cmd/lumen` | **CLEAN** |

Bundle deltas:
- vs Phase 1 close (147.09 / 38.87): JS +7.43 kB / +1.69 kB gzipped, CSS +2.86 kB / +0.29 kB gzipped — three new pages (Watchlist, Recommended, Discover) plus the watchlist toggle on Item Detail.
- vs Session 4.5 close (140.37 / 36.71): JS +14.15 kB / +3.76 kB gzipped, CSS +5.02 kB / +0.64 kB gzipped — full Session 5 surface area.

### Highlights

- **`/api/watchlist`** + **`/api/watchlist/{add,remove}`** via `metadata.provider.plex.tv` (read) and `discover.provider.plex.tv` (mutate). 5-min in-memory cache, invalidated on Add/Remove.
- **Watchlist page**: type filter (All/Movies/TV Shows), sort dropdown (Date Added/Title/Release Year), count, plex.tv thumbs rendered directly with `referrerpolicy=no-referrer` (no proxy round-trip).
- **Recommended page**: 4 watchlist-namespace shelves (Pick Up Again dropped per Byron — Home already pins Continue Watching, upstream data unreliable).
- **Discover page**: 8 home-namespace shelves per spec §12.4. `top_watchlisted` slug preserved with underscore (Plex's spelling).
- **Add to Watchlist button on Item Detail**: optimistic toggle, reverts on failure, dispatches `lumen:data-invalidated` 350ms after success for cross-resource refetch. Disabled with explanatory tooltip for items lacking a `plex://` GUID.

### DevTools-confirmed endpoints

- `PUT https://discover.provider.plex.tv/actions/addToWatchlist?ratingKey=<plexTvRk>` — header-only `X-Plex-Token`, empty body, 200 with `{"MediaContainer":{"size":0}}`. Captured Sun 26 Apr 2026 09:10 by Byron.
- `PUT https://discover.provider.plex.tv/actions/removeFromWatchlist?ratingKey=<plexTvRk>` — inferred symmetric. Manual smoke will validate.

### New patterns introduced this phase

1. **`metadata.provider.plex.tv` base** added to `plex.Client` (`metadataBase` field). Distinct from `discoverBase` because read vs write paths use different plex.tv subdomains.

2. **Plex.tv ratingKey extraction from GUID** — `web/src/util/plexGuid.ts` — `plex://movie/<id>` → `<id>` regex pull. The discover-namespace ratingKey IS the trailing GUID segment; no separate lookup needed.

3. **Optimistic toggle override pattern** — `Signal<boolean | null>` overlaid on top of a server-truth resource via `createMemo`. Override flips immediately, reverts on API failure, server resource catches up via `lumen:data-invalidated` event. Reusable for any future mutation that benefits from instant UI feedback.

4. **`watchlistAction` private helper** mirroring `scrobbleAction` — two public methods + one shared private helper for action-style endpoints. Pattern reaffirmed (this is now the 3rd action-helper instance: scrobble, removeFromCW from Session 4.5, watchlistAction).

### Spec deviations

- **§12.3 Recommended page — "Pick Up Again" shelf dropped.** Reason: Home already pins Continue Watching at the top, and Plex Web's upstream "Pick Up Again" data is unreliable (stale watchlist entries reported by Byron during execution). Recommended now has 4 shelves instead of 5. Capture 2 from the original plan no longer needed — saved one DevTools round-trip.
- **§12.6.2 Subtitle picker — DROPPED** (Phase 1, Task 1). PotPlayer's domain.

### Known issues carried forward

1. **`/discover-item/<ratingKey>` route is a 404 stub** — Recommended and Discover cards link there. Post-1.0 polish: implement a plex.tv-source Item Detail variant that handles non-server-local items.
2. **Watchlist page cards link to `/watchlist/<ratingKey>` (also a 404 stub)** — same plex.tv item detail story.
3. **Watchlist removal from the page itself** — spec §12.2 called for a bin icon + Undo toast on Watchlist cards. Item Detail's Add/Remove button covers the action; bin-icon-on-card deferred to post-1.0.
4. **Stargaze movie thumbnail 404 mystery** — still deferred per Byron's earlier call.
5. **Pre-existing `TestImageProxyForwardsWithTokenServerSide`** — Session 2 carry-over, not addressed this session.
6. **Watchlist page silent error swallow** — `.catch(() => [])` in shelf resources collapses network failure into "Nothing here yet." UX. Post-1.0 polish: distinguish "loaded but empty" from "load failed".

### Manual smoke test pending

Byron to walk through:

1. Settings → Accounts & Servers → Re-authenticate (Phase 1 carry-over if not already smoked).
2. Item Detail with a known IMDB ID — yellow IMDB pill renders.
3. Item Detail with cast/crew metadata — Cast and Crew grids render.
4. Item Detail with a YouTube trailer Extra — Play Trailer enabled, modal plays.
5. Left menu → Watchlist — page loads, type filter and sort dropdowns work, count updates, plex.tv thumbs render directly.
6. Item Detail with a `plex://` GUID — Add to Watchlist button enabled. Click flips to Remove. Verify in Plex Web that the item appears on the watchlist. Click again to remove; verify removal in Plex Web.
7. Item Detail with no `plex://` GUID (server-local-only) — button disabled with explanatory tooltip.
8. Left menu → Recommended — 4 shelves load.
9. Left menu → Discover — 8 shelves load.

### Reviewer cadence (Phase 2)

2-stage reviews per task per Byron's directive — spec compliance reviewer first, then code quality reviewer. All 9 implementation tasks landed cleanly. Two re-dispatches required:

- **Task 10**: stage 2 reviewer flagged missing `Guid` array absorber (Session 3 critical gotcha #6). Fixed at `70d66d5`.
- **Task 12**: stage 2 reviewer flagged broken thumb URL composition (every poster 400'd through the proxy) AND skeleton fallback escaping the grid container. Fixed at `611268a`.

Both catches were specifically things combined-review would have likely missed — vindicating the 2-stage cadence for the Plex Discover stack.

### Next steps

1. Manual smoke test (Byron).
2. Final whole-branch code review.
3. Session 5 close-out commit message + findings finalisation.

---

## Post-Smoke Polish (Phase A + A.5 + A.6)

Manual smoke surfaced six issues spanning subtle bugs and UX gaps. Three rounds of iterate followed.

### Smoke results — what Byron found

| # | Issue | Severity | Outcome |
|---|---|---|---|
| 1 | Re-Auth | ✅ working | — |
| 2 | IMDB pill rendered but no score | bug | OMDB key was unactivated; activation email needed clicking. Confirmed via direct curl returning `{"Response":"False","Error":"Invalid API key!"}`. |
| 3 | Cast & Crew rendered but no thumbs + no pagination | spec gap | Deferred to Session 6 (image-proxy rework + 2-row scroll). |
| 4 | Play Trailer "No trailer available" | spec gap | Plex Extras YouTube IDs sparse on shared/older libraries. Deferred to Session 6 (TMDB integration). |
| 5 | Watchlist stuck loading, single XHR firing | **bug** | Initial host fix (Phase A) → 502 with `"watchlist: status 400"`. Plex Discover caps container size at ~100; Phase A.5 paginates at 100/chunk. |
| 6 | Add/Remove Watchlist | ✅ working | — |
| 7 | Recommended + Discover loading content but no thumbs | bug | Phase A: `HubItem.Thumb` extension + SPA renders absolute URLs directly. |
| 8 | Discover Trending Trailers "Nothing here yet" | bug | Two causes: (a) missing `includeMeta=1` + container-size on hub requests (Phase A), (b) `Part.id` shape mismatch — server-local int vs Discover composite string (Phase A.5/A.6). |

### Phase A — initial fixes (3 commits + janitorial)

```
6abd0ae feat(spa): hub thumb rendering on Recommended + Discover
9425756 feat(plex): HubItem.Thumb + match Plex Web request shape
db399b2 fix(plex): watchlist host correction + pagination params
97302ef chore(session-5): retire stale 'Session 5' sentinels in disabled UI + comments
```

#### `db399b2` — Watchlist host correction

Plex Web DevTools capture revealed the watchlist endpoint is now at `discover.provider.plex.tv/library/sections/watchlist/all`, not `metadata.provider.plex.tv` as the original design spec said. Plex consolidated the watchlist read paths onto the discover host. Speculative `metadataBase` field on `plex.Client` dropped — never actually working.

#### `9425756` — HubItem.Thumb + Plex Web request parity

Plex Web's hub request includes `includeMeta=1` + `X-Plex-Container-Start=0` + `X-Plex-Container-Size=24`. Without these, some clip-type hubs (notably `home/trending-trailers` — 59 items in the live response) return empty for our backend while other home shelves work. Bumped to `X-Plex-Container-Size=50` to cover all eight Discover shelves with one round trip.

`HubItem` gained a `Thumb` field — plex.tv hub thumbs are absolute URLs (`https://metadata-static.plex.tv/...` or `https://image.tmdb.org/...`), no proxy round-trip needed.

#### `6abd0ae` — SPA hub thumb rendering

Recommended + Discover cards render `<img src={thumb}>` directly with `referrerpolicy="no-referrer"`. Posters fade to the empty `.recommended-poster` / `.discover-poster` container background on load failure via `onError`, matching the Watchlist page behaviour from Session 5 Phase 2.

#### `97302ef` — Janitorial: retire stale "Session 5" sentinels

Final-review pass found six stale `"Session 5"` placeholder strings in shipped UI surfaces (OMDB key placeholder, kiosk button tooltip, Home Stargaze stub reasons, etc.) that read as lies once the session shipped. Plus one stale TODO in `internal/server/api_items.go` for an OMDB inline-enrichment that was actually delivered via the separate `/api/imdb/<id>` endpoint pattern. All retired in one commit.

### Phase A.5 — Second-round smoke regressions (2 commits)

```
a05308f fix(plex): Part.ID is string (was int) — supports plex.tv Discover composite IDs
0ae7e4c fix(plex): paginate watchlist in 100-item chunks
```

#### `0ae7e4c` — Watchlist 400 cap

After Phase A's host fix, /api/watchlist returned 502 with body `"watchlist: status 400"`. Plex Discover rejects `X-Plex-Container-Size > ~100` with a 400 (our 500-item single-call was over the cap). Plex Web pages 50→100→100→… — we now mirror by paging at 100 until `totalSize` is reached or 1000 items collected (generous cap, well above any realistic library).

Added `totalSize` parsing on `watchlistWire.MediaContainer` for the termination condition.

#### `a05308f` — Part.ID string (broken — see A.6)

After Phase A's `includeMeta=1` addition to hub requests, /api/hubs/home/trending-trailers started 502'ing with `json: cannot unmarshal string into Go struct field Part.MediaContainer.Metadata.Media.Part.id of type int`. Plex's Discover hub items have `Part.id` as composite UUID-shaped strings (e.g. `691648f137d5bdeaa81f55b1-6918087fc7abb5aa29a67b10`) while server-local items return numeric IDs. Hot fix: change `Part.ID int → string`.

`api_play.go`'s two `%d` format calls became direct string passthroughs.

**This commit broke server-local Plex parsing.** See A.6.

### Phase A.6 — Regression fix from third smoke (1 commit)

```
51576b9 fix(plex): Part.ID custom unmarshaler accepts int OR string
```

#### `51576b9` — Part.ID custom unmarshaler

Byron's third smoke: "Continue Watching failed: onDeck failed for all 2 servers. Every other shelf is empty. When attempting to view a library folder nothing is loading there either."

The `int → string` change broke EVERY server-local request. Plex Media Server returns `Part.id` as a JSON number (e.g. `12345`). Go's JSON decoder refuses to unmarshal a bare number into a string field. The fix in A.5 had passed our existing tests because the test fixtures used STRING ids — sloppy on my part for not testing both shapes.

**Fix:** new `PartID` type with custom `UnmarshalJSON` that accepts both shapes — quoted string passes through; bare number is read as text. Underlying type stays string so callers just need a one-character `string(part.ID)` cast.

**Regression guard:** new [internal/plex/types_test.go](../internal/plex/types_test.go) with `TestPartID_AcceptsBothShapes` locking in three sub-cases (numeric server id, string composite id, string-numeric lookalike). This shape mismatch can never sneak through silently again.

**Final smoke confirmed full recovery** — Watchlist loading 471 items, IMDB pill populating after OMDB key activation, Trending Trailers loading "Super Mario Galaxy Movie" in position 1, Continue Watching restored, Library browse restored, Item Detail accessible.

### Lessons learned (Session 5 + post-smoke)

1. **Plex's mixed JSON shapes across surfaces.** Server-local responses and plex.tv Discover responses are NOT interchangeable wire shapes even on overlapping field names. `Part.id` was the bite this session. Future Plex types decoding fields used across both surfaces must accept both shapes or be tested against fixtures from both.
2. **The 2-stage review caught real things in Session 5 Phase 2 but missed the Stargaze direction-finding and Plex Web parity gaps.** Static review against the spec doesn't catch issues that only surface when the live API behaves differently than the spec described. Plex Web DevTools captures = ground truth — keep using them.
3. **Test fixtures should cover both wire-shape variants when a type travels across surfaces.** The Phase A.5 regression slipped because our `Part` test only used string IDs. The Phase A.6 fix added a multi-case test that covers both shapes — this pattern should be applied to any future Plex type used in both server-local and Discover contexts.
4. **OMDB API keys require activation.** The activation email step is undocumented in our Settings UI copy. Worth adding a one-line note next to the key input pointing users at the activation email — Session 6 polish item.
5. **Smoke + iterate beats over-planning when bugs are the issue.** Session 4.5 used this pattern; Session 5 post-smoke used it again successfully. The key is honest commit messages so the iteration history reads cleanly later.

### Final state at hand-off to Session 6

- **All Session 5 features functional.** IMDB pill, Cast & Crew (no thumbs yet — Session 6), Play Trailer (Plex Extras only — TMDB lands Session 6), Inline PIN re-auth, Watchlist (read + Add/Remove), Recommended (4 shelves), Discover (8 shelves, including Trending Trailers).
- **All gates clean.** `go build` / `go vet` / `gofmt -l` clean. `go test ./internal/plex/...` PASS including new `TestPartID_AcceptsBothShapes`. `go test ./internal/server/...` only fails the documented Session 2 carry-over `TestImageProxyForwardsWithTokenServerSide` (acceptable). `npx tsc --noEmit` clean. `npm run build` clean.
- **Bundle:** 155.21 kB JS / 50.40 kB gzipped, 41.91 kB CSS / 7.38 kB gzipped. Vs Session 4.5 close (140.37 / 36.71): JS +14.84 kB / +3.87 kB gzipped, CSS +5.20 kB / +0.65 kB gzipped — proportional to 5 new SPA components + 3 new pages + watchlist add/remove wiring.
- **Session 6 plan ready** at [docs/superpowers/plans/2026-04-27-lumen-session-6.md](superpowers/plans/2026-04-27-lumen-session-6.md). 18 tasks across 7 phases; reviewer cadence pre-set; subagent-driven-development handoff ready when Byron gives the go.

### Carry-forward to Session 6

Captured in the Session 6 plan as in-scope:

- Stargaze movie thumbnail 404 fix (image-proxy token try-with-fallback + dimension presets) — root cause now diagnosed via Plex Web capture, fix is small.
- Cast/Crew thumbnails (free benefit of Stargaze image-proxy fix) + 2-row pagination with horizontal scroll.
- TMDB trailer integration (free key entry in Settings, /api/tmdb/trailer endpoint, TMDB-first with Plex Extras fallback).
- Home UI parity for Recommended/Discover (refactor to share existing `<Shelf />` component for drag-drop / chevrons / wrapper styling).
- Watchlist card hover actions (Play / Remove / Mark Watched).
- plex.tv Discover Item Detail page (`/discover-item/<rk>` and `/watchlist/<rk>` were 404 stubs).
- Card UX additions on plex.tv pages (functionality + info improvements; brainstorm-first).
- Trailer card functionality (Trending Trailers items have HLS playback URLs — distinct from YouTube-embed TMDB trailers).

Out of scope (still post-1.0):

- Watchlist bin icon + Undo toast on the Watchlist *page itself* (Phase 4.4 covers card hover Remove instead, which subsumes the use case).
- Vimeo / non-YouTube trailer sources.
- Plex.tv avatar endpoint for Settings user thumbnail.
