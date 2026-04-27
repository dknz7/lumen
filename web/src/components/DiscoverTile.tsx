import { A, useNavigate } from "@solidjs/router";
import { createMemo, createSignal, Show, createContext, useContext, JSX } from "solid-js";
import type { HubItem } from "../api/types";
import { api } from "../api/client";
import { CircleCheck, ImageOff, Play, Plus } from "./icons";
import "./DiscoverTile.css";

// Context plumbed by the host page (Recommended / Discover) so DiscoverTile
// instances rendered deep inside Shelf's renderItem closure can:
//   - read the current watchlist Set<ratingKey> for the +/✓ toggle state
//   - hand a resolved YouTube id up to the page-level TrailerModal
// Page-scoped (not module-level) so each page mounts its own modal + resource.
export interface DiscoverTileContextValue {
  inWatchlistSet: () => Set<string>;
  openTrailer: (youtubeID: string, title: string) => void;
}

const DiscoverTileContext = createContext<DiscoverTileContextValue>();

export function DiscoverTileProvider(props: {
  value: DiscoverTileContextValue;
  children: JSX.Element;
}) {
  return (
    <DiscoverTileContext.Provider value={props.value}>
      {props.children}
    </DiscoverTileContext.Provider>
  );
}

function useDiscoverTile(): DiscoverTileContextValue {
  const ctx = useContext(DiscoverTileContext);
  if (!ctx) {
    // Defensive — DiscoverTile should never mount outside a host page that
    // wraps it in DiscoverTileProvider. If this ever fires, the page wiring
    // got broken; fall back to no-op stubs so the tile still renders.
    return {
      inWatchlistSet: () => new Set<string>(),
      openTrailer: () => {},
    };
  }
  return ctx;
}

export interface DiscoverTileProps {
  item: HubItem;
}

