# Session 6.5 — V1.0 Bug-Fix Iteration + Polish Findings

**Date:** 2026-04-28
**Status:** ✅ **V1.0 SHIPPED.** All carried-forward Session 6 items closed plus a substantial smoke-driven feature/fix loop. Lumen is feature-complete for Byron's local Plex-companion use case and replaces the official Plex desktop app.
**Base SHA:** `193b878` (Session 6 close)
**Final HEAD:** `44d86c9` (this doc lands on top, becoming the v1.0 ship marker)
**Commit count:** 8 commits since Session 6 close.

## Phase status

Session 6.5 was framed as "bug-fix iteration only" but ended up shipping multiple new features the smoke loop surfaced as gaps. Phases evolved organically from Byron's testing, not a plan document.

| Phase | Concern | Commits | Status |
|---|---|---|---|
| 1 | DiscoverTile TV show card title rendering | `5eab5ce` | DONE |
| 2 | Stargaze Trending Movies + Trending Shows (Plex Collections) | `4d74144` | DONE |
| 3 | Search bar (was non-functional stub) | `295e501` | DONE |
| 4 | Library Release Date sort precision | `5564a93` | DONE |
| 5 | Library Card watchlist + button + ItemDetail roll-up | `e90b7a8` | DONE |
| 6 | Season pill navigation on episode pages | `4e0d6e9` | DONE |
| 7 | Tile + shelf reliability (focus-refetch click loss + clip render fixes) | `52fd913` | DONE |
| 8 | Kiosk → Fullscreen API (drop unimplemented kiosk plumbing) | `44d86c9` | DONE |
| 9 | Findings + V1.0 ship note | THIS DOC | DONE |

## Verification at v1.0 close

