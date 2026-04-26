import { Show, createSignal, onCleanup } from "solid-js";
import ModalShell from "./ModalShell";
import { api } from "../../api/client";
import "./ReAuthModal.css";

export interface ReAuthModalProps {
  open: boolean;
  onClose: () => void;
  onLinked: () => void;
}

export default function ReAuthModal(props: ReAuthModalProps) {
  const [code, setCode] = createSignal<string>("");
  const [linkURL, setLinkURL] = createSignal<string>("");
  const [status, setStatus] = createSignal<"idle" | "polling" | "linked" | "expired" | "error">("idle");
  const [errMsg, setErrMsg] = createSignal<string>("");
  let pollTimer: number | undefined;

  async function start() {
    setStatus("polling");
    setErrMsg("");
    try {
      const r = await api.authStart();
      setCode(r.code);
      setLinkURL(r.linkURL);
      window.open(r.linkURL, "_blank", "noreferrer");
      pollLoop();
    } catch (e) {
      setStatus("error");
      setErrMsg((e as Error).message);
    }
  }

  function pollLoop() {
    pollTimer = window.setInterval(async () => {
      try {
        const r = await api.authPoll();
        if (r.status === "linked") {
          window.clearInterval(pollTimer);
          setStatus("linked");
          props.onLinked();
        } else if (r.status === "expired") {
          window.clearInterval(pollTimer);
          setStatus("expired");
        }
      } catch (e) {
        // Transient — keep polling. The /api/auth/poll handler bridges
        // brief network blips itself.
        console.warn("auth poll error", e);
      }
    }, 2000);
  }

  onCleanup(() => {
    if (pollTimer) window.clearInterval(pollTimer);
  });

  return (
    <ModalShell open={props.open} onCancel={props.onClose} ariaLabel="Re-authenticate with Plex">
      <div class="reauth-modal">
        <h3>Re-authenticate with Plex</h3>
        <Show when={status() === "idle"}>
          <p>Click Start to mint a Plex PIN. We'll open plex.tv/link in your browser — enter the code there.</p>
          <div class="reauth-actions">
            <button class="btn-primary" onClick={start}>Start</button>
            <button class="btn" onClick={props.onClose}>Cancel</button>
          </div>
        </Show>
        <Show when={status() === "polling"}>
          <p>Enter this code at <a href={linkURL()} target="_blank" rel="noreferrer">plex.tv/link</a>:</p>
          <div class="reauth-code">{code()}</div>
          <p class="reauth-hint">Waiting for you to link the PIN…</p>
          <button class="btn" onClick={props.onClose}>Cancel</button>
        </Show>
        <Show when={status() === "linked"}>
          <p>Linked successfully. You're authenticated.</p>
          <button class="btn-primary" onClick={props.onClose}>Done</button>
        </Show>
        <Show when={status() === "expired"}>
          <p>The PIN expired. Please try again.</p>
          <div class="reauth-actions">
            <button class="btn-primary" onClick={start}>Retry</button>
            <button class="btn" onClick={props.onClose}>Cancel</button>
          </div>
        </Show>
        <Show when={status() === "error"}>
          <p>Failed: {errMsg()}</p>
          <div class="reauth-actions">
            <button class="btn-primary" onClick={start}>Retry</button>
            <button class="btn" onClick={props.onClose}>Cancel</button>
          </div>
        </Show>
      </div>
    </ModalShell>
  );
}
