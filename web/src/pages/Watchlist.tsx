import { createMemo, createResource, createSignal, For, Show } from "solid-js";
import { useNavigate } from "@solidjs/router";
import type { Match, WatchlistItem } from "../api/types";
import { api } from "../api/client";
import Skeleton from "../components/Skeleton";
import TrailerModal from "../components/Modal/TrailerModal";
import { CircleCheck, Play, Trash2 } from "../components/icons";
import { refetchOnFocus } from "../util/focusRefetch";
import { createInViewport } from "../util/inViewport";
import { useAvailability } from "../state/availability";
import { stableArrayByKey } from "../util/stableArray";
import "./Watchlist.css";
import { toast, errorMessage } from "../components/Toast";

function isAbsoluteURL(s: string): boolean {
  return /^https?:\/\//i.test(s);
}

// parseRes maps a Plex resolution string to a sortable numeric height. Plex
// surfaces these as "sd", "480", "720", "1080", "4k" (and rarely the bare
// pixel-height strings like "576" for PAL DVD rips). Anything we don't
// recognise sorts last (0). Higher number wins in the bestMatch sort.
function parseRes(r: string | undefined): number {
  if (!r) return 0;
  const lower = r.toLowerCase().trim();
  if (lower === "sd") return 480;
  if (lower === "4k" || lower === "uhd" || lower === "2160") return 2160;
  if (lower === "1440" || lower === "qhd") return 1440;
  const n = parseInt(lower, 10);
  return isNaN(n) ? 0 : n;
}

type TypeFilter = "all" | "movie" | "show";
type SortKey = "addedAt" | "title" | "year";