| Gate | Result |
|---|---|
| `go vet ./...` | **CLEAN** |
| `go test ./internal/plex/...` | **PASS** (incl. new `TestGetHubFiltersPlaceholdersAndSurfacesSeasonFields`, `TestGetCollections*`, `TestGetCollectionItems*`, `TestSearchDiscover*`, `TestAddItemToWatchlist*`, `TestRemoveItemFromWatchlist*`, `TestExtractPlexTvRatingKey`) |
| `go test ./internal/server/...` | Only documented Session-2 carry-over `TestImageProxyForwardsWithTokenServerSide` failing (cache-leak through prod cache dir on Byron's machine; passes on CI/fresh) |
| `gofmt -l cmd internal probe` | One CRLF/LF diff on `internal/server/api_discover_item.go` (Session 6 carry-over, content-equivalent) |
| `cd web && npx tsc --noEmit` | **CLEAN** |
| `cd web && npm run build` | **CLEAN** — JS 522.74 kB (gzip 163.79 kB), CSS 51.90 kB (gzip 8.56 kB), 7.0 s |
| `go build -o lumen.exe ./cmd/lumen` | **CLEAN** — 12.29 MB binary |

### Bundle delta vs Session 6 close

Session 6 close: 515.95 kB JS / 162.43 kB gzipped, 47.86 kB CSS / 8.07 kB gzipped.
Session 6.5 close: 522.74 kB JS / 163.79 kB gzipped, 51.90 kB CSS / 8.56 kB gzipped.

- **JS:** +6.79 kB / +1.36 kB gzipped — new SearchFlydown + SearchResults + Shortcuts + stableArrayByKey + per-type render branches. Modest given the feature surface added.
- **CSS:** +4.04 kB / +0.49 kB gzipped — Search styles, DiscoverTile-subtitle/Card-watchlist-btn additions.

## Phase highlights

### Phase 1 — DiscoverTile TV show card title rendering (`5eab5ce`)

The Session 6 carry-over. Byron's smoke had caught Coming Soon shelf cards rendering with wrong / missing show titles. Plex Web emits a `MediaContainer.Meta.DisplayFields` directive that prescribes per-type rendering rules (season → parentTitle / title / originallyAvailableAt; episode → grandparentTitle / episodeIdentifier / originallyAvailableAt; show / movie → title / originallyAvailableAt). Lumen had been ignoring it entirely with one-size-fits-all title/year/contentRating. New per-type render lives in `DiscoverTile.tsx`'s `primaryTitle()` / `subtitle()` / `dateLine()` accessors. `formatAirDate` helper formats future `originallyAvailableAt` as "May 10, 2026" (Coming Soon parity); falls back to year for past-released items so trending shelves don't show misleading first-air dates. Also filtered `type:"placeholder"` ad-tile slots from `GetHub` (Plex injects these in some hubs). Wire fattened with `parentTitle` / `parentIndex` / `index` / `grandparentTitle` for episode + season tile rendering. Rating pill dropped on Byron's call.

### Phase 2 — Stargaze Trending Movies + Trending Shows custom collections (`4d74144`)

The Home page had two intentionally-stubbed shelves on the Stargaze group ("Plex Collections — deferred"). Phase 2 lit them up. New `internal/plex/collections.go` with `Collection` struct + `GetCollections(server, libraryID)` + `GetCollectionItems(server, collectionRK)`. Two new HTTP routes under `/api/servers/<machineID>/`. SPA gets `api.collections()` + `api.collectionItems()`. New `ShelfDef` kind `"server-collection"` with a discriminated `collectionTitle` field for **lookup-by-title resolution** — admin-rename tolerant; if the collection's ratingKey changes (admin rebuild), Lumen still finds it as long as the title stays stable. **Display label decoupled from lookup key**: Lumen's shelf reads "Trending TV Shows" while the actual Plex collection is named "Trending Shows" — Byron's call to keep Lumen's labels distinct. ShelfLoader refactored into a dispatcher + `RecentShelf` + `CollectionShelf` for isolated Solid hook scoping per kind.

### Phase 3 — Cross-source search bar (`295e501`)

The TopBar's search input was a `console.log` stub. Built the full thing.
- **Backend:** new `(*Client).SearchDiscover` hitting `discover.provider.plex.tv/library/search` with the full param contract (`searchTypes=movies,tv`, `searchProviders=discover,plexAVOD`, `includeMetadata=1`, `filterPeople=1`, `limit=30`). Decodes the doubly-nested envelope (`MediaContainer.SearchResults[]` groups → each group's `SearchResult[]` items → `{Metadata, score}`); flattens non-empty groups, sorts by relevance score descending. New `GET /api/search` fans out to every connected server's `/search` AND discover in parallel goroutines; per-source failures degrade into empty buckets with operator-log lines (boundary error scrub pattern from Session 6).
- **SPA:** new `api.search(query)` returning `{ servers: [{...,items}], discover: Item[] }`. New `SearchFlydown` debounces input 300 ms, fetches when query ≥ 2 chars, renders results grouped per source (Stargaze / DKNZPLEX / Discover), 5 per source. Click on a server result navigates directly to that server's item detail. Click on a discover result fires `api.availability` and routes to in-library detail if matched, else to discover-item detail (mirrors the Watchlist pattern). New `/search?q=...` full results page with the same grouped layout, no per-source cap. TopBar wiring: onInput opens flydown, Enter navigates to `/search?q=...`, click outside closes flydown.

**Post-smoke fix (round 1):** initial implementation returned zero discover results. Root cause: missing `searchProviders=discover,plexAVOD` query param — without it, Plex's API silently returns empty SearchResults groups even for terms that should match. Plus the flydown rendered transparent because `var(--bg-card)` was undefined; swapped to `var(--bg-menu)`. Per-request server-side logging added so future search regressions surface in the operator log without DevTools roundtrip.

### Phase 4 — Library Release Date sort precision (`5564a93`)

One-line fix: `{ value: "year:desc", label: "Release Year (newest)" }` → `{ value: "originallyAvailableAt:desc", label: "Release Date (newest)" }`. Plex's `year` integer sort treats Jan 1 and Dec 31 of the same year as a tie with arbitrary order. `originallyAvailableAt:desc` is the full-date sort key — what Plex Web's "Release Date" filter uses. Movies now sort by actual cinema release; TV episodes sort by actual air date.

### Phase 5 — Library Card watchlist add + ItemDetail watchlist roll-up (`e90b7a8`)

Library cards gained an "Add to Watchlist" hover action. The non-trivial bit: Plex's watchlist is keyed at the **show level** — adding an episode or season needs to walk up to the parent show's plex.tv ratingKey, not the episode/season's own. New `(*Client).AddItemToWatchlist` (and symmetric `RemoveItemFromWatchlist`) does the type-aware roll-up server-side: `resolveWatchlistTarget` calls `GetItem(ratingKey)`, switches on `Type`, walks up via `parentRatingKey` (season → show) or `grandparentRatingKey` (episode → show), and returns the show-level Item. `extractPlexTvRatingKey` parses the trailing segment of `plex://show/<rk>` GUIDs. Two new endpoints: `POST /api/watchlist/add-from-item` and `POST /api/watchlist/remove-from-item`, both taking `{server, ratingKey}` JSON. SPA `Card.tsx` gained an optional `enableWatchlistAdd` prop; Library page passes it on every Card. Plus icon swaps to a persistent LED-teal CircleCheck after success (matches DiscoverTile pattern).

**Post-smoke fix:** Byron's first smoke surfaced episode toggle was 400ing on `/api/watchlist/add` — turned out he was clicking the `ItemDetail.tsx` watchlist button (separate code path) which still used the OLD `api.watchlistAdd(rk)` with the server-local ratingKey. Same root cause, same fix at the new layer: ItemDetail toggle now uses the `from-item` endpoints, and `effectiveWatchlistGuid()` picks the right GUID by type (episode → grandparentGuid; season → parentGuid; movie/show → guid) for the in-watchlist check. Wire surface extended with `Item.ParentRatingKey`, `Item.ParentGuid`, `Item.GrandparentGuid` (the first was missing entirely; the latter two were needed for episode/season ItemDetail in-watchlist checks since the watchlist contains show-level rks, not episode/season rks).

### Phase 6 — Season pill navigation on episode pages (`4e0d6e9`)

When viewing e.g. S04E16 of a show, clicking the "Season 3" pill below the hero used to update the inline episode list but **leave the page on S04E16** — confusing UX (the user expected to land in season 3). `Episodes.tsx` gained an optional `navigateOnSeasonChange` prop. When true, a pill click sets a `pendingNavigate` flag; an effect waits for the new season's episodes resource to resolve, then jumps to `eps[0].ratingKey`. Same-pill clicks no-op. Empty seasons clear the flag and stay put. `ItemDetail.tsx` enables the prop only when `item.type === "episode"` so show-detail pages keep the inline-filter UX (browsing seasons without leaving the show overview).

### Phase 7 — Tile + shelf reliability (`52fd913`)

The biggest deep-dive of the session — three related fixes in the v1.0 polish loop, bundled in one commit:

1. **Focus-refetch click loss across ALL shelf pages.** When the browser window regained focus (alt-tab back to Lumen), `refetchOnFocus` fired on every resource. Initial round-1 fix used `stableArrayByKey` and an `<A>` wrap on DiscoverTile — but the bug PERSISTED. Round-2 deep-dive uncovered TWO actual remount paths: (a) `itemList()` short-circuited on `items.loading` returning `undefined` during refetch → `Shelf.isPaginated()` flipped false → entire grid was REPLACED with the skeleton fallback, then rebuilt fresh when refetch resolved; (b) `Shelf.tsx`'s `pages()` memo creates new `.slice()` arrays per refetch, so the outer `<For each={pages()}>` saw new slice references and remounted every page div, killing tile children regardless of inner item-ref stability. Final fix: optimised `stableArrayByKey` to return the previous array reference when all items match by key+position (short-circuits the entire reactive chain on no-op refetch); removed the `.loading` check from itemList in Discover/Recommended/Home/CW (Solid's createResource preserves the previous value during refetch — UI stays put); switched Shelf's outer iteration from `<For>` to `<Index>` for index-keyed page-div slot stability.

2. **Trailer card watchlist add 400 error.** `DiscoverTile`'s clip variants passed the clip's own ratingKey to `addToWatchlist`, but Plex's catalog only accepts movie/show ratingKeys. New `watchlistTargetRk()` accessor resolves clips to `parentRatingKey` (the show/movie's plex.tv catalog rk, already populated by `hubs.go` from `primaryGuid`). Same accessor used for the `isInWatchlist` check so the +/✓ state on trailer cards finally reflects the parent show's actual watchlist membership.

