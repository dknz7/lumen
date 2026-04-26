import { onCleanup, onMount } from "solid-js";

/**
 * Run the given refetch function whenever the browser window regains focus
 * or the page becomes visible. Used to keep Lumen's data fresh after the
 * user switches to Plex Web / Plex Desktop / etc. and changes state there.
 *
 * Both `focus` and `visibilitychange` are listened to because:
 *  - `focus` fires when the browser window regains focus (alt-tab).
 *  - `visibilitychange` fires when the tab itself becomes visible (Chrome's
 *    background-tab throttling can suppress focus on tab switch).
 */
export function refetchOnFocus(refetch: () => void) {
  onMount(() => {
    const onFocus = () => refetch();
    const onVisible = () => {
      if (document.visibilityState === "visible") refetch();
    };
    window.addEventListener("focus", onFocus);
    document.addEventListener("visibilitychange", onVisible);
    onCleanup(() => {
      window.removeEventListener("focus", onFocus);
      document.removeEventListener("visibilitychange", onVisible);
    });
  });
}
