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
  type: string;
  year?: number;
  summary?: string;
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