3. **Season-trailer card render parent-title surfacing.** Clip-type tiles fell through to the default render (just `title`), so TV-show season trailers showed bare "Season 9" with no show name context. Wire capture confirmed: clips with `parentTitle` set are season-typed clips where `title` carries the season label and `parentTitle` carries the show name. Render now: `clip` with `parentTitle` → parentTitle / title / formatted date — same three-line shape as season type. e.g. "Rick and Morty / Season 9 / May 24, 2026", "Devil May Cry (2025) / Devil May Cry 2 / May 10, 2026". Movie trailer clips with no `parentTitle` keep the existing render (just title + date).

### Phase 8 — Kiosk → Fullscreen API (`44d86c9`)

Stripped the unimplemented kiosk-mode setting + browser dropdown. The TopBar's Maximize2 button now toggles browser fullscreen via the Fullscreen API (`document.documentElement.requestFullscreen()` / `document.exitFullscreen()`) — same effect as F11. State tracked via `fullscreenchange` event so the icon flips correctly when user exits via Esc/F11. Settings: "Kiosk & Shortcuts" tab → "Shortcuts"; only the desktop `.lnk` button remains. KioskShortcuts.tsx removed; Shortcuts.tsx is the slim replacement. Backend `KioskConfig` struct + UI field + defaults dropped from Go config; settings PATCH handling stripped; TS `UISettings` cleaned up. Existing `config.json` files with kiosk keys silently ignore them on load (Go drops unknown JSON fields), and the keys are removed from disk on next save — clean migration.

