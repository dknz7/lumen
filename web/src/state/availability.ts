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
// cached for the session. Someone who never scrolls past the first screen
// costs one request instead of five hundred.

const CHUNK = 40;
const DEBOUNCE_MS = 120;

const cache = new Map<string, Match[]>();
const listeners = new Map<string, Set<(m: Match[]) => void>>();
const queued = new Set<string>();
const inFlight = new Set<string>();
let timer: number | undefined;

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

  void api
    .availabilityBatch(batch)
    .then((map) => {
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
      batch.forEach((guid) => resolve(guid, []));
    })
    .finally(() => {
      batch.forEach((g) => inFlight.delete(g));
      if (queued.size > 0) scheduleFlush();
    });

  if (queued.size > 0) scheduleFlush(); // anything beyond CHUNK
}

/** Clears the session cache. Call when the set of servers changes. */
export function invalidateAvailability() {
  cache.clear();
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
      let set = listeners.get(g);
      if (!set) listeners.set(g, (set = new Set()));
      set.add(setValue);
    }
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
