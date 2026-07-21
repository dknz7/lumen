# IMDB Pill Consolidation Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** One shared yellow clickable IMDB pill (rating + imdb.com link) on both ItemDetail and DiscoverItem detail pages; DiscoverItem's dead span-pill and grey action button removed.

**Architecture:** Lift the private `IMDBPill` out of `ItemDetail.tsx` into `web/src/components/IMDBPill.tsx` verbatim (it is self-contained: `{ imdbId?: string }` prop, own OMDB fetch via `api.imdb`, `<a>` when id present, span fallback when not). Both pages consume it; DiscoverItem drops its duplicate rating resource and grey button.

**Tech Stack:** SolidJS + TypeScript, Vite (embedded into `lumen.exe` via `internal/server/web/dist`).

**Spec:** `docs/superpowers/specs/2026-07-22-imdb-pill-consolidation-design.md`

## Global Constraints

- No CSS changes — `pill-imdb` / `pill-imdb-link` styles live in `ItemDetail.css`, already imported by both pages.
- No behavioural change on ItemDetail (including the no-id span fallback it renders today and episode → parent-show id resolution via `effectiveImdbId`).
- Discover/Watchlist tiles, OMDB fetch path, RT/audience pills: untouched.
- No frontend test runner exists — verification is `cd web && npx tsc --noEmit` per task, full build at the end.
- Working directory: `C:\Users\dicke\Desktop\Dump Zone\STACK\04-DEV\lumen`.

---

### Task 1: Shared component + ItemDetail switch

**Files:**
- Create: `web/src/components/IMDBPill.tsx`
- Modify: `web/src/pages/ItemDetail.tsx` (delete local `IMDBPill` at lines 400-435; add import)

**Interfaces:**
- Produces: default export `IMDBPill(props: { imdbId?: string })` from `web/src/components/IMDBPill.tsx` — consumed by Task 2.

- [ ] **Step 1: Create the shared component**

Create `web/src/components/IMDBPill.tsx`:

```tsx
import { createResource, Show } from "solid-js";
import { api } from "../api/client";
import type { OMDBRating } from "../api/types";

// The one IMDB pill, shared by ItemDetail and DiscoverItem heroes. Fetches
// its own OMDB rating; renders as a link to imdb.com when an imdbId is
// present, and a plain non-link span when not (no imdbId → nothing to link
// to). The link never depends on the rating fetch — unrated/unreleased
// titles show "—" but still link.
export default function IMDBPill(props: { imdbId?: string }) {
  const [rating] = createResource(
    () => props.imdbId,
    async (id) => (id ? api.imdb(id) : null)
  );
  const value = () => (
    <Show when={rating()} fallback={<span class="pill-imdb-value">—</span>}>
      {(r) => <span class="pill-imdb-value">{(r() as OMDBRating).imdbRating ?? "—"}</span>}
    </Show>
  );
  return (
    <Show
      when={props.imdbId}
      fallback={
        <span class="pill pill-imdb">
          <span class="pill-imdb-label">IMDB</span>
          {value()}
        </span>
      }
    >
      <a
        class="pill pill-imdb pill-imdb-link"
        href={`https://www.imdb.com/title/${props.imdbId}/`}
        target="_blank"
        rel="noreferrer"
        title="Open on IMDB"
      >
        <span class="pill-imdb-label">IMDB</span>
        {value()}
      </a>
    </Show>
  );
}
```

Note vs the original: the duplicated rating `<Show>` block is hoisted into the
`value()` helper, and the stale comment about "DiscoverItem keeps a separate
IMDB action button" is dropped (that button is deleted in Task 2). Everything
else — classes, structure, fallback — is byte-identical behaviour.

- [ ] **Step 2: Switch ItemDetail to the shared component**

In `web/src/pages/ItemDetail.tsx`:

(a) Delete the entire local `function IMDBPill(props: { imdbId?: string }) { … }` (currently lines 400-435, between `formatBytes`'s closing brace and `function CastCrew`).

(b) Add the import alongside the other component imports at the top of the file:

```tsx
import IMDBPill from "../components/IMDBPill";
```

The Hero's usage `<IMDBPill imdbId={props.effectiveImdbId} />` stays untouched.

- [ ] **Step 3: Typecheck**

Run: `cd web && npx tsc --noEmit`
Expected: no errors

- [ ] **Step 4: Commit**

```bash
git add web/src/components/IMDBPill.tsx web/src/pages/ItemDetail.tsx
git commit -m "refactor(web): extract shared IMDBPill component"
```

---

### Task 2: DiscoverItem consolidation

**Files:**
- Modify: `web/src/pages/DiscoverItem.tsx` (hero span pill → shared component; grey button removed; duplicate rating resource removed)

**Interfaces:**
- Consumes: default export `IMDBPill(props: { imdbId?: string })` from Task 1.

- [ ] **Step 1: Swap the hero pill and drop the duplicates**

In `web/src/pages/DiscoverItem.tsx`:

(a) Add the import at the top with the other component imports:

```tsx
import IMDBPill from "../components/IMDBPill";
```

(b) Delete the page-level rating resource (currently lines 64-68):

```tsx
  // OMDB IMDB rating pill — same lookup ItemDetail uses.
  const [imdbRating] = createResource(
    () => item()?.imdbId,
    async (id) => (id ? api.imdb(id) : null)
  );
