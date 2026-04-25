import { Show } from "solid-js";
import { playbackStore } from "../state/playback";
import { api } from "../api/client";
import { formatAddedTimestamp } from "../util/date";
import "./NowPlaying.css";

export default function NowPlaying() {
  const s = playbackStore.state;
  const pct = () => {
    const st = s();
    if (!st.duration) return 0;
    return Math.min(100, Math.max(0, (st.position / st.duration) * 100));
  };
  const fmt = (ns: number) => {
    const totalSec = Math.floor(ns / 1_000_000_000);
    const m = Math.floor(totalSec / 60);
    const sec = totalSec % 60;
    const h = Math.floor(m / 60);
    const min = m % 60;
    if (h > 0) return `${h}:${String(min).padStart(2, "0")}:${String(sec).padStart(2, "0")}`;
    return `${min}:${String(sec).padStart(2, "0")}`;
  };

  return (
    <Show when={s().active}>
      <div class="now-playing">
        <Show when={s().thumbPath && s().serverID}>
          <img
            class="np-thumb"
            src={api.image(s().serverID!, s().thumbPath!)}
            alt=""
          />
        </Show>
        <div class="np-meta">
          <div class="np-pills">
            <Show when={s().showTitle}>
              <span class="np-pill np-pill-strong">{s().showTitle}</span>
            </Show>
            <Show when={s().seasonIndex && s().episodeIndex}>
              <span class="np-pill np-pill-mono">S{s().seasonIndex} · E{s().episodeIndex}</span>
            </Show>
            <span class="np-pill np-pill-strong">{s().title}</span>
            <Show when={s().originallyAvailableAt}>
              <span class="np-pill">Released {s().originallyAvailableAt}</span>
            </Show>
            <Show when={s().addedAt}>
              <span class="np-pill">{formatAddedTimestamp(s().addedAt)}</span>
            </Show>
            <Show when={s().transcoding}>
              <span class="np-pill np-pill-warn">TRANSCODE</span>
            </Show>
            <Show when={s().quality}>
              <span class="np-pill np-pill-mono">{s().quality}</span>
            </Show>
          </div>
          <div class="np-progress">
            <span class="np-time">{fmt(s().position)}</span>
            <div class="np-progress-track">
              <div class="np-progress-fill" style={{ width: `${pct()}%` }} />
            </div>
            <span class="np-time">{fmt(s().duration)}</span>
          </div>
        </div>
      </div>
    </Show>
  );
}
