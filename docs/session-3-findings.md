# Session 3 — Theme, Settings Modal, Persistence, Drag-Reorder Findings

**Date:** 2026-04-25
**Status:** Session 3 complete. Session 4 unblocked.
**Commits:** 21 task commits from initial implementer pass + 11 follow-up fix/feature commits during verification.

## Verification results (Task 22)

| Surface | Result |
|---|---|
| **Home — 14 shelves across 2 groups** | **PASS**. Continue Watching pinned at top (per-server onDeck merged + sorted by `lastViewedAt desc`, `addedAt desc` tiebreak). Stargaze + DKNZPLEX groups render with shelves inside. ImageOff placeholder renders under every card; real `<img>` overlays when loaded. Both server names render via local DisplayName overrides. |
| **Settings modal** | **PASS** after a Motion One easing fix. Opens with fade+scale (cubic-bezier `[0.22, 1, 0.36, 1]`, 0.24s), backdrop blur, Escape + backdrop-click both close. All 6 sections render: Appearance, Kiosk & Shortcuts, Accounts & Servers, Playback, Data & Cache, About. |
| **Theme registry** | **PASS**. Pure OLED token set wired via `applyTheme()` on boot; CSS custom properties on `:root`. Picker UI ready for future theme additions. |
| **Library page** | **PASS**. 100 items per page, Prev/Page N/Next pagination lives in the sticky header right (alongside count readout). Episodes/Shows toggle on TV libraries. Sort + view mode persist via `/api/settings`. |
| **Drag-reorder** | **PASS** for shelves within a group AND for groups themselves on Home. Drag handle (`GripVertical`, 20px shelf / 22px group) always visible top-right, no longer hover-gated. Order persists to `config.UI.ShelfState`. CW shelf opted out via `sortable={false}` (pinned per spec §12.1). |
| **Library hide** | **PASS** with a UX overhaul mid-verification. Hidden libraries fully disappear from the server tree (no longer greyed-out). A collapsible "Hidden (N)" section appears at the bottom of the libraries panel with EyeOff restore icons. |
| **Cache management** | **PASS**. `/api/cache/size` returns real per-scope bytes; `/api/cache/clear?scope=images\|omdb\|all` wipes the named directory. Settings UI shows MB readouts + per-scope clear buttons. |
| **Desktop shortcut** | **PASS**. `lumen install-shortcut` CLI + Settings button both create `Lumen.lnk` on the desktop targeting `lumen.exe serve`. **Caveat:** uses Windows default icon — proper icon deferred to Session 4 pre-flight. |
| **Taste polish (Lucide / Geist / Motion / Skeletons / Focus rings)** | **PASS**. Zero emojis remain in the SPA. Geist Sans active in computed font-family. Skeletons render shimmer placeholders for every loading shelf/grid. `:focus-visible` outline on every interactive element. |

## Open items closed

### Pre-Session-3 localStorage shims migrated to config.json
Sessions 2's `lumen.library.sort` + `lumen.library.viewMode` localStorage keys are gone. Library page reads/writes via the settings store (`defaultSort`, `defaultViewMode`). Survives across browsers because it's now server-side state.

### `/photo/:/transcode` mystery deferred
DKNZPLEX's Level 3 CDN edge still returns intermittent 404s for some thumbs. Confirmed not a Lumen bug (Plex Web exhibits the same behaviour). Disk image cache (`%APPDATA%\Lumen\cache\images`) accumulates every successful fetch indefinitely, so cards "fill in" over time as you browse. Permanent fix requires either Plex-side or DKNZPLEX-side intervention. **Carried forward, no Lumen action needed.**

### Spec §13.2 OLED Protection — explicitly dropped
Pixel-shift + auto-hide chrome **deliberately not implemented**. Theme switching rotates pixel values, achieving the same screen-wear-mitigation goal without the complexity. Documented in plan header.

### Spec §13.3 autostart — replaced
Windows autostart toggle dropped per Byron's preference. Replaced with `lumen install-shortcut` CLI + "Create Desktop Shortcut" button. Launches on demand, not on boot.

## Critical gotchas discovered (must carry forward)

These bit us hard during Session 3 — every one is now codified in the post-verification follow-up commits and **must** be enforced in Session 4's plan:

