import { Show, createEffect, onCleanup } from "solid-js";
import ModalShell from "./ModalShell";
import "./TrailerModal.css";

export interface TrailerModalProps {
  open: boolean;
  onClose: () => void;
  youtubeID?: string;
  title?: string;
}

// TrailerModal embeds the YouTube player in an iframe when youtubeID is set.
// Plex-hosted (.flv) trailers are NOT supported in v1.0 — the user's call:
// Plex Extras YouTube IDs are the common case; if a server-hosted trailer
// is ever needed we'll handle the .flv case separately.
export default function TrailerModal(props: TrailerModalProps) {
  let iframeRef: HTMLIFrameElement | undefined;
  // Stop playback on close: clear the src so the iframe tears down cleanly.
  createEffect(() => {
    if (!props.open && iframeRef) {
      iframeRef.src = "about:blank";
    }
  });
  onCleanup(() => {
    if (iframeRef) iframeRef.src = "about:blank";
  });
  const src = () =>
    props.youtubeID
      ? `https://www.youtube-nocookie.com/embed/${encodeURIComponent(props.youtubeID)}?autoplay=1&modestbranding=1&rel=0`
      : "about:blank";
  return (
    <ModalShell open={props.open} onCancel={props.onClose} ariaLabel={props.title ?? "Trailer"}>
      <div class="trailer-modal">
        <header class="trailer-header">
          <h3>{props.title ?? "Trailer"}</h3>
          <button class="trailer-close" onClick={props.onClose} aria-label="Close trailer">×</button>
        </header>
        <Show when={props.youtubeID} fallback={
          <div class="trailer-empty">No trailer available.</div>
        }>
          <iframe
            ref={iframeRef}
            class="trailer-iframe"
            src={src()}
            allow="autoplay; encrypted-media; picture-in-picture"
            allowfullscreen
          />
        </Show>
      </div>
    </ModalShell>
  );
}
