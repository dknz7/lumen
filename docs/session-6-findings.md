# Session 6 — Stargaze fix, TMDB trailers, Discover/Recommended UI parity, plex.tv detail page Findings

**Date:** 2026-04-27
**Status:** All 18 plan tasks DONE. Phase 4.6 closed with backdrop-fix follow-up. One outstanding cosmetic regression (DiscoverTile TV show card title rendering) carried forward to Session 6.5.
**Plan:** [docs/superpowers/plans/2026-04-27-lumen-session-6.md](superpowers/plans/2026-04-27-lumen-session-6.md)
**Commit count:** 33 commits since Session 5 close.
**Base SHA:** `aa34ff5` (Session 5 close — Phase 2.5 plan addendum).
**Final HEAD:** `33c823b` (this doc lands one commit on top).

## Phase status

| Phase | Tasks | Commits | Status |
|---|---|---|---|
| 1 — Stargaze image proxy fix | 1, 2, 3 | `3e40855` `e4883cc` `963c140` `a5f80f6` | DONE |
| 2 — Cast/Crew pagination | 4 | `3b63293` (later superseded by single-row scroll in `2b3918c` + `fd6a980`) | DONE |
| 2.5 — IMDB pill parent fallback | 4.5 | `f834052` | DONE |
| 3 — TMDB trailer integration | 5–9 | `e5e9a79` `8d0d5fa` `b38c717` `eb622e4` `73de6fd` `8b2efce` `80ecf3e` | DONE |
| Settings polish (Byron-driven) | — | `8953fac` | DONE |
| Library / CW polish (Byron-driven) | — | `f09cd9f` | DONE |
| 4 — Recommended + Discover refactor | 10, 11 | `1261398` `e62aad7` | DONE |
| 4.5 — Discover Item Detail page | 12.5 | `1b44adc` + 6 post-smoke fixes (`cab0a14` `82e2193` `efa6a75` `6b840e8` `2b3918c` `634cdd8` `fd6a980`) | DONE |
| 12 — HubItem wire fattening | 12 | `6ef85ad` (+ `a229eaf` post-smoke for primaryGuid) | DONE |
| 4.4 — Watchlist hover actions | 11.5 | `5977d3f` + `b7f3430` (null-guard) + `2cda3c0` (UX overhaul) | DONE |
| 4.6 — HLS trailer modal | 12.8 | `5da1b65` + `33c823b` (backdrop fix) | DONE |
| 5 — Verification + findings | 13 | THIS DOC | DONE |

## Verification results

| Gate | Result |
|---|---|
| `go build ./...` | **CLEAN** |
| `go vet ./...` | **CLEAN** |
| `gofmt -l cmd internal probe` | One CRLF line-ending diff on `internal/server/api_discover_item.go` (pre-existing in `2b3918c`, content-equivalent — cosmetic). |
| `go test ./internal/plex/...` | **PASS** (incl. all new `TestGetDiscoverItem_*`, `TestGetHubSurfacesExtendedFields`, `TestTMDBLookupTrailerByIMDBID_*`) |
| `go test ./internal/server/...` | Pre-existing `TestImageProxyForwardsWithTokenServerSide` still failing per Session 2 carry-over. All other tests PASS. |
| `npx tsc --noEmit` | **CLEAN** |
| `npm run build` | **CLEAN** — JS 515.95 kB (gzip 162.43 kB), CSS 47.86 kB (gzip 8.07 kB), 7.31 s |
| `go build -o lumen.exe ./cmd/lumen` | **CLEAN** |

### Bundle deltas vs Session 5 close

Session 5 close: 155.21 kB JS / 50.40 kB gzipped, 41.91 kB CSS / 7.38 kB gzipped.
Session 6 close: 515.95 kB JS / 162.43 kB gzipped, 47.86 kB CSS / 8.07 kB gzipped.

