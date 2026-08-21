import { Show } from "solid-js";
// @ts-expect-error — motionone/solid's package.json exports field hides its d.ts file
import { Motion, Presence } from "@motionone/solid";
import { createEscapeHandler, createFocusTrap } from "../util/focusTrap";
import "./CloseConfirmModal.css";

export default function CloseConfirmModal(props: {
  open: boolean;
  onCancel: () => void;
  onConfirm: () => void;
}) {
  let dialogRef: HTMLDivElement | undefined;

  createEscapeHandler(() => props.open, () => props.onCancel());
  createFocusTrap({ container: () => dialogRef, isOpen: () => props.open });

  return (
    <Presence>
      <Show when={props.open}>
        <Motion.div
          class="close-confirm-backdrop"
          onClick={props.onCancel}
          role="presentation"
          initial={{ opacity: 0 }}
          animate={{ opacity: 1 }}
          exit={{ opacity: 0 }}
          transition={{ duration: 0.18, easing: [0.16, 1, 0.3, 1] }}
        >
          <Motion.div
            class="close-confirm-modal"
            ref={dialogRef}
            tabindex="-1"
            onClick={(e: Event) => e.stopPropagation()}
            role="dialog"
            aria-modal="true"
            aria-label="Close Lumen"
            initial={{ opacity: 0, scale: 0.96, y: 8 }}
            animate={{ opacity: 1, scale: 1, y: 0 }}
            exit={{ opacity: 0, scale: 0.96, y: 8 }}
            transition={{ duration: 0.24, easing: [0.22, 1, 0.36, 1] }}
          >
            <h2 class="close-confirm-title">Close Lumen?</h2>
            <p class="close-confirm-body">
              This will end the lumen.exe process and close this window.
            </p>
            <div class="close-confirm-actions">
              <button class="close-confirm-cancel" onClick={props.onCancel}>
                Cancel
              </button>
              <button class="close-confirm-confirm" onClick={props.onConfirm}>
                Close Lumen
              </button>
            </div>
          </Motion.div>
        </Motion.div>
      </Show>
    </Presence>
  );
}
