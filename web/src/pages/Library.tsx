import { useParams } from "@solidjs/router";
import { createEffect, createResource, createSignal, For, Show } from "solid-js";
import { api } from "../api/client";
import type { Item, Library as LibraryType } from "../api/types";
import Card from "../components/Card";
import "./Library.css";

const SORT_OPTIONS = [
  { value: "addedAt:desc", label: "Date Added (newest)" },
  { value: "titleSort:asc", label: "Title (A→Z)" },
  { value: "year:desc",    label: "Release Year (newest)" },
  { value: "rating:desc",  label: "Rating (highest)" },
  { value: "lastViewedAt:desc", label: "Last Viewed" },
];

// Plex library "type" query param: 4=episode. Empty = library default (Shows
// for TV libraries, Movies for Movies libraries). Default is "4" (Episodes)
// for TV libraries per Byron's design call — he finds episodes more useful
// at the browse level than the show roll-up view.
const VIEW_MODE_OPTIONS = [
  { value: "",  label: "Shows" },
  { value: "4", label: "Episodes" },
];

const PAGE_SIZE = 50;

// localStorage keys for sticky dropdown preferences. Survive refresh; cleared
// only when browser data is cleared. Session 3 replaces this with config.json
// persistence via the Settings panel.
const LS_SORT = "lumen.library.sort";
const LS_VIEW = "lumen.library.viewMode";

function loadLS(key: string, fallback: string): string {
  try {
    return localStorage.getItem(key) ?? fallback;
  } catch {
    return fallback;
  }
}

function saveLS(key: string, value: string) {
  try {
    localStorage.setItem(key, value);
  } catch {
    // Quota exceeded or storage unavailable — silently ignore.
  }
}

export default function Library() {
  const params = useParams();
  const [sort, setSort] = createSignal(loadLS(LS_SORT, SORT_OPTIONS[0].value));
  // Episodes is the default for TV libraries per Byron's preference.
  const [viewMode, setViewMode] = createSignal(loadLS(LS_VIEW, "4"));
  const [page, setPage] = createSignal(0);

  const [allLibs] = createResource(
    () => params.serverID,
    (serverID) => api.libraries(serverID)
  );
  const currentLibrary = () =>
    ((allLibs() ?? []) as LibraryType[]).find((l) => l.key === params.libraryID);
  const isTVLibrary = () => currentLibrary()?.type === "show";

  // Reset page on library/server change. Sort and viewMode persist.
  createEffect(() => {
    params.serverID; params.libraryID;
    setPage(0);
  });

  // Reset page whenever sort/mode changes, and persist the new preference.
  createEffect(() => {
    const s = sort();
    saveLS(LS_SORT, s);
    setPage(0);
  });
  createEffect(() => {
    const v = viewMode();
    saveLS(LS_VIEW, v);
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
      // Only apply the type filter when it's a TV library — it'd be a no-op on
      // Movies libraries but pointless to send.
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
            ← Prev
          </button>
          <span class="page-indicator">Page {page() + 1}</span>
          <button
            disabled={!hasNextPage()}
            onClick={() => setPage(page() + 1)}
            aria-label="Next page"
          >
            Next →
          </button>
        </nav>
      </Show>
    </div>
  );
}
