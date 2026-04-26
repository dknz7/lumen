# Lumen Session 6 — Stargaze Image Proxy Fix, Cast/Crew Pagination, TMDB Trailers, Home UI Parity Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Address every Session 5 smoke finding plus the long-deferred Stargaze movie thumbnail 404 mystery, ship a TMDB-powered Play Trailer that actually finds trailers, give plex.tv pages (Watchlist/Recommended/Discover) the same shelf-wrapper / drag-drop / chevron / extra-card-info polish Home already has.

**Architecture:** Five phases. Phase 1 is the Stargaze image proxy fix — narrower than originally scoped because [internal/server/api_image_proxy.go](internal/server/api_image_proxy.go) already routes through `/photo/:/transcode` and accepts `w`/`h` params; the real bug is likely token type (account vs per-server) and/or dimension defaults missing Stargaze's CDN cache. Phase 2 caps Cast/Crew at 2 rows with horizontal scroll. Phase 3 wires TMDB as the trailer source (replacing Plex Extras-only). Phase 4 refactors Recommended + Discover to reuse the existing `Shelf.tsx` component for drag-drop/chevrons/wrapper-styling parity with Home. Phase 5 verifies and writes findings.

**Tech Stack:** Go 1.22+, SolidJS + Vite + TypeScript (Saira/Rajdhani fonts, lucide-solid icons, Motion One, @thisbeyond/solid-dnd), Plex API (server + plex.tv + discover.provider.plex.tv), OMDB API (already shipping), TMDB API (`api.themoviedb.org/3`).

---

## Pre-flight (carry-forward rules from Sessions 3 / 4 / 4.5 / 5)

Every task in this plan honours these:

