import type { Server, Library, Item, HubItem, Match, DiscoverItem } from "./types";
import { imageDims, type ImageDimPreset } from "../util/imageDims";

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

  watchlist: () => getJSON<import("./types").WatchlistItem[]>("/api/watchlist"),

  watchlistAdd: async (plexTvRatingKey: string) => {
    const res = await fetch("/api/watchlist/add", {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({ ratingKey: plexTvRatingKey }),
    });
    if (!res.ok) throw new Error(`${res.status} POST /api/watchlist/add: ${await res.text()}`);
    return res.json();
  },
  watchlistRemove: async (plexTvRatingKey: string) => {
    const res = await fetch("/api/watchlist/remove", {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({ ratingKey: plexTvRatingKey }),
    });
    if (!res.ok) throw new Error(`${res.status} POST /api/watchlist/remove: ${await res.text()}`);
    return res.json();
  },

  availability: (guid: string) =>
    getJSON<Match[]>(`/api/availability?guid=${encodeURIComponent(guid)}`),

  // Rich metadata for a plex.tv-source item (Recommended/Discover/Watchlist
  // tile click destination). Distinct from `item()` which hits a server-local
  // /api/items/<rk> — this hits discover.provider.plex.tv via the proxy.
  discoverItem: async (ratingKey: string): Promise<DiscoverItem> => {
    const res = await fetch(`/api/discover-item/${encodeURIComponent(ratingKey)}`);
    if (!res.ok) throw new Error(`${res.status} GET /api/discover-item/${ratingKey}`);
    return res.json();
  },

  imdb: async (id: string): Promise<import("./types").OMDBRating | null> => {
    const res = await fetch(`/api/imdb/${encodeURIComponent(id)}`);
    if (res.status === 404 || res.status === 503) return null;
    if (!res.ok) throw new Error(`${res.status} GET /api/imdb/${id}`);
    return res.json();
  },

  tmdbTrailer: async (imdbID: string, mediaType: "movie" | "show"): Promise<string | null> => {
    const res = await fetch(`/api/tmdb/trailer/${encodeURIComponent(imdbID)}?type=${mediaType}`);
    if (res.status === 404 || res.status === 503) return null;
    if (!res.ok) throw new Error(`${res.status} GET /api/tmdb/trailer/${imdbID}`);
    const body = await res.json();
    return body.youtubeID || null;
  },

  onDeck: (serverID: string) =>
    getJSON<Item[]>(`/api/servers/${encodeURIComponent(serverID)}/ondeck`),

  seasons: (serverID: string, showRatingKey: string) =>
    getJSON<import("./types").Season[]>(
      `/api/servers/${encodeURIComponent(serverID)}/seasons/${encodeURIComponent(showRatingKey)}`
    ),

  seasonEpisodes: (serverID: string, seasonRatingKey: string) =>
    getJSON<Item[]>(
      `/api/servers/${encodeURIComponent(serverID)}/seasons/${encodeURIComponent(seasonRatingKey)}/episodes`
    ),

  // Plex's native recently-added feed for a library. Matches Plex Web's Home
  // shelves — spec §12.1 "Recently Released" rows use this, not sort=addedAt.
  recentlyAdded: (serverID: string, libraryID: string, size = 20) =>
    getJSON<Item[]>(
      `/api/servers/${encodeURIComponent(serverID)}/libraries/${encodeURIComponent(libraryID)}/recentlyAdded?size=${size}`
    ),

  // Marks an item as fully watched on its Plex server (viewCount=1, advances
  // watched state, enters watch history). Used by the "Mark as Watched" tick
  // on Continue Watching cards.
  scrobble: async (serverID: string, ratingKey: string) => {
    const url = `/api/servers/${encodeURIComponent(serverID)}/scrobble?ratingKey=${encodeURIComponent(ratingKey)}`;
    const res = await fetch(url, { method: "POST" });
    if (!res.ok) {
      throw new Error(`${res.status} ${url}: ${await res.text()}`);
    }
    return res.json();
  },

  // Removes the show containing this item from Continue Watching via Plex's
  // first-class PUT /actions/removeFromContinueWatching endpoint. Pass the
  // EPISODE ratingKey for shows — Plex's logic handles the whole-show
  // removal semantic. Removal syncs cross-device (Plex Web, mobile, TV apps).
  removeFromCW: async (serverID: string, ratingKey: string) => {
    const url = `/api/servers/${encodeURIComponent(serverID)}/cw/remove?ratingKey=${encodeURIComponent(ratingKey)}`;
    const res = await fetch(url, { method: "POST" });
    if (!res.ok) {
      throw new Error(`${res.status} POST ${url}: ${await res.text()}`);
    }
    return res.json();
  },

  // Resets a Plex item's playback state (viewCount=0, viewOffset=0) via the
  // legacy /:/unscrobble endpoint. Kept for any future "reset progress"
  // power-user feature — the bin icon now uses removeFromCW instead, since
  // unscrobble alone doesn't actually evict items from Plex's onDeck.
  unscrobble: async (serverID: string, ratingKey: string) => {
    const url = `/api/servers/${encodeURIComponent(serverID)}/unscrobble?ratingKey=${encodeURIComponent(ratingKey)}`;
    const res = await fetch(url, { method: "POST" });
    if (!res.ok) {
      throw new Error(`${res.status} ${url}: ${await res.text()}`);
    }
    return res.json();
  },

  // Session 2 just needs the path-building helper — actual images are <img src=...>.
  image: (serverID: string, path: string, preset?: ImageDimPreset) => {
    const base = `/api/image-proxy?server=${encodeURIComponent(serverID)}&path=${encodeURIComponent(path)}`;
    if (!preset) return base;
    const { w, h } = imageDims[preset];
    return `${base}&w=${w}&h=${h}`;
  },

  // Asks the lumen.exe process to gracefully shut down. Used by the "Close
  // Lumen" confirmation in the top bar. The server flushes a 200 then exits
  // ~150 ms later, so the returned promise may reject on the network tear-down
  // — callers should `.catch(() => {})` rather than awaiting strictly.
  authStart: async (): Promise<{ pinId: number; code: string; linkURL: string }> => {
    const res = await fetch("/api/auth/start", { method: "POST" });
    if (!res.ok) throw new Error(`${res.status} POST /api/auth/start`);
    return res.json();
  },
  authPoll: async (): Promise<{ status: "none" | "pending" | "linked" | "expired" }> => {
    const res = await fetch("/api/auth/poll", { method: "POST" });
    if (res.status === 410) return { status: "expired" };
    if (!res.ok) throw new Error(`${res.status} POST /api/auth/poll`);
    return res.json();
  },

  quit: async () => {
    const res = await fetch("/api/quit", { method: "POST" });
    if (!res.ok) {
      throw new Error(`${res.status} POST /api/quit: ${await res.text()}`);
    }
    return res.json();
  },

  play: async (serverID: string, ratingKey: string, resumeFromOffset?: number) => {
    const res = await fetch("/api/play", {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({ serverID, ratingKey, resumeFromOffset }),
    });
    if (!res.ok) {
      throw new Error(`${res.status} POST /api/play: ${await res.text()}`);
    }
    return res.json();
  },

  playTranscode: async (serverID: string, ratingKey: string) => {
    const res = await fetch("/api/play/transcode", {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({ serverID, ratingKey }),
    });
    if (!res.ok) {
      throw new Error(`${res.status} POST /api/play/transcode: ${await res.text()}`);
    }
    return res.json();
  },

  playStop: async () => {
    const res = await fetch("/api/play/stop", { method: "POST" });
    if (!res.ok) {
      throw new Error(`${res.status} POST /api/play/stop: ${await res.text()}`);
    }
    return res.json();
  },

  playbackState: async () => {
    const res = await fetch("/api/playback");
    if (!res.ok) throw new Error(`GET /api/playback: ${res.status}`);
    return res.json() as Promise<import("./types").PlaybackState>;
  },
};
