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
 * object reference for any item whose key (typically ratingKey) matches AND
 * whose data is unchanged. `<For>` then sees stable refs and skips remount
 * for those. New keys mount fresh, missing keys unmount, and an item whose
 * data changed takes the fresh object, so only that one card remounts.
 *
 * The "data is unchanged" check is what keeps this honest. It used to reuse
 * the old object on key match alone, which froze every item's data for as
 * long as the view stayed mounted: a refetch added new items but left each
 * existing card showing whatever it first loaded, so watched state, titles
 * and posters went stale and the grid became a mix of fresh and frozen.
 * Lumen lives in the tray for days, so "as long as the view stayed mounted"
 * was days. Equality is by JSON, which is exact for API payloads: they are
 * plain JSON data (parsed responses, or spread copies of them, like Home's
 * Continue Watching items) and no field changes between identical fetches.
 * Items must not be mutated in place: each one's JSON is computed once.
 *
 * Usage:
 *   const [items] = createResource<HubItem[]>(() => api.hub("home", slug));
 *   const stable = stableArrayByKey(() => items() ?? [], (it) => it.ratingKey);
 *   <For each={stable()}>{...}</For>
 */
export function stableArrayByKey<T extends object>(
  source: () => T[] | undefined,
  keyOf: (item: T) => string,
): () => T[] {
  // Serialised form of each object, computed once per object. Weak, so
  // objects that drop out of the list are collected along with their entry.
  const serialised = new WeakMap<T, string>();
  const sig = (it: T): string => {
    let s = serialised.get(it);
    if (s === undefined) {
      s = JSON.stringify(it);
      serialised.set(it, s);
    }
    return s;
  };

  return createMemo<T[]>((prev) => {
    const next = source() ?? [];
    if (!prev || prev.length === 0) return next;
    if (next.length === 0) return next;
    const prevByKey = new Map<string, T>();
    for (const it of prev) prevByKey.set(keyOf(it), it);
    let allSame = next.length === prev.length;
    const result: T[] = next.map((it, i) => {
      const old = prevByKey.get(keyOf(it));
      const stable = old !== undefined && sig(old) === sig(it) ? old : it;
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
