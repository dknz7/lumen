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
  // openHLSTrailer is called for clip-type hub items that carry their own
  // native HLS playback URL (Trending Trailers / New Trailers). Routed to
  // a sibling page-level HLSTrailerModal instead of the YouTube modal.
  openHLSTrailer: (hlsUrl: string, title: string) => void;
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
      openHLSTrailer: () => {},
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
  const href = () => {
    // For clip items (trailer cards on Trending Trailers / New Trailers
    // shelves), the card's ratingKey is the clip's id which 404s on
    // Plex's /library/metadata/<rk> endpoint. Navigate to the underlying
    // movie/show instead via parentRatingKey (or grandparentRatingKey
    // for episode-clips).
    if (props.item.type === "clip") {
      const parentRk = props.item.parentRatingKey || props.item.grandparentRatingKey;
      if (parentRk) {
        return `/discover-item/${encodeURIComponent(parentRk)}`;
      }
      // No parent ratingKey known — fall through to the clip's own rk;
      // the backend's friendly 404 message will render correctly.
    }
    return `/discover-item/${encodeURIComponent(props.item.ratingKey)}`;
  };

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
      // Cascade: native HLS (Trending Trailers carry their own
      // Media[].Part[].key — Phase 4.6 Task 12.8) → TMDB (via imdbId
      // — Task 12 surfaced it on HubItem) → Plex Extras youtubeID
      // (only present on Item, not HubItem; kept here as a defensive
      // optional cast) → "no trailer".
      if (props.item.hlsUrl) {
        ctx.openHLSTrailer(props.item.hlsUrl, props.item.title);
        return;
      }
      let youtubeID: string | null = null;

      if (props.item.imdbId) {
        // For clips, type === "clip" → falls to "movie". Most TMDB trailer
        // lookups for clips are for movies; if it's a TV trailer the IMDB
        // id may still resolve via TMDB's /find endpoint. If it doesn't,
        // the cascade falls back gracefully.
        const mediaType: "movie" | "show" =
          props.item.type === "show" ||
          props.item.type === "season" ||
          props.item.type === "episode"
            ? "show"
            : "movie";
        try {
          youtubeID = await api.tmdbTrailer(props.item.imdbId, mediaType);
        } catch (err) {
          console.warn("DiscoverTile: TMDB trailer lookup failed; falling back", err);
        }
      }
      const trailerExtra = (props.item as HubItem & { trailer?: { youtubeID?: string } }).trailer;
      if (!youtubeID && trailerExtra?.youtubeID) {
        youtubeID = trailerExtra.youtubeID;
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
    if (t.closest(".discover-tile-action-btn") || t.closest(".discover-tile-play-overlay")) return;
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

  // Per-type render contract derived from Plex Web's
  // MediaContainer.Meta.DisplayFields (Session 6.5 capture):
  //   season  → parentTitle / title / date
  //   episode → grandparentTitle / S{parentIndex}E{index} / date
  //   show / movie / clip → title / — / (date or year)
  // Date prefers a near-future originallyAvailableAt (Coming Soon use case);
  // older originallyAvailableAt falls back to year so library/trending shelves
  // don't render misleading first-air dates on long-running shows.
  function primaryTitle(): string {
    if (props.item.type === "season" && props.item.parentTitle) {
      return props.item.parentTitle;
    }
    if (props.item.type === "episode" && props.item.grandparentTitle) {
      return props.item.grandparentTitle;
    }
    return props.item.title;
  }

  function subtitle(): string {
    if (props.item.type === "season") return props.item.title;
    if (props.item.type === "episode") {
      const s = props.item.parentIndex;
      const e = props.item.index;
      if (s !== undefined && e !== undefined) {
        return `S${s} · E${e}` + (props.item.title ? ` · ${props.item.title}` : "");
      }
      return props.item.title;
    }
    return "";
  }

  function dateLine(): string {
    return formatAirDate(props.item.originallyAvailableAt, props.item.year);
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
          {/* Centered Play Trailer overlay — large, fades in on hover. Mirrors
              Card.tsx's hover-to-play pattern (Session 4.5). */}
          <button
            type="button"
            class="discover-tile-play-overlay"
            title="Play trailer"
            aria-label="Play trailer"
            disabled={trailerBusy()}
            onClick={handleTrailerClick}
          >
            <Play size={28} fill="currentColor" />
          </button>
          {/* Top-right corner — Watchlist toggle only now. */}
          <div class="discover-tile-actions">
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
          <div class="discover-tile-title">{primaryTitle()}</div>
        </A>
        <Show when={subtitle()}>
          <div class="discover-tile-subtitle">{subtitle()}</div>
        </Show>
        <Show when={dateLine()}>
          <div class="discover-tile-year">{dateLine()}</div>
        </Show>
      </div>
    </div>
  );
}

// formatAirDate — "2026-05-10" → "May 10, 2026" when the date is in the
// future (Coming Soon parity); falls back to the year string for items
// already aired so non-Coming-Soon shelves don't show misleading
// first-air dates. Empty string when neither input has data.
function formatAirDate(iso: string | undefined, year: number | undefined): string {
  if (iso) {
    const d = new Date(iso + "T00:00:00");
    if (!isNaN(d.getTime()) && d.getTime() > Date.now()) {
      return d.toLocaleDateString("en-US", {
        year: "numeric",
        month: "long",
        day: "numeric",
      });
    }
  }
  return year ? String(year) : "";
}
