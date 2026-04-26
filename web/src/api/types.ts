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

export interface WatchlistItem {
  ratingKey: string;
  guid?: string;
  title: string;
  type: string;
  year?: number;
  thumb?: string;
}