export default function Watchlist() {
  const [items, { refetch: refetchItems }] = createResource<WatchlistItem[]>(() => api.watchlist());
  refetchOnFocus(refetchItems);

  const [typeFilter, setTypeFilter] = createSignal<TypeFilter>("all");
  const [sortKey, setSortKey] = createSignal<SortKey>("addedAt");

  // Page-level TrailerModal state. Cards trigger via the prop-drilled
  // openTrailer callback below. Prop-drill (not context) chosen because
  // Watchlist is a flat list — one parent, direct children — so a context
  // provider would be ceremony for no reuse benefit.
  const [trailerOpen, setTrailerOpen] = createSignal(false);
  const [trailerYouTubeID, setTrailerYouTubeID] = createSignal<string | undefined>();
  const [trailerTitle, setTrailerTitle] = createSignal("");
  const openTrailer = (id: string, title: string) => {
    setTrailerYouTubeID(id);
    setTrailerTitle(title);
    setTrailerOpen(true);
  };

  // Stabilise item refs across refetches so cards don't remount on focus.
  const stableItems = stableArrayByKey<WatchlistItem>(
    () => items() ?? [],
    (it) => it.ratingKey,
  );

  const visible = createMemo<WatchlistItem[]>(() => {
    const all = stableItems();
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
        <h1 class="page-heading">Watchlist</h1>
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
          <Show when={!items.error && items()} fallback="…">
            {visible().length} item{visible().length === 1 ? "" : "s"}
          </Show>
        </div>
      </header>
      <Show when={items.error}>
        <div class="watchlist-error" role="alert">
          <p>Couldn't load your Watchlist — {errorMessage(items.error)}</p>
          <button class="shelf-retry" onClick={() => refetchItems()}>Try again</button>
        </div>
      </Show>
      <Show
        when={!items.error && items()}
        fallback={<div class="watchlist-grid"><Skeleton kind="card" count={12} /></div>}
      >
        <ul class="watchlist-grid">
          <For each={visible()}>
            {(it) => <WatchlistCard item={it} openTrailer={openTrailer} />}
          </For>
        </ul>
        <Show when={visible().length === 0}>
          <div class="watchlist-empty">Your Watchlist is empty.</div>
        </Show>
      </Show>
      <TrailerModal
        open={trailerOpen()}
        onClose={() => setTrailerOpen(false)}
        youtubeID={trailerYouTubeID()}
        title={trailerTitle()}
      />
    </div>
  );
}

function WatchlistCard(props: {
  item: WatchlistItem;
  openTrailer: (youtubeID: string, title: string) => void;
}) {
  const navigate = useNavigate();

  // Availability is requested only once this card scrolls into view, and the
  // request is coalesced with its neighbours into a single batch. Asking for
  // all 528 up front took ~42 seconds and hammered both Plex servers on every
  // visit and every window focus.
  let cardRef: HTMLLIElement | undefined;
  const onScreen = createInViewport(() => cardRef);
  const availability = useAvailability(() => props.item.guid, onScreen);

  // Best local match: highest resolution wins, tiebreak alphabetically by
  // server display name — so a 4K copy beats a 1080p one, deterministically.
  const bestMatch = createMemo<Match | null>(() => {
    const list = availability() ?? [];
    if (list.length === 0) return null;
    const sorted = [...list].sort((a, b) => {
      const ra = parseRes(a.resolution);
      const rb = parseRes(b.resolution);
      if (rb !== ra) return rb - ra;
      return (a.serverName ?? "").localeCompare(b.serverName ?? "");
    });
    return sorted[0];
  });

  async function handlePlay() {
    const m = bestMatch();
    if (m) {
      // In library — play best available local copy.
      try {
        await api.play(m.machineIdentifier, m.ratingKey);
      } catch (e) {
        toast.error(`Couldn't start playback — ${errorMessage(e)}`);
      }
      return;
    }
    // Not in library — resolve a trailer via the same cascade DiscoverTile
    // uses (discoverItem → imdbId → tmdbTrailer → YouTube id) and open the
    // page-level TrailerModal.
    try {
      const detail = await api.discoverItem(props.item.ratingKey);
      if (!detail.imdbId) {
        toast.info("No trailer available — Plex has no IMDB id for this title.");
        return;
      }
      const mediaType: "movie" | "show" =
        detail.type === "show" || detail.type === "season" || detail.type === "episode"
          ? "show"
          : "movie";
      const youtubeID = await api.tmdbTrailer(detail.imdbId, mediaType);
      if (!youtubeID) {
        toast.info("No trailer available for this title.");
        return;
      }
      props.openTrailer(youtubeID, detail.title);
    } catch (e) {
      toast.error(`Couldn't load the trailer — ${errorMessage(e)}`);
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
      toast.error(`Couldn't remove from your Watchlist — ${errorMessage(e)}`);
    }
  }

  async function handleMarkWatched() {
    // Lazy availability check (Option B from spec): the button is always
    // visually enabled; we resolve availability on click and alert if there's
    // no local copy. Kept lazy here even though Play uses eager — Mark Watched
    // is rarely the first interaction on a watchlist card, and the existing
    // null-guard pattern from b7f3430 stays unchanged per the smoke spec.
    try {
      const matches = (await api.availability(props.item.guid ?? "")) ?? [];
      if (matches.length === 0) {
        toast.info("This title isn't on any of your servers, so there's nothing to mark watched.");
        return;
      }
      const m = matches[0];
      await api.scrobble(m.machineIdentifier, m.ratingKey);
      setTimeout(
        () => window.dispatchEvent(new CustomEvent("lumen:data-invalidated")),
        350,
      );
    } catch (e) {
      toast.error(`Couldn't mark as watched — ${errorMessage(e)}`);
    }
  }

  // Click anywhere on the body navigates. In-library lands on the server-local
  // Item Detail page (so the user is on the playable copy). Not-in-library OR
  // availability-still-loading falls back to the Discover Item Detail page,
  // which surfaces MORE WAYS TO WATCH so a late-arriving server-local match
  // is still reachable.
  function handleBodyClick(e: MouseEvent) {
    const t = e.target as HTMLElement;
    if (
      t.closest(".watchlist-card-action-btn") ||
      t.closest(".watchlist-card-play-overlay")
    ) {
      return;
    }
    const m = bestMatch();
    if (m) {
      navigate(`/item/${encodeURIComponent(m.machineIdentifier)}/${encodeURIComponent(m.ratingKey)}`);
    } else {
      navigate(`/watchlist/${encodeURIComponent(props.item.ratingKey)}`);
    }
  }

  function handleBodyKeyDown(e: KeyboardEvent) {
    if (e.key === "Enter" || e.key === " ") {
      e.preventDefault();
      const m = bestMatch();
      if (m) {
        navigate(`/item/${encodeURIComponent(m.machineIdentifier)}/${encodeURIComponent(m.ratingKey)}`);
      } else {
        navigate(`/watchlist/${encodeURIComponent(props.item.ratingKey)}`);
      }
    }
  }

  return (
    <li class="watchlist-card" ref={cardRef}>
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
