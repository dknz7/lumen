import { createEffect, createSignal, onCleanup, Show } from "solid-js";
import ModalShell from "./ModalShell";
import { playbackStore } from "../../state/playback";
import { api } from "../../api/client";
import "./NextEpisodeModal.css";

const COUNTDOWN_MS = 5000;
const TICK_MS = 100;

export default function NextEpisodeModal() {
  const info = playbackStore.nextEpisode;
  const [remaining, setRemaining] = createSignal(COUNTDOWN_MS);
  let timer: number | undefined;

  function close() {
    window.clearInterval(timer);
    playbackStore.dismissNextEpisode();
  }

  async function playNow() {
    const i = info();
    if (!i) return;
    window.clearInterval(timer);
    // Leave modal visible during the API calls so Cancel still wins.
    try {
      await api.playStop();
      await api.play(i.serverID, i.ratingKey);
    } catch (e) {
      console.error("auto-play next failed:", e);
      alert(`Failed to play next episode: ${(e as Error).message}`);
    } finally {
      playbackStore.dismissNextEpisode();
    }
  }

  createEffect(() => {
    const i = info();
    if (!i) return;
    setRemaining(COUNTDOWN_MS);
    timer = window.setInterval(() => {
      setRemaining((r) => {
        const next = r - TICK_MS;
        if (next <= 0) {
          window.clearInterval(timer);
          playNow();
          return 0;
        }
        return next;
      });
    }, TICK_MS);
    onCleanup(() => window.clearInterval(timer));
  });

  const pct = () => 100 - (remaining() / COUNTDOWN_MS) * 100;

  return (
    <ModalShell open={info() !== null} onCancel={close} ariaLabel="Next episode">
      <h2 class="nem-title">Next Episode in {Math.ceil(remaining() / 1000)}s</h2>
      <Show when={info()}>
        {(i) => (
          <div class="nem-card">
            <Show when={i().thumbPath}>
              <img class="nem-thumb" src={api.image(i().serverID, i().thumbPath!)} alt="" />
            </Show>
            <div class="nem-meta">
              <div class="nem-ep">S{i().season} · E{i().episode}</div>
              <div class="nem-name">{i().title}</div>
            </div>
          </div>
        )}
      </Show>
      <div class="nem-progress">
        <div class="nem-progress-fill" style={{ width: `${pct()}%` }} />
      </div>
      <div class="nem-actions">
        <button class="nem-cancel" onClick={close}>Cancel</button>
        <button class="nem-now" onClick={playNow} autofocus>Play Now</button>
      </div>
    </ModalShell>
  );
}
