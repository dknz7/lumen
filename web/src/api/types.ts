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
  duration?: number;   // media length in ms
  viewOffset?: number; // resume position in ms (0 = unstarted)
  // Episode-specific — populated only when type === "episode"
  index?: number;              // episode number
  parentIndex?: number;        // season number
  parentTitle?: string;
  parentThumb?: string;
  grandparentTitle?: string;   // show name
  grandparentThumb?: string;   // show poster
  grandparentArt?: string;     // show backdrop
  grandparentRatingKey?: string;
}

export interface HubItem {
  guid?: string;
  ratingKey: string;
  title: string;
  type: string;
  year?: number;
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
