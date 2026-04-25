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
      if (evt && evt.type === "next-episode-prompt") setNextEpisode(evt.payload);
    });

    es.addEventListener("transcode-prompt", (ev) => {
      const evt = parse(ev);
      if (evt && evt.type === "transcode-prompt") setTranscodePrompt(evt.payload);
    });

    es.addEventListener("stopped", () => {
      setState({ active: false, position: 0, duration: 0, state: "stopped" });
    });

    es.onerror = (e) => {
      console.error("playback SSE error", e);
      // Browser auto-reconnects. On reconnect, the server sends a fresh
      // initial `state` event so the store re-syncs without action here.
    };
  }

  function dismissNextEpisode() { setNextEpisode(null); }
  function dismissTranscodePrompt() { setTranscodePrompt(null); }

  return {
    state,
    nextEpisode,
    transcodePrompt,
    endedAt,
    connect,
    dismissNextEpisode,
    dismissTranscodePrompt,
  };
}

export const playbackStore = createRoot(createPlaybackStore);
