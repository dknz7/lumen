import { createEffect, Show } from "solid-js";
import ModalShell from "./ModalShell";
import { playbackStore } from "../../state/playback";
import { api } from "../../api/client";
import "./NextEpisodeModal.css";
import { toast, errorMessage } from "../Toast";

export default function NextEpisodeModal() {
  const info = playbackStore.nextEpisode;

  async function play(target: { serverID: string; ratingKey: string }) {
    try {
      await api.playStop();
      await api.play(target.serverID, target.ratingKey);
    } catch (e) {
      console.error("play next failed:", e);
      toast.error(`Couldn't play the next episode — ${errorMessage(e)}`);
    } finally {
      playbackStore.dismissNextEpisode();
    }
  }

  function playNow() {
    const i = info();
    if (i) play(i);
  }

  // Backend says the episode truly ended (natural EOF or PotPlayer closed
  // past the watched threshold): advance immediately. The store already
  // swallowed the event if the user Dismissed.
  createEffect(() => {
    const over = playbackStore.episodeOver();
    if (!over) return;
    playbackStore.clearEpisodeOver();
    play(over);
  });

  return (
    <ModalShell open={info() !== null} onCancel={playbackStore.cancelBinge} ariaLabel="Next episode">
      <h2 class="nem-title">Up Next</h2>
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
      <div class="nem-actions">
        <button class="nem-cancel" onClick={playbackStore.cancelBinge}>Dismiss</button>
        <button class="nem-now" onClick={playNow} autofocus>Play Now</button>
      </div>
    </ModalShell>
  );
}