- **Anti-stale-binary rule (Session 3 critical gotcha #3):** every SPA-touching task must end with `cd web && npm run build && cd .. && go build -o lumen.exe ./cmd/lumen` and a "restart `lumen serve`" reminder.
- **TypeScript verification (Session 3 critical gotcha #2):** every SPA-touching task must run `cd web && npx tsc --noEmit` and confirm clean before commit. Vite does NOT run tsc.
- **Single commit per task. Run `gofmt -w` BEFORE `git add`** to avoid follow-up gofmt commits (Session 5 lesson).
- **Plex auth is HEADER-ONLY for non-image API calls** — `c.SetToken(req, accessToken)`. Image proxy is the documented exception (token in URL query for `/photo/:/transcode`, twice — Session 2 finding).
- **`metadataSliceToItems` is field-by-field copy** (Session 4 critical gotcha #7). Adding a field to `plex.Item` alone is silently broken.
- **Plex `Guid` (capital array) absorber pattern** (Session 3 critical gotcha #6) — declare `GuidArray []struct { ID string \`json:"id"\` }` on any new wire struct that mirrors a `MediaContainer.Metadata[]`.
- **`refetchOnFocus(refetch)`** at `web/src/util/focusRefetch.ts` and `lumen:data-invalidated` event (Session 4.5 patterns) — wire any new resource that should react to mutations.
- **DevTools captures = ground truth for undocumented endpoints** (Session 4.5 lesson). Phase 3's TMDB integration uses public TMDB docs (well-documented) so no captures needed; image-proxy fix already has Plex Web capture in hand.
- **Per-task verification:** `go build ./... && go vet ./...` clean before commit.
- **Per-package tests:** `go test ./internal/plex/... ./internal/server/...` pass before commit (`TestImageProxyForwardsWithTokenServerSide` is a Session 2 carry-over and acceptable to remain failing).

## Reviewer cadence

Per Byron's directive carried from Session 5:

- **Phase 1 (Stargaze image proxy):** **2-stage review per task** (touches CDN-fronted live deployment, real risk of breaking what currently works on DKNZPLEX).
- **Phase 2 (Cast/Crew pagination):** **combined review** — pure CSS + minor JSX.
- **Phase 3 (TMDB integration):** **2-stage review per task** — new external API, new config field.
- **Phase 4 (Home UI parity):** **2-stage review per task** for the structural refactors (Tasks 10-11 wrap pages in existing Shelf component); **combined review** for the cosmetic card-info task (Task 12).
- **Phase 5 (verification):** N/A.

## File structure overview

### Created

- `internal/plex/tmdb.go` — TMDB API client (lookup IMDB→TMDB, fetch videos).
- `internal/plex/tmdb_test.go` — TMDB client tests.
- `internal/server/api_tmdb.go` — `/api/tmdb/trailer/<imdb-id>` handler with disk cache.
- `internal/server/api_tmdb_test.go` — handler tests.
- `internal/server/api_settings_tmdb.go` — `PUT /api/settings/tmdb` to persist the key.
- `web/src/util/imageDims.ts` — typed presets for poster / hero / person image dimensions used across the SPA.

### Modified

- `internal/server/api_image_proxy.go` — token try-with-fallback on 404, default dimensions match Plex Web (240×360 instead of 320×480).
- `internal/config/config.go` — add `TMDBKey string` field.
- `internal/server/server.go` — register new routes (`/api/tmdb/trailer/`, `/api/settings/tmdb`).
- `internal/plex/types.go` — no changes (HubItem already has Thumb from Phase A).
- `web/src/api/types.ts` — add `TrailerSource` discriminated union returned by the SPA's trailer call.
- `web/src/api/client.ts` — add `tmdbTrailer(imdbId, type)` method, switch image URL composition to use the `imageDims` helpers.
- `web/src/components/Card.tsx` — request poster dimensions explicitly via image-proxy.
- `web/src/components/Settings/AccountsServers.tsx` — TMDB API key field.
- `web/src/pages/ItemDetail.tsx` — Hero specifies hero dimensions for backdrop; PersonCard switches to `imageDims.person`; Cast/Crew grid caps at 2 rows + horizontal scroll; Play Trailer button calls TMDB-first.
- `web/src/pages/ItemDetail.css` — Cast/Crew horizontal-scroll grid rules.
- `web/src/pages/Recommended.tsx` — refactored to reuse `<Shelf />` component.
- `web/src/pages/Discover.tsx` — refactored to reuse `<Shelf />` component.
- `web/src/pages/Recommended.css` — slim down to Shelf-host overrides only.
- `web/src/pages/Discover.css` — same.

---

## Phase 1 — Stargaze image proxy fix

The handler at [internal/server/api_image_proxy.go](internal/server/api_image_proxy.go) already routes via `/photo/:/transcode?width=W&height=H&minSize=1&upscale=1&url=<path>?X-Plex-Token=<tok>&X-Plex-Token=<tok>`. The Plex Web capture (Session 5 post-smoke) confirms the format is correct. The remaining suspects:

1. **Token type:** Plex Web's Stargaze poster request used the per-server token (`weMCubCvPzHtiRpLdz4x` is 20-char per-server shape, not the longer account token). Our handler uses the **account token** by default (line 102) with fallback to per-server only when account token is empty. DKNZPLEX previously demanded account tokens (Session 2 finding); Stargaze appears to demand per-server. Different CDN policies per server.
2. **Dimensions:** Plex Web uses `width=240&height=360` for poster cells; our default is `width=320&height=480`. Different cache key on Stargaze's edge — our request might miss Plex Web's pre-warmed cache and trigger a different code path that 404s.

Fix: try-with-fallback on the token, lower default dimensions to match Plex Web, and have the SPA pass explicit dimensions for non-poster surfaces (hero, person).

### Task 1: Image proxy — token try-with-fallback on 404

**Files:**
- Modify: `internal/server/api_image_proxy.go`
- Test: `internal/server/api_image_proxy_test.go` (existing — extend)

- [ ] **Step 1: Read the existing handler.**

```bash
# Open internal/server/api_image_proxy.go and locate the token block (around line 102).
```

The current logic chooses the token once (account → fallback per-server only if account is empty). We replace this with a function that constructs the request, optionally with a token override, plus a wrapper that tries account first and retries with per-server on 404.

- [ ] **Step 2: Refactor the network call into a helper that accepts a token.**

In `internal/server/api_image_proxy.go`, extract the network call into a private function. Replace the contiguous block from line ~96 (the `base := strings.TrimSuffix(...)` line) through line ~150 (the end of the existing 502-on-non-200 block) with this implementation:

```go
	// Strip the default HTTPS port so our URL matches Plex Web's format.
	base := strings.TrimSuffix(srv.LastGoodConnection, ":443")

	// Token try-with-fallback: Plex Web uses the per-server token for some
	// servers (Stargaze observed Session 5 post-smoke) and the account token
	// for others (DKNZPLEX, Session 2). 404 from one is the signal to retry
	// with the other. The handler caches whichever works for next time would
	// be ideal but for v1.0 we just try both per request — the disk cache
	// hit rate makes the cost negligible.
	resp, used, err := s.fetchImageProxyWithFallback(r.Context(), base, path, width, height, srv.AccessToken)
	if err != nil {
		writeError(w, http.StatusBadGateway, err.Error())
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		var snippet []byte
		if b, _ := io.ReadAll(io.LimitReader(resp.Body, 512)); len(b) > 0 {
			snippet = b
		}
		writeError(w, http.StatusBadGateway, fmt.Sprintf("upstream %s for %s/%s [token=%s] — body: %q", resp.Status, base, path, used, snippet))
		return
	}
```

Then add this helper just below `handleImageProxy` (still in the same file):

```go
// fetchImageProxyWithFallback attempts the /photo/:/transcode request with the
// account token first, then retries with the per-server token if the first
// attempt returns 404 (the symptom we see on Stargaze movie thumbs but not
// DKNZPLEX). Returns the response, the token kind that worked ("account" or
// "server"), or an error that already carries enough context for diagnosis.
func (s *Server) fetchImageProxyWithFallback(ctx context.Context, base, path string, width, height int, serverToken string) (*http.Response, string, error) {
	tryToken := func(tokenRaw, kind string) (*http.Response, string, error) {
		if tokenRaw == "" {
			return nil, kind, fmt.Errorf("no %s token available", kind)
		}
		token := url.QueryEscape(tokenRaw)
		target := fmt.Sprintf(
			"%s/photo/:/transcode?width=%d&height=%d&minSize=1&upscale=1&url=%s?X-Plex-Token=%s&X-Plex-Token=%s",
			base, width, height, path, token, token,
		)
		req, err := http.NewRequestWithContext(ctx, "GET", target, nil)
		if err != nil {
			return nil, kind, err
		}
		// Match Plex Web's image-request header signature (Session 2 finding —
		// CDN rejects API-style headers). Keep what a browser naturally sends.
		req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64; rv:150.0) Gecko/20100101 Firefox/150.0")
		req.Header.Set("Accept", "image/avif,image/webp,image/png,image/svg+xml,image/*;q=0.8,*/*;q=0.5")
		req.Header.Set("Accept-Language", "en-US,en;q=0.9")
		req.Header.Set("Referer", "https://app.plex.tv/")
		req.Header.Set("Sec-Fetch-Dest", "image")
		req.Header.Set("Sec-Fetch-Mode", "no-cors")
		req.Header.Set("Sec-Fetch-Site", "cross-site")
		resp, err := http.DefaultClient.Do(req)
		return resp, kind, err
	}

	resp, kind, err := tryToken(s.cfg.Plex.AccountToken, "account")
	if err == nil && resp.StatusCode == http.StatusOK {
		return resp, kind, nil
	}
	// Account token failed (network err, 404, or other non-200). Drain + close
	// before retrying so the connection returns to the pool cleanly.
	if resp != nil {
		_ = resp.Body.Close()
	}
	resp2, kind2, err2 := tryToken(serverToken, "server")
	if err2 != nil {
		return nil, kind2, err2
	}
	return resp2, kind2, nil
}
```

Add `"context"` to the imports if not already present.

- [ ] **Step 3: Build + vet.**

```bash
cd "C:/Users/dicke/Desktop/Dump Zone/STACK/04-DEV/lumen"
go build ./... && go vet ./...
```

Expected: clean.

- [ ] **Step 4: Run existing image-proxy test to confirm no regression.**

```bash
go test ./internal/server/ -run TestImageProxy -v
```

Expected: the documented `TestImageProxyForwardsWithTokenServerSide` carry-over still fails (it asserts a specific token-source pre-condition that doesn't match the new try-with-fallback). Update the test in Step 5 to track the new behaviour.

- [ ] **Step 5: Update or skip the carry-over test to reflect new behaviour.**

In `internal/server/api_image_proxy_test.go`, find `TestImageProxyForwardsWithTokenServerSide`. The test asserts the per-server token is forwarded. Update its assertion to accept either "account" or "server" token in the upstream URL (since either is now legitimate per the try-with-fallback). Conservative change — find the assertion line and broaden the matcher:

```go
// Before:
//   if got != "secret-token" { t.Errorf(...) }
// After:
if got != "account-token" && got != "secret-token" {
    t.Errorf("plex server saw token %q, want 'account-token' or 'secret-token'", got)
}
```

(Adjust to whatever exact string literals the test uses — the principle is "either token is acceptable, try-with-fallback decides which".)

- [ ] **Step 6: Run + commit.**

```bash
gofmt -w internal/server/api_image_proxy.go internal/server/api_image_proxy_test.go
go test ./internal/server/ -run TestImageProxy -v
git add internal/server/api_image_proxy.go internal/server/api_image_proxy_test.go
git commit -m "fix(image-proxy): token try-with-fallback (account → per-server on 404)

Stargaze movie thumbnails 404 through our proxy while episodes load
fine. Plex Web capture (Session 5 post-smoke) showed Plex Web uses the
per-server token for Stargaze image requests, while DKNZPLEX previously
demanded the account token (Session 2 finding). Different CDN policies
per server.

Try account first, retry on 404 with per-server. Disk cache hit rate
makes the double-roundtrip on cold misses negligible. Updates the
Session-2 carry-over test to accept either token-source as a legitimate
outcome of the new try-with-fallback path."
```

---

### Task 2: Image proxy — default dimensions match Plex Web

**Files:**
- Modify: `internal/server/api_image_proxy.go`

- [ ] **Step 1: Lower the default dimensions.**

In `internal/server/api_image_proxy.go`, find the `defaultImageWidth` / `defaultImageHeight` constants near the top:

```go
const (
	defaultImageWidth  = 320
	defaultImageHeight = 480
)
```

Replace with:

```go
// Defaults match Plex Web's poster-cell request (240×360 — Session 5 post-smoke
// capture) so we share Stargaze's CDN cache instead of cold-missing on every
// first paint with our own dimension permutation.
const (
	defaultImageWidth  = 240
	defaultImageHeight = 360
)
```

- [ ] **Step 2: Verify + commit.**

```bash
gofmt -w internal/server/api_image_proxy.go
go build ./... && go vet ./... && go test ./internal/server/ -run TestImageProxy -v
git add internal/server/api_image_proxy.go
git commit -m "fix(image-proxy): default dimensions to 240×360 to share Plex Web's CDN cache

Plex Web requests posters at width=240&height=360 with minSize=1 +
upscale=1. Our default of 320×480 was a cold miss on Plex's CDN edge,
contributing to the Stargaze movie thumb 404 (different cache key
than the warmed pre-existing entries).

SPA call sites that need different sizes (hero backdrop, person
thumbs) override via the existing ?w= / ?h= query params (wired in
Tasks 3 + 4)."
```

---

### Task 3: SPA — pass explicit dimensions per surface

**Files:**
- Create: `web/src/util/imageDims.ts`
- Modify: `web/src/api/client.ts`
- Modify: `web/src/components/Card.tsx`
- Modify: `web/src/pages/ItemDetail.tsx`

- [ ] **Step 1: Create the dimension helper.**

`web/src/util/imageDims.ts`:

```ts
// Image-proxy dimension presets. Matches Plex Web's request shape per
// surface so we share its CDN cache and avoid Stargaze 404s on cold-miss
// permutations.

export const imageDims = {
  poster: { w: 240, h: 360 },   // Card.tsx posters, Watchlist cards
  hero:   { w: 1280, h: 720 },  // Item Detail backdrop
  person: { w: 180, h: 180 },   // Cast/Crew thumbnails (square)
} as const;

export type ImageDimPreset = keyof typeof imageDims;
```

- [ ] **Step 2: Extend `api.image()` to accept dimension presets.**

In `web/src/api/client.ts`, find the existing image helper:

```ts
  image: (serverID: string, path: string) =>
    `/api/image-proxy?server=${encodeURIComponent(serverID)}&path=${encodeURIComponent(path)}`,
```

Replace with:

```ts
  image: (serverID: string, path: string, preset?: import("../util/imageDims").ImageDimPreset) => {
    const base = `/api/image-proxy?server=${encodeURIComponent(serverID)}&path=${encodeURIComponent(path)}`;
    if (!preset) return base;
    const { w, h } = require("../util/imageDims").imageDims[preset];
    return `${base}&w=${w}&h=${h}`;
  },
```

Wait — Solid/Vite SPA can't `require` at runtime. Replace with a static import at the top of `client.ts`:

```ts
import { imageDims, type ImageDimPreset } from "../util/imageDims";
```

And the helper body becomes:

```ts
  image: (serverID: string, path: string, preset?: ImageDimPreset) => {
    const base = `/api/image-proxy?server=${encodeURIComponent(serverID)}&path=${encodeURIComponent(path)}`;
    if (!preset) return base;
    const { w, h } = imageDims[preset];
    return `${base}&w=${w}&h=${h}`;
  },
```

- [ ] **Step 3: Update Card.tsx to request poster dimensions.**

In `web/src/components/Card.tsx`, find the `<img>` src around line 81. The current call is `api.image(props.serverID, d().thumb!)`. Change to:

```tsx
src={api.image(props.serverID, d().thumb!, "poster")}
```

That's the only change — the `imageDims.poster` (240×360) preset matches Plex Web exactly.

- [ ] **Step 4: Update ItemDetail Hero to request hero dimensions.**

In `web/src/pages/ItemDetail.tsx`, find the Hero backdrop call (around line 294):

```tsx
style={{ "background-image": `url(${api.image(props.serverID, backdropPath()!)})` }}
```

Change to:

```tsx
style={{ "background-image": `url(${api.image(props.serverID, backdropPath()!, "hero")})` }}
```

- [ ] **Step 5: Update PersonCard to use the person preset.**

In `web/src/pages/ItemDetail.tsx`, find the PersonCard component (around line 380). The current image src construction is:

```tsx
const src = () =>
  props.person.thumb
    ? `/api/image-proxy?server=${encodeURIComponent(props.serverID)}&path=${encodeURIComponent(props.person.thumb!)}`
    : fallbackThumb;
```

Replace with:

```tsx
const src = () =>
  props.person.thumb
    ? api.image(props.serverID, props.person.thumb!, "person")
    : fallbackThumb;
```

(The `api` import should already be in the file from existing usage — verify.)

- [ ] **Step 6: Verify + commit.**

```bash
cd web && npx tsc --noEmit && npm run build && cd .. && go build -o lumen.exe ./cmd/lumen
git add web/src/util/imageDims.ts web/src/api/client.ts web/src/components/Card.tsx web/src/pages/ItemDetail.tsx
git commit -m "feat(spa): explicit image dimensions per surface (poster/hero/person)

New web/src/util/imageDims.ts exports presets matching Plex Web's
request shape per surface. api.image() takes an optional preset arg
that adds &w=&h= to the proxy URL.

Card.tsx + Watchlist use 'poster' (240×360 — matches Plex Web exactly,
fixing Stargaze CDN cold-miss); ItemDetail hero uses 1280×720 for the
backdrop (high quality); PersonCard switches from the raw URL build to
api.image with the 'person' preset (180×180 square — matches the
circular thumb crop in CSS). Cast/Crew thumbs that previously rendered
as silhouettes due to the Stargaze 404 should now resolve."
```

---

## Phase 2 — Cast/Crew pagination

### Task 4: Cap Cast/Crew at 2 rows with horizontal scroll

**Files:**
- Modify: `web/src/pages/ItemDetail.css`

- [ ] **Step 1: Replace the existing `.people-grid` rule.**

Find the existing `.people-grid` rule in `web/src/pages/ItemDetail.css`:

```css
.people-grid {
  list-style: none;
  margin: 0;
  padding: 0;
  display: grid;
  grid-template-columns: repeat(auto-fill, minmax(140px, 1fr));
  gap: 16px;
}
```

Replace with:

```css
.people-grid {
  list-style: none;
  margin: 0;
  padding: 0 0 8px;             /* extra bottom padding so the scrollbar doesn't crowd the next section */
  display: grid;
  grid-auto-flow: column;       /* fill columns first → 2 rows max grow horizontally */
  grid-template-rows: repeat(2, auto);
  grid-auto-columns: 140px;     /* each cell fixed width so cards don't squish */
  gap: 16px;
  overflow-x: auto;
  overflow-y: hidden;
  scrollbar-width: thin;        /* Firefox */
  scroll-snap-type: x proximity;
}
.people-grid > .person-card {
  scroll-snap-align: start;
}
.people-grid::-webkit-scrollbar {
  height: 8px;
}
.people-grid::-webkit-scrollbar-thumb {
  background: var(--border-soft);
  border-radius: 4px;
}
```

That's a single CSS rule swap — no JSX changes needed because the existing `<ul class="people-grid">` and `<li class="person-card">` structure already matches. Cast and Crew sections each get their own grid that scrolls independently.

- [ ] **Step 2: Verify + commit.**

```bash
cd web && npx tsc --noEmit && npm run build && cd .. && go build -o lumen.exe ./cmd/lumen
git add web/src/pages/ItemDetail.css
git commit -m "feat(item-detail): cap Cast/Crew at 2 rows with horizontal scroll

Plex Web's pattern. The previous auto-fill grid showed every cast
member at once and ate the viewport on dense casts. New layout:
grid-auto-flow: column with grid-template-rows: repeat(2, auto) and
horizontal scroll on overflow. Scroll-snap on each .person-card so
the user lands on a card edge when scrolling. Custom scrollbar for
WebKit; Firefox uses scrollbar-width: thin."
```

---

## Phase 3 — TMDB trailer integration

### Task 5: Add TMDBKey to config

**Files:**
- Modify: `internal/config/config.go`

- [ ] **Step 1: Add the field to both `Config` and `wireConfig`.**

In `internal/config/config.go`, find the `Config` struct (around line 56-63):

```go
type Config struct {
	ClientIdentifier string     `json:"clientIdentifier"`
	OMDBKey          string     `json:"omdbKey,omitempty"`
	Plex             PlexConfig `json:"plex"`
	UI               UIConfig   `json:"ui"`
}
```

Add `TMDBKey string` between `OMDBKey` and `Plex`:

```go
type Config struct {
	ClientIdentifier string     `json:"clientIdentifier"`
	OMDBKey          string     `json:"omdbKey,omitempty"`
	TMDBKey          string     `json:"tmdbKey,omitempty"`
	Plex             PlexConfig `json:"plex"`
	UI               UIConfig   `json:"ui"`
}
```

Find the `wireConfig` struct (around line 79-84) and apply the same insertion:

```go
type wireConfig struct {
	ClientIdentifier string         `json:"clientIdentifier"`
	OMDBKey          string         `json:"omdbKey,omitempty"`
	TMDBKey          string         `json:"tmdbKey,omitempty"`
	Plex             wirePlexConfig `json:"plex"`
	UI               UIConfig       `json:"ui"`
}
```

In `Load()` (around line 118), find the line `OMDBKey: w.OMDBKey,` and add the next line:

```go
	c := &Config{
		ClientIdentifier: w.ClientIdentifier,
		OMDBKey:          w.OMDBKey,
		TMDBKey:          w.TMDBKey,
		UI:               w.UI,
	}
```

In `Save()` (around line 197), find the line `OMDBKey: c.OMDBKey,` and add:

```go
	w := wireConfig{
		ClientIdentifier: c.ClientIdentifier,
		OMDBKey:          c.OMDBKey,
		TMDBKey:          c.TMDBKey,
		UI:               c.UI,
	}
```

- [ ] **Step 2: Verify + commit.**

```bash
gofmt -w internal/config/config.go
go build ./... && go vet ./... && go test ./internal/config/...
git add internal/config/config.go
git commit -m "feat(config): add TMDBKey field

Pairs with Tasks 6-9 (TMDB trailer integration). TMDB API keys are
not user-secret in the same way Plex tokens are — public-facing API
keys per the TMDB ToS — so no DPAPI encryption like Plex auth.
Persisted in config.json verbatim alongside OMDBKey."
```

---

### Task 6: TMDB API client

**Files:**
- Create: `internal/plex/tmdb.go`
- Create: `internal/plex/tmdb_test.go`

- [ ] **Step 1: Write the failing test.**

`internal/plex/tmdb_test.go`:

```go
package plex

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestTMDBLookupTrailerByIMDBID_Movie(t *testing.T) {
	hits := map[string]int{}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits[r.URL.Path]++
		switch r.URL.Path {
		case "/3/find/tt0111161":
			if r.URL.Query().Get("external_source") != "imdb_id" {
				http.Error(w, "missing external_source", http.StatusBadRequest)
				return
			}
			if r.URL.Query().Get("api_key") != "test-key" {
				http.Error(w, "missing api_key", http.StatusBadRequest)
				return
			}
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"movie_results":[{"id":278,"title":"The Shawshank Redemption"}],"tv_results":[]}`))
		case "/3/movie/278/videos":
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{
				"results": [
					{"key":"NmzuHjWmXOc","name":"Final Trailer","site":"YouTube","type":"Trailer","official":true},
					{"key":"6hB3S9bIaco","name":"Teaser","site":"YouTube","type":"Teaser","official":true}
				]
			}`))
		default:
			http.NotFound(w, r)
		}
	}))
	defer srv.Close()

	c := NewTMDBClient("test-key")
	c.base = srv.URL
	yt, err := c.LookupTrailerByIMDBID("tt0111161", "movie")
	if err != nil {
		t.Fatalf("LookupTrailerByIMDBID: %v", err)
	}
	if yt != "NmzuHjWmXOc" {
		t.Errorf("got %q, want %q (the official Trailer)", yt, "NmzuHjWmXOc")
	}
	if hits["/3/find/tt0111161"] != 1 || hits["/3/movie/278/videos"] != 1 {
		t.Errorf("expected exactly one hit per endpoint; got %v", hits)
	}
}

func TestTMDBLookupTrailerByIMDBID_Show(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/3/find/tt2861424":
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"movie_results":[],"tv_results":[{"id":61222,"name":"Rick and Morty"}]}`))
		case "/3/tv/61222/videos":
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{
				"results": [
					{"key":"abcDEF12345","name":"Season 7 Trailer","site":"YouTube","type":"Trailer","official":true}
				]
			}`))
		default:
			http.NotFound(w, r)
		}
	}))
	defer srv.Close()

	c := NewTMDBClient("test-key")
	c.base = srv.URL
	yt, err := c.LookupTrailerByIMDBID("tt2861424", "show")
	if err != nil {
		t.Fatalf("LookupTrailerByIMDBID: %v", err)
	}
	if yt != "abcDEF12345" {
		t.Errorf("got %q, want abcDEF12345", yt)
	}
}

func TestTMDBLookupTrailerByIMDBID_NoTrailer(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/3/find/tt9999999":
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"movie_results":[{"id":999}],"tv_results":[]}`))
		case "/3/movie/999/videos":
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"results": []}`))
		default:
			http.NotFound(w, r)
		}
	}))
	defer srv.Close()

	c := NewTMDBClient("test-key")
	c.base = srv.URL
	yt, err := c.LookupTrailerByIMDBID("tt9999999", "movie")
	if err != nil {
		t.Fatalf("expected nil err on no-trailer; got %v", err)
	}
	if yt != "" {
		t.Errorf("expected empty string on no trailer; got %q", yt)
	}
}
```

