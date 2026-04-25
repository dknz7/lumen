import { A } from "@solidjs/router";
import { createSignal } from "solid-js";
import { api } from "../api/client";
import type { Item } from "../api/types";
import { CircleCheck, ImageOff, Trash2 } from "./icons";
import "./Card.css";

export interface CardProps {
  item: Item;
  serverID: string;
  /** If provided, a bin icon appears on hover; click fires the callback. */
  onRemove?: () => void;
  /** If provided, a tick icon appears above the bin; click fires the callback.
   *  Used by Continue Watching cards for "Mark as Watched". */
  onMarkWatched?: () => void;
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
  return (
    <A class="card" href={`/item/${props.serverID}/${d().linkKey}`}>
      <div class="card-poster">
        <div class="card-poster-placeholder" aria-hidden="true">
          <ImageOff size={32} strokeWidth={1.5} />
        </div>
        {hasImg() && (
          <img
            class="card-poster-img"
            src={api.image(props.serverID, d().thumb!)}
            alt=""
            loading="lazy"
            onError={() => setImgFailed(true)}
          />
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
      </div>
      <div class="card-meta">
        <div class="card-title">{d().title}</div>
        {d().subtitle && <div class="card-subtitle">{d().subtitle}</div>}
        {d().year && <div class="card-year">{d().year}</div>}
      </div>
    </A>
  );
}
