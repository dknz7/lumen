import { Show, createEffect, onCleanup } from "solid-js";
// hls.js/light — slimmer build (no DRM / alt-audio / advanced features); the
// full library is ~160 kB gzipped, the light build is ~50 kB. Trailer
// playback only needs basic HLS fetch + MSE; nothing more. The light
// build ships without its own .d.ts so we re-use the main package's types
// (light is API-compatible — a strict subset).
// @ts-expect-error — hls.js/light has no bundled type declarations.
import Hls from "hls.js/light";
import type HlsType from "hls.js";
import ModalShell from "./ModalShell";
import "./HLSTrailerModal.css";

export interface HLSTrailerModalProps {
  open: boolean;
  onClose: () => void;
  hlsUrl?: string;
  title?: string;
}

// HLSTrailerModal plays clip-type hub trailers (Trending Trailers / New
// Trailers shelves) directly inside the SPA via HTML5 <video>. Safari and
// most Chromium builds play HLS natively; Firefox needs hls.js to attach.
// Mirrors TrailerModal's shell + behaviour: backdrop, Escape close, Motion
// entrance, src cleared on close so playback halts.
export default function HLSTrailerModal(props: HLSTrailerModalProps) {
  let videoRef: HTMLVideoElement | undefined;
  let hlsInstance: HlsType | undefined;

  function teardown() {
    if (hlsInstance) {
      hlsInstance.destroy();
      hlsInstance = undefined;
    }
    if (videoRef) {
      videoRef.pause();
      videoRef.removeAttribute("src");
      videoRef.load();
    }
  }

  createEffect(() => {
    const url = props.hlsUrl;
    if (!props.open || !url || !videoRef) {
      teardown();
      return;
    }
    // Reattach if URL changed: tear down any prior session first.
    teardown();
    if (videoRef.canPlayType("application/vnd.apple.mpegurl")) {
      // Native HLS path — Safari, most Chromium builds.
      videoRef.src = url;
    } else if (Hls.isSupported()) {
      const hls = new Hls();
      hls.loadSource(url);
      hls.attachMedia(videoRef);
      hlsInstance = hls;
    }
    // If neither path is available the <video> stays empty and the user
    // sees a blank player; rare enough (Firefox Mobile in restricted
    // configs) that we don't bother with extra fallback UI.
  });

  onCleanup(teardown);

  return (
    <ModalShell open={props.open} onCancel={props.onClose} ariaLabel={props.title ?? "Trailer"}>
      <div class="hls-trailer-modal">
        <header class="hls-trailer-header">
          <h3>{props.title ?? "Trailer"}</h3>
          <button class="hls-trailer-close" onClick={props.onClose} aria-label="Close trailer">×</button>
        </header>
        <Show when={props.hlsUrl} fallback={
          <div class="hls-trailer-empty">No trailer available.</div>
        }>
          <video
            ref={videoRef}
            class="hls-trailer-video"
            controls
            autoplay
            playsinline
          />
        </Show>
      </div>
    </ModalShell>
  );
}