## Critical gotchas discovered

These are the load-bearing lessons unique to Session 6.5.

1. **Plex Web's `MediaContainer.Meta.DisplayFields` directive** — Plex's hub responses include a per-type rendering directive telling clients which fields to display per item type. Plex Web honours it for the per-type "season → parentTitle / title / date" pattern; Lumen had been ignoring it. Capture from real DevTools is the only way to find these — the directive is not documented anywhere we'd otherwise look. **Pattern: when a tile feels under-baked compared to Plex Web, capture the hub response and look for `Meta.DisplayFields` first.**

2. **Plex's `searchProviders=discover,plexAVOD` is a silent gatekeeper.** Without it, `/library/search` returns an envelope with size:0 groups even for terms that should match. Wasted ~30 mins on initial smoke debugging zero discover results before the capture diff revealed the missing param. **Pattern: when Plex's `discover.provider.plex.tv` returns suspiciously empty results, diff the request URL against a known-working Plex Web capture before debugging the response.**

3. **Plex `/library/search` is doubly-nested** — `MediaContainer.SearchResults[]` (groups by source: "plex"/"Free On Demand" + "external"/"More Ways To Watch") → each group's `SearchResult[]` (singular noun, plural slice) → each item is `{Metadata, score}` (score is a sibling of Metadata, NOT inside it). Easy to mis-fixture without a real capture.

4. **Plex watchlist is show-keyed, not item-keyed.** Adding an episode or season requires resolving to the parent show's plex.tv ratingKey first. The clip-trailer case has a similar shape: the clip's own ratingKey is rejected; `parentRatingKey` (show/movie level, set by `hubs.go` from `primaryGuid`) is what the catalog accepts. **Pattern: any `/actions/addToWatchlist` or `/actions/removeFromWatchlist` call must use a show or movie plex.tv ratingKey — never an episode, season, or clip rk.**

