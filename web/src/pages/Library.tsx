import { useParams } from "@solidjs/router";
import { createEffect, createResource, createSignal, For, Show } from "solid-js";
import { api } from "../api/client";
import type { Item } from "../api/types";
import Card from "../components/Card";
import "./Library.css";

const SORT_OPTIONS = [
  { value: "addedAt:desc", label: "Date Added (newest)" },
  { value: "titleSort:asc", label: "Title (A→Z)" },
  { value: "year:desc",    label: "Release Year (newest)" },
  { value: "rating:desc",  label: "Rating (highest)" },
  { value: "lastViewedAt:desc", label: "Last Viewed" },
];

const PAGE_SIZE = 50;

export default function Library() {
  const params = useParams();
  const [sort, setSort] = createSignal(SORT_OPTIONS[0].value);
  const [page, setPage] = createSignal(0);

  // Reset page to 0 whenever the library or sort changes.
  createEffect(() => {
    params.serverID; params.libraryID; sort();
    setPage(0);
  });

  const [items] = createResource(
    () => ({ server: params.serverID, lib: params.libraryID, sort: sort(), page: page() }),
    ({ server, lib, sort, page }) =>
      // Fetch PAGE_SIZE + 1 so we can detect if a next page exists without a total count.
      api.items(server, lib, { sort, start: page * PAGE_SIZE, size: PAGE_SIZE + 1 })
  );

  const currentPageItems = () => {
    const all = (items() ?? []) as Item[];
    return all.slice(0, PAGE_SIZE);
  };
  const hasNextPage = () => ((items() ?? []) as Item[]).length > PAGE_SIZE;

  return (
    <div class="library-page">
      <header class="library-header">
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