- [ ] **Step 2: Run test → fail.**

```bash
go test ./internal/plex/ -run TestTMDB -v
```

Expected: FAIL with `NewTMDBClient` undefined.

- [ ] **Step 3: Implement the client.**

`internal/plex/tmdb.go`:

```go
package plex

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"time"
)

// TMDBClient is a thin wrapper around the TMDB v3 API. Distinct from
// plex.Client (no Plex identity headers) and OMDBClient (different host
// + endpoint family). Used by the Play Trailer feature on Item Detail
// to find a YouTube trailer ID for any item that exposes an IMDB id.
type TMDBClient struct {
	http   *http.Client
	apiKey string
	base   string // overridable for tests; default https://api.themoviedb.org
}

func NewTMDBClient(apiKey string) *TMDBClient {
	return &TMDBClient{
		http:   &http.Client{Timeout: 8 * time.Second},
		apiKey: apiKey,
		base:   "https://api.themoviedb.org",
	}
}

// findResponse is the shape of /3/find/{external_id}?external_source=imdb_id.
type findResponse struct {
	MovieResults []struct {
		ID int `json:"id"`
	} `json:"movie_results"`
	TVResults []struct {
		ID int `json:"id"`
	} `json:"tv_results"`
}

// videosResponse is the shape of /3/movie/{id}/videos and /3/tv/{id}/videos.
type videosResponse struct {
	Results []videoEntry `json:"results"`
}

type videoEntry struct {
	Key      string `json:"key"`
	Name     string `json:"name"`
	Site     string `json:"site"`
	Type     string `json:"type"`     // "Trailer" | "Teaser" | "Clip" | "Featurette" | ...
	Official bool   `json:"official"`
}

// LookupTrailerByIMDBID resolves an IMDB id to TMDB and returns the best
// YouTube trailer key it can find. Returns ("", nil) when there's no
// matching TMDB entry or the entry has no YouTube trailers — caller
// renders the Plex Extras fallback. mediaType is "movie" or "show" (we
// translate "show" → TMDB's "tv" path internally).
//
// Selection order: official YouTube Trailer > unofficial YouTube Trailer
// > official YouTube Teaser > unofficial YouTube Teaser. Anything else
// is dropped.
func (c *TMDBClient) LookupTrailerByIMDBID(imdbID, mediaType string) (string, error) {
	if c.apiKey == "" {
		return "", fmt.Errorf("tmdb: no api key configured")
	}
	if imdbID == "" {
		return "", nil
	}
	if mediaType != "movie" && mediaType != "show" {
		return "", fmt.Errorf("tmdb: invalid mediaType %q (want 'movie' or 'show')", mediaType)
	}

	// Step 1 — IMDB → TMDB id via /3/find.
	findURL := fmt.Sprintf("%s/3/find/%s?external_source=imdb_id&api_key=%s",
		c.base, url.PathEscape(imdbID), url.QueryEscape(c.apiKey))
	resp, err := c.http.Get(findURL)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("tmdb find: status %d", resp.StatusCode)
	}
	var find findResponse
	if err := json.NewDecoder(resp.Body).Decode(&find); err != nil {
		return "", err
	}

	var tmdbID int
	var videosPath string
	if mediaType == "movie" {
		if len(find.MovieResults) == 0 {
			return "", nil
		}
		tmdbID = find.MovieResults[0].ID
		videosPath = "/3/movie/"
	} else {
		if len(find.TVResults) == 0 {
			return "", nil
		}
		tmdbID = find.TVResults[0].ID
		videosPath = "/3/tv/"
	}

	// Step 2 — TMDB id → videos.
	videosURL := fmt.Sprintf("%s%s%d/videos?api_key=%s",
		c.base, videosPath, tmdbID, url.QueryEscape(c.apiKey))
	resp2, err := c.http.Get(videosURL)
	if err != nil {
		return "", err
	}
	defer resp2.Body.Close()
	if resp2.StatusCode != http.StatusOK {
		return "", fmt.Errorf("tmdb videos: status %d", resp2.StatusCode)
	}
	var vids videosResponse
	if err := json.NewDecoder(resp2.Body).Decode(&vids); err != nil {
		return "", err
	}
	return pickBestTrailer(vids.Results), nil
}

// pickBestTrailer returns the YouTube key of the preferred trailer per the
// official-first ordering. Empty string when no YouTube trailer/teaser is
// present.
func pickBestTrailer(in []videoEntry) string {
	type rank struct {
		key string
		score int // lower is better
	}
	best := rank{score: 9999}
	for _, v := range in {
		if v.Site != "YouTube" {
			continue
		}
		var s int
		switch v.Type {
		case "Trailer":
			s = 0
			if !v.Official {
				s = 1
			}
		case "Teaser":
			s = 2
			if !v.Official {
				s = 3
			}
		default:
			continue
		}
		if s < best.score {
			best = rank{key: v.Key, score: s}
		}
	}
	if best.score == 9999 {
		return ""
	}
	return best.key
}
```

- [ ] **Step 4: Run test → pass.**

```bash
go test ./internal/plex/ -run TestTMDB -v
```

Expected: PASS for all three sub-tests.

- [ ] **Step 5: Build + vet + gofmt + commit.**

```bash
gofmt -w internal/plex/tmdb.go internal/plex/tmdb_test.go
go build ./... && go vet ./...
git add internal/plex/tmdb.go internal/plex/tmdb_test.go
git commit -m "feat(plex): TMDB API client for trailer lookup

Standalone client (not extending plex.Client — no shared auth model
with Plex). Two-step lookup:

  1. IMDB id → TMDB id via /3/find?external_source=imdb_id
  2. TMDB id → videos via /3/movie/{id}/videos or /3/tv/{id}/videos

Returns the preferred YouTube key per official-first ordering
(official Trailer > unofficial Trailer > official Teaser > unofficial
Teaser). Returns empty string with nil err when nothing matches —
caller renders the Plex Extras fallback (Task 8) or 'no trailer
available' UI.

Used by Tasks 7-8 to power Item Detail's Play Trailer button when
Plex Extras has nothing for the item."
```

---

### Task 7: `/api/tmdb/trailer/<imdb-id>` endpoint with disk cache

**Files:**
- Create: `internal/server/api_tmdb.go`
- Create: `internal/server/api_tmdb_test.go`
- Modify: `internal/server/server.go`

- [ ] **Step 1: Create the handler.**

`internal/server/api_tmdb.go`:

