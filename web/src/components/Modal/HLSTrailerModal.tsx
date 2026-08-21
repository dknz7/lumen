import { Show, createEffect, onCleanup } from "solid-js";
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
// hls.js/light — the slimmer build (no DRM, no alt-audio); trailer playback
// needs basic HLS fetch + MSE and nothing more. It has no bundled .d.ts, so
// types come from the main package, which it is a strict subset of.
//
// Loaded on demand rather than imported at module scope. Statically importing
// it here pulled ~354 kB of minified JS into the single initial chunk — via
// Discover and Recommended, which import this modal eagerly — for a feature
// most sessions never touch. Safari and most Chromium builds play HLS
// natively and never load it at all.
let hlsModule: Promise<{ default: typeof HlsType }> | undefined;
function loadHls() {
  // @ts-expect-error — hls.js/light has no bundled type declarations.
  hlsModule ??= import("hls.js/light");
  return hlsModule as Promise<{ default: typeof HlsType }>;
}

export default function HLSTrailerModal(props: HLSTrailerModalProps) {
  let videoRef: HTMLVideoElement | undefined;
  let hlsInstance: HlsType | undefined;
  // Bumped on every (re)attach. An async import that resolves after the user
  // has already closed or switched trailers must not attach to a dead video.
  let attachToken = 0;

  function teardown() {
    // Invalidate any pending async attach as well as the live instance.
    attachToken++;
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
    const video = videoRef;
    if (!props.open || !url || !video) {
      teardown();
      return;
    }
    // Reattach if the URL changed: tear down any prior session first.
    teardown();
    const token = ++attachToken;

    if (video.canPlayType("application/vnd.apple.mpegurl")) {
      // Native HLS — Safari and most Chromium builds, including WebView2.
      // hls.js is never fetched on this path.
      video.src = url;
      return;
    }

    void loadHls().then(({ default: Hls }) => {
      // Closed, or moved to a different trailer, while the chunk downloaded.
      if (token !== attachToken || !props.open) return;
      if (!Hls.isSupported()) return;
      const hls = new Hls();
      hls.loadSource(url);
      hls.attachMedia(video);
      hlsInstance = hls;
    });
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
