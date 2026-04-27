export interface Server {
  machineIdentifier: string;
  name: string;
  displayName: string;
  baseURL: string;
  status: "connected" | "offline";
}

export interface Library {
  id: string;
  key: string;
  title: string;
  type: string;
}

// Collection is one curated list inside a Plex library section. Used by
// Home.tsx's "server-collection" shelf to resolve a collection by title
// (e.g. Stargaze's "Trending Movies" / "Trending Shows" custom collections).
export interface Collection {
  ratingKey: string;
  title: string;
  type: string;
}

export interface Item {
  ratingKey: string;
  guid?: string;
  title: string;
  type: string; // "movie" | "show" | "season" | "episode"
  year?: number;
  summary?: string;
  thumb?: string;
  art?: string;
  duration?: number;     // media length in ms
  viewOffset?: number;   // resume position in ms (0 = unstarted)
  addedAt?: number;      // epoch seconds when added to library
  lastViewedAt?: number; // epoch seconds of most recent view
  // Episode-specific — populated only when type === "episode"
  index?: number;              // episode number
  parentIndex?: number;        // season number
  parentTitle?: string;
  parentThumb?: string;
  grandparentTitle?: string;   // show name
  grandparentThumb?: string;   // show poster
  grandparentArt?: string;     // show backdrop
  grandparentRatingKey?: string;
  viewCount?: number;          // 0 / undefined = unwatched, ≥1 = watched
  originallyAvailableAt?: string; // air date "YYYY-MM-DD"
  imdbId?: string;
  roles?: Person[];
  directors?: Person[];
  writers?: Person[];
  trailer?: TrailerInfo;
}

export interface Season {
  ratingKey: string;
  index: number;
  title: string;
  leafCount: number;
  viewedLeafCount: number;
  thumb?: string;
}

export interface HubItem {
  guid?: string;
  ratingKey: string;
  title: string;
  type: string;
  year?: number;
  thumb?: string; // absolute URL — render direct <img>, no proxy
  imdbId?: string;
  parentRatingKey?: string;
  grandparentRatingKey?: string;
  contentRating?: string;
  studio?: string;
  tagline?: string;
  addedAt?: number;
  originallyAvailableAt?: string;
  // Per-type display fields — Plex Web's MediaContainer.Meta.DisplayFields
  // varies by item type. Season tiles render parentTitle / title / date;
  // episode tiles render grandparentTitle / S{parentIndex}E{index} / date.
  parentTitle?: string;
  parentIndex?: number;
  index?: number;
  grandparentTitle?: string;
  // For clip-type hub items only: native HLS playback URL extracted from
  // Media[].Part[].key by the backend, qualified to an absolute URL with
  // the account token applied. Hand directly to <video src> or
  // hls.js loadSource(). Empty for non-clip items.
  hlsUrl?: string;
}

export interface Match {
  serverName: string;
  machineIdentifier: string;
  ratingKey: string;
  libraryName?: string;
  resolution: string;
  container: string;
  bitrate: number;
  size: number;
  codec?: string;
}

export interface PlaybackState {
  active: boolean;
  ratingKey?: string;
  serverID?: string;
  title?: string;
  showTitle?: string;
  position: number; // ns (Go time.Duration JSON encoding)
  duration: number; // ns
  state: "playing" | "paused" | "stopped" | "unknown";
  quality?: string;
  transcoding?: boolean;
  thumbPath?: string;
  episodeIndex?: number;
  seasonIndex?: number;
  addedAt?: number;
  originallyAvailableAt?: string;
}

export interface NextEpisodeInfo {
  ratingKey: string;
  serverID: string;
  title: string;
  season: number;
  episode: number;
  thumbPath?: string;
}

export interface TranscodePromptInfo {
  ratingKey: string;
  serverID: string;
  title: string;
  reason: string;
}

export type PlaybackEvent =
  | { type: "state"; state: PlaybackState }
  | { type: "ended" }
  | { type: "next-episode-prompt"; payload: NextEpisodeInfo }
  | { type: "transcode-prompt"; payload: TranscodePromptInfo }
  | { type: "stopped" };

export interface Person {
  id: number;
  name: string;
  tag?: string;
  thumb?: string;
}

export interface TrailerInfo {
  title?: string;
  plexKey?: string;
  youtubeID?: string;
}

export interface OMDBRating {
  imdbID: string;
  title?: string;
  year?: string;
  rated?: string;
  imdbRating?: string;
  imdbVotes?: string;
}

// One rating row from discover.provider.plex.tv — typically Plex returns
// 3-4 (RT critic + RT audience + IMDB audience + TMDB audience). Image
// scheme tells the SPA which logo/badge to render.
export interface DiscoverRating {
  type: string;   // "critic" | "audience"
  image: string;  // "rottentomatoes://image.rating.rotten" | "imdb://image.rating" | "themoviedb://image.rating"
  value: number;
}

// Curated shape for plex.tv-source items (movies/shows from Recommended,
// Discover, Watchlist that may not be on any local server). Distinct from
// Item: no Media/Part chain, but richer marketing metadata.
export interface DiscoverItem {
  ratingKey: string;
  guid: string;
  imdbId?: string;
  title: string;
  type: string;
  year?: number;
  summary?: string;
  tagline?: string;
  contentRating?: string;
  studios?: string[];
  originallyAvailableAt?: string;
  duration?: number; // milliseconds
  thumb?: string;
  art?: string;
  addedAt?: number;
  publicPagesURL?: string;
  genres?: string[];
  ratings?: DiscoverRating[];
  cast?: Person[];
  directors?: Person[];
  writers?: Person[];
}

export interface WatchlistItem {
  ratingKey: string;
  guid?: string;
  title: string;
  type: string;
  year?: number;
  thumb?: string;
}