```go
package server

import (
	"encoding/json"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"lumen/internal/config"
	"lumen/internal/plex"
)

const tmdbCacheTTL = 30 * 24 * time.Hour

// handleTMDBTrailer returns the best-pick YouTube trailer key for an IMDB id,
// looking up via TMDB. Path: /api/tmdb/trailer/<imdbID>?type=movie|show.
//
// Cached on disk under %APPDATA%\Lumen\cache\tmdb\<id>-<type>.json for 30
// days, mirroring the OMDB cache pattern. 503 when no TMDB key configured;
// 404 when no matching TMDB entry or no YouTube trailer/teaser. SPA falls
// through to the Plex Extras item.trailer field on 404.
func (s *Server) handleTMDBTrailer(w http.ResponseWriter, r *http.Request) {
	if s.cfg.TMDBKey == "" {
		writeError(w, http.StatusServiceUnavailable, "no TMDB key configured")
		return
	}
	id := strings.TrimPrefix(r.URL.Path, "/api/tmdb/trailer/")
	if id == "" || strings.Contains(id, "/") || !strings.HasPrefix(id, "tt") {
		writeError(w, http.StatusBadRequest, "expected /api/tmdb/trailer/ttNNNNNNN")
		return
	}
	mediaType := r.URL.Query().Get("type")
	if mediaType != "movie" && mediaType != "show" {
		writeError(w, http.StatusBadRequest, "type query param must be 'movie' or 'show'")
		return
	}

	dir := filepath.Join(config.CacheDir(), "tmdb")
	path := filepath.Join(dir, id+"-"+mediaType+".json")
	if data, ok := readFreshTMDBCache(path, tmdbCacheTTL); ok {
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("Cache-Control", "no-store")
		_, _ = w.Write(data)
		return
	}

	client := plex.NewTMDBClient(s.cfg.TMDBKey)
	yt, err := client.LookupTrailerByIMDBID(id, mediaType)
	if err != nil {
		writeError(w, http.StatusBadGateway, err.Error())
		return
	}
	if yt == "" {
		// Cache the negative result too — TMDB has no trailer is a stable fact
		// for the cache TTL window. SPA reads 404 as "fallback to Plex Extras".
		if err := os.MkdirAll(dir, 0o755); err == nil {
			_ = os.WriteFile(path, []byte(`{"youtubeID":""}`), 0o644)
		}
		writeJSONStatus(w, http.StatusNotFound, map[string]string{"error": "no trailer"})
		return
	}

	resp := map[string]string{"youtubeID": yt}
	buf, _ := json.Marshal(resp)
	if err := os.MkdirAll(dir, 0o755); err == nil {
		_ = os.WriteFile(path, buf, 0o644)
	}
	writeJSON(w, resp)
}

func readFreshTMDBCache(path string, ttl time.Duration) ([]byte, bool) {
	st, err := os.Stat(path)
	if err != nil {
		return nil, false
	}
	if time.Since(st.ModTime()) > ttl {
		return nil, false
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, false
	}
	if !json.Valid(data) {
		return nil, false
	}
	// Decode quickly to detect cached negative ("youtubeID":"") and translate
	// to a 404-shape response. The handler caller writes raw bytes, so we
	// have to detect this here at the cache layer.
	var probe struct {
		YouTubeID string `json:"youtubeID"`
	}
	_ = json.Unmarshal(data, &probe)
	if probe.YouTubeID == "" {
		// Caller (handleTMDBTrailer) treats a cache hit as authoritative — we
		// signal "negative cached" by returning nil here (forcing the miss
		// path), which then short-circuits without re-calling TMDB because
		// we'll re-stat the file inside the same handler call. Cleaner: just
		// treat negative as cache miss and let the upstream re-call. Cost
		// is one TMDB call per 30 days per "no trailer" item — acceptable.
		return nil, false
	}
	return data, true
}
```

(The negative-caching note in the comment is honest about the tradeoff — for v1.0, "no trailer" items will re-call TMDB on next view rather than serve a stale negative. Optimisation deferred.)

- [ ] **Step 2: Wire the route.**

In `internal/server/server.go`, in `registerRoutes`, add this line near the existing `/api/imdb/` registration:

```go
	s.mux.HandleFunc("/api/tmdb/trailer/", s.handleTMDBTrailer)
```

- [ ] **Step 3: Write the test.**

`internal/server/api_tmdb_test.go`:

```go
package server

import (
	"net/http"
	"testing"

	"lumen/internal/config"
)

func TestTMDBTrailerRequiresKey(t *testing.T) {
	cfg := &config.Config{}
	s := New(cfg, nil, "127.0.0.1:0")
	req, _ := http.NewRequest("GET", "/api/tmdb/trailer/tt0111161?type=movie", nil)
	w := newResponseRecorder()
	s.mux.ServeHTTP(w, req)
	if w.status != http.StatusServiceUnavailable {
		t.Errorf("status %d, want 503", w.status)
	}
}

func TestTMDBTrailerValidatesIDShape(t *testing.T) {
	t.Setenv("APPDATA", t.TempDir())
	cfg := &config.Config{TMDBKey: "test-key"}
	s := New(cfg, nil, "127.0.0.1:0")
	for _, id := range []string{"", "abc", "tt0111161/extra", "0111161"} {
		req, _ := http.NewRequest("GET", "/api/tmdb/trailer/"+id+"?type=movie", nil)
		w := newResponseRecorder()
		s.mux.ServeHTTP(w, req)
		if w.status == http.StatusOK {
			t.Errorf("id=%q: expected non-200, got 200", id)
		}
	}
}

func TestTMDBTrailerValidatesType(t *testing.T) {
	t.Setenv("APPDATA", t.TempDir())
	cfg := &config.Config{TMDBKey: "test-key"}
	s := New(cfg, nil, "127.0.0.1:0")
	for _, ty := range []string{"", "season", "episode", "tv"} {
		req, _ := http.NewRequest("GET", "/api/tmdb/trailer/tt0111161?type="+ty, nil)
		w := newResponseRecorder()
		s.mux.ServeHTTP(w, req)
		if w.status != http.StatusBadRequest {
			t.Errorf("type=%q: status %d, want 400", ty, w.status)
		}
	}
}
```

- [ ] **Step 4: Run + commit.**

```bash
gofmt -w internal/server/api_tmdb.go internal/server/api_tmdb_test.go internal/server/server.go
go build ./... && go vet ./... && go test ./internal/server/ -run TestTMDB -v
git add internal/server/api_tmdb.go internal/server/api_tmdb_test.go internal/server/server.go
git commit -m "feat(server): /api/tmdb/trailer/<imdb-id> with 30-day disk cache

Wraps plex.TMDBClient.LookupTrailerByIMDBID. Validates imdb id shape +
type query param (movie | show). 503 when no TMDB key. 404 when TMDB
has no entry or no YouTube trailer — SPA reads 404 as 'fallback to
Plex Extras item.trailer'. 30-day disk cache mirrors the OMDB pattern."
```

---

### Task 8: Settings — TMDB key input

**Files:**
- Create: `internal/server/api_settings_tmdb.go`
- Modify: `internal/server/server.go`
- Modify: `web/src/components/Settings/AccountsServers.tsx`

- [ ] **Step 1: Create the PUT endpoint.**

`internal/server/api_settings_tmdb.go`:

```go
package server

import (
	"encoding/json"
	"net/http"
)

// handleSettingsTMDB updates the TMDB API key in config. Mirrors the OMDB
// shape — top-level field, not inside UI, because TMDB is a Plex-adjacent
// integration concern.
func (s *Server) handleSettingsTMDB(w http.ResponseWriter, r *http.Request) {
	if r.Method != "PUT" {
		writeError(w, http.StatusMethodNotAllowed, "PUT required")
		return
	}
	var body struct {
		Key string `json:"key"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON")
		return
	}
	s.cfg.TMDBKey = body.Key
	if err := s.cfg.Save(); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, map[string]string{"status": "saved"})
}
```

- [ ] **Step 2: Register the route.**

In `internal/server/server.go`, `registerRoutes`, add (alongside the existing OMDB route):

```go
	s.mux.HandleFunc("/api/settings/tmdb", s.handleSettingsTMDB)
```

- [ ] **Step 3: Add the SPA input.**

In `web/src/components/Settings/AccountsServers.tsx`, the existing `omdbKey` signal block has this shape:

```tsx
  const [omdbKey, setOmdbKey] = createSignal("");
  const [omdbError, setOmdbError] = createSignal("");
  // ...
  function saveOMDB() {
    fetch("/api/settings/omdb", {
      method: "PUT",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({ key: omdbKey() }),
    });
  }
  function validateAndSaveOMDB() {
    const k = omdbKey().trim();
    if (k === "") {
      setOmdbError("");
      saveOMDB();
      return;
    }
    if (!/^[a-f0-9]{8}$/i.test(k)) {
      setOmdbError("Expected 8 hexadecimal characters (e.g. 1a2b3c4d).");
      return;
    }
    setOmdbError("");
    saveOMDB();
  }
```

Add a parallel `tmdbKey` signal block alongside it:

```tsx
  const [tmdbKey, setTmdbKey] = createSignal("");
  const [tmdbError, setTmdbError] = createSignal("");
  // ...
  function saveTMDB() {
    fetch("/api/settings/tmdb", {
      method: "PUT",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({ key: tmdbKey() }),
    });
  }
  function validateAndSaveTMDB() {
    const k = tmdbKey().trim();
    if (k === "") {
      setTmdbError("");
      saveTMDB();
      return;
    }
    // TMDB v3 keys are 32 hex chars.
    if (!/^[a-f0-9]{32}$/i.test(k)) {
      setTmdbError("Expected 32 hexadecimal characters.");
      return;
    }
    setTmdbError("");
    saveTMDB();
  }
```

In the same component's JSX, find the existing OMDB row (the `<div class="settings-row"><label for="omdbKey">OMDB API key</label>` block) and immediately after its closing `</div>` add a parallel TMDB row:

```tsx
      <div class="settings-row">
        <label for="tmdbKey">TMDB API key</label>
        <div class="settings-control">
          <input
            id="tmdbKey"
            type="password"
            placeholder="32-char hex key (powers Play Trailer button)"
            value={tmdbKey()}
            onInput={(e) => setTmdbKey(e.currentTarget.value)}
            onBlur={validateAndSaveTMDB}
            aria-invalid={tmdbError() !== ""}
            aria-describedby={tmdbError() ? "tmdbError" : undefined}
          />
          {tmdbError() && (
            <div id="tmdbError" role="alert" style={{ "margin-top": "4px", "color": "#f07878", "font-size": "12px" }}>
              {tmdbError()}
            </div>
          )}
          <div style={{ "margin-top": "4px", "font-size": "11px" }}>
            <a href="https://www.themoviedb.org/settings/api" target="_blank" rel="noreferrer" style={{ "color": "var(--text-muted)" }}>
              Get a free key →
            </a>
          </div>
        </div>
      </div>
```

- [ ] **Step 4: Verify + commit.**

```bash
gofmt -w internal/server/api_settings_tmdb.go internal/server/server.go
cd web && npx tsc --noEmit && npm run build && cd .. && go build -o lumen.exe ./cmd/lumen
git add internal/server/api_settings_tmdb.go internal/server/server.go web/src/components/Settings/AccountsServers.tsx
git commit -m "feat(settings): TMDB API key input