- **JS:** +360.74 kB / +112.03 kB gzipped — the bulk is `hls.js/light` (Phase 4.6) plus the new DiscoverItem page, DiscoverTile, HLSTrailerModal, hover-action machinery on Watchlist + DiscoverTile, and TMDB trailer routing on ItemDetail. Bundle-warning territory (Vite flagged the >500 kB chunk); code-splitting the HLS modal route into a dynamic chunk would shave ~108 kB gzipped from the initial bundle. Acceptable for v1.0.
- **CSS:** +5.95 kB / +0.69 kB gzipped — DiscoverItem page + DiscoverTile + HLSTrailerModal styles + Watchlist hover overlay + trailer-modal `:has()` overrides.

## Phase highlights

### Phase 1 — Stargaze image proxy fix (`3e40855` `e4883cc` `963c140` `a5f80f6`)

The Session 2 carry-over closes here. Plex's image-transcode endpoint is per-server, not per-account — and it 404s on the account token for some metadata pulled from agent-sourced sources (Stargaze/agent posters). `3e40855` introduced a try-with-fallback: account token first, on 404 retry with the per-server token. `e4883cc` followed up to close the fallback's response body on transport error and align retry comments. `963c140` defaulted dimensions to 240×360 so we share Plex Web's CDN cache for poster requests rather than hitting an unshared cache slot. `a5f80f6` plumbed explicit per-surface dimensions (poster / hero / person) up through the SPA so each surface hits the matching CDN slot.

### Phase 2 — Cast/Crew pagination (`3b63293`)

Originally implemented as a 2-row column-flow scroll. **Superseded** in Phase 4.5 work: `2b3918c` (DiscoverItem) and `fd6a980` (ItemDetail) both consolidated to a single-row horizontal scroll combining Cast + Crew per Byron's later UX call. The 2-row pattern is gone; the single-row pattern is the live one.

### Phase 2.5 — IMDB pill parent fallback (`f834052`)

Episodes don't carry their own IMDB ID — the `imdbId` field belongs to the parent show. Pill now climbs to `parentRatingKey` and resolves the show's rating when the episode lacks its own.

### Phase 3 — TMDB trailer integration (`e5e9a79` → `80ecf3e`)

Seven commits from config field through SPA wiring. New `TMDBKey` config field; new `TMDBClient` (mirror of `OMDBClient`); `b38c717` polished error wrapping + `url.Values` for consistency; new `/api/tmdb/trailer/<imdb-id>` endpoint with 30-day disk cache (mirror of OMDB cache pattern); `73de6fd` is the **critical** API-key scrub fix (see Gotcha 5); `8b2efce` added the Settings input; `80ecf3e` flipped Play Trailer to prefer TMDB-with-Plex-Extras-fallback. Disk cache lives at `%APPDATA%\Lumen\cache\tmdb\` with mtime-TTL gate (same shape as the OMDB cache from Session 5).

### Settings + Library polish (Byron-driven, `8953fac` `f09cd9f`)

`8953fac` adds an explicit Save button and a body-size guard on key-bearing endpoints. `f09cd9f` bumps Library to 200 items/page and reshapes Continue Watching to a 2-row horizontal scroll matching Plex Web parity.

### Phase 4 — Recommended + Discover refactor (`1261398` `e62aad7`)

Both pages now use the existing `<Shelf />` component (drag-drop / chevrons / wrapper styling) and the new shared `<DiscoverTile />` (replaces the bespoke per-page card). `e62aad7` is the post-implementation tweak: keyboard activation (`Enter` / `Space`) on the tile, plus lifting `.shelf-stub` from `Home.css` into `Shelf.css` since `Shelf.tsx` actually owns the empty-state concept (cross-page CSS coupling pitfall — see Pattern 12).

### Phase 4.5 — plex.tv Discover Item Detail page (`1b44adc` + 6 post-smoke iteration commits)

Original commit shipped the page. Six follow-ups stabilised it under Byron's smoke:
- `cab0a14` — string IDs in Role/Director/Writer (the Session 5 Phase A.6 lesson reapplied)
- `82e2193` — surface resource errors visibly (`<Show when={!resource.error}>` outer wrap) + per-request backend logging
- `efa6a75` — `Rating` float-vs-array case-collision absorber (Gotcha 1, instance 3)
- `6b840e8` — polymorphic `summary` field (string OR array) + raw-body dump on decode fail (Gotcha 2)
- `2b3918c` + `fd6a980` — unified single-row Cast & Crew (rolled the originally-separate 2-row design into one scroll, structurally mirroring DiscoverItem and ItemDetail)
- `634cdd8` — centred Play overlay + Watchlist-only top-right cluster on DiscoverTile (final UX polish)

Each post-smoke commit corresponds to a real wire-shape surprise from Plex's Discover service. The defensive logging from `82e2193` paid for itself instantly across the next four iteration commits.

### Phase 12 — HubItem wire fattening (`6ef85ad` + `a229eaf`)

Task 12 originally landed `parentRatingKey`, `imdbId`, `contentRating`, `studio`, `tagline`, `addedAt`, `originallyAvailableAt`. Smoke iteration revealed clip-type hub items don't populate `parentRatingKey` directly — they expose `primaryGuid` (e.g. `plex://show/<rk>`). `a229eaf` parses the trailing GUID segment as the fallback when the field is empty AND `Type == "clip"`. (Gotcha 4.)

