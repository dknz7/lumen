import { onCleanup, onMount } from "solid-js";

/**
 * Run the given refetch function whenever the browser window regains focus,
 * the page becomes visible, OR a global lumen:data-invalidated event fires.
 *
 * - focus / visibilitychange: cross-tab sync (user changed state in Plex Web,
 *   came back to Lumen tab → refetch).
 * - lumen:data-invalidated: within-Lumen mutation sync (user clicked Mark
 *   Watched, Pot Player closed, etc. → refetch all subscribers).
 *
 * Mutation paths dispatch the event via:
 *   window.dispatchEvent(new CustomEvent('lumen:data-invalidated'));
 */
export function refetchOnFocus(refetch: () => void) {
  onMount(() => {
    const onFocus = () => refetch();
    const onVisible = () => {
      if (document.visibilityState === "visible") refetch();
    };
    const onInvalidated = () => refetch();

    window.addEventListener("focus", onFocus);
    document.addEventListener("visibilitychange", onVisible);
    window.addEventListener("lumen:data-invalidated", onInvalidated);

    onCleanup(() => {
      window.removeEventListener("focus", onFocus);
      document.removeEventListener("visibilitychange", onVisible);
      window.removeEventListener("lumen:data-invalidated", onInvalidated);
    });
  });
}