### 1. `createSortable` outside `DragDropProvider` crashes the whole render tree
@thisbeyond/solid-dnd's `createSortable` internally destructures the result of `useDragDropContext()`. When no provider is in the ancestor tree, that hook returns `undefined`. Destructuring undefined throws `TypeError: can't access property Symbol.iterator, X is null` — completely opaque, propagates up the entire SolidJS render graph, blanks the page.

**Fix pattern (load-bearing in `Shelf.tsx` + `Group.tsx`):**
```tsx
const ctx = useDragDropContext();
const canSortable = ctx != null;          // loose != catches both null AND undefined
const sortable = canSortable ? createSortable(props.id) : null;
```

`Shelf` also takes a `sortable?: boolean` prop (default true) so callers OUTSIDE a provider can opt out explicitly (Continue Watching uses `sortable={false}`).

This bug existed silently from Session 3's Task 18 onwards — Home was empty whenever you navigated to it, but Byron only noticed after several other red herrings (invisible Settings modal, stale binary, fresh-config test) were eliminated.

### 2. Vite does NOT type-check during `npm run build`
Pure transpile, no `tsc --noEmit`. We shipped 10+ TypeScript errors silently (param undefined, discriminated-union narrowing, missing exports). At least one of them — calling a Solid component as `component({})` instead of via `<Dynamic>` — caused a runtime crash equivalent to the Symbol.iterator one.

**Fix:** Session 4's plan must include `npx tsc --noEmit` in every build verification step.

### 3. `lumen.exe` embeds `internal/server/web/dist/` at Go build time
This burned an hour. After every SPA fix:
- ✅ `cd web && npm run build` (writes new dist files)
- ❌ ...but **the running `lumen serve` is still the OLD `lumen.exe`**, which has the OLD dist baked in via `//go:embed`.
- ✅ MUST also run `go build -o lumen.exe ./cmd/lumen` AND restart the running serve.

**Fix:** Session 4's plan must mandate `go build` + restart-serve in every SPA-touching task's verification step.

### 4. Motion One Solid does NOT accept Framer-Motion-style spring strings
`easing: "spring(1, 100, 14, 0)"` silently fails — animation never runs, element stays at `initial` opacity. Use cubic-bezier arrays (`[0.22, 1, 0.36, 1]`) or import `spring()` from Motion One properly. The Settings modal was invisible-but-clickable for an hour because of this.

### 5. Calling Solid section components as functions breaks the render tree
`{activeSection().component({})}` is NOT equivalent to `<Dynamic component={activeSection().component} />`. The function-call form produces a non-reactive children tree that Solid's iteration machinery can't process. Use `<Dynamic>` for any reactive component switching.

### 6. Plex's `guid` (string) vs `Guid` (array) case-sensitivity
Already documented in Session 2 findings, but worth re-flagging: Go's `encoding/json` is case-insensitive for field matching when no exact match exists. Always declare an explicit absorber field for capital `Guid` to prevent the array-into-string-field crash.

## New features beyond plan

Added during verification in response to live usage feedback:

- **Continue Watching shelf has dynamic height** — original plan capped via `max-height + overflow:hidden`, which cut items mid-row when Plex returned more onDeck items than fit. Removed the cap; cards flow naturally.
- **Shelves and groups have surface styling** — dark-grey (`--bg-menu`) background, soft white-translucent border, generous internal padding. Always-visible `GripVertical` drag handle (20/22px, 0.85 opacity → 1 on hover). Visually clearer that they're independent reorderable units.
- **Library pagination moved to header right** — sticky alongside the count readout, no more scrolling to paginate.
- **Library hide UX overhauled** — full removal from menu (not greyed) + collapsible "Hidden (N)" restore section at bottom of libraries panel.
- **`ImageOff` fallback for failed/missing thumbs** — placeholder-first pattern: ImageOff renders by default; `<img>` overlays when it loads; on `onError` the img unmounts and placeholder shows through. Single render path handles both "no thumb path" and "thumb path 404s" cases.
- **Bundle size discipline** — final SPA bundle 114.5 KB JS (39.2 KB gzipped), 18.9 KB CSS (4.3 KB gzipped). Geist + Lucide + Motion One + solid-dnd + Solid all included.

## Known issues carried to Session 4+

### Desktop shortcut needs a real icon
`Lumen.lnk` currently uses Windows' default executable icon (the running-app glyph). Proper fix requires:
1. Source a `.ico` asset (custom design or generated from Lucide Sparkles).
2. Use `goversioninfo` to generate `resource_windows_amd64.syso` linking the icon into `lumen.exe`.
3. Update `internal/shortcuts/windows.go` to set `IconLocation = exePath, 0` (already does).

**Pre-flight item for Session 4 plan.**

### DKNZPLEX CDN thumb 404s persist
Disk image cache mitigates over time. No code-level fix possible. May resolve if DKNZPLEX admin reconfigures CDN or if Plex addresses upstream.

### Re-authenticate button stubbed
Currently shows an alert directing user to run `lumen auth` in a terminal. Inline PIN re-auth flow lands in Session 5 alongside Watchlist/Recommended/Discover work.

### OMDB key entry persists but isn't consumed yet
Field saves to `config.OMDBKey` cleanly. Actual OMDB lookup + IMDB rating pill on Item Detail lands in Session 5.