### Phase 4.4 — Watchlist hover actions (`5977d3f` + `b7f3430` + `2cda3c0`)

Original commit added Play / Remove / Mark Watched hover overlays. Smoke surfaced a null crash: `api.availability(guid)` resolves to `null` (not `[]`) for items not on any server, and the Play/Mark Watched handlers dereferenced unguarded. `b7f3430` defensively null-guarded the call sites. `2cda3c0` overhauled Play to use smart routing — highest-resolution match in library, fall through to TMDB trailer modal if nothing in library — and routed title-click based on availability.

### Phase 4.6 — HLS trailer modal (`5da1b65` + `33c823b`)

Trending Trailers clips have HLS playback URLs (different from the YouTube-embed TMDB trailers). New HLSTrailerModal uses native HTML5 HLS for Safari/Chromium and `hls.js/light` fallback for Firefox. `33c823b` is the backdrop-fix follow-up: ModalShell defaults sized for small confirm dialogs (`width: min(440px, 92vw)`) clipped the wider trailer panel; `:has()` overrides on `.modal-shell:has(.trailer-modal)` and `.modal-shell:has(.hls-trailer-modal)` strip the small-dialog chrome so the inner trailer frame is the visible surface. Pattern reusable for any modal-content variant.

## Critical gotchas discovered

These are the load-bearing lessons. Future-Archie picking up Lumen cold MUST know these.

1. **The Plex case-collision absorber pattern struck three times this session.** Plex's `discover.provider.plex.tv` responses emit BOTH a lowercase scalar AND a capital array form for some fields. Go's case-insensitive JSON matcher tries to slot the scalar into the array field unless an explicit absorber is declared. Three instances handled:
   - `guid` (string) vs `Guid` (array of `{id}` objects) — Session 3 finding
   - `studio` (string) vs `Studio` (array of `{tag}` objects) — `1b44adc`
   - `rating` (float) vs `Rating` (array of `{type, image, value}` objects) — `efa6a75`

   **Pattern:** declare BOTH fields with explicit case-sensitive `json:"<lowercase>"` AND `json:"<capital>"` tags. The absorber sink (lowercase) doesn't need to be used; just declared so it doesn't collide.

   **Test fixtures must include all collision keys real Plex emits.** Synthetic fixtures with only the array form passed `go test` but failed at runtime against real responses. Capture from DevTools is the only way to know what fields Plex really sends.

2. **Polymorphic field handling — `summary` can be a string OR an array.** For some discover items, Plex returns `"summary": "string..."`; for others, `"summary": [{"text": "..."}, ...]`. Custom `UnmarshalJSON` (`summaryField` type in `internal/plex/discover_item.go`) tries common shapes in turn and falls back to empty string on unknown — graceful degradation rather than 502. **Test fixtures must cover both shapes.** Other potentially polymorphic fields (`tagline`, `title`, `contentRating`) are uncovered today; the raw-body dump in `discover_item.go`'s decode path will surface them on first encounter.

