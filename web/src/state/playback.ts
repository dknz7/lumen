import { createRoot, createSignal } from "solid-js";
import type {
  PlaybackState,
  PlaybackEvent,
  NextEpisodeInfo,
  TranscodePromptInfo,
} from "../api/types";

function createPlaybackStore() {
  const [state, setState] = createSignal<PlaybackState>({
    active: false,
    position: 0,
    duration: 0,
    state: "stopped",
  });

  // Modal triggers — set when a prompt event arrives, cleared by the modal
  // via dismiss* functions. `endedAt` is a timestamp signal so consumers can
  // react to the "ended" event without needing a boolean flag to reset.
  const [nextEpisode, setNextEpisode] = createSignal<NextEpisodeInfo | null>(null);
  const [transcodePrompt, setTranscodePrompt] = createSignal<TranscodePromptInfo | null>(null);
  const [endedAt, setEndedAt] = createSignal<number>(0);
  // Set when the backend says the episode truly ended (natural EOF or player
  // closed past the watched threshold) — the SPA advances immediately.
  const [episodeOver, setEpisodeOver] = createSignal<NextEpisodeInfo | null>(null);
  // Dismiss = opt out of auto-advance for this transition only. Reset when
  // the next prompt arrives (i.e. the next episode's own 95% mark).
  let bingeCancelled = false;

  let es: EventSource | null = null;

  function connect() {
    if (es) return;
    es = new EventSource("/api/playback/stream");

    // The Go server (Task 18) sends every event with its own SSE event-name
    // matching the playback.Event.Type field. Each event's `data` field is
    // the full Event envelope: { type, state, payload }. We register one
    // listener per named event type — es.onmessage would never fire because
    // the server never sends event: message.

    const parse = (ev: Event): PlaybackEvent | null => {
      try {
        return JSON.parse((ev as MessageEvent).data) as PlaybackEvent;
      } catch (e) {
        console.error("playback: parse SSE data", e);
        return null;
      }
    };

    es.addEventListener("state", (ev) => {
      const evt = parse(ev);
      if (evt && evt.type === "state") setState(evt.state);
    });

    es.addEventListener("ended", () => {
      setEndedAt(Date.now());
    });

    es.addEventListener("next-episode-prompt", (ev) => {
      const evt = parse(ev);
      if (evt && evt.type === "next-episode-prompt") {
        bingeCancelled = false;
        setNextEpisode(evt.payload);
      }
    });

    es.addEventListener("episode-over", (ev) => {
      const evt = parse(ev);
      if (evt && evt.type === "episode-over" && !bingeCancelled) {
        setEpisodeOver(evt.payload);
      }
    });

    es.addEventListener("transcode-prompt", (ev) => {
      const evt = parse(ev);
      if (evt && evt.type === "transcode-prompt") setTranscodePrompt(evt.payload);
    });

    es.addEventListener("stopped", () => {
      setState({ active: false, position: 0, duration: 0, state: "stopped" });
      // The session is gone — any pending Up Next card or auto-advance is
      // stale (e.g. user crossed 95%, rewound, then closed the player).
      setNextEpisode(null);
      setEpisodeOver(null);
      // Plex's /library/metadata/<key> cache lags ~100-300ms after the final
      // ReportTimeline POST that Manager.Stop() fires. Wait before broadcasting
      // invalidation so subscribers (ItemDetail, Episodes, etc.) refetch fresh
      // data instead of stale viewOffset/viewCount.
      setTimeout(() => {
        window.dispatchEvent(new CustomEvent("lumen:data-invalidated"));
      }, 500);
    });

    es.onerror = (e) => {
      console.error("playback SSE error", e);
      // Browser auto-reconnects. On reconnect, the server sends a fresh
      // initial `state` event so the store re-syncs without action here.
    };
  }

  function dismissNextEpisode() { setNextEpisode(null); }
  function dismissTranscodePrompt() { setTranscodePrompt(null); }
  // Dismiss button: hide the card AND opt out of auto-advance this episode.
  function cancelBinge() {
    bingeCancelled = true;
    setNextEpisode(null);
  }
  function clearEpisodeOver() { setEpisodeOver(null); }

  return {
    state,
    nextEpisode,
    transcodePrompt,
    endedAt,
    episodeOver,
    connect,
    dismissNextEpisode,
    dismissTranscodePrompt,
    cancelBinge,
    clearEpisodeOver,
  };
}

export const playbackStore = createRoot(createPlaybackStore);
