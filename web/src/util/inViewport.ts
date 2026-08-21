import { createSignal, onCleanup, type Accessor } from "solid-js";

/**
 * createInViewport reports whether an element has entered the viewport.
 *
 * Latches: once true, it stays true. Callers use it to trigger a one-off fetch,
 * and un-setting it on scroll-away would cancel work already done (and thrash
 * as the user scrolls back and forth).
 *
 * rootMargin pre-loads a screen ahead so data is usually there by the time the
 * card is actually looked at.
 */
export function createInViewport(
  el: Accessor<Element | undefined>,
  rootMargin = "600px",
): Accessor<boolean> {
  const [visible, setVisible] = createSignal(false);

  // IntersectionObserver is unavailable in some test/JSDOM environments; there,
  // treating everything as visible preserves the previous eager behaviour
  // rather than silently loading nothing.
  if (typeof IntersectionObserver === "undefined") {
    setVisible(true);
    return visible;
  }

  const observer = new IntersectionObserver(
    (entries) => {
      for (const e of entries) {
        if (e.isIntersecting) {
          setVisible(true);
          observer.disconnect();
          return;
        }
      }
    },
    { rootMargin },
  );

  // Poll for the element on the next frame: refs are assigned after the
  // component body runs.
  queueMicrotask(() => {
    const node = el();
    if (node) observer.observe(node);
    else setVisible(true); // no element to observe — don't strand the caller
  });

  onCleanup(() => observer.disconnect());
  return visible;
}