3. **String vs int IDs across server-local and discover surfaces.** Server-local Plex returns `Role[].id` / `Director[].id` / `Writer[].id` as INT. Discover returns them as UUID-shaped STRING. The `personsFromRole` / `personsFromCrew` helpers in `libraries.go` only accept the int shape; discover must INLINE-ADAPT (drop the ID field, since `Person.ID` isn't surfaced in the SPA anyway). Same Session 5 Phase A.5/A.6 lesson as `Part.ID`. Fixed in `cab0a14` but the test fixture initially LIED (used int IDs when real Plex sends strings) — the bug shipped clean against the broken fixture and was only caught by Byron's smoke. **Lesson: fixtures derived from real DevTools captures, not synthetic.**

4. **Hub clip items use `primaryGuid` (e.g. `plex://show/<rk>`) to point at the parent — they don't populate `parentRatingKey` directly.** Task 12 expected the literal field; reality is parse-from-primaryGuid for clip items. Fixed in `a229eaf`. Pattern: always parse `primaryGuid`'s trailing segment as a fallback when `ParentRatingKey` is empty AND `Type == "clip"`.

5. **API key leak in upstream-error responses (CRITICAL — Task 7 fix).** Go's `*url.Error.Error()` includes the full URL on transport failure. TMDB and OMDB clients put the API key in the URL query string, so transport errors leaked the key into the response body via `writeError(w, status, err.Error())`. Fixed in `73de6fd` (TMDB + OMDB sibling) by logging detailed error server-side via `log.Printf` and responding with a generic message. **Pattern: NEVER bubble `err.Error()` from an external HTTP client straight into the response body. Always log-then-generic at the boundary.** All other `writeError(..., err.Error())` call sites should be audited; affected handlers fixed, some other handlers (e.g. config Save) still use the raw bubble for non-external-API errors which is acceptable.

6. **ModalShell sizing assumption — small confirm-dialog only.** `ModalShell.css` styles `.modal-shell` with `width: min(440px, 92vw)` + padding for Resume / Restart / NextEpisode / TranscodePrompt small dialogs. Trailer modals nest a wider `.trailer-modal` (or `.hls-trailer-modal`) with `width: min(960px, 90vw)` inside — overflows the 440 px outer frame. Fix in `33c823b`: `.modal-shell:has(.trailer-modal)` and `.modal-shell:has(.hls-trailer-modal)` strip the small-dialog chrome (width / padding / bg / border / shadow), letting the inner trailer panel be the visible frame. **ModalShell isn't a one-size-fits-all wrapper — extending its sizing requires `:has()` overrides at each consumer's CSS, OR a refactor of ModalShell to accept a size variant.**

7. **Defensive logging unblocked smoke iteration enormously.** Commit `82e2193` added per-request `log.Printf` lines on `/api/discover-item/<rk>` (request received, cache hit/miss, fetch outcome, error detail). When the rating-scalar bug fired in Byron's smoke, the terminal log surfaced the exact decode error in <1 second of clicking — no DevTools roundtrip needed. The raw-body dump (`6b840e8`) added the same for unknown wire shapes: when Plex returns something we can't decode, the log shows the first 512 bytes of the response. **Pattern: any handler that calls an external HTTP API should log every request + the upstream error detail server-side. The boundary error scrub keeps responses generic for the SPA but the operator log retains full detail.**

8. **`dist/index.html` is git-tracked but conventionally drifts unstaged.** Per `.gitignore`, only `internal/server/web/dist/index.html` is tracked under `dist/` — everything else is ignored. Looking at session-5 commit history, the `index.html` hasn't been committed since `f274a11` (Session 4.5). The dirty state in the working tree is expected; staging it on every SPA-touching commit would inflate the diff with hash churn. **Don't `git add web/...` blindly — stage explicit source paths.** Task 9 (`80ecf3e`) accidentally included it; subsequent commits stayed disciplined.

## New patterns introduced this session

1. **`StudioString` / `RatingScalar` absorber sinks** — Session 3 `Guid`-vs-`guid` pattern extended to two more fields.
2. **`summaryField` custom unmarshaler** for polymorphic string-or-array. Falls back to empty string on unknown shape rather than 502.
3. **Boundary error scrub** — log detailed server-side, respond generic. Applied to TMDB / OMDB / discover-item handlers.
4. **`<Show when={!resource.error}>` outer wrap** to surface resource errors visibly instead of holding the loading skeleton forever (`82e2193`).
5. **`:has()` CSS override** for differently-sized modal content inside ModalShell (`33c823b`).
6. **Per-request backend logging on every external-API handler** (`82e2193` precedent — extend to other handlers as a polish opportunity).
7. **Centred Play overlay + top-right action cluster** for trailer cards (DiscoverTile clip variant in `634cdd8`, mirrored on Watchlist cards in `5977d3f`).
8. **Watchlist Play smart routing** — highest-resolution match preferred, fall through to TMDB trailer modal if not in library (`2cda3c0`).
9. **HLS playback in-browser via `hls.js/light` + native HLS detection** (`5da1b65`). +108 kB gzipped bundle hit; acceptable for the feature.
10. **Page-scoped SolidJS context (`DiscoverTileContext`)** for plumbing watchlist set + modal openers across Shelf-rendered tiles. Two-method shape (`openTrailer`, `openHLSTrailer`) chosen over discriminated-union for incremental adoption.
11. **`<div role="link" tabindex="0">` + `onClick` + `onKeyDown` (Enter/Space)** pattern for keyboard-accessible card body navigation when nested-anchor would break action buttons inside.
12. **Cross-page CSS coupling pitfall lesson** (`e62aad7`) — `.shelf-stub` originally lived in `Home.css` but was rendered from Recommended/Discover. Lifted to `Shelf.css` since `Shelf.tsx` owns the empty-state concept.

## Spec deviations

- **Phase 2 Task 4 (2-row Cast/Crew column-flow)** was superseded by single-row horizontal scroll (`2b3918c` then `fd6a980`) per Byron's later UX call. The 2-row pattern is gone.
- **Task 12 expanded scope** from "contentRating pill" to also include `parentRatingKey`, `imdbId`, `studio`, `tagline`, `addedAt`, `originallyAvailableAt` — Byron-driven so DiscoverTile's trailer cascade and clip-card routing could light up.
- **Task 12.5 hit 6 post-smoke iteration commits** before stabilising (string IDs, error display, rating absorber, summary polymorphism, single-row Cast & Crew, centred Play overlay, ItemDetail Cast & Crew JSX merge).
- **Task 11.5 also hit smoke iteration** — original `5977d3f` had a null-crash on `api.availability` returning `null` for items not on any server; fixed defensively in `b7f3430`, then UX-overhauled in `2cda3c0`.

## Known issues carried forward

1. **DiscoverTile TV show card title rendering (Coming Soon shelf).** Byron's smoke surfaced cards rendering as "Devil May Cry (2025)" / "Devil May Cry 2" / "May 10, 2026" instead of Plex Web's "Devil May Cry 2" / "2026" / "TV-MA pill". Field-shape mismatch — needs a Coming Soon hubs response capture from DevTools + targeted `DiscoverTile.tsx` fix. Captured for **Session 6.5 bug-fix iteration**.

2. **Pre-existing `TestImageProxyForwardsWithTokenServerSide` flakiness.** Session 2 carry-over. Cache-leak through to production cache directory (`%APPDATA%\Lumen\cache\images`) makes the test order-dependent on Byron's machine. Passes cleanly on CI / fresh machines.

3. **Settings inputs don't hydrate from server on page load.** Both OMDB + TMDB key inputs appear empty after refresh even when keys are saved. Sibling-pair behaviour, intentional parity (per Byron's call). Future enhancement: `GET /api/settings` returning masked indicators (e.g. `"********ab12"`).