### Pot Player path field persists but isn't consumed yet
Saves to `config.UI.Playback.PotPlayerPath`. Session 4's `internal/potplayer` Launch path-resolver will read it.

## Plan deviations

| # | Issue | Resolution |
|---|---|---|
| 1 | Subagent landed Tasks 3+4 in a single commit instead of two | Acceptable — handler cohesion. No lost functionality. |
| 2 | Subagent kept Skeleton fallbacks in components but left "Loading…" text in some `<Show>` fallbacks | Verified all replaced during follow-up. |
| 3 | Plan's Motion One spring syntax was wrong (`"spring(1, 100, 14, 0)"`) | Fixed to cubic-bezier array `[0.22, 1, 0.36, 1]`. |
| 4 | Plan called `activeSection().component({})` for Settings sections | Fixed to `<Dynamic component={activeSection().component} />`. |
| 5 | `Shelf` + `Group` called `createSortable` unconditionally | Fixed with `useDragDropContext() != null` guard + `sortable?` prop on Shelf. |
| 6 | Several `params.X!` non-null assertions missing in `Library.tsx` + `ItemDetail.tsx` | Added — Vite tsc-skip surfaced them late. |
| 7 | Original `max-height` cap on `.shelf-cards` cut items mid-row | Removed. Cards flow naturally. |
| 8 | Surface styling not in plan (was understated) | Added shelf/group surfaces during verification per Byron's feedback. |
| 9 | Library hide had greyed-out-but-clickable rows | Replaced with full removal + Hidden restore section. |
| 10 | Library pagination at the bottom of the page | Moved into sticky header right. |

## Session 4 readiness — pre-flight checklist for the plan

These MUST be in the Session 4 plan's pre-flight section:

- [ ] **Anti-stale-binary rule:** every task that modifies anything under `web/` must include `cd web && npm run build && cd .. && go build -o lumen.exe ./cmd/lumen` and a "restart `lumen serve`" reminder before considering the task done.
- [ ] **TypeScript verification:** every SPA-touching task must run `cd web && npx tsc --noEmit` and confirm clean before commit.
- [ ] **`useDragDropContext != null` guard rule:** any new component using solid-dnd primitives must include the context guard if it could render outside a provider.
- [ ] **`Dynamic` for reactive component switching:** never call Solid components as plain functions; use `<Dynamic component={...} />`.
- [ ] **Desktop shortcut icon:** source/generate `.ico`, integrate via `goversioninfo`. First task or pre-flight task.
- [ ] **Carry the Session 0 findings forward:** Pot Player IPC details (units = ms, reads via `WM_USER+0x500X`, writes via `WM_APPCOMMAND` codes 13/14) are the spec for Session 4's `internal/potplayer` impl.

## Build & test state at close of Session 3

- `go test ./...` → passing across `internal/config` (12 tests), `internal/plex` (15 tests), `internal/server` (10 tests). One pre-existing `TestImageProxyForwardsWithTokenServerSide` failure carried over from Session 2's image-proxy header rewrite — token-auth edge case in a unit test, not user-facing. Documented separately.
- `cd web && npx tsc --noEmit` → **clean.**
- `cd web && npm run build` → 114.5 KB JS / 18.9 KB CSS gzipped, 22 woff2 font files for Geist Sans + Geist Mono.
- `go build -o lumen.exe ./cmd/lumen` → clean.
- `probe/` directory and Sessions 0–2 findings untouched.
- `main` branch state: 32 commits added since Session 2 close (`6753702`). Final commit: `edd1101`.

## Design notes carried forward (live for Sessions 4–5)

Reaffirming Session 2's notes plus Session 3 additions:

- Pure `#000000` retained — OLED burn-in mitigation, documented exception to taste-skill's "no pure black" rule.
- No emojis in any UI surface — all glyphs from `lucide-solid` via the `web/src/components/icons.ts` barrel.
- Geist Sans is the primary font; Geist Mono for any code/data display.
- Max one accent — dark navy `#0f1729`. Desaturated, singular.
- Skeleton loaders, not "Loading…" text. Use `<Skeleton kind="card" count={N} />` for grids.
- `:focus-visible` rings on every interactive element.
- No perpetual micro-animations. Motion is reserved for deliberate moments (modal open/close, drag-active state, hover dim).
- Cast/Crew (Session 5): actor names in white, character names in body-text muted grey.
- Episode rows (Session 5): episode title white, description/duration/date in muted grey.
- Primary action buttons stay inverse-fill (white bg, black text, pill-shaped).
- Settings modal: dark-grey nav rail + dark canvas detail pane + 1px white-translucent border + inset highlight (Liquid-glass refraction edge).