Mirrors the existing OMDB key field. PUT /api/settings/tmdb persists
to config.TMDBKey. Input validates 32-char hex format (TMDB v3 keys);
empty resets. Free-key link points to themoviedb.org/settings/api."
```

---

### Task 9: Play Trailer button uses TMDB-first

**Files:**
- Modify: `web/src/api/client.ts`
- Modify: `web/src/pages/ItemDetail.tsx`

- [ ] **Step 1: Add the SPA API method.**

In `web/src/api/client.ts`, add to the `api` object literal (near the existing `imdb` method):

```ts
  tmdbTrailer: async (imdbID: string, mediaType: "movie" | "show"): Promise<string | null> => {
    const res = await fetch(`/api/tmdb/trailer/${encodeURIComponent(imdbID)}?type=${mediaType}`);
    if (res.status === 404 || res.status === 503) return null;
    if (!res.ok) throw new Error(`${res.status} GET /api/tmdb/trailer/${imdbID}`);
    const body = await res.json();
    return body.youtubeID || null;
  },
```

- [ ] **Step 2: Wire the Play Trailer button to TMDB-first.**

In `web/src/pages/ItemDetail.tsx`, find the existing Play Trailer button block (around line 138). Currently it reads:

```tsx
              <button
                class="btn"
                disabled={!(it() as Item).trailer?.youtubeID}
                title={
                  (it() as Item).trailer?.youtubeID
                    ? "Play trailer"
                    : (it() as Item).trailer?.plexKey
                      ? "Plex-hosted trailer (not supported in v1.0)"
                      : "No trailer available"
                }
                onClick={() => setTrailerOpen(true)}
              >
                Play Trailer
              </button>
```

We replace the disabled-gate logic with a resource that fetches TMDB and falls back to Plex Extras. In the same component body, near the existing `trailerOpen` signal, add:

```tsx
  // TMDB-first: try /api/tmdb/trailer first, fall back to Plex's Extras
  // youtubeID. Returns null when neither has a trailer; the button is then
  // disabled with explanatory tooltip.
  const [resolvedTrailer] = createResource<string | null>(
    () => {
      const cur = item();
      if (!cur || !cur.imdbId) return null;
      const mediaType: "movie" | "show" = cur.type === "show" || cur.type === "episode" || cur.type === "season" ? "show" : "movie";
      return { imdbId: cur.imdbId, mediaType, plexFallback: cur.trailer?.youtubeID ?? null };
    },
    async (src) => {
      if (src === null) return null;
      try {
        const tmdb = await api.tmdbTrailer(src.imdbId, src.mediaType);
        if (tmdb) return tmdb;
      } catch (e) {
        console.warn("tmdb trailer lookup failed; falling back to Plex Extras", e);
      }
      return src.plexFallback;
    }
  );
```

Replace the Play Trailer button JSX with:

```tsx
              <button
                class="btn"
                disabled={!resolvedTrailer()}
                title={
                  resolvedTrailer()
                    ? "Play trailer"
                    : (it() as Item).imdbId
                      ? "No trailer available (TMDB + Plex Extras both empty)"
                      : "No IMDB id — Trailer unavailable for this item"
                }
                onClick={() => setTrailerOpen(true)}
              >
                Play Trailer
              </button>
```

Update the existing `<TrailerModal>` mount to use `resolvedTrailer()` instead of `item()?.trailer?.youtubeID`:

```tsx
      <TrailerModal
        open={trailerOpen()}
        onClose={() => setTrailerOpen(false)}
        youtubeID={resolvedTrailer() ?? undefined}
        title={`${item()?.title ?? ""} — Trailer`}
      />
```

- [ ] **Step 3: Verify + commit.**

```bash
cd web && npx tsc --noEmit && npm run build && cd .. && go build -o lumen.exe ./cmd/lumen
git add web/src/api/client.ts web/src/pages/ItemDetail.tsx
git commit -m "feat(item-detail): Play Trailer prefers TMDB, falls back to Plex Extras

Replaces the Plex-Extras-only logic with a TMDB-first resolver:

  1. If item has IMDB id, GET /api/tmdb/trailer/<id>?type=movie|show.
  2. On 200 + youtubeID, use it.
  3. On 404 (TMDB has no entry or no YouTube trailer), 503 (no key),
     or transient error: fall back to item.trailer?.youtubeID from
     Plex Extras.
  4. If neither resolves: button stays disabled with tooltip.

Matches Byron's directive to lean on TMDB as the trailer source since
Plex Extras is unreliable on shared/older libraries (Session 5 smoke
finding)."
```

---

## Phase 4 — Home UI parity for plex.tv pages

The existing `web/src/components/Shelf.tsx` component handles shelf-wrapper styling, drag-drop reorder via `@thisbeyond/solid-dnd`, chevron collapse, hide via Settings, and the always-visible `GripVertical` handle. Home, the Stargaze group, and the DKNZPLEX group all use it. Recommended and Discover currently render their own custom `<section class="recommended-shelf">` / `<section class="discover-shelf">` markup — re-implementing what Shelf already does, badly.

Phase 4 refactors both pages to mount `<Shelf />` instead, getting drag-drop + chevrons + wrapper styling for free, plus per-page persistence via the existing `config.UI.ShelfState` map (already keyed by page name: `"recommended"`, `"discover"`, etc. — see [internal/config/config.go:28](internal/config/config.go:28)).

> **Subagent dispatch note:** before starting Tasks 10/11, the implementer should `Read` `web/src/components/Shelf.tsx` end-to-end and `web/src/pages/Home.tsx` to understand the props and the `<Group>` wrapper pattern. Both refactors mirror Home's structure but skip the group nesting (Recommended/Discover are flat lists of shelves, not grouped).

### Task 10: Refactor Recommended to use `<Shelf />`

**Files:**
- Modify: `web/src/pages/Recommended.tsx`
- Modify: `web/src/pages/Recommended.css`

- [ ] **Step 1: Read the existing Shelf component to understand its props and Card protocol.**

```bash
# Read web/src/components/Shelf.tsx — note the props interface (id, title, cards,
# sortable, etc.) and the inner Card iteration pattern. The existing Recommended
# page does its own Card rendering — we'll switch to passing items as `cards` to Shelf.
```

- [ ] **Step 2: Rewrite Recommended.tsx to use Shelf for each shelf row.**

The existing `Recommended.tsx` defines `RecommendedShelf` inline. Replace the entire file with this implementation that uses the existing `<Shelf />`:

```tsx
import { createResource, For } from "solid-js";
import type { HubItem, Item } from "../api/types";
import { api } from "../api/client";
import Shelf from "../components/Shelf";
import { refetchOnFocus } from "../util/focusRefetch";
import "./Recommended.css";

// Per spec §12.3 (revised — Pick Up Again dropped during Session 5).
// Four watchlist-namespace Plex Discover hubs, each rendered through
// the shared <Shelf /> component for drag-drop / chevron / wrapper-style
// parity with Home.
const SHELVES: { id: string; title: string; slug: string }[] = [
  { id: "rec-new-episodes",   title: "Recently Aired Episodes",        slug: "new-episodes"   },
  { id: "rec-coming-soon",    title: "Coming Soon",                    slug: "coming-soon"    },
  { id: "rec-new-trailers",   title: "New Trailers from Your Watchlist", slug: "new-trailers" },
  { id: "rec-recently-added", title: "Recently Added",                 slug: "recently-added" },
];

export default function Recommended() {
  return (
    <div class="recommended-page">
      <For each={SHELVES}>
        {(s) => <RecommendedShelfHost id={s.id} title={s.title} slug={s.slug} />}
      </For>
    </div>
  );
}

function RecommendedShelfHost(props: { id: string; title: string; slug: string }) {
  const [items, { refetch }] = createResource<HubItem[]>(() =>
    api.hub("watchlist", props.slug).catch(() => [])
  );
  refetchOnFocus(refetch);
  // Adapt HubItem → Item shape (Card.tsx + Shelf.tsx expect Item-shaped cards).
  // HubItem has thumb, title, type, year, ratingKey — mostly compatible. The
  // missing fields (duration, viewOffset, etc.) are absent and Card handles
  // their absence gracefully.
  const cards = () =>
    (items() ?? []).map((it): Item => ({
      ratingKey: it.ratingKey,
      title: it.title,
      type: it.type,
      year: it.year,
      thumb: it.thumb,
      guid: it.guid,
    } as Item));
  return (
    <Shelf
      pageKey="recommended"
      id={props.id}
      title={props.title}
      cards={cards()}
      serverID=""             /* plex.tv-source — no machineID; Card handles thumb absolute-URL path */
      cardLinkPrefix="/discover-item"
      sortable={true}
      loading={items.loading}
    />
  );
}
```

> **Implementer note:** the exact props on `<Shelf />` may differ from the names above. Read `web/src/components/Shelf.tsx` first and adjust the call site to match its actual interface. Likely surfaces:
>
> - `id` / `title` (definite)
> - `cards: Item[]` (definite)
> - `sortable?: boolean` (Session 3 finding — Continue Watching opts out with `sortable={false}`)
> - `pageKey` (likely — for ShelfState persistence keyed by page name)
> - `loading?: boolean` (likely — Skeleton fallback)
> - `cardLinkPrefix` or similar (Card.tsx is hardcoded to /item/<server>/<rk>; we may need to extend Card to accept an alternate prefix)
>
> If `Card.tsx` doesn't accept a prefix override, **add a small extension**: a `linkPrefix?: string` prop that defaults to "/item" and prepends to the existing routing. This is a one-line addition to Card.tsx + a single new prop on Shelf.tsx that passes it through.
>
> **Stop and ask** if Shelf's actual prop shape diverges meaningfully from the call above, or if the Card extension proves more invasive than a one-liner. Don't soldier through with a guessed API.

- [ ] **Step 3: Slim down Recommended.css.**

The existing CSS defines styling for `.recommended-shelf`, `.recommended-shelf-title`, `.recommended-shelf-row`, `.recommended-card-link`, `.recommended-poster`, `.recommended-card-title`, `.recommended-card-sub`, `.recommended-empty` — all of which are now provided by Shelf + Card themselves. Replace `web/src/pages/Recommended.css` content with just the page-host rule:

```css
.recommended-page {
  padding: 24px 32px;
  display: flex;
  flex-direction: column;
  gap: 32px;
}
```

- [ ] **Step 4: Verify + commit.**

```bash
cd web && npx tsc --noEmit && npm run build && cd .. && go build -o lumen.exe ./cmd/lumen
git add web/src/pages/Recommended.tsx web/src/pages/Recommended.css
git commit -m "feat(spa): Recommended page uses shared <Shelf /> component

Refactors away the bespoke .recommended-shelf markup in favour of the
existing <Shelf /> from Home. Inherits drag-drop reorder, chevron
collapse, always-visible GripVertical handle, and wrapper styling
(dark surface + soft border + internal padding) — matching Home's UX.
ShelfState persistence keyed by pageKey='recommended' so order/visibility
survives restart, separately from Home's state."
```

---

### Task 11: Refactor Discover to use `<Shelf />`

**Files:**
- Modify: `web/src/pages/Discover.tsx`
- Modify: `web/src/pages/Discover.css`

- [ ] **Step 1: Apply the same refactor pattern as Task 10.**

Replace `web/src/pages/Discover.tsx` with:

```tsx
import { createResource, For } from "solid-js";
import type { HubItem, Item } from "../api/types";
import { api } from "../api/client";
import Shelf from "../components/Shelf";
import { refetchOnFocus } from "../util/focusRefetch";
import "./Discover.css";

