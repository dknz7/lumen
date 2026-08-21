import { api } from "../api/client";
import type { Library } from "../api/types";

// A per-server library cache.
//
// A library list changes when someone adds a library to their Plex server —
// i.e. almost never — but Lumen was re-fetching it constantly. Measured on one
// Home page load: /api/servers/<id>/libraries requested 17 times (LeftMenu,
// the hidden-libraries resolver, and one per Recently-Added shelf, each with
// its own createResource), and 4-5 times on every other route.
//
// In-flight requests are shared, not just completed ones, so ten components
// mounting together produce one request rather than ten.

const cache = new Map<string, Promise<Library[]>>();

/** Returns the server's libraries, fetching at most once per server. */
export function librariesFor(serverID: string): Promise<Library[]> {
  const hit = cache.get(serverID);
  if (hit) return hit;

  const p = api.libraries(serverID).catch((e) => {
    // Don't cache a failure — a transient error would otherwise be permanent
    // for the rest of the session.
    cache.delete(serverID);
    throw e;
  });
  cache.set(serverID, p);
  return p;
}

/**
 * Drops cached libraries. Call after anything that can change the set of
 * servers or their libraries — /api/servers/refresh, re-authentication.
 */
export function invalidateLibraries(serverID?: string) {
  if (serverID) cache.delete(serverID);
  else cache.clear();
}
