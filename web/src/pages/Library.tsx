import { useParams, useSearchParams } from "@solidjs/router";
import { createEffect, createResource, createSignal, For, Match, on, onCleanup, Show, Switch, untrack } from "solid-js";
import { api } from "../api/client";
import type { Item, Library as LibraryType } from "../api/types";
import Card from "../components/Card";
import Skeleton from "../components/Skeleton";
import { store as settingsStore } from "../state/settings";
import { refetchOnFocus } from "../util/focusRefetch";
import { stableArrayByKey } from "../util/stableArray";
import "./Library.css";

const SORT_OPTIONS = [
  { value: "addedAt:desc", label: "Date Added (newest)" },
  { value: "titleSort:asc", label: "Title (A→Z)" },
  { value: "originallyAvailableAt:desc", label: "Release Date (newest)" },
  { value: "rating:desc",  label: "Rating (highest)" },
  { value: "lastViewedAt:desc", label: "Last Viewed" },
];

// Plex library "type" query param: 4=episode. Empty = library default (Shows
// for TV libraries, Movies for Movies libraries). Default is "episodes"
// for TV libraries per Byron's design call — he finds episodes more useful
// at the browse level than the show roll-up view.
// Session 3: previously persisted in localStorage (LS_SORT, LS_VIEW);
// now persists to config.json via the settings store.
const VIEW_MODE_OPTIONS = [
  { value: "",  label: "Shows" },
  { value: "4", label: "Episodes" },
];

// Map between settings store value ("episodes"|"shows"|"") and Plex type filter ("4"|"")
const viewModeToFilter = (mode: string) => (mode === "episodes" ? "4" : "");
const filterToViewMode = (filter: string) => (filter === "4" ? "episodes" : "");

const PAGE_SIZE = 200;

