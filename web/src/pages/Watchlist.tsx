import { createMemo, createResource, createSignal, For, Show } from "solid-js";
import { A } from "@solidjs/router";
import type { WatchlistItem } from "../api/types";
import { api } from "../api/client";
import Skeleton from "../components/Skeleton";
import { refetchOnFocus } from "../util/focusRefetch";
import "./Watchlist.css";

function isAbsoluteURL(s: string): boolean {
  return /^https?:\/\//i.test(s);
}

type TypeFilter = "all" | "movie" | "show";
type SortKey = "addedAt" | "title" | "year";

export default function Watchlist() {
  const [items, { refetch }] = createResource<WatchlistItem[]>(() => api.watchlist());
  refetchOnFocus(refetch);

  const [typeFilter, setTypeFilter] = createSignal<TypeFilter>("all");
  const [sortKey, setSortKey] = createSignal<SortKey>("addedAt");

  const visible = createMemo<WatchlistItem[]>(() => {
    const all = items() ?? [];
    const filtered = typeFilter() === "all" ? all : all.filter((i) => i.type === typeFilter());
    const sorted = [...filtered];
    const key = sortKey();
    if (key === "title") {
      sorted.sort((a, b) => a.title.localeCompare(b.title));
    } else if (key === "year") {
      sorted.sort((a, b) => (b.year ?? 0) - (a.year ?? 0));
    }
    // addedAt is the wire default — items already arrive in that order.
    return sorted;
  });

  return (
    <div class="watchlist-page">
      <header class="watchlist-header">
        <div class="watchlist-controls">
          <label>
            Type
            <select value={typeFilter()} onChange={(e) => setTypeFilter(e.currentTarget.value as TypeFilter)}>
              <option value="all">All</option>
              <option value="movie">Movies</option>
              <option value="show">TV Shows</option>
            </select>
          </label>
          <label>
            Sort
            <select value={sortKey()} onChange={(e) => setSortKey(e.currentTarget.value as SortKey)}>
              <option value="addedAt">Date Added</option>
              <option value="title">Title</option>
              <option value="year">Release Year</option>
            </select>
          </label>
        </div>
        <div class="watchlist-count">
          <Show when={items()} fallback="…">
            {visible().length} item{visible().length === 1 ? "" : "s"}
          </Show>
        </div>
      </header>
      <Show when={items()} fallback={<div class="watchlist-grid"><Skeleton kind="card" count={12} /></div>}>
        <ul class="watchlist-grid">
          <For each={visible()}>
            {(it) => <WatchlistCard item={it} />}
          </For>
        </ul>
        <Show when={visible().length === 0}>
          <div class="watchlist-empty">Your Watchlist is empty.</div>
        </Show>
      </Show>
    </div>
  );
}

function WatchlistCard(props: { item: WatchlistItem }) {
  // plex.tv ratingKeys aren't routable to a server-local item directly —
  // the card link goes to /watchlist/<ratingKey>; that route lands in
  // Task 15 alongside the Add/Remove from Item Detail wiring. Click-through
  // to plex.tv item detail will be wired then.
  return (
    <li class="watchlist-card">
      <A href={`/watchlist/${encodeURIComponent(props.item.ratingKey)}`} class="watchlist-card-link">
        <div class="watchlist-poster">
          <Show when={props.item.thumb && isAbsoluteURL(props.item.thumb)} fallback={<div class="watchlist-poster-empty" />}>
            <img src={props.item.thumb!} alt={props.item.title} referrerpolicy="no-referrer"
                 onError={(e) => { (e.currentTarget as HTMLImageElement).style.display = "none"; }} />
          </Show>
        </div>
        <div class="watchlist-meta">
          <div class="watchlist-title">{props.item.title}</div>
          <div class="watchlist-sub">
            {props.item.type === "movie" ? "Movie" : "TV Show"}
            <Show when={props.item.year}> · {props.item.year}</Show>
          </div>
        </div>
      </A>
    </li>
  );
}
