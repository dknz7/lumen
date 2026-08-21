import { createMemo } from "solid-js";

/**
 * Maintain stable item identity across refetches of a resource-backed array.
 *
 * Solid's `<For>` is keyed by reference equality — when a refetch resolves
 * with a new array of newly-parsed objects, every item's reference differs
 * from the previous render even when the underlying data is identical.
 * `<For>` then unmounts and remounts every child, which:
 *   - Drops in-flight click events (mousedown on old DOM, mouseup on new
 *     DOM → browser doesn't fire click). an earlier session: this manifested as
 *     "double-click required" on Discover tiles after window-focus refetch.
 *   - Causes a visible flicker as posters re-fetch / reattach.
 *   - Wastes DOM work proportional to list size.
 *
 * stableArrayByKey memoises the array so each refetch reuses the previous
 * object reference for any item whose key (typically ratingKey) matches.
 * `<For>` then sees stable refs and skips remount entirely. New keys mount
 * fresh, missing keys unmount — same as before. The trade-off: if the
 * server-side data for an existing key changes (e.g. a tile's title or
 * thumb is updated), the SPA continues showing the previous snapshot until
 * the key churns. Acceptable for marketing-style hub data; reactive child
 * state (watchlist set, etc.) updates separately and isn't gated by this.
 *
 * Usage:
 *   const [items] = createResource<HubItem[]>(() => api.hub("home", slug));
 *   const stable = stableArrayByKey(() => items() ?? [], (it) => it.ratingKey);
 *   <For each={stable()}>{...}</For>
 */
export function stableArrayByKey<T>(
  source: () => T[] | undefined,
  keyOf: (item: T) => string,
): () => T[] {
  return createMemo<T[]>((prev) => {
    const next = source() ?? [];
    if (!prev || prev.length === 0) return next;
    if (next.length === 0) return next;
    const prevByKey = new Map<string, T>();
    for (const it of prev) prevByKey.set(keyOf(it), it);
    let allSame = next.length === prev.length;
    const result: T[] = next.map((it, i) => {
      const stable = prevByKey.get(keyOf(it)) ?? it;
      if (allSame && stable !== prev[i]) allSame = false;
      return stable;
    });
    // When every item matches prev by key AND position, return the
    // PREVIOUS array reference so downstream memos (Shelf's pages slicing,
    // <For> keying) see no change at all and short-circuit. This is what
    // actually prevents the focus-refetch click-loss bug — the chain only
    // remounts when something legitimately changed.
    return allSame ? prev : result;
  }, [] as T[]);
}