5. **Solid `<For>` keys by reference equality, and resource refetches always create new array references.** Even if your data is identical between fetches, every refetch makes the outer For diff "everything is different" and remount all children. Click events landing during this remount are dropped (mousedown on old DOM, mouseup on new DOM → browser doesn't fire click). The fix is **two-layered**: (a) memoise the array so refetch with same data returns the previous outer reference (`stableArrayByKey` returns `prev` when all items match by key+position — short-circuits the reactive chain entirely); (b) for cases where data legitimately changes, use `<Index>` instead of `<For>` for the OUTER iteration so page-div slots stay mounted. **Pattern: any time `<For>` iterates over a `createResource` result, both layers are needed.**

6. **`Shelf.tsx`'s `pages()` memo creates new slice arrays via `.slice()` per call.** This was a hidden remount source even when items inside were stable refs — the outer `<For each={pages()}>` saw new slice references on every reactive update. `<Index>` for outer + `stableArrayByKey` upstream solved it together; neither alone was sufficient.

7. **`createResource` preserves the previous value during refetch — but `loading` is true.** This is critical for keeping UI smooth: gating `itemList()` on `items.loading` causes the entire grid to disappear and reappear during refetch. Don't short-circuit on `.loading`. Only check `.error` and `!items()` (initial-fetch state). Loading state can be surfaced via a subtle indicator (overlay spinner / fade) if needed, but **never destroy the rendered tree just because a refetch is in flight**.

8. **Browser Fullscreen API > kiosk-mode browser flags.** Edge/Chrome have a `--kiosk` command-line flag for true OS-level kiosk mode; Firefox doesn't. The Fullscreen API works identically across all three and is what F11 hits internally. Replacing Lumen's stubbed kiosk-mode launcher with a JS-driven Fullscreen toggle dropped a chunk of speculative complexity (browser detection, command-line flag wiring, settings UI) at zero feature cost. **Pattern: prefer JS APIs over OS / browser-specific launchers when the API exists across the targets you care about.**

9. **Two-round bug fix when the diagnosis is incomplete.** The focus-refetch click loss was first "fixed" in round 1 with `stableArrayByKey` + `<A>` wrap; bug persisted. Round 2 ultrathink uncovered that the actual remount paths were upstream (itemList → undefined) AND further downstream (Shelf's pages slice arrays). **Pattern: when a "fix" fails smoke and the symptom is identical, the diagnosis is incomplete — go back to first principles and trace the FULL reactivity chain end-to-end before iterating.**

## New patterns introduced

1. **`stableArrayByKey<T>(source, keyOf): () => T[]`** — utility that memoises an array, returning the previous reference when all items match by key+position. Required for any `<For>` iterating over a refetched resource. Located at `web/src/util/stableArray.ts`.
2. **`<Index>` for outer pages iteration in Shelf** — keeps page-div slots mounted when the outer pages array legitimately changes; inner `<For>` still keys by item reference.
3. **Per-type DiscoverTile render** — discriminated by `item.type` with branches for season / episode / clip-with-parentTitle / show / movie / clip-without-parentTitle. Title / subtitle / dateLine accessors compute per-branch.
4. **`formatAirDate` heuristic** — future `originallyAvailableAt` formats as "May 10, 2026" (Coming Soon parity); past dates fall back to year so trending shelves don't show misleading first-air dates.
5. **Lookup-by-title for Plex Collections** — admin-rename tolerant resolution. Maps `collectionTitle` (the lookup key) separately from the SPA's display label.
6. **Server roll-up helper pattern** — `resolveWatchlistTarget(server, ratingKey)` walks server-local item → parent or grandparent show. Same pattern reusable for any "resolve to canonical entity" need.
7. **Display-label vs lookup-key decoupling** — Lumen's shelf labels can differ from Plex's catalog names without breaking the integration (e.g. "Trending TV Shows" in Lumen → "Trending Shows" in Plex's collection name).
8. **Search fan-out + per-source bucketing** — backend handler issues per-server + discover queries in parallel goroutines, returns `{servers: [...], discover: [...]}` so the SPA can group results visually. Per-source failures degrade into empty buckets, never fail the whole request.
9. **Item wire surface extension for type-aware operations** — `Item.ParentRatingKey`, `Item.ParentGuid`, `Item.GrandparentGuid` were missing entirely. Added so episode/season ItemDetail can resolve to the show level for watchlist checks without a backend round-trip.
10. **Fullscreen API toggle with `fullscreenchange` listener** — icon state stays in sync regardless of how the user exits (button / Esc / F11). Works on Firefox + Chrome + Edge.

## Spec deviations / scope evolution

Session 6.5 was pitched as "bug-fix iteration" but ended up shipping multiple new feature builds. Each evolved organically from Byron's smoke loop:

- **Search bar** — pitched as a single bug ("non-functional search"); turned into a substantial backend + flydown + full results page build. Took the largest commit of the session (~14 files).
- **Plex Collections** — wasn't on any plan; emerged when Byron noticed the "(Plex Collections — deferred)" stub message during smoke.
- **Library Card watchlist** — Byron asked for a small UI affordance; uncovered a latent ItemDetail bug (broken for episodes/seasons since first build) and required type-aware roll-up + 3 new wire fields.
- **Kiosk → Fullscreen** — Byron's late-session call to drop unimplemented complexity. Trade: lose a never-implemented feature plan; gain working F11-style fullscreen across all browsers.
- **Focus-refetch click loss** — multi-round fix; round 1 was insufficient and bug persisted, requiring round 2 ultrathink.

## Known issues carried forward at v1.0 close

These are explicitly deferred — not bugs to fix in v1.0, but worth noting for any future v1.1+ work.

1. **Pre-existing `TestImageProxyForwardsWithTokenServerSide` flakiness** — Session 2 carry-over. Cache-leak through the production cache directory (`%APPDATA%\Lumen\cache\images`) makes the test order-dependent on Byron's machine. Passes cleanly on CI / fresh machines. Root fix would require cache-dir injection in the test setup.
2. **Settings inputs don't hydrate from server on page load** — OMDB + TMDB key inputs appear empty after refresh even when keys are saved (intentional sibling-pair parity). Future enhancement: `GET /api/settings` returning masked indicators (e.g. `"********ab12"`).
3. **HLS bundle size growth** (~108 kB gzipped from `hls.js/light`) — code-splitting `HLSTrailerModal` into a dynamic chunk would shave this from the initial bundle. Acceptable for v1.0.
4. **Watchlist cards fire N parallel `/api/availability/<guid>` calls on mount** — Byron explicitly accepted for v1.0 since Lumen runs on his desktop. Could batch via `/api/availability/batch` if the watchlist grows >100 items.
5. **`it() as DiscoverItem` cast repeated 9+ times in DiscoverItem.tsx** — cosmetic ergonomics. Lift `const data = it() as DiscoverItem` once at children scope.
6. **Cast/Crew + MORE WAYS TO WATCH structural copies across DiscoverItem and ItemDetail** — future `<MoreWaysToWatch>` and `<CastCrew>` shared component extract.
7. **Single-search-source for `/api/search`** — fans out to ALL servers regardless of which the user wants to query. Per-source filter (e.g. "search Stargaze only") would be a v1.1 enhancement.
8. **Other Plex wire fields potentially polymorphic** — `tagline`, `title`, `contentRating` not yet stress-tested against real captures. The `discover_item.go` raw-body dump is the safety net.

## Manual smoke results at v1.0 close

Byron smoked every page through multiple iteration rounds. Final-state outcomes:

- ✅ DiscoverTile per-type render (Coming Soon, Trending shelves) — TV shows / seasons / episodes all render parent show name + season label + air date
- ✅ Stargaze Trending Movies + Trending Shows — populate from custom collections, lookup-by-title robust
- ✅ Search bar — flydown opens at 2+ chars after 300 ms debounce, three sources rendered, click routes correctly per source, Enter navigates to full results
- ✅ Library Release Date sort — meaningful order (newest 2026 release on top, not random within year)
- ✅ Library Card watchlist + button — Plus → CircleCheck on add, episode/season correctly rolls up to show on watchlist, no 400 errors
- ✅ Season pill on episode pages — clicks navigate to S0XE01 of selected season; show pages still inline-filter
- ✅ Discover/Home/Recommended/Library/Watchlist tile clicks — bulletproof during alt-tab focus refetch (round 2 fix)
- ✅ Discover trailer cards — clip watchlist add uses parent-rk; season trailers render with show name
- ✅ Top-bar fullscreen toggle — Maximize2 ↔ Minimize2 swap, F11 / Esc keep icon in sync
- ✅ Settings → Shortcuts — only desktop shortcut button remains; lnk creation works
- ✅ Episode/season ItemDetail watchlist toggle — no longer 400s; correctly reflects parent show watchlist state

## Build & test state at close

`go build ./...` clean. `go vet ./...` clean. `gofmt -l cmd internal probe` reports the documented Session 6 CRLF/LF carry-over only. `go test ./internal/plex/...` PASS (all new tests in this session included). `go test ./internal/server/...` only the documented Session-2 carry-over fails. `npx tsc --noEmit` clean. `npm run build` clean — JS 522.74 kB / gzip 163.79 kB, CSS 51.90 kB / gzip 8.56 kB. `go build -o lumen.exe ./cmd/lumen` clean — 12.29 MB.

`main` branch state at v1.0 close: 8 commits added since Session 6 close (`193b878`). Final commit before this doc: `44d86c9`.

## V1.0 ship note

**Lumen is feature-complete for Byron's local Plex-companion use case and replaces the official Plex desktop app.**

### Shipped feature set

- **Multi-server Plex companion** — Stargaze + DKNZPLEX server registries, server-scoped routing, displayName resolution, hidden-libraries support, drag-drop server group ordering on Home
- **Home page** — Continue Watching merged across servers + per-server "Recently Released" shelves + Stargaze custom collection shelves (Trending Movies + Trending Shows). All shelves drag-drop reorderable; group + shelf state persisted to `config.json`
- **Library browse** — paginated grid, sort dropdown (5 options including precise Release Date), shows-vs-episodes view toggle for TV libraries, hidden-libraries menu, per-server zoom slider
- **Item Detail** — full metadata page for movies / shows / seasons / episodes with hero, IMDB pill (via OMDB), TMDB trailer cascade, Cast & Crew single-row, MORE WAYS TO WATCH availability across servers, Resume / Restart / Mark Watched / Watchlist actions, season pills with navigate-to-S0XE01 on episode pages, type-aware watchlist roll-up
- **Watchlist** — full plex.tv watchlist sync, hover Play / Remove / Mark Watched on every card, smart-routing Play (highest-resolution match across servers, falls through to TMDB trailer if not in any library), title-click avail-aware route
- **Recommended** — `watchlist`-namespace hubs (Coming Soon, New Trailers From Your Watchlist, Recently Added, Recently Aired Episodes) with per-type DiscoverTile render and trailer modals
- **Discover** — `home`-namespace hubs (Coming Soon, New Trailers, Trending Trailers, Most Watchlisted, Trending on Plex, Upcoming Blockbusters, Highly Anticipated, Trending on Apple TV) with HLS trailer playback for clip variants and TMDB trailer fallback for movies/shows
- **Discover Item Detail** — plex.tv-source detail page for items not in any local library, with availability check, IMDB pill, MORE WAYS TO WATCH, Watchlist toggle
- **Cross-source search** — `/search?q=` debounced flydown + full results page, fans out across servers + plex.tv discover, click-routes by source with availability resolution for discover items
- **Playback** — Pot Player launch with state polling via SSE, Resume / Restart / Mark Watched / Remove from CW / NextEpisode / TranscodePrompt modal flows
- **Settings** — Appearance (theme, zoom, card size, density, rows-per-shelf, font, layout, defaults) / Shortcuts (desktop .lnk) / Accounts & Servers (server rename, refresh, hidden libraries) / Playback (Pot Player path) / Data & Cache (size + clear) / About
- **Top bar** — search flydown, fullscreen toggle (F11-style), back / home navigation, zoom slider, close confirmation
- **Image proxy** — per-server token rotation with account-token fallback (handles Stargaze agent-sourced posters), CDN cache key alignment, configurable per-surface dimensions (poster / hero / person)
- **Auth** — Plex PIN flow with browser auto-launch, account token DPAPI-encrypted at rest

### Architecture

Single Windows binary (`lumen.exe`, ~12.3 MB). Embedded SolidJS SPA at `127.0.0.1:7832`. Go HTTP server proxies all Plex API calls (account token never reaches the SPA, always header-only). Config in `%APPDATA%\Lumen\config.json` with DPAPI-encrypted secrets. Pot Player launched as child process for playback. Desktop shortcut (`Lumen.lnk`) runs `lumen.exe serve` → server starts → default browser opens to localhost.

### Operating model

- **`lumen auth`** — first-time Plex PIN authentication
- **`lumen list`** — discover servers (run after auth)
- **`lumen serve`** — start the HTTP server + open browser; this is what the desktop shortcut runs

### Known limitations at v1.0

See "Known issues carried forward" section above. None are user-blocking; all are explicitly deferred enhancements.

### Session count and commit history

| Session | Status | Notes |
|---|---|---|
| Session 1 | DONE | Foundation |
| Session 2 | DONE | Home page + 14 shelves; first-class Plex integration |
| Session 3 | DONE | Settings / Item Detail / Episodes; first plex.tv discover |
| Session 4 | DONE | Pot Player playback + scrobble + remove-from-CW + auth flow |
| Session 4.5 | DONE | Polish PR (23+4 commits) |
| Session 5 | DONE | Watchlist / Recommended / Discover / OMDB IMDB / Discover Item Detail (21+10 commits) |
| Session 6 | DONE | Stargaze image proxy fix + TMDB trailers + Discover/Recommended UI parity + plex.tv detail page (33 commits) |
| **Session 6.5** | **DONE — V1.0 SHIPPED** | **Bug-fix + feature-evolution iteration (8 commits + this doc)** |

## Pre-flight checklist for any future v1.1+

- [ ] Settings hydration with masked indicators
- [ ] HLSTrailerModal code-split for bundle size
- [ ] Cast/Crew + MORE WAYS TO WATCH shared component extract
- [ ] `TestImageProxyForwardsWithTokenServerSide` cache-dir injection root fix
- [ ] Per-source search filter UI (Stargaze-only / Discover-only)
- [ ] `/api/availability/batch` for Watchlist N-parallel call optimisation
- [ ] Other Plex wire-shape stress testing (raw-body dump catches them)
- [ ] DiscoverItem.tsx cast-repetition cleanup

## Design notes locked at v1.0

These are the architectural decisions that should not be revisited without strong reason:

- **Header-only Plex auth** for non-image API calls. Image proxy is the documented exception (token in URL query for `/photo/:/transcode`, twice).
- **Account token never reaches the SPA.** Always proxy through Lumen's HTTP server.
- **Config tokens DPAPI-encrypted at rest.** Plaintext only in memory.
- **Per-server tokens stored separately from account token.** Used for server-local API calls only.
- **Plex.tv ratingKey vs server-local ratingKey are different namespaces.** Bridge via `extractPlexTvRatingKey(plex://<type>/<rk>)`.
- **Plex hubs use case-collision absorbers** for guid/Guid, studio/Studio, rating/Rating. Wire fattening should always include both lowercase scalar and capital array form for known polymorphic fields.
- **Polymorphic field handling via custom UnmarshalJSON** for fields like `summary` (string-or-array). Raw-body dump on decode failure is the safety net for unknown shapes.
- **Boundary error scrub pattern** — log detailed error server-side via `log.Printf`, respond with generic message to the SPA. Critical for any handler calling external HTTP APIs.
- **Test fixtures must mirror real Plex captures.** Synthetic fixtures lied repeatedly across the session series. Capture from DevTools → fixture from capture → test from fixture.
- **Solid `<For>` over a refetched resource needs `stableArrayByKey` upstream + `<Index>` for outer pages.** Both layers required to prevent click-loss and remount flicker.
- **Plex watchlist is show-keyed.** Episode/season clicks must roll up to parent show via `AddItemToWatchlist` / `RemoveItemFromWatchlist` helpers.
- **Plex `originallyAvailableAt` is YYYY-MM-DD string.** `formatAirDate` formats future dates as "May 10, 2026"; past dates fall back to year.
- **MediaContainer.Meta.DisplayFields is the authoritative per-type render directive.** Hub-style components should consult it (or mirror its rules).
- **Lookup-by-title for Plex Collections.** Admin-rename tolerant; never hardcode collection ratingKeys.
- **Display label vs lookup key are decoupled.** Lumen's shelf names don't have to match Plex's catalog names.

---

*V1.0 shipped 2026-04-28. Time to use Lumen.*
