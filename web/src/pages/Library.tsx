import { useParams } from "@solidjs/router";
import { createResource, createSignal, For, Show } from "solid-js";
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

export default function Library() {
  const params = useParams();
  const [sort, setSort] = createSignal(SORT_OPTIONS[0].value);

  const [items] = createResource(
    () => ({ server: params.serverID, lib: params.libraryID, sort: sort() }),
    ({ server, lib, sort }) => api.items(server, lib, { sort, size: 200 })
  );

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
          {(list) => <span class="library-count">{(list() as Item[]).length} items</span>}
        </Show>
      </header>
      <div class="library-grid">
        <Show when={items()} fallback={<div class="library-loading">Loading…</div>}>
          {(list) => (
            <For each={list() as Item[]}>
              {(it) => (
                <Card
                  title={it.title}
                  year={it.year}
                  ratingKey={it.ratingKey}
                  serverID={params.serverID}
                />
              )}
            </For>
          )}
        </Show>
      </div>
    </div>
  );
}
