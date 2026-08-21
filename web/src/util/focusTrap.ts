import { onCleanup, createEffect } from "solid-js";

// Modal focus management.
//
// Lumen's modals set role="dialog" but did nothing else: opening Settings left
// focus on the trigger button, the first Tab landed on a control BEHIND the
// backdrop, and 25 tabs later focus still hadn't entered the dialog. Because
// the modal renders at the end of the DOM, a keyboard user had to traverse the
// entire page — hundreds of stops — to reach it. Closing dropped focus onto
// <body>, losing the user's place entirely.
//
// This does the three things a dialog owes a keyboard user:
//   1. move focus in on open,
//   2. keep Tab inside while open,
//   3. put focus back where it came from on close.

const FOCUSABLE = [
  "a[href]",
  "button:not([disabled])",
  "input:not([disabled]):not([type='hidden'])",
  "select:not([disabled])",
  "textarea:not([disabled])",
  "[tabindex]:not([tabindex='-1'])",
].join(",");

function focusableWithin(root: HTMLElement): HTMLElement[] {
  return Array.from(root.querySelectorAll<HTMLElement>(FOCUSABLE)).filter((el) => {
    // offsetParent is null for display:none; a zero-size rect catches
    // visibility:hidden and collapsed containers.
    if (el.offsetParent === null && getComputedStyle(el).position !== "fixed") return false;
    const r = el.getBoundingClientRect();
    return r.width > 0 || r.height > 0;
  });
}

export interface FocusTrapOptions {
  /** The dialog element. Returned lazily because it mounts after open flips. */
  container: () => HTMLElement | undefined;
  /** Whether the modal is currently open. */
  isOpen: () => boolean;
}

export function createFocusTrap(opts: FocusTrapOptions) {
  let previouslyFocused: HTMLElement | null = null;

  function onKeyDown(e: KeyboardEvent) {
    if (e.key !== "Tab") return;
    const root = opts.container();
    if (!root) return;

    const items = focusableWithin(root);
    if (items.length === 0) {
      // Nothing focusable inside — keep focus on the dialog rather than
      // letting Tab escape to the page behind the backdrop.
      e.preventDefault();
      root.focus();
      return;
    }

    const first = items[0];
    const last = items[items.length - 1];
    const active = document.activeElement as HTMLElement | null;

    // Focus escaped the dialog somehow (clicked the backdrop, say) — pull it back.
    if (!active || !root.contains(active)) {
      e.preventDefault();
      first.focus();
      return;
    }
    if (e.shiftKey && active === first) {
      e.preventDefault();
      last.focus();
    } else if (!e.shiftKey && active === last) {
      e.preventDefault();
      first.focus();
    }
  }

  createEffect(() => {
    if (opts.isOpen()) {
      previouslyFocused = document.activeElement as HTMLElement | null;
      document.addEventListener("keydown", onKeyDown, true);

      // The dialog mounts in the same tick that `open` flips, so wait a frame
      // for the element (and Motion's entry animation) before focusing.
      requestAnimationFrame(() => {
        const root = opts.container();
        if (!root) return;
        const items = focusableWithin(root);
        (items[0] ?? root).focus();
      });
    } else {
      document.removeEventListener("keydown", onKeyDown, true);
      // Return focus to whatever opened the modal — but only if it's still in
      // the document and still focusable.
      if (previouslyFocused && document.contains(previouslyFocused)) {
        previouslyFocused.focus();
      }
      previouslyFocused = null;
    }
  });

  onCleanup(() => document.removeEventListener("keydown", onKeyDown, true));
}

// --- Escape handling -------------------------------------------------------
//
// Every modal bound its own document-level keydown listener, and SettingsModal
// forgot to check props.open at all — so Escape anywhere in the app called its
// onClose. With ReAuth open on top of Settings, one Escape closed both.
//
// A shared stack fixes the ordering: Escape only reaches the topmost modal.

const escapeStack: Array<() => void> = [];

function onDocumentKeyDown(e: KeyboardEvent) {
  if (e.key !== "Escape" || escapeStack.length === 0) return;
  e.stopPropagation();
  escapeStack[escapeStack.length - 1]();
}

/**
 * Registers an Escape handler that only fires while this modal is the topmost
 * open one.
 */
export function createEscapeHandler(isOpen: () => boolean, onEscape: () => void) {
  let registered: (() => void) | null = null;

  function unregister() {
    if (!registered) return;
    const i = escapeStack.lastIndexOf(registered);
    if (i >= 0) escapeStack.splice(i, 1);
    registered = null;
    if (escapeStack.length === 0) {
      document.removeEventListener("keydown", onDocumentKeyDown, true);
    }
  }

  createEffect(() => {
    if (isOpen()) {
      if (registered) return;
      registered = onEscape;
      if (escapeStack.length === 0) {
        document.addEventListener("keydown", onDocumentKeyDown, true);
      }
      escapeStack.push(registered);
    } else {
      unregister();
    }
  });

  onCleanup(unregister);
}