```

(c) Change the Hero call site (currently line 145) from:

```tsx
            <Hero item={it() as DiscoverItem} imdbRating={imdbRating() ?? null} />
```

to:

```tsx
            <Hero item={it() as DiscoverItem} />
```

(d) Delete the grey IMDB action button from the action row (currently lines 170-179):

```tsx
              <Show when={(it() as DiscoverItem).imdbId}>
                <a
                  class="btn"
                  href={`https://www.imdb.com/title/${(it() as DiscoverItem).imdbId}/`}
                  target="_blank"
                  rel="noreferrer"
                >
                  IMDB
                </a>
              </Show>
```

(e) In the `Hero` component (currently line 251), change the signature from:

```tsx
function Hero(props: { item: DiscoverItem; imdbRating: OMDBRating | null }) {
```

to:

```tsx
function Hero(props: { item: DiscoverItem }) {
```

(f) In Hero's meta-pills, replace the hand-rolled span (currently lines 299-305):

```tsx
          <Show when={props.item.imdbId}>
            <span class="pill pill-imdb">
              <span class="pill-imdb-label">IMDB</span>
              <span class="pill-imdb-value">
                {props.imdbRating?.imdbRating ?? "—"}
              </span>
            </span>
          </Show>
```

with:

```tsx
          <Show when={props.item.imdbId}>
            <IMDBPill imdbId={props.item.imdbId} />
          </Show>
```

(g) Remove `OMDBRating` from the type-only import at line 4 (it becomes unused):

```tsx
import type { DiscoverItem, DiscoverRating, Match, Person } from "../api/types";
```

If `createResource` is now unused in the file after (b), remove it from the solid-js import too — check remaining usages first (`watchlist` and `resolvedTrailer` resources also use it, so it almost certainly stays).

- [ ] **Step 2: Typecheck**

Run: `cd web && npx tsc --noEmit`
Expected: no errors (an unused-import error here means step (g) was missed)

- [ ] **Step 3: Commit**

```bash
git add web/src/pages/DiscoverItem.tsx
git commit -m "fix(web): single clickable IMDB pill on Discover detail pages"
```

---

### Task 3: Build + smoke handoff

**Files:**
- Modify (generated): `internal/server/web/dist/*` (only `index.html` is tracked; assets are gitignored by design)
- Output: `lumen.exe`

- [ ] **Step 1: Build SPA and binary**

Run: `cd web && npm run build`
Expected: vite build succeeds into `../internal/server/web/dist`

Run (repo root): `go build -o lumen.exe ./cmd/lumen`
Expected: builds cleanly

- [ ] **Step 2: Commit the embedded dist**

```bash
git add internal/server/web/dist
git commit -m "build(web): embed IMDB pill consolidation"
```

- [ ] **Step 3: Smoke test (Byron)**

Restart `lumen.exe`, hard-refresh the browser (SPA changed), then per the spec:
1. Server item → single yellow pill, rating loads, links to imdb.com.
2. Episode page → pill uses parent show's id.
3. `/discover-item/62deb8bf9576053a6b5c5ee7` (Avengers: Doomsday) → single yellow pill, links, `—` rating acceptable while unreleased, grey button gone.
