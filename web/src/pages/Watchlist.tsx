import { createMemo, createResource, createSignal, For, Show } from "solid-js";
import { useNavigate } from "@solidjs/router";
import type { WatchlistItem } from "../api/types";
import { api } from "../api/client";
import Skeleton from "../components/Skeleton";
import { CircleCheck, Play, Trash2 } from "../components/icons";
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
  const navigate = useNavigate();
  const detailHref = () => `/watchlist/${encodeURIComponent(props.item.ratingKey)}`;

  async function handlePlay() {
    try {
      // Defensive `?? []`: api.availability is typed Promise<Match[]> but the
      // backend returns null (not []) for the empty case, which would make
      // matches.length crash. Belt-and-braces guard at the call site.
      const matches = (await api.availability(props.item.guid ?? "")) ?? [];
      if (matches.length === 0) {
        // No local copy — navigate to the watchlist item detail page so the
        // user can decide (remove, wait for availability, click through).
        navigate(detailHref());
        return;
      }
      const m = matches[0];
      await api.play(m.machineIdentifier, m.ratingKey);
    } catch (e) {
      alert(`Play failed: ${(e as Error).message}`);
    }
  }

  async function handleRemove() {
    try {
      await api.watchlistRemove(props.item.ratingKey);
      // Brief Plex propagation delay before broadcasting invalidation so the
      // page-level watchlist resource refetches with fresh state and the
      // card disappears from the grid (see refetchOnFocus subscription).
      setTimeout(
        () => window.dispatchEvent(new CustomEvent("lumen:data-invalidated")),
        350,
      );
    } catch (e) {
      alert(`Remove from watchlist failed: ${(e as Error).message}`);
    }
  }

  async function handleMarkWatched() {
    // Lazy availability check (Option B from spec): the button is always
    // visually enabled; we resolve availability on click and alert if there's
    // no local copy. Eager per-card availability calls would mean N parallel
    // round-trips per page render — out of scope for v1.0.
    try {
      const matches = (await api.availability(props.item.guid ?? "")) ?? [];
      if (matches.length === 0) {
        alert("This title isn't on any of your servers — can't mark watched.");
        return;
      }
      const m = matches[0];
      await api.scrobble(m.machineIdentifier, m.ratingKey);
      setTimeout(
        () => window.dispatchEvent(new CustomEvent("lumen:data-invalidated")),
        350,
      );
    } catch (e) {
      alert(`Mark watched failed: ${(e as Error).message}`);
    }
  }

  // Click anywhere on the body navigates to the detail page. Using
  // onClick + navigate (not wrapping the card in <A>) so the absolute-positioned
  // action buttons can stop propagation without nested-anchor warnings.
  // Mirrors DiscoverTile's pattern.
  function handleBodyClick(e: MouseEvent) {
    const t = e.target as HTMLElement;
    if (
      t.closest(".watchlist-card-action-btn") ||
      t.closest(".watchlist-card-play-overlay")
    ) {
      return;
    }
    navigate(detailHref());
  }

  function handleBodyKeyDown(e: KeyboardEvent) {
    if (e.key === "Enter" || e.key === " ") {
      e.preventDefault();
      navigate(detailHref());
    }
  }

  return (
    <li class="watchlist-card">
      <div
        class="watchlist-card-body"
        onClick={handleBodyClick}
        onKeyDown={handleBodyKeyDown}
        role="link"
        tabindex="0"
      >
        <div class="watchlist-poster">
          <Show when={props.item.thumb && isAbsoluteURL(props.item.thumb)} fallback={<div class="watchlist-poster-empty" />}>
            <img src={props.item.thumb!} alt={props.item.title} referrerpolicy="no-referrer"
                 onError={(e) => { (e.currentTarget as HTMLImageElement).style.display = "none"; }} />
          </Show>
          {/* Centered Play overlay — fades in on hover. Mirrors
              DiscoverTile's clip-variant pattern (commit 634cdd8). */}
          <button
            type="button"
            class="watchlist-card-play-overlay"
            title="Play"
            aria-label="Play"
            onClick={(e) => {
              e.preventDefault();
              e.stopPropagation();
              handlePlay();
            }}
          >
            <Play size={28} fill="currentColor" />
          </button>
          {/* Top-right action cluster — Mark Watched then Remove. */}
          <div class="watchlist-card-actions">
            <button
              type="button"
              class="watchlist-card-action-btn watchlist-card-mark-watched"
              title="Mark as watched"
              aria-label="Mark as watched"
              onClick={(e) => {
                e.preventDefault();
                e.stopPropagation();
                handleMarkWatched();
              }}
            >
              <CircleCheck size={14} />
            </button>
            <button
              type="button"
              class="watchlist-card-action-btn watchlist-card-remove"
              title="Remove from Watchlist"
              aria-label="Remove from Watchlist"
              onClick={(e) => {
                e.preventDefault();
                e.stopPropagation();
                handleRemove();
              }}
            >
              <Trash2 size={14} />
            </button>
          </div>
        </div>
        <div class="watchlist-meta">
          <div class="watchlist-title">{props.item.title}</div>
          <div class="watchlist-sub">
            {props.item.type === "movie" ? "Movie" : "TV Show"}
            <Show when={props.item.year}> · {props.item.year}</Show>
          </div>
        </div>
      </div>
    </li>
  );
}