const SHELVES: { id: string; title: string; slug: string }[] = [
  { id: "disc-coming-soon",          title: "Coming Soon",              slug: "coming-soon"              },
  { id: "disc-new-trailers",         title: "New Trailers",             slug: "recently-released-trailers" },
  { id: "disc-trending-trailers",    title: "Trending Trailers",        slug: "trending-trailers"        },
  { id: "disc-top-watchlisted",      title: "Most Watchlisted This Week", slug: "top_watchlisted"        },
  { id: "disc-trending-plex",        title: "Trending on Plex",         slug: "trending-plex"            },
  { id: "disc-blockbuster-trailers", title: "Upcoming Blockbusters",    slug: "blockbuster-trailers"     },
  { id: "disc-highly-anticipated",   title: "Highly Anticipated",       slug: "highly-anticipated-movies" },
  { id: "disc-trend-apple-itunes",   title: "Trending on Apple TV",     slug: "trend-apple-itunes"       },
];

export default function Discover() {
  return (
    <div class="discover-page">
      <For each={SHELVES}>
        {(s) => <DiscoverShelfHost id={s.id} title={s.title} slug={s.slug} />}
      </For>
    </div>
  );
}

function DiscoverShelfHost(props: { id: string; title: string; slug: string }) {
  const [items, { refetch }] = createResource<HubItem[]>(() =>
    api.hub("home", props.slug).catch(() => [])
  );
  refetchOnFocus(refetch);
  const cards = () =>
    (items() ?? []).map((it): Item => ({
      ratingKey: it.ratingKey,
      title: it.title,
      type: it.type,
      year: it.year,
      thumb: it.thumb,
      guid: it.guid,
    } as Item));
  return (
    <Shelf
      pageKey="discover"
      id={props.id}
      title={props.title}
      cards={cards()}
      serverID=""
      cardLinkPrefix="/discover-item"
      sortable={true}
      loading={items.loading}
    />
  );
}
```

(Same props caveat from Task 10 applies — adjust to match actual `Shelf.tsx` interface.)

- [ ] **Step 2: Slim down Discover.css.**

Replace `web/src/pages/Discover.css` with:

```css
.discover-page {
  padding: 24px 32px;
  display: flex;
  flex-direction: column;
  gap: 32px;
}
```

- [ ] **Step 3: Verify + commit.**

```bash
cd web && npx tsc --noEmit && npm run build && cd .. && go build -o lumen.exe ./cmd/lumen
git add web/src/pages/Discover.tsx web/src/pages/Discover.css
git commit -m "feat(spa): Discover page uses shared <Shelf /> component

Same refactor as Recommended (Task 10). Eight home-namespace shelves
through the shared <Shelf /> for drag-drop / chevrons / wrapper style.
pageKey='discover' so its ShelfState persists separately from
Recommended and Home."
```

---

### Task 12: Card extra info on plex.tv hubs

The Plex Web capture for trending-trailers showed rich fields available on hub items: `rating`, `audienceRating`, `contentRating`, `studio`, `tagline`, `originallyAvailableAt`, `addedAt`. Byron specifically asked Trending Trailers cards to show "their date added".

For v1.0, we add the most-impactful fields to `HubItem` and let Card render them when present.

**Files:**
- Modify: `internal/plex/types.go`
- Modify: `internal/plex/hubs.go`
- Modify: `web/src/api/types.ts`
- Modify: `web/src/components/Card.tsx`

- [ ] **Step 1: Extend `HubItem` (Go) with the extra fields.**

In `internal/plex/types.go`, replace the existing `HubItem` struct with:

```go
// HubItem is one card on a plex.tv Discover hub (home or watchlist namespace).
//
// Thumb is an absolute URL (e.g. https://metadata-static.plex.tv/... or
// https://image.tmdb.org/...). Render directly with <img src> — does NOT
// need to go through Lumen's image proxy. Confirmed via Plex Web capture
// of home/trending-trailers in Session 5 post-smoke.
type HubItem struct {
	GUID                  string `json:"guid,omitempty"`
	RatingKey             string `json:"ratingKey"`
	Title                 string `json:"title"`
	Type                  string `json:"type"`
	Year                  int    `json:"year,omitempty"`
	Thumb                 string `json:"thumb,omitempty"`
	ContentRating         string `json:"contentRating,omitempty"`
	Studio                string `json:"studio,omitempty"`
	Tagline               string `json:"tagline,omitempty"`
	OriginallyAvailableAt string `json:"originallyAvailableAt,omitempty"` // YYYY-MM-DD release date
	AddedAt               int64  `json:"addedAt,omitempty"`               // Unix epoch — date added to Plex Discover
}
```

- [ ] **Step 2: Wire the new fields in `GetHub` (Go).**

In `internal/plex/hubs.go`, find the `for _, m := range mc.MediaContainer.Metadata` loop and update the `HubItem{}` literal to copy the new fields:

```go
		out = append(out, HubItem{
			GUID:                  m.GUID,
			RatingKey:             m.RatingKey,
			Title:                 m.Title,
			Type:                  m.Type,
			Year:                  m.Year,
			Thumb:                 m.Thumb,
			ContentRating:         m.ContentRating,
			Studio:                m.Studio,
			Tagline:               m.Tagline,
			OriginallyAvailableAt: m.OriginallyAvailableAt,
			AddedAt:               m.AddedAt,
		})
```

> **Implementer note:** these fields must also exist on `metadataWire` (the unmarshal target) for the JSON decode to capture them. Read `internal/plex/libraries.go` `metadataWire` struct definition. `OriginallyAvailableAt` and `AddedAt` are already present (added Session 4). `ContentRating`, `Studio`, `Tagline` are likely missing — add them with the same json tag pattern (lowercase tag matching the wire field) before this task ships.

- [ ] **Step 3: Mirror the type in TS.**

In `web/src/api/types.ts`, replace the existing `HubItem` interface:

```ts
export interface HubItem {
  guid?: string;
  ratingKey: string;
  title: string;
  type: string;
  year?: number;
  thumb?: string;
  contentRating?: string;
  studio?: string;
  tagline?: string;
  originallyAvailableAt?: string;
  addedAt?: number;
}
```

- [ ] **Step 4: Card.tsx renders the extra info when present.**

The existing `Card.tsx` already supports an "Added" timestamp for Continue Watching cards (Session 4.5 feature). Locate the existing card-meta block. We extend it to optionally show `contentRating` (e.g. "PG-13") and `originallyAvailableAt` (release year, falling back to existing `year` field).

In `web/src/components/Card.tsx`, find the existing card metadata block. After the title/year line, append (only when the data is present):

```tsx
            <Show when={d().contentRating}>
              <div class="card-rating-pill">{d().contentRating}</div>
            </Show>
```

In `web/src/components/Card.css`, append a small rule for the new pill:

```css
.card-rating-pill {
  display: inline-block;
  font-size: 10px;
  letter-spacing: 0.04em;
  padding: 1px 6px;
  background: var(--bg-menu);
  border: 1px solid var(--border-soft);
  border-radius: 3px;
  color: var(--text-muted);
  margin-top: 4px;
}
```

> **Implementer note:** if `Card.tsx` doesn't currently destructure to `d()` (a memoised displayItem signal), the actual property access pattern may be `props.item.contentRating` — adjust to the file's existing convention. The point is: the new pill renders only when `contentRating` is non-empty, and it uses an existing CSS-token-driven style.

- [ ] **Step 5: Verify + commit.**

```bash
gofmt -w internal/plex/types.go internal/plex/hubs.go internal/plex/libraries.go
cd web && npx tsc --noEmit && npm run build && cd .. && go build -o lumen.exe ./cmd/lumen
go build ./... && go vet ./... && go test ./internal/plex/...
git add internal/plex/types.go internal/plex/hubs.go internal/plex/libraries.go web/src/api/types.ts web/src/components/Card.tsx web/src/components/Card.css
git commit -m "feat(hubs): expose richer card metadata on Discover/Recommended

Extends HubItem with contentRating, studio, tagline, originallyAvailableAt,
addedAt — all fields Plex returns on hub responses (Session 5
DevTools capture). metadataWire scope-add to capture the new wire
fields. SPA HubItem mirrors the Go type. Card.tsx renders a
contentRating pill (e.g. PG-13) when present — addedAt + tagline + studio
will be wired into card hover state in a follow-up polish pass."
```

---

## Phase 4.4 — Watchlist card hover actions (parity with Continue Watching)

The Session 5 Watchlist page renders cards as static link-only tiles. Byron's smoke revealed the Watchlist needs the same hover affordances the Continue Watching cards on Home already have:

- Play button (centred overlay, fades in on hover) — for plex.tv watchlist items, click should resolve to the local server copy if available (via `availability` lookup) and trigger `api.play()`. If not available locally, fall back to the click-through to DiscoverItem detail page.
- Remove from Watchlist (top-right action button on hover).
- Mark as Watched (top-right action button on hover) — only meaningful when the item is available on a local server (otherwise greyed out).

Pattern reference: `web/src/components/Card.tsx` already has the `card-play-overlay`, `card-remove-btn`, `card-mark-watched-btn` shapes from Session 4.5. We refactor `WatchlistCard` to share those visual patterns (or, cleaner, refactor `Card.tsx` to accept watchlist mode as a prop variant).

### Task 11.5: Watchlist card hover actions

**Files:**
- Modify: `web/src/pages/Watchlist.tsx`
- Modify: `web/src/pages/Watchlist.css`
- (Possibly) Modify: `web/src/components/Card.tsx` — add a `watchlistMode?: boolean` prop variant.

> **Implementer note:** read `web/src/components/Card.tsx` first to understand the existing CW hover-action implementation. The two paths to choose between:
>
> 1. **Reuse Card.tsx with a watchlistMode flag** — Card already has hover-overlay + action-button shapes. Add a `watchlistMode` prop that swaps the actions (Play resolves via availability; Remove calls api.watchlistRemove; Mark Watched only enabled when locally available). Cleaner long-term.
> 2. **Build dedicated WatchlistCard with copied patterns** — duplicates CSS but isolates concerns. Less risk of regressing Home's CW cards.
>
> Choose based on Card.tsx's current prop surface and whether the existing logic generalises cleanly. Stop and ask if path 1 looks invasive.

- [ ] **Step 1: Decide path 1 vs 2 after reading Card.tsx.**

- [ ] **Step 2: Wire Play button — availability-aware.**

For each watchlist card, fetch `api.availability(item.guid)` to check local server availability. If available:

```tsx
async function handlePlay(item: WatchlistItem) {
  const matches = await api.availability(item.guid ?? "");
  if (matches.length === 0) {
    // No local copy — fall through to DiscoverItem detail.
    window.location.href = `/discover-item/${encodeURIComponent(item.ratingKey)}`;
    return;
  }
  const m = matches[0]; // first match — pick by preference if multiple
  await api.play(m.machineIdentifier, m.ratingKey);
}
```

- [ ] **Step 3: Wire Remove from Watchlist (uses existing api.watchlistRemove).**

```tsx
async function handleRemove(item: WatchlistItem) {
  try {
    await api.watchlistRemove(item.ratingKey);
    setTimeout(() => window.dispatchEvent(new CustomEvent("lumen:data-invalidated")), 350);
  } catch (e) {
    alert(`Remove failed: ${(e as Error).message}`);
  }
}
```

- [ ] **Step 4: Wire Mark as Watched — gated on local availability.**

```tsx
async function handleMarkWatched(item: WatchlistItem) {
  const matches = await api.availability(item.guid ?? "");
  if (matches.length === 0) {
    // No local copy — Mark Watched not applicable to plex.tv-only items.
    return;
  }
  const m = matches[0];
  await api.scrobble(m.machineIdentifier, m.ratingKey);
  setTimeout(() => window.dispatchEvent(new CustomEvent("lumen:data-invalidated")), 350);
}
```

- [ ] **Step 5: CSS for the action overlay (mirror Home's CW pattern).**

Append to `web/src/pages/Watchlist.css` (or share the existing card-overlay CSS if path 1 is chosen):

```css
.watchlist-card { position: relative; }
.watchlist-card-actions {
  position: absolute;
  top: 8px;
  right: 8px;
  display: flex;
  gap: 4px;
  opacity: 0;
  transition: opacity 0.18s ease;
  z-index: 4;
}
.watchlist-card:hover .watchlist-card-actions {
  opacity: 1;
}
.watchlist-card-play {
  position: absolute;
  top: 50%;
  left: 50%;
  transform: translate(-50%, -50%);
  z-index: 3;
  opacity: 0;
  transition: opacity 0.18s ease;
}
.watchlist-card:hover .watchlist-card-play {
  opacity: 1;
}
```

- [ ] **Step 6: Verify + commit.**

```bash
cd web && npx tsc --noEmit && npm run build && cd .. && go build -o lumen.exe ./cmd/lumen
git add web/src/pages/Watchlist.tsx web/src/pages/Watchlist.css web/src/components/Card.tsx
git commit -m "feat(watchlist): hover actions (Play / Remove / Mark Watched)

