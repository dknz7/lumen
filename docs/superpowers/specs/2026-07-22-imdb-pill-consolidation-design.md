# IMDB Pill Consolidation

**Date:** 2026-07-22
**Status:** Approved by Byron

## Problem

Server-item detail pages (`ItemDetail`) render one yellow IMDB pill that both
loads the OMDB rating and links to the title's imdb.com page. Discover
detail pages (`DiscoverItem`, e.g. `/discover-item/<ratingKey>`) instead
render two IMDB elements, both wrong in a different way:

- a hand-rolled yellow `<span>` pill in the hero — shows the rating but is
  not a link;
- a separate grey "IMDB" action button lower down — links correctly but
  shows no rating.

Cause: `IMDBPill` lives as a private component inside `ItemDetail.tsx`;
`DiscoverItem` re-implemented half of it twice instead of reusing it.
Audit confirmed this is the only duplication — all other IMDB references in
the SPA are trailer-resolution plumbing (`imdbId` → TMDB) and the OMDB
API-key settings field, not UI.

## Design

Extract the proven pill; delete the duplicates.

1. **New shared component `web/src/components/IMDBPill.tsx`** — a verbatim
   lift of the current `IMDBPill` from `ItemDetail.tsx`: props
   `{ imdbId?: string }`, fetches the OMDB rating itself via
   `api.imdb(id)`, renders `<a class="pill pill-imdb pill-imdb-link">` to
   `https://www.imdb.com/title/<imdbId>/` when an id is present. Rating
   missing/unavailable (e.g. unreleased titles) → value shows `—` but the
   pill still links; the link never depends on the rating fetch. No id →
   callers hide it via their existing `Show` guards.
   The stale comment referencing "DiscoverItem keeps a separate IMDB action
   button" is dropped during the lift.
2. **`ItemDetail.tsx`** — delete the local `IMDBPill`, import the shared
   one. No behavioural change; episode pages keep resolving the parent
   show's id via the existing `effectiveImdbId` memo passed as the prop.
3. **`DiscoverItem.tsx`** —
   - Hero: replace the `<span class="pill pill-imdb">…</span>` block with
     `<IMDBPill imdbId={props.item.imdbId} />` (inside the existing
     `Show when={props.item.imdbId}` guard).
   - Remove the grey IMDB action button (`<a class="btn" …imdb.com…>`)
     from the actions row.
   - Remove the page-level `imdbRating` resource and the Hero's
     `imdbRating` prop — the shared component fetches for itself.
4. **CSS:** none. `pill-imdb` / `pill-imdb-link` styles live in
   `ItemDetail.css`, which `DiscoverItem.tsx` already imports.

## Out of Scope

- Discover/Watchlist tiles and any mini-badges — detail pages only.
- The OMDB fetch path (`api.imdb`), rating formats, and the RT/audience
  rating pills on DiscoverItem's hero are untouched.

## Testing

- `cd web && npx tsc --noEmit` clean; `npm run build`; rebuild `lumen.exe`.
- Smoke (Byron): a server item (pill links + rating), an episode (parent
  show's id), a Discover item not on any server, e.g. Avengers: Doomsday
  `/discover-item/62deb8bf9576053a6b5c5ee7` (single yellow pill, links,
  `—` rating acceptable while unreleased, grey button gone).