export default function Library() {
  const params = useParams();
  // Page lives in the URL (?page=N) so the browser back button restores it
  // when returning from an item-detail page, and so refresh/share/bookmark
  // round-trips don't lose the user's place. Page 0 is represented by
  // omitting the param to keep canonical library URLs clean.
  const [searchParams, setSearchParams] = useSearchParams<{ page?: string }>();
  const page = (): number => {
    const n = parseInt(searchParams.page ?? "0", 10);
    return Number.isFinite(n) && n >= 0 ? n : 0;
  };
  const setPage = (n: number) =>
    setSearchParams({ page: n === 0 ? undefined : String(n) }, { replace: true });

  // Note: the zoom slider's effect on card sizes is handled globally in
  // state/settings.ts, which sets --card-width on :root. Library no longer
  // needs its own scoped override — the global value drives the grid.

  // Initialise from settings store; fall back to defaults if store not yet loaded.
  const [sort, setSort] = createSignal(settingsStore.settings()?.defaultSort ?? SORT_OPTIONS[0].value);
  const [viewMode, setViewMode] = createSignal(
    viewModeToFilter(settingsStore.settings()?.defaultViewMode ?? "episodes")
  );

  const [allLibs, { refetch: refetchAllLibs }] = createResource(
    () => params.serverID,
    async (serverID) => {
      try {
        return await api.libraries(serverID);
      } catch (err) {
        // Forensic log so the next intermittent stall captures itself in
        // DevTools without the user having to plan ahead — this resource
        // and items both stalling simultaneously is the failure mode behind
        // the rare "first-click skeleton" bug.
        console.error("Library: api.libraries failed", { serverID, err });
        throw err;
      }
    }
  );
  const currentLibrary = () =>
    ((allLibs() ?? []) as LibraryType[]).find((l) => l.key === params.libraryID);
  const isTVLibrary = () => currentLibrary()?.type === "show";
  const libName = () => currentLibrary()?.title ?? "";

  // All three reset-page effects use `on(..., { defer: true })` so they
  // skip the initial mount run — otherwise a back-nav from item-detail to
  // /library/:s/:l?page=4 would immediately wipe ?page=4 because the
  // sort/viewMode effects run on first mount with the settings-store
  // value and would call setPage(0). defer makes them only fire on
  // *subsequent* changes (real user interaction).

  // Reset page on library/server change.
  createEffect(on(
    () => [params.serverID, params.libraryID],
    () => setPage(0),
    { defer: true },
  ));

  // Reset page whenever sort/mode changes, and persist the new preference.
  // patch() reads settingsStore.settings() internally; without untrack the
  // effect would subscribe to the settings signal and re-fire (resetting
  // page to 0) whenever any setting elsewhere changes — including the
  // debounced PUT response that lands ~300ms after this very call. That
  // race caused the Page-2 snap-back regression (Task 29b smoke test).
  createEffect(on(sort, (s) => {
    untrack(() => {
      settingsStore.patch({ defaultSort: s });
    });
    setPage(0);
  }, { defer: true }));
  createEffect(on(viewMode, (v) => {
    untrack(() => {
      settingsStore.patch({ defaultViewMode: filterToViewMode(v) as any });
    });
    setPage(0);
  }, { defer: true }));

  // Resource value stamps the items with the server/lib they were fetched
  // against. Solid's createResource preserves the previous value during a
  // refetch — so when the user crosses between servers, params.serverID
  // flips immediately while items() still holds the *previous* server's
  // payload for ~1s. Cards rendered in that window combine the new
  // serverID with stale thumb paths (different ratingKey namespaces per
  // server) → upstream Plex CDN returns 404 → img.onError sets
  // imgFailed → placeholders cascade in the user's view. Stamping +
  // matching below makes the grid wait for fresh data instead of
  // rendering with mismatched coordinates.
  const [items, { refetch: refetchItems }] = createResource(
    () => ({ server: params.serverID!, lib: params.libraryID!, sort: sort(), page: page(), type: viewMode(), isTV: isTVLibrary() }),
    async ({ server, lib, sort, page, type, isTV }) => {
      const opts: { sort: string; start: number; size: number; filters?: Record<string, string> } = {
        sort,
        start: page * PAGE_SIZE,
        size: PAGE_SIZE + 1,
      };
      // Only apply the type filter when it's a TV library.
      if (isTV && type) opts.filters = { type };
      try {
        const list = await api.items(server, lib, opts);
        return { server, lib, items: list };
      } catch (err) {
        console.error("Library: api.items failed", { server, lib, sort, page, type, isTV, err });
        throw err;
      }
    }
  );

  // Auto-retry once on transient failures (network blip, brief Plex
  // slowness, backend connection-pool hiccup). Most "1-in-20 skeleton stall"
  // reports get masked invisibly by this; if the retry also fails the
  // error UI below surfaces it instead of leaving the user staring at an
  // infinite skeleton.
  let autoRetried = false;
  createEffect(() => {
    if (items.error && !autoRetried) {
      autoRetried = true;
      console.warn("Library: items errored, auto-retrying once in 600ms", items.error);
      setTimeout(() => refetchItems(), 600);
    } else if (items() && !items.error) {
      autoRetried = false; // reset so future failures get their own retry
    }
  });

  // Stuck-loading detector — if neither data nor error has arrived after
  // 8 seconds, surface a manual retry instead of an infinite skeleton.
  // Covers the case where a request silently hangs (HTTP/2 stream wedged,
  // OS network stack blip, etc.) without a fetch-level error.
  const [stuckLoading, setStuckLoading] = createSignal(false);
  createEffect(() => {
    const isLoading = !items() && !items.error;
    if (!isLoading) {
      setStuckLoading(false);
      return;
    }
    setStuckLoading(false);
    const t = setTimeout(() => setStuckLoading(true), 8000);
    onCleanup(() => clearTimeout(t));
  });

  const fetchError = (): Error | undefined =>
    (items.error as Error | undefined) ?? (allLibs.error as Error | undefined);
  const manualRetry = () => {
    autoRetried = false;
    setStuckLoading(false);
    refetchAllLibs();
    refetchItems();
  };

  // Only expose items when their stamped server+lib match the current
  // params — guards against the stale-while-refetching cross-server race
  // described above. Sort/page/viewMode changes don't gate (same server,
  // image URLs stay valid); focus-refetch with identical params stays
  // through stableArrayByKey unchanged.
  const matchedItems = (): Item[] | undefined => {
    const i = items();
    if (!i) return undefined;
    if (i.server !== params.serverID || i.lib !== params.libraryID) return undefined;
    return i.items;
  };

  // Stabilise item refs across refetches so cards don't remount when
  // the focus-refetch lands. Prevents click-lost flicker; also saves
  // poster reflow for unchanged items.
  const stableItems = stableArrayByKey<Item>(
    () => matchedItems() ?? [],
    (it) => it.ratingKey,
  );

  const currentPageItems = () => stableItems().slice(0, PAGE_SIZE);
  const hasNextPage = () => stableItems().length > PAGE_SIZE;

  // Refresh both lib list (in case a library was added/removed) and the
  // current page of items (in case watched state or items list changed
  // while the user was over in Plex Web).
  refetchOnFocus(() => {
    refetchAllLibs();
    refetchItems();
  });

  return (
    <div class="library-page">
      <header class="library-header">
        <h1 class="library-name">{libName()}</h1>
        <Show when={isTVLibrary()}>
          <label>
            View:
            <select value={viewMode()} onChange={(e) => setViewMode(e.currentTarget.value)}>
              <For each={VIEW_MODE_OPTIONS}>
                {(o) => <option value={o.value}>{o.label}</option>}
              </For>
            </select>
          </label>
        </Show>
        <label>
          Sort:
          <select value={sort()} onChange={(e) => setSort(e.currentTarget.value)}>
            <For each={SORT_OPTIONS}>
              {(o) => <option value={o.value}>{o.label}</option>}
            </For>
          </select>
        </label>
        <Show when={matchedItems()}>
          <div class="library-header-right">
            <span class="library-count">
              Page {page() + 1} · showing {currentPageItems().length} items
            </span>
            <nav class="library-pagination" aria-label="Pagination">
              <button
                disabled={page() === 0}
                onClick={() => setPage(page() - 1)}
                aria-label="Previous page"
              >
                Prev
              </button>
              <span class="page-indicator">Page {page() + 1}</span>
              <button
                disabled={!hasNextPage()}
                onClick={() => setPage(page() + 1)}
                aria-label="Next page"
              >
                Next
              </button>
            </nav>
          </div>
        </Show>
      </header>
      <div class="library-grid">
        <Switch fallback={<Skeleton kind="card" count={12} />}>
          <Match when={matchedItems()}>
            <For each={currentPageItems()}>
              {(it) => <Card item={it} serverID={params.serverID!} enableWatchlistAdd />}
            </For>
          </Match>
          <Match when={fetchError()}>
            <div class="library-fetch-error">
              <h2>Couldn't load this library</h2>
              <p class="library-fetch-error-message">{fetchError()?.message}</p>
              <button type="button" class="library-fetch-retry" onClick={manualRetry}>
                Retry
              </button>
            </div>
          </Match>
          <Match when={stuckLoading()}>
            <div class="library-fetch-error">
              <h2>Still loading…</h2>
              <p class="library-fetch-error-message">
                This is taking longer than usual. Plex may be slow to respond, or
                the request got stuck. Hit retry — or refresh if it persists.
              </p>
              <button type="button" class="library-fetch-retry" onClick={manualRetry}>
                Retry
              </button>
            </div>
          </Match>
        </Switch>
      </div>
    </div>
  );
}