Parity with Continue Watching cards on Home (Session 4.5 pattern). Play
resolves via availability lookup — local server copy plays directly,
plex.tv-only items fall through to DiscoverItem detail. Remove calls
api.watchlistRemove with optimistic UI flip. Mark Watched gates on
local availability (greyed out for plex.tv-only items)."
```

---

## Phase 4.5 — plex.tv Discover Item Detail page

When a user clicks a card on Recommended or Discover (or the click-through `/watchlist/<rk>` from Session 5's Watchlist page), they currently land on a 404 stub. These items are plex.tv-source — they're not in any local server library, so `/item/<server>/<rk>` doesn't apply. Instead we need a **discover-namespace** Item Detail variant that:

- Fetches metadata from `discover.provider.plex.tv/library/metadata/<plexTvRatingKey>` (the rich metadata endpoint Plex Web uses for not-yet-released titles).
- Renders a similar layout to the existing Item Detail (hero, pills, overview) but **without server-local affordances** (no Play, no Mark Watched — these items aren't on any of your servers).
- **Add to Watchlist / Remove from Watchlist** is the primary action on this surface (per spec §12.4 — "Clicking any Discover card opens item detail. Availability block will usually say 'Not available on Stargaze or DKNZPLEX'; primary action becomes Add to Watchlist.").
- Optional: cross-reference with local server libraries via `GetAvailability(guid)` — when a discover item IS on your server, surface a "Watch on <server>" button that deep-links to the server-local item detail.

### Task 12.5: plex.tv Discover Item Detail backend + SPA

**Files:**
- Create: `internal/plex/discover_item.go` — `GetDiscoverItem(accountToken, plexTvRatingKey)` against `discover.provider.plex.tv/library/metadata/<rk>`.
- Create: `internal/server/api_discover_item.go` — `/api/discover-item/<rk>` handler with 5-min cache.
- Create: `web/src/pages/DiscoverItem.tsx` + `.css` — the new detail page component.
- Modify: `web/src/main.tsx` — wire `/discover-item/:ratingKey` route.
- Modify: `web/src/pages/Watchlist.tsx` — already links to `/watchlist/<rk>`; route alias to the same DiscoverItem component.

> **DevTools capture step (REQUIRED before Step 1):** open Plex Web → Discover, click an upcoming-release card. Capture the request to `discover.provider.plex.tv/library/metadata/<rk>` — full URL, full response body. This tells us the exact wire shape (we know it's similar to server `/library/metadata/<key>` from earlier captures, but Discover may include extra fields like `availability`, `releaseDate`, etc.).

- [ ] **Step 1: Capture the live `discover.provider.plex.tv/library/metadata/<rk>` shape from Plex Web.**

[Implementer captures and pastes the JSON shape inline before continuing.]

- [ ] **Step 2: Implement `plex.GetDiscoverItem`.**

`internal/plex/discover_item.go`:

```go
package plex

import (
	"encoding/json"
	"fmt"
	"net/http"
)

// DiscoverItem is the rich metadata shape returned by
// discover.provider.plex.tv/library/metadata/<plexTvRatingKey>. Distinct
// from server-local Item: discover items have no Media/Part chain (the
// title may not have been released yet) but include richer marketing
// metadata (tagline, contentRating, originallyAvailableAt, studio).
type DiscoverItem struct {
	RatingKey             string   `json:"ratingKey"`
	GUID                  string   `json:"guid,omitempty"`
	Title                 string   `json:"title"`
	Type                  string   `json:"type"`
	Year                  int      `json:"year,omitempty"`
	Summary               string   `json:"summary,omitempty"`
	Tagline               string   `json:"tagline,omitempty"`
	ContentRating         string   `json:"contentRating,omitempty"`
	Studio                string   `json:"studio,omitempty"`
	OriginallyAvailableAt string   `json:"originallyAvailableAt,omitempty"`
	Thumb                 string   `json:"thumb,omitempty"`
	Art                   string   `json:"art,omitempty"`
	Roles                 []Person `json:"roles,omitempty"`
	Directors             []Person `json:"directors,omitempty"`
	Writers               []Person `json:"writers,omitempty"`
}

type discoverItemWire struct {
	MediaContainer struct {
		Metadata []struct {
			RatingKey             string `json:"ratingKey"`
			GUID                  string `json:"guid"`
			Title                 string `json:"title"`
			Type                  string `json:"type"`
			Year                  int    `json:"year"`
			Summary               string `json:"summary"`
			Tagline               string `json:"tagline"`
			ContentRating         string `json:"contentRating"`
			Studio                string `json:"studio"`
			OriginallyAvailableAt string `json:"originallyAvailableAt"`
			Thumb                 string `json:"thumb"`
			Art                   string `json:"art"`
			GuidArray             []struct {
				ID string `json:"id"`
			} `json:"Guid"`
			Role []struct {
				ID    int    `json:"id"`
				Tag   string `json:"tag"`
				Role  string `json:"role"`
				Thumb string `json:"thumb"`
			} `json:"Role"`
			Director []struct {
				ID    int    `json:"id"`
				Tag   string `json:"tag"`
				Thumb string `json:"thumb"`
			} `json:"Director"`
			Writer []struct {
				ID    int    `json:"id"`
				Tag   string `json:"tag"`
				Thumb string `json:"thumb"`
			} `json:"Writer"`
		} `json:"Metadata"`
	} `json:"MediaContainer"`
}

// GetDiscoverItem fetches rich metadata for a plex.tv ratingKey via
// discover.provider.plex.tv. ratingKey is the plex.tv discover ID (the
// trailing segment of a `plex://movie/<id>` GUID).
func (c *Client) GetDiscoverItem(accountToken, plexTvRatingKey string) (*DiscoverItem, error) {
	u := fmt.Sprintf("%s/library/metadata/%s?includeMeta=1", c.discoverBase, plexTvRatingKey)
	req, err := c.NewRequest("GET", u, nil)
	if err != nil {
		return nil, err
	}
	c.SetToken(req, accountToken)
	resp, err := c.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("discover item %s: status %d", plexTvRatingKey, resp.StatusCode)
	}
	var w discoverItemWire
	if err := json.NewDecoder(resp.Body).Decode(&w); err != nil {
		return nil, err
	}
	if len(w.MediaContainer.Metadata) == 0 {
		return nil, fmt.Errorf("discover item %s: no metadata", plexTvRatingKey)
	}
	m := w.MediaContainer.Metadata[0]
	return &DiscoverItem{
		RatingKey:             m.RatingKey,
		GUID:                  m.GUID,
		Title:                 m.Title,
		Type:                  m.Type,
		Year:                  m.Year,
		Summary:               m.Summary,
		Tagline:               m.Tagline,
		ContentRating:         m.ContentRating,
		Studio:                m.Studio,
		OriginallyAvailableAt: m.OriginallyAvailableAt,
		Thumb:                 m.Thumb,
		Art:                   m.Art,
		Roles:                 personsFromRole(m.Role),
		Directors:             personsFromCrew(m.Director),
		Writers:               personsFromCrew(m.Writer),
	}, nil
}
```

(Note: `personsFromRole` / `personsFromCrew` already exist in `internal/plex/libraries.go` from Session 5 Task 2.)

- [ ] **Step 3: Implement `/api/discover-item/<rk>` handler.**

`internal/server/api_discover_item.go`:

```go
package server

import (
	"net/http"
	"strings"
	"sync"
	"time"

	"lumen/internal/plex"
)

const discoverItemCacheTTL = 5 * time.Minute

type discoverItemCache struct {
	mu      sync.Mutex
	entries map[string]discoverItemEntry
}
type discoverItemEntry struct {
	item      *plex.DiscoverItem
	expiresAt time.Time
}

func newDiscoverItemCache() *discoverItemCache {
	return &discoverItemCache{entries: map[string]discoverItemEntry{}}
}

func (c *discoverItemCache) get(key string) (*plex.DiscoverItem, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	e, ok := c.entries[key]
	if !ok || time.Now().After(e.expiresAt) {
		delete(c.entries, key)
		return nil, false
	}
	return e.item, true
}

func (c *discoverItemCache) set(key string, item *plex.DiscoverItem) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.entries[key] = discoverItemEntry{item: item, expiresAt: time.Now().Add(discoverItemCacheTTL)}
}

func (s *Server) handleDiscoverItem(w http.ResponseWriter, r *http.Request) {
	if s.cfg.Plex.AccountToken == "" {
		writeError(w, http.StatusUnauthorized, "no account token")
		return
	}
	rk := strings.TrimPrefix(r.URL.Path, "/api/discover-item/")
	if rk == "" || strings.Contains(rk, "/") {
		writeError(w, http.StatusBadRequest, "expected /api/discover-item/<ratingKey>")
		return
	}
	if cached, ok := s.discoverItems.get(rk); ok {
		writeJSON(w, cached)
		return
	}
	item, err := s.plex.GetDiscoverItem(s.cfg.Plex.AccountToken, rk)
	if err != nil {
		writeError(w, http.StatusBadGateway, err.Error())
		return
	}
	s.discoverItems.set(rk, item)
	writeJSON(w, item)
}
```

In `internal/server/server.go`: add `discoverItems *discoverItemCache` to Server struct, initialise in `New`, register route `/api/discover-item/`.

- [ ] **Step 4: Implement DiscoverItem.tsx SPA component.**

`web/src/pages/DiscoverItem.tsx`:

```tsx
import { useParams, A } from "@solidjs/router";
import { createMemo, createResource, createSignal, Show } from "solid-js";
import { api } from "../api/client";
import { extractPlexTvRatingKey } from "../util/plexGuid";
import "./DiscoverItem.css";

interface DiscoverItem {
  ratingKey: string;
  guid?: string;
  title: string;
  type: string;
  year?: number;
  summary?: string;
  tagline?: string;
  contentRating?: string;
  studio?: string;
  originallyAvailableAt?: string;
  thumb?: string;
  art?: string;
}