export default function DiscoverTile(props: DiscoverTileProps) {
  const ctx = useDiscoverTile();
  const navigate = useNavigate();
  const isClip = () => props.item.type === "clip";
  const href = () => `/discover-item/${encodeURIComponent(props.item.ratingKey)}`;

  const [imgFailed, setImgFailed] = createSignal(false);
  const hasImg = () => !!props.item.thumb && !imgFailed();

  // Watchlist toggle state — optimistic override mirrors ItemDetail's pattern.
  const [override, setOverride] = createSignal<boolean | null>(null);
  const isInWatchlist = createMemo(() => {
    const o = override();
    if (o !== null) return o;
    return ctx.inWatchlistSet().has(props.item.ratingKey);
  });

  // Trailer-button busy state so rapid clicks don't fire concurrent TMDB lookups.
  const [trailerBusy, setTrailerBusy] = createSignal(false);

  async function handleTrailerClick(e: MouseEvent) {
    e.preventDefault();
    e.stopPropagation();
    if (trailerBusy()) return;
    setTrailerBusy(true);
    try {
      // Cascade: TMDB (via imdbId, if HubItem ever exposes one) → Plex Extras
      // youtubeID (likewise) → "no trailer". HubItem today is light (Session 5),
      // so unless Task 12 fattens it both branches are absent and we fall to
      // the alert. Keeping the cascade structure here means the upgrade path
      // is just a wire-shape change, no component rewrite.
      const anyItem = props.item as HubItem & {
        imdbId?: string;
        trailer?: { youtubeID?: string };
        parentType?: string;
        grandparentType?: string;
      };
      let youtubeID: string | null = null;

      if (anyItem.imdbId) {
        const parentT = anyItem.parentType ?? anyItem.grandparentType ?? "movie";
        const mediaType: "movie" | "show" =
          parentT === "show" || parentT === "season" || parentT === "episode" ? "show" : "movie";
        try {
          youtubeID = await api.tmdbTrailer(anyItem.imdbId, mediaType);
        } catch (err) {
          console.warn("DiscoverTile: TMDB trailer lookup failed; falling back", err);
        }
      }
      if (!youtubeID && anyItem.trailer?.youtubeID) {
        youtubeID = anyItem.trailer.youtubeID;
      }
      if (!youtubeID) {
        alert("No trailer available");
        return;
      }
      ctx.openTrailer(youtubeID, props.item.title);
    } finally {
      setTrailerBusy(false);
    }
  }

  async function handleWatchlistClick(e: MouseEvent) {
    e.preventDefault();
    e.stopPropagation();
    const wasIn = isInWatchlist();
    setOverride(!wasIn);
    try {
      if (wasIn) {
        await api.watchlistRemove(props.item.ratingKey);
      } else {
        await api.watchlistAdd(props.item.ratingKey);
      }
      // Brief Plex propagation delay before broadcasting invalidation so the
      // page-level watchlist resource refetches with fresh state.
      setTimeout(
        () => window.dispatchEvent(new CustomEvent("lumen:data-invalidated")),
        350,
      );
    } catch (err) {
      setOverride(wasIn);
      alert(`Watchlist toggle failed: ${(err as Error).message}`);
    }
  }

  // Click anywhere on the body navigates. Using onClick + navigate (not
  // wrapping the whole tile in <A>) so the absolute-positioned action buttons
  // can capture clicks via stopPropagation without nested-anchor warnings.
  function handleBodyClick(e: MouseEvent) {
    // Don't navigate if the click landed on an action button (their handlers
    // already preventDefault + stopPropagation, but belt-and-braces).
    const t = e.target as HTMLElement;
    if (t.closest(".discover-tile-action-btn")) return;
    navigate(href());
  }

  // Browsers don't auto-translate keyboard events into clicks for ARIA
  // role="link" on a non-native element, so without this handler keyboard /
  // screen-reader users could focus the tile but not activate it. The
  // action-button skip-check from handleBodyClick is mouse-only (focus on a
  // child <button> consumes Enter/Space itself), so we just navigate directly.
  function handleBodyKeyDown(e: KeyboardEvent) {
    if (e.key === "Enter" || e.key === " ") {
      e.preventDefault();
      navigate(href());
    }
  }

  return (
    <div
      class="discover-tile"
      onClick={handleBodyClick}
      onKeyDown={handleBodyKeyDown}
      role="link"
      tabindex="0"
    >
      <div class="discover-tile-poster">
        <div class="discover-tile-poster-placeholder" aria-hidden="true">
          <ImageOff size={32} strokeWidth={1.5} />
        </div>
        {hasImg() && (
          <img
            class="discover-tile-poster-img"
            src={props.item.thumb!}
            alt=""
            loading="lazy"
            referrerpolicy="no-referrer"
            onError={() => setImgFailed(true)}
          />
        )}
        <Show when={isClip()}>
          <div class="discover-tile-actions">
            <button
              type="button"
              class="discover-tile-action-btn"
              title="Play trailer"
              aria-label="Play trailer"
              disabled={trailerBusy()}
              onClick={handleTrailerClick}
            >
              <Play size={14} fill="currentColor" />
            </button>
            <button
              type="button"
              class="discover-tile-action-btn"
              classList={{ "is-on": isInWatchlist() }}
              title={isInWatchlist() ? "Remove from Watchlist" : "Add to Watchlist"}
              aria-label={isInWatchlist() ? "Remove from Watchlist" : "Add to Watchlist"}
              onClick={handleWatchlistClick}
            >
              <Show when={isInWatchlist()} fallback={<Plus size={14} />}>
                <CircleCheck size={14} />
              </Show>
            </button>
          </div>
        </Show>
      </div>
      <div class="discover-tile-meta">
        <A
          class="discover-tile-title-link"
          href={href()}
          onClick={(e) => e.stopPropagation()}
        >
          <div class="discover-tile-title">{props.item.title}</div>
        </A>
        <Show when={props.item.year}>
          <div class="discover-tile-year">{props.item.year}</div>
        </Show>
      </div>
    </div>
  );
}
