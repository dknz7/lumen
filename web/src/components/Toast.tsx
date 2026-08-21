import { createSignal, For, onCleanup, Show } from "solid-js";
// @ts-expect-error — motionone/solid's package.json exports field hides its d.ts file
import { Motion, Presence } from "@motionone/solid";
import { X } from "./icons";
import "./Toast.css";

// Lumen used to report every failure with alert(). That is a blocking, modal,
// OS-chrome dialog with no styling, no stacking, and no way to dismiss several
// at once — and inside a WebView2 window it looks even more out of place than
// it does in a browser. One of them ("No trailer available") wasn't even an
// error, just a normal outcome punished with a system dialog.
//
// This is the replacement: non-blocking, stacked, auto-dismissing, and it
// cannot interrupt playback or trap focus.

export type ToastKind = "error" | "success" | "info";

interface Toast {
  id: number;
  kind: ToastKind;
  message: string;
  /** Optional action, e.g. "Retry". Dismisses the toast when clicked. */
  action?: { label: string; run: () => void };
}

let nextID = 1;
const [toasts, setToasts] = createSignal<Toast[]>([]);

// Errors linger — the user may have been looking elsewhere. Confirmations go
// quickly; they are reassurance, not information.
const TTL: Record<ToastKind, number> = {
  error: 7000,
  success: 2500,
  info: 4000,
};

function push(kind: ToastKind, message: string, action?: Toast["action"]) {
  const id = nextID++;
  setToasts((list) => [...list, { id, kind, message, action }]);
  setTimeout(() => dismiss(id), TTL[kind]);
  return id;
}

export function dismiss(id: number) {
  setToasts((list) => list.filter((t) => t.id !== id));
}

/**
 * toast — the app-wide notifier.
 *
 *   toast.error("Couldn't start playback", { label: "Retry", run: play })
 *   toast.success("Added to Watchlist")
 *   toast.info("No trailer available for this title")
 */
export const toast = {
  error: (message: string, action?: Toast["action"]) => push("error", message, action),
  success: (message: string) => push("success", message),
  info: (message: string) => push("info", message),
};

/**
 * errorMessage turns a thrown value into something worth showing a person.
 *
 * The API client throws `${status} ${url}: ${body}` where body is often JSON
 * like {"error":"..."}. Showing that raw is how you get a toast that reads
 * `500 /api/play: {"error":"pot player not found"}`.
 */
export function errorMessage(e: unknown, fallback = "Something went wrong"): string {
  if (!(e instanceof Error)) return fallback;
  const m = e.message;
  const jsonStart = m.indexOf("{");
  if (jsonStart >= 0) {
    try {
      const parsed = JSON.parse(m.slice(jsonStart));
      if (parsed && typeof parsed.error === "string" && parsed.error) {
        return parsed.error.charAt(0).toUpperCase() + parsed.error.slice(1);
      }
    } catch {
      // Not JSON after all — fall through to the raw message.
    }
  }
  // Strip a leading "404 /api/whatever: " prefix, which means nothing to a user.
  const stripped = m.replace(/^\d{3}\s+\/\S*:\s*/, "").trim();
  return stripped || fallback;
}

/** ToastHost renders the stack. Mount once, near the root. */
export default function ToastHost() {
  onCleanup(() => setToasts([]));

  return (
    <div class="toast-host" role="region" aria-label="Notifications">
      <Presence>
        <For each={toasts()}>
          {(t) => (
            <Motion.div
              class={`toast toast--${t.kind}`}
              // Errors interrupt; assertive is right for them. Successes should
              // not talk over whatever a screen reader is currently saying.
              role={t.kind === "error" ? "alert" : "status"}
              aria-live={t.kind === "error" ? "assertive" : "polite"}
              initial={{ opacity: 0, y: 12, scale: 0.97 }}
              animate={{ opacity: 1, y: 0, scale: 1 }}
              exit={{ opacity: 0, y: 8, scale: 0.97 }}
              transition={{ duration: 0.2, easing: [0.22, 1, 0.36, 1] }}
            >
              <span class="toast-message">{t.message}</span>
              <Show when={t.action}>
                {(a) => (
                  <button
                    class="toast-action"
                    onClick={() => {
                      dismiss(t.id);
                      a().run();
                    }}
                  >
                    {a().label}
                  </button>
                )}
              </Show>
              <button
                class="toast-dismiss"
                onClick={() => dismiss(t.id)}
                aria-label="Dismiss notification"
              >
                <X size={13} />
              </button>
            </Motion.div>
          )}
        </For>
      </Presence>
    </div>
  );
}
