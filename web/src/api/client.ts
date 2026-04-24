import type { Server, Library, Item, HubItem, Match } from "./types";

async function getJSON<T>(url: string): Promise<T> {
  const res = await fetch(url);
  if (!res.ok) {
    const body = await res.text();
    throw new Error(`${res.status} ${url}: ${body}`);
  }
  return res.json() as Promise<T>;
}

export const api = {
  servers: () => getJSON<Server[]>("/api/servers"),

  libraries: (serverID: string) =>
    getJSON<Library[]>(`/api/servers/${encodeURIComponent(serverID)}/libraries`),

  items: (serverID: string, libraryID: string, opts?: { sort?: string; start?: number; size?: number; filters?: Record<string, string> }) => {
    const qs = new URLSearchParams();
    if (opts?.sort) qs.set("sort", opts.sort);
    if (opts?.start !== undefined) qs.set("start", String(opts.start));
    if (opts?.size !== undefined) qs.set("size", String(opts.size));
    if (opts?.filters) {
      // Server expects query params prefixed with "filter." — matches api_servers.go decoder.
      for (const [k, v] of Object.entries(opts.filters)) {
        qs.set(`filter.${k}`, v);
      }
    }
    const s = qs.toString();
    return getJSON<Item[]>(
      `/api/servers/${encodeURIComponent(serverID)}/libraries/${encodeURIComponent(libraryID)}/items${s ? "?" + s : ""}`
    );
  },

  item: (serverID: string, ratingKey: string) =>
    getJSON<Item>(`/api/items/${encodeURIComponent(ratingKey)}?server=${encodeURIComponent(serverID)}`),

  hub: (namespace: "home" | "watchlist", slug: string) =>
    getJSON<HubItem[]>(`/api/hubs/${namespace}/${encodeURIComponent(slug)}`),

  availability: (guid: string) =>
    getJSON<Match[]>(`/api/availability?guid=${encodeURIComponent(guid)}`),

  onDeck: (serverID: string) =>
    getJSON<Item[]>(`/api/servers/${encodeURIComponent(serverID)}/ondeck`),

  // Plex's native recently-added feed for a library. Matches Plex Web's Home
  // shelves — spec §12.1 "Recently Released" rows use this, not sort=addedAt.
  recentlyAdded: (serverID: string, libraryID: string, size = 20) =>
    getJSON<Item[]>(
      `/api/servers/${encodeURIComponent(serverID)}/libraries/${encodeURIComponent(libraryID)}/recentlyAdded?size=${size}`
    ),

  // Removes an item from the server's Continue Watching by scrobbling as watched.
  // Returns 200 on success. Throws via getJSON-like error on failure.
  removeFromOnDeck: async (serverID: string, ratingKey: string) => {
    const url = `/api/servers/${encodeURIComponent(serverID)}/ondeck/remove?ratingKey=${encodeURIComponent(ratingKey)}`;
    const res = await fetch(url, { method: "POST" });
    if (!res.ok) {
      throw new Error(`${res.status} ${url}: ${await res.text()}`);
    }
    return res.json();
  },

  // Session 2 just needs the path-building helper — actual images are <img src=...>.
  image: (serverID: string, path: string) =>
    `/api/image-proxy?server=${encodeURIComponent(serverID)}&path=${encodeURIComponent(path)}`,
};
