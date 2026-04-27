import { A } from "@solidjs/router";
import { createSignal } from "solid-js";
import { api } from "../api/client";
import type { Item } from "../api/types";
import { CircleCheck, ImageOff, Play, Plus, Trash2 } from "./icons";
import { formatAddedTimestamp } from "../util/date";
import "./Card.css";

export interface CardProps {
  item: Item;
  serverID: string;
  /** If provided, a bin icon appears on hover; click fires the callback. */
  onRemove?: () => void;
  /** If provided, a tick icon appears above the bin; click fires the callback.
   *  Used by Continue Watching cards for "Mark as Watched". */
  onMarkWatched?: () => void;
  /** If true, a Plus icon appears top-right; click adds the item to the
   *  watchlist (TV episodes/seasons roll up to the parent show server-side).
   *  Used by Library cards. After successful add the icon swaps to a
   *  CircleCheck for the rest of the session. */
  enableWatchlistAdd?: boolean;
}

function derive(item: Item) {
  if (item.type === "episode") {
    const season = item.parentIndex ?? 0;
    const episode = item.index ?? 0;
    const se = season && episode ? `S${season} · E${episode}` : "";
    return {
      title: item.grandparentTitle ?? item.title,
      subtitle: se + (item.title && se ? ` · ${item.title}` : item.title ?? ""),
      thumb: item.grandparentThumb ?? item.thumb,
      year: undefined as number | undefined,
      linkKey: item.ratingKey,
    };
  }
  return {
    title: item.title,
    subtitle: undefined as string | undefined,
    thumb: item.thumb,
    year: item.year,
    linkKey: item.ratingKey,
  };
}

export default function Card(props: CardProps) {
  const d = () => derive(props.item);
  // Placeholder-first: ImageOff always renders underneath; <img> overlays when
  // it loads. If thumb is missing OR the load 404s, placeholder stays visible.
  const [imgFailed, setImgFailed] = createSignal(false);
  const hasImg = () => !!d().thumb && !imgFailed();

  const progressPct = () => {
    const dur = props.item.duration ?? 0;
    const off = props.item.viewOffset ?? 0;
    if (!dur || !off) return 0;
    const p = (off / dur) * 100;
    return Math.max(0, Math.min(100, p));
  };

  // Watchlist-add UX state — once successfully added, the button stays as
  // CircleCheck for the session (matches DiscoverTile + Watchlist patterns
  // per Byron's design call). Optimistic flip on click, revert on error.
  const [watchlistAdded, setWatchlistAdded] = createSignal(false);
  const [watchlistBusy, setWatchlistBusy] = createSignal(false);

  async function handleAddToWatchlist(e: MouseEvent) {
    e.preventDefault();
    e.stopPropagation();
    if (watchlistAdded() || watchlistBusy()) return;
    setWatchlistBusy(true);
    setWatchlistAdded(true); // optimistic
    try {
      await api.watchlistAddFromItem(props.serverID, props.item.ratingKey);
      // Brief Plex propagation delay before broadcasting invalidation so the
      // watchlist resource on other pages picks up the change.
      setTimeout(() => window.dispatchEvent(new CustomEvent("lumen:data-invalidated")), 350);
    } catch (err) {
      setWatchlistAdded(false);
      alert(`Add to Watchlist failed: ${(err as Error).message}`);
    } finally {
      setWatchlistBusy(false);
    }
  }

  async function handlePlayClick(e: MouseEvent) {
    e.preventDefault();
    e.stopPropagation();
    try {
      const offset = props.item.viewOffset ?? 0;
      if (offset > 0) {
        await api.play(props.serverID, props.item.ratingKey, offset);
      } else {
        await api.play(props.serverID, props.item.ratingKey);
      }
    } catch (err) {
      console.error("card play failed:", err);
      alert(`Play failed: ${(err as Error).message}`);
    }
  }

  return (
    <A class="card" href={`/item/${props.serverID}/${d().linkKey}`}>
      <div class="card-poster">
        <div class="card-poster-placeholder" aria-hidden="true">
          <ImageOff size={32} strokeWidth={1.5} />
        </div>
        {hasImg() && (
          <img
            class="card-poster-img"
            src={api.image(props.serverID, d().thumb!, "poster")}
            alt=""
            loading="lazy"
            onError={() => setImgFailed(true)}
          />
        )}
        <button
          class="card-play-btn"
          title="Play"
          aria-label="Play"
          onClick={handlePlayClick}
        >
          <Play size={36} fill="currentColor" />
        </button>
        {props.enableWatchlistAdd && (
          <button
            class="card-watchlist-btn"
            classList={{ "is-on": watchlistAdded() }}
            title={watchlistAdded() ? "Added to Watchlist" : "Add to Watchlist"}
            aria-label={watchlistAdded() ? "Added to Watchlist" : "Add to Watchlist"}
            disabled={watchlistBusy() || watchlistAdded()}
            onClick={handleAddToWatchlist}
          >
            {watchlistAdded() ? <CircleCheck size={14} /> : <Plus size={14} />}
          </button>
        )}
        {props.onMarkWatched && (
          <button
            class="card-mark-watched-btn"
            title="Mark as watched"
            aria-label="Mark as watched"
            onClick={(e) => {
              e.preventDefault();
              e.stopPropagation();
              props.onMarkWatched!();
            }}
          >
            <CircleCheck size={14} />
          </button>
        )}
        {props.onRemove && (
          <button
            class="card-remove-btn"
            title="Remove from Continue Watching"
            aria-label="Remove from Continue Watching"
            onClick={(e) => {
              e.preventDefault();
              e.stopPropagation();
              props.onRemove!();
            }}
          >
            <Trash2 size={14} />
          </button>
        )}
        {progressPct() > 0 && (
          <div class="card-progress-track">
            <div class="card-progress-fill" style={{ width: `${progressPct()}%` }} />
          </div>
        )}
        {(props.item.viewCount ?? 0) > 0 && (
          <div class="card-watched-ribbon" aria-label="Watched">WATCHED</div>
        )}
      </div>
      <div class="card-meta">
        <div class="card-title">{d().title}</div>
        {d().subtitle && <div class="card-subtitle">{d().subtitle}</div>}
        {d().year && <div class="card-year">{d().year}</div>}
        {props.item.addedAt && <div class="card-added">{formatAddedTimestamp(props.item.addedAt)}</div>}
      </div>
    </A>
  );
}
