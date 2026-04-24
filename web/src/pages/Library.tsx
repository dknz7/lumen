import { useParams } from "@solidjs/router";
import { createEffect, createResource, createSignal, For, Show } from "solid-js";
import { api } from "../api/client";
import type { Item, Library as LibraryType } from "../api/types";
import Card from "../components/Card";
import { store as settingsStore } from "../state/settings";
import "./Library.css";

const SORT_OPTIONS = [
  { value: "addedAt:desc", label: "Date Added (newest)" },
  { value: "titleSort:asc", label: "Title (A→Z)" },
  { value: "year:desc",    label: "Release Year (newest)" },
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

const PAGE_SIZE = 50;

export default function Library() {
  const params = useParams();

  // Initialise from settings store; fall back to defaults if store not yet loaded.
  const [sort, setSort] = createSignal(settingsStore.settings()?.defaultSort ?? SORT_OPTIONS[0].value);
  const [viewMode, setViewMode] = createSignal(
    viewModeToFilter(settingsStore.settings()?.defaultViewMode ?? "episodes")
  );
  const [page, setPage] = createSignal(0);

  const [allLibs] = createResource(
    () => params.serverID,
    (serverID) => api.libraries(serverID)
  );
  const currentLibrary = () =>
    ((allLibs() ?? []) as LibraryType[]).find((l) => l.key === params.libraryID);
  const isTVLibrary = () => currentLibrary()?.type === "show";

  // Reset page on library/server change.
  createEffect(() => {
    params.serverID; params.libraryID;
    setPage(0);
  });

  // Reset page whenever sort/mode changes, and persist the new preference.
  createEffect(() => {
    const s = sort();
    settingsStore.patch({ defaultSort: s });
    setPage(0);
  });
  createEffect(() => {
    const v = viewMode();
    settingsStore.patch({ defaultViewMode: filterToViewMode(v) as any });
    setPage(0);
  });

  const [items] = createResource(
    () => ({ server: params.serverID, lib: params.libraryID, sort: sort(), page: page(), type: viewMode(), isTV: isTVLibrary() }),
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

  const currentPageItems = () => {
    const all = (items() ?? []) as Item[];
    return all.slice(0, PAGE_SIZE);
  };
  const hasNextPage = () => ((items() ?? []) as Item[]).length > PAGE_SIZE;

  return (
    <div class="library-page">
      <header class="library-header">
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
          <span class="library-count">
            Page {page() + 1} · showing {currentPageItems().length} items
          </span>
        </Show>
      </header>
      <div class="library-grid">
        <Show when={items()} fallback={<div class="library-loading">Loading…</div>}>
          <For each={currentPageItems()}>
            {(it) => <Card item={it} serverID={params.serverID} />}
          </For>
        </Show>
      </div>
      <Show when={items()}>
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
      </Show>
    </div>
  );
}
