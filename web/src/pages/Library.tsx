import { useParams } from "@solidjs/router";
import { createEffect, createResource, createSignal, For, Show, untrack } from "solid-js";
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

  // Note: the zoom slider's effect on card sizes is handled globally in
  // state/settings.ts, which sets --card-width on :root. Library no longer
  // needs its own scoped override — the global value drives the grid.

  // Initialise from settings store; fall back to defaults if store not yet loaded.
  const [sort, setSort] = createSignal(settingsStore.settings()?.defaultSort ?? SORT_OPTIONS[0].value);
  const [viewMode, setViewMode] = createSignal(
    viewModeToFilter(settingsStore.settings()?.defaultViewMode ?? "episodes")
  );
  const [page, setPage] = createSignal(0);

  const [allLibs, { refetch: refetchAllLibs }] = createResource(
    () => params.serverID,
    (serverID) => api.libraries(serverID)
  );
  const currentLibrary = () =>
    ((allLibs() ?? []) as LibraryType[]).find((l) => l.key === params.libraryID);
  const isTVLibrary = () => currentLibrary()?.type === "show";
  const libName = () => currentLibrary()?.title ?? "";

  // Reset page on library/server change.
  createEffect(() => {
    params.serverID; params.libraryID;
    setPage(0);
  });

  // Reset page whenever sort/mode changes, and persist the new preference.
  // patch() reads settingsStore.settings() internally; without untrack the
  // effect would subscribe to the settings signal and re-fire (resetting
  // page to 0) whenever any setting elsewhere changes — including the
  // debounced PUT response that lands ~300ms after this very call. That
  // race caused the Page-2 snap-back regression (Task 29b smoke test).
  createEffect(() => {
    const s = sort();
    untrack(() => {
      settingsStore.patch({ defaultSort: s });
    });
    setPage(0);
  });
  createEffect(() => {
    const v = viewMode();
    untrack(() => {
      settingsStore.patch({ defaultViewMode: filterToViewMode(v) as any });
    });
    setPage(0);
  });

  const [items, { refetch: refetchItems }] = createResource(
    () => ({ server: params.serverID!, lib: params.libraryID!, sort: sort(), page: page(), type: viewMode(), isTV: isTVLibrary() }),
    ({ server, lib, sort, page, type, isTV }) => {
      const opts: { sort: string; start: number; size: number; filters?: Record<string, string> } = {
        sort,
        start: page * PAGE_SIZE,
        size: PAGE_SIZE + 1,
      };
      // Only apply the type filter when it's a TV library.
      if (isTV && type) opts.filters = { type };
      return api.items(server, lib, opts);
    }
  );

  // Stabilise item refs across refetches so cards don't remount when
  // the focus-refetch lands. Prevents click-lost flicker; also saves
  // poster reflow for unchanged items.
  const stableItems = stableArrayByKey<Item>(
    () => (items() as Item[] | undefined) ?? [],
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
        <Show when={items()}>
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
        <Show when={items()} fallback={<Skeleton kind="card" count={12} />}>
          <For each={currentPageItems()}>
            {(it) => <Card item={it} serverID={params.serverID!} enableWatchlistAdd />}
          </For>
        </Show>
      </div>
    </div>
  );
}