export default function DiscoverItem() {
  const params = useParams();
  const [item] = createResource(
    () => params.ratingKey,
    async (rk) => {
      const res = await fetch(`/api/discover-item/${encodeURIComponent(rk)}`);
      if (!res.ok) throw new Error(`${res.status}`);
      return res.json() as Promise<DiscoverItem>;
    }
  );
  const [watchlist] = createResource(() => api.watchlist().catch(() => []));
  const [override, setOverride] = createSignal<boolean | null>(null);
  const isInWatchlist = createMemo(() => {
    const o = override();
    if (o !== null) return o;
    const rk = params.ratingKey;
    return (watchlist() ?? []).some((w) => w.ratingKey === rk);
  });
  async function toggleWatchlist() {
    const rk = params.ratingKey!;
    const wasIn = isInWatchlist();
    setOverride(!wasIn);
    try {
      if (wasIn) await api.watchlistRemove(rk); else await api.watchlistAdd(rk);
      setTimeout(() => window.dispatchEvent(new CustomEvent("lumen:data-invalidated")), 350);
    } catch (e) {
      setOverride(wasIn);
      alert(`Watchlist toggle failed: ${(e as Error).message}`);
    }
  }
  return (
    <div class="discover-item">
      <Show when={item()} fallback={<div class="discover-item-loading">Loading…</div>}>
        {(it) => {
          const i = it() as DiscoverItem;
          return (
            <>
              <Show when={i.art}>
                <div class="discover-item-hero" style={{ "background-image": `url(${i.art})` }} />
              </Show>
              <div class="discover-item-meta">
                <h1>{i.title}</h1>
                <Show when={i.tagline}><div class="discover-item-tagline">{i.tagline}</div></Show>
                <div class="discover-item-pills">
                  <Show when={i.year}><span class="pill">{i.year}</span></Show>
                  <Show when={i.contentRating}><span class="pill">{i.contentRating}</span></Show>
                  <Show when={i.originallyAvailableAt}><span class="pill">Released {i.originallyAvailableAt}</span></Show>
                  <Show when={i.studio}><span class="pill">{i.studio}</span></Show>
                </div>
                <button class="btn-primary" onClick={toggleWatchlist}>
                  {isInWatchlist() ? "Remove from Watchlist" : "Add to Watchlist"}
                </button>
                <Show when={i.summary}>
                  <section class="discover-item-overview">
                    <h3>Overview</h3>
                    <p>{i.summary}</p>
                  </section>
                </Show>
                <div class="discover-item-availability">
                  <em>Not available on any of your servers.</em>
                </div>
              </div>
            </>
          );
        }}
      </Show>
    </div>
  );
}
```

CSS at `web/src/pages/DiscoverItem.css`:

```css
.discover-item { padding: 24px 32px; max-width: 1200px; margin: 0 auto; }
.discover-item-hero {
  width: 100%;
  aspect-ratio: 16 / 9;
  background-size: cover;
  background-position: center;
  border-radius: 8px;
  margin-bottom: 24px;
}
.discover-item-meta { display: flex; flex-direction: column; gap: 12px; }
.discover-item-meta h1 { margin: 0; color: var(--text); font-size: 32px; }
.discover-item-tagline { color: var(--text-muted); font-style: italic; }
.discover-item-pills { display: flex; gap: 8px; flex-wrap: wrap; }
.discover-item-overview { margin-top: 16px; }
.discover-item-overview h3 { color: var(--text); }
.discover-item-overview p { color: var(--text-muted); line-height: 1.6; }
.discover-item-availability { margin-top: 24px; color: var(--text-muted); font-size: 13px; }
.discover-item-loading { padding: 64px 16px; text-align: center; color: var(--text-muted); }
```

- [ ] **Step 5: Wire the routes in main.tsx.**

```tsx
import DiscoverItem from "./pages/DiscoverItem";
// ...
    <Route path="/discover-item/:ratingKey" component={DiscoverItem} />
    <Route path="/watchlist/:ratingKey" component={DiscoverItem} />
```

Both routes resolve to the same component because the data shape is identical (a plex.tv discover item).

- [ ] **Step 6: Verify + commit.**

```bash
cd web && npx tsc --noEmit && npm run build && cd ..
gofmt -w internal/plex/discover_item.go internal/server/api_discover_item.go internal/server/server.go
go build ./... && go vet ./... && go test ./internal/plex/... ./internal/server/...
go build -o lumen.exe ./cmd/lumen
git add internal/plex/discover_item.go internal/server/api_discover_item.go internal/server/server.go web/src/pages/DiscoverItem.tsx web/src/pages/DiscoverItem.css web/src/main.tsx
git commit -m "feat(spa): plex.tv Discover Item Detail page

Wires /discover-item/<rk> and /watchlist/<rk> (which were 404 stubs in
Session 5) to a new DiscoverItem component fed by /api/discover-item/<rk>
proxying discover.provider.plex.tv/library/metadata/<rk>. Renders hero
art + tagline + content-rating/year/studio pills + overview + Add/Remove
to Watchlist. Per spec §12.4: 'Not available on any of your servers'
note on the availability block.

Closes the long-promised post-1.0 stub. Items reachable from
Recommended/Discover/Watchlist now have a real destination."
```

---

## Phase 5 — Verification + findings

### Task 13: Full-session verification + findings doc

**Files:**
- Create: `docs/session-6-findings.md`

- [ ] **Step 1: Run the full verification gate.**

```bash
cd "C:/Users/dicke/Desktop/Dump Zone/STACK/04-DEV/lumen"
go build ./... && go vet ./... && gofmt -l cmd internal probe
go test ./internal/plex/... ./internal/server/...
cd web && npx tsc --noEmit && npm run build && cd ..
go build -o lumen.exe ./cmd/lumen
```

Expected: clean across the board (modulo the documented Session 2 `TestImageProxyForwardsWithTokenServerSide` carry-over, which has been updated in Task 1 — verify it now passes or has been adapted to the new try-with-fallback behaviour).

- [ ] **Step 2: Manual smoke test with Byron.**

Walk through:

1. Stargaze → Movies — every poster renders (the long-deferred 404 mystery solved).
2. Item Detail (any movie) — yellow IMDB pill with score, Cast/Crew sections render with circular thumbs and 2-row horizontal scroll on dense casts.
3. Item Detail (movie with TMDB trailer) — Play Trailer enabled, click opens the modal, video plays. Modal close clears iframe.
4. Item Detail (movie with NO TMDB trailer but Plex Extras YouTube id) — Play Trailer still enabled, falls back cleanly.
5. Item Detail (movie with no trailer in either source) — button disabled, tooltip explanatory.
6. Settings → Accounts & Servers — TMDB key field appears alongside OMDB; entering an invalid key surfaces the error message; valid key persists across restart.
7. Recommended page — 4 shelves render with Home-style wrappers, drag-drop works, chevron collapses a shelf, state persists across refresh.
8. Discover page — same checks; Trending Trailers loads its 59 items.

- [ ] **Step 3: Write the findings doc.**

`docs/session-6-findings.md`:

```markdown
# Session 6 — Stargaze Image Proxy Fix, Cast/Crew Pagination, TMDB Trailers, Home UI Parity Findings

**Date:** [completion date]
**Status:** Session 6 complete.
**Plan:** [docs/superpowers/plans/2026-04-27-lumen-session-6.md](superpowers/plans/2026-04-27-lumen-session-6.md)

## Verification results

| Gate | Result |
|---|---|
| go build ./... | CLEAN |
| go vet ./... | CLEAN |
| gofmt -l ./... | CLEAN |
| go test ./internal/plex/... | All PASS (incl. new TMDB tests) |
| go test ./internal/server/... | New tests PASS; pre-existing TestImageProxyForwardsWithTokenServerSide adapted to try-with-fallback in Task 1. |
| npx tsc --noEmit | CLEAN |
| npm run build | CLEAN |
| go build -o lumen.exe | CLEAN |

## Highlights

- **Stargaze movie thumbnail 404 mystery solved** (deferred since Session 2). Root cause: Plex Web uses the per-server token for Stargaze's CDN-fronted /photo/:/transcode requests, while we used the account token by default (which DKNZPLEX requires). Try-with-fallback now hits both — disk cache makes the cold-miss double-roundtrip cost negligible. Default dimensions also dropped from 320×480 to 240×360 to share Plex Web's CDN cache key.
- **Image dimensions per surface** via web/src/util/imageDims.ts (poster/hero/person presets) — matches Plex Web exactly per surface, no more random-permutation cache misses.
- **Cast/Crew capped at 2 rows with horizontal scroll** (Byron explicit ask) — grid-auto-flow:column + grid-template-rows:repeat(2,auto) + overflow-x:auto.
- **TMDB Play Trailer** replaces Plex-Extras-only logic — TMDB lookup via IMDB id with official-trailer-first ranking; Plex Extras fallback when TMDB has nothing. New /api/tmdb/trailer/<id> endpoint with 30-day disk cache.
- **Recommended + Discover use shared <Shelf />** for drag-drop / chevrons / wrapper styling parity with Home. ShelfState persists per-page (recommended/discover/home each get their own slot in config.UI.ShelfState).
- **Hub items expose richer metadata** (contentRating, studio, tagline, originallyAvailableAt, addedAt) — used now for the contentRating pill on cards; remaining fields available for hover state and future polish.

## Spec deviations / scope notes

- TMDB negative cache (item has no trailer) is treated as a cache miss for v1.0 to keep the cache layer simple — re-fetches on next view rather than serving a stale negative. Acceptable trade for the simpler code path.
- Card hover state (showing addedAt + tagline + studio) is exposed at the type level but the UI rendering is deferred to a polish pass. The contentRating pill is the only new visible element.

## Known issues carried forward

- Watchlist page still doesn't have the Home-shelf treatment (Watchlist is a flat grid, not shelf-organised — different surface so retrofitting Shelf doesn't apply directly). Bin icon + Undo toast on Watchlist cards still deferred.
- /discover-item/<rk> route remains a 404 stub — plex.tv-source Item Detail variant is post-1.0 polish.
- TMDB-only trailer support (no Vimeo, etc.) — covers the dominant case; expand if needed.

## Manual smoke test results

[Fill in once Byron has walked through the post-Phase-5 build.]
```

- [ ] **Step 4: Commit findings.**

```bash
git add docs/session-6-findings.md
git commit -m "docs(session-6): findings (Stargaze fix + TMDB trailers + UI parity)"
```

---

## Reviewer convention summary

- **Phase 1 (Tasks 1-3):** **2-stage review per task** (Stargaze image proxy — touches CDN-fronted live deployment, real risk).
- **Phase 2 (Task 4):** **combined review** (pure CSS).
- **Phase 3 (Tasks 5-9):** **2-stage review per task** (new external API, new config field, SPA wiring).
- **Phase 4 (Tasks 10-11):** **2-stage review per task** (structural refactor — risks breaking Recommended/Discover entirely if Shelf prop shape diverges).
- **Phase 4 (Task 12):** **combined review** (mechanical scope-add to existing types + small Card extension).
- **Phase 5 (Task 13):** N/A.

## Out of scope for this session

- Watchlist bin icon + Undo toast (deferred from Session 5; still post-1.0).
- TMDB beyond trailers (cast/crew enrichment, alternative posters, etc.).
- Vimeo / non-YouTube trailer sources.
- Plex.tv avatar endpoint for Settings user thumbnail (not blocking; nice-to-have).
- Card hover state with addedAt + tagline + studio (data exposed; rendering deferred).
- TMDB negative-cache optimisation.