4. **Watchlist cards fire N parallel `/api/availability/<guid>` calls on mount.** Byron explicitly accepted this for v1.0 since Lumen runs on his desktop. If the watchlist grows >100 items or the server gets stressed, consider batching (`/api/availability/batch`) or lazy-on-hover.

5. **Cast/Crew section is a structural copy across DiscoverItem and ItemDetail.** Future-cleanup candidate: extract `<MoreWaysToWatch>` and `<CastCrew>` shared components.

6. **`api.availability` typing-lie.** `Promise<Match[]>` resolved with `null` when no matches, unguarded on consumers. Symptom-fixed at Watchlist call sites (`b7f3430`); root-cause fix deferred (would require empty-state UX redesign on DiscoverItem + ItemDetail's MORE WAYS TO WATCH block).

7. **`tagline`, `title`, `contentRating` etc. potentially polymorphic but not tested.** Plex's discover wire surprised us four times this session; assume more shapes lurk. The `discover_item.go` raw-body dump on decode failure is the safety net.

8. **HLS bundle size growth (~108 kB gzipped from `hls.js/light`).** Acceptable for the feature. Code-splitting the HLSTrailerModal route into a dynamic chunk would shave this from the initial bundle. Vite is now flagging the >500 kB chunk warning.

9. **`it() as DiscoverItem` cast repeated 9+ times in DiscoverItem.tsx.** Cosmetic ergonomics; could lift `const data = it() as DiscoverItem` once at the children scope. Deferred.

## Manual smoke results

Byron walked through every page during Phase 4.5 smoke iteration. Key outcomes:

- Stargaze image proxy: working (Session 2 carry-over closed)
- Discover Item Detail page: working after 6 post-smoke iterate commits
- Trailer modals (HLS + YouTube): working after backdrop fix (`33c823b`)
- Recommended + Discover shelf refactor: working
- Watchlist Play smart routing: working (Pineapple Express → Stargaze 1080p direct play; Hokum → trailer modal)
- Cast & Crew unified across all detail pages: working
- Trailer card click → parent detail page: working after `a229eaf`
- DiscoverTile TV show card title rendering: outstanding (Session 6.5 territory)

## Pre-flight checklist for Session 6.5

- [ ] DiscoverTile TV show card title fix — needs Coming Soon hubs response capture from DevTools, then targeted SPA + maybe HubItem wire fattening for `parentTitle`.
- [ ] (Optional) Other smoke regressions surfaced during further iteration.
- [ ] (Optional) Pre-existing carry-overs: `TestImageProxyForwardsWithTokenServerSide`, settings hydration, availability typing-lie root fix, Cast/Crew shared component extract.

## Build & test state at close

`go build ./...` clean. `go vet ./...` clean. `gofmt -l cmd internal probe` reports one CRLF-only diff on `internal/server/api_discover_item.go` (pre-existing in `2b3918c`, content-equivalent — cosmetic line-ending churn). `go test ./internal/plex/...` PASS (incl. all new `TestGetDiscoverItem_*` + `TestGetHubSurfacesExtendedFields` + `TestTMDBLookupTrailerByIMDBID_*`). `go test ./internal/server/...` only `TestImageProxyForwardsWithTokenServerSide` fails (Session 2 carry-over). `npx tsc --noEmit` clean. `npm run build` clean. `go build -o lumen.exe ./cmd/lumen` clean.

`main` branch state: 33 commits added since Session 5 close. Final commit before this doc: `33c823b`.

## Design notes carried forward

Mostly carry forward from Session 4.5 + Session 5. New design notes specific to Session 6:

- **Trailer modal sizing** uses `:has()` overrides on `.modal-shell:has(.trailer-modal)` / `.modal-shell:has(.hls-trailer-modal)` to bypass ModalShell's small-dialog defaults.
- **Card hover overlay z-index hierarchy:** dim 1, ribbon/progress 2, play overlay 3, action cluster 4 (Card.tsx pattern, mirrored in DiscoverTile + Watchlist).
- **plex.tv-source images render direct** with `referrerpolicy="no-referrer"`, never through `api.image()` (which expects server-local Plex paths).
- **HLS `<video>` in modal:** HTML5 native HLS for Safari/Chromium, `hls.js/light` fallback for Firefox.
- **Centred Play overlay** at 56 px circular (`rgba(0, 0, 0, 0.6)` + 2 px white border, scale-on-hover) — mirrors Card.css's `.card-play-btn`.
