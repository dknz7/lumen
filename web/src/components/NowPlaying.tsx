import { Show } from "solid-js";
import { playbackStore } from "../state/playback";
import { api } from "../api/client";
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
          <div class="np-title">
            <Show when={s().showTitle} fallback={<span>{s().title}</span>}>
              <span class="np-show">{s().showTitle}</span>
              <span class="np-ep"> · {s().title}</span>
            </Show>
          </div>
          <div class="np-progress">
            <div class="np-progress-track">
              <div class="np-progress-fill" style={{ width: `${pct()}%` }} />
            </div>
            <div class="np-times">
              <span>{fmt(s().position)}</span>
              <span class="np-sep">/</span>
              <span>{fmt(s().duration)}</span>
            </div>
          </div>
        </div>
        <div class="np-quality">
          <Show when={s().transcoding}>
            <span class="np-badge np-badge-warn">TRANSCODE</span>
          </Show>
          <Show when={s().quality}>
            <span class="np-badge">{s().quality}</span>
          </Show>
        </div>
      </div>
    </Show>
  );
}
