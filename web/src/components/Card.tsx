import { A } from "@solidjs/router";
import { api } from "../api/client";
import type { Item } from "../api/types";
import { Trash2 } from "./icons";
import "./Card.css";

export interface CardProps {
  item: Item;
  serverID: string;
  /** If provided, a bin icon appears on hover; click fires the callback. */
  onRemove?: () => void;
}

// Derived display fields for a single card. Episodes surface the show's name
// as the primary title and format S/E + episode title as the subtitle.
function derive(item: Item) {
  if (item.type === "episode") {
    const season = item.parentIndex ?? 0;
    const episode = item.index ?? 0;
    const se = season && episode ? `S${season} · E${episode}` : "";
    return {
      title: item.grandparentTitle ?? item.title,
      // "S1 · E3 · Episode Title" — drop the title if it's just "Episode 3" or similar
      subtitle: se + (item.title && se ? ` · ${item.title}` : item.title ?? ""),
      thumb: item.grandparentThumb ?? item.thumb,
      // For episodes we don't show year — shown at the show level, not episode
      year: undefined as number | undefined,
      linkKey: item.ratingKey, // still deep-link to the episode
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
        {d().thumb ? (
          <img src={api.image(props.serverID, d().thumb!)} alt={d().title} loading="lazy" />
        ) : (
          <div class="card-poster-placeholder">
            <span>{d().title.slice(0, 1)}</span>
          </div>
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
