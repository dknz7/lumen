import { createEffect, createSignal, onCleanup, type Accessor } from "solid-js";
import { api } from "../api/client";
import type { Match } from "../api/types";

// Viewport-driven availability lookups.
//
// The Watchlist used to fire one /api/availability call per card the moment the
// page mounted: on a 528-item watchlist that was 528 concurrent requests taking
// ~42 seconds to settle, re-issued on every window focus, each one fanning out
// to every configured Plex server. Batching alone doesn't fix it — the backend
// still makes one round trip per (guid, server), and a remote server makes that
// latency-bound rather than something concurrency can flatten.
//
// So: stop asking about things nobody is looking at. Cards request availability
// when they scroll into view, requests coalesce into chunks, and results are
// cached until the next lumen:data-invalidated. Someone who never scrolls past
// the first screen costs one request instead of five hundred.
//
// The cache used to live for the whole session, and Lumen lives in the tray
// for days, so a title that landed on a server after the first lookup kept
// its old "not in your library" state until F5. Now an invalidation (which the
// shell fires on every restore from tray or minimise) drops the cache, and
// cards that are on screen ask again. Off-screen cards ask when they scroll
// in, so a refresh costs one batch for what's visible, not the whole list.

const CHUNK = 40;
const DEBOUNCE_MS = 120;

const cache = new Map<string, Match[]>();
const listeners = new Map<string, Set<(m: Match[]) => void>>();
const queued = new Set<string>();
const inFlight = new Set<string>();
let timer: number | undefined;

// Bumped on every invalidation. A batch remembers the generation it was sent
// in; if an invalidation lands while it's in flight, its answer predates the
// refresh, so it's dropped and the guids are asked again.
let generation = 0;
const [epoch, setEpoch] = createSignal(0);

window.addEventListener("lumen:data-invalidated", () => invalidateAvailability());

function scheduleFlush() {
  if (timer === undefined) timer = window.setTimeout(flush, DEBOUNCE_MS);
}

function resolve(guid: string, matches: Match[]) {
  const set = listeners.get(guid);
  if (set) {
    // Copy before iterating: a listener may unsubscribe as it runs.
    Array.from(set).forEach((fn) => fn(matches));
    listeners.delete(guid);
  }
}

function flush() {
  timer = undefined;
  if (queued.size === 0) return;

  const batch = Array.from(queued).slice(0, CHUNK);
  batch.forEach((g) => {
    queued.delete(g);
    inFlight.add(g);
  });
  const sentIn = generation;
  // Re-ask for anything still being watched, once this batch is out of the
  // way (enqueue skips guids that are in flight).
  const requeueWatched = () =>
    batch.forEach((g) => {
      if (listeners.has(g)) queued.add(g);
    });

  void api
    .availabilityBatch(batch)
    .then((map) => {
      if (sentIn !== generation) return requeueWatched();
      for (const guid of batch) {
        const matches = map[guid] ?? [];
        cache.set(guid, matches);
        resolve(guid, matches);
      }
    })
    .catch((e) => {
      // Availability is an enhancement — a failure should grey out a Play
      // button, never wedge the page. Resolve empty so nothing spins forever.
      console.error("availability batch failed:", e);
      if (sentIn !== generation) return requeueWatched();
      batch.forEach((guid) => resolve(guid, []));
    })
    .finally(() => {
      batch.forEach((g) => inFlight.delete(g));
      if (queued.size > 0) scheduleFlush();
    });

  if (queued.size > 0) scheduleFlush(); // anything beyond CHUNK
}

/**
 * Drops cached results; on-screen cards ask again. Runs on every
 * lumen:data-invalidated, and is also callable directly.
 */
export function invalidateAvailability() {
  generation++;
  cache.clear();
  setEpoch(generation);
}

/**
 * useAvailability resolves matches for a guid, but only once `active` is true —
 * i.e. once the card is actually on screen.
 *
 * Returns undefined while unresolved, then an array (empty means "not on any
 * of your servers").
 */
export function useAvailability(
  guid: Accessor<string | undefined>,
  active: Accessor<boolean>,
): Accessor<Match[] | undefined> {
  const [value, setValue] = createSignal<Match[] | undefined>(undefined);
  let subscribedTo: string | undefined;

  function unsubscribe() {
    if (!subscribedTo) return;
    const set = listeners.get(subscribedTo);
    set?.delete(setValue);
    if (set && set.size === 0) listeners.delete(subscribedTo);
    subscribedTo = undefined;
  }

  createEffect(() => {
    epoch(); // re-run after an invalidation
    const g = guid();
    if (!g || !active()) return;

    const hit = cache.get(g);
    if (hit) {
      setValue(hit);
      return;
    }
    if (subscribedTo !== g) {
      unsubscribe();
      subscribedTo = g;
    }
    // Always (re)register: resolve() drops a guid's listeners once answered,
    // so after an invalidation the card has to sign up again. Until the new
    // answer lands the card keeps showing the old one, so nothing flickers.
    let set = listeners.get(g);
    if (!set) listeners.set(g, (set = new Set()));
    set.add(setValue);
    enqueue(g);
  });

  onCleanup(unsubscribe);
  return value;
}

function enqueue(guid: string) {
  if (cache.has(guid) || inFlight.has(guid) || queued.has(guid)) return;
  queued.add(guid);
  scheduleFlush();
}
