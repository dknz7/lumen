export interface UISettings {
  theme: string;
  zoom: number;
  cardSize: "s" | "m" | "l" | "xl";
  cardDensity: number;
  rowsPerShelf: 1 | 2 | 3 | 4;
  fontSize: number;
  cardLayout: "poster" | "landscape";
  defaultSort: string;
  defaultViewMode: "shows" | "episodes" | "";
  playback: { potPlayerPath: string };
  window: WindowSettings;
  hiddenLibraries: string[];
  /** "serverID:libraryKey:collection title" entries — see Settings > Home Shelves. */
  collectionShelves: string[];
  shelfState: Record<string, PageShelfState>;
  /** "serverID:libraryKey" → that library's own remembered sort/view. */
  libraryPrefs: Record<string, LibraryPref>;
  /** Plex library type (e.g. "movie", "show") → last sort/view picked on any library of that type. */
  typePrefs: Record<string, LibraryPref>;
}

/** A remembered sort and TV view mode; a missing field falls through to the next level. */
export interface LibraryPref {
  sort?: string;
  viewMode?: "shows" | "episodes" | "";
}

export interface WindowSettings {
  /** "tray" keeps Lumen running when the window is closed; "quit" exits. */
  closeAction: "tray" | "quit";
  /** Minimising hides to the tray rather than the taskbar. */
  minimizeToTray: boolean;
  /** Launch straight to the tray with no window. */
  startHidden: boolean;
}

export interface PageShelfState {
  groupOrder?: string[];
  groupCollapsed?: Record<string, boolean>;
  shelfOrder?: Record<string, string[]>;
  shelfPrefs?: Record<string, { hidden?: boolean; collapsed?: boolean }>;
}

export const settingsAPI = {
  get: async (): Promise<UISettings> => {
    const res = await fetch("/api/settings");
    if (!res.ok) throw new Error(`GET settings: ${res.status}`);
    return res.json();
  },
  put: async (patch: Partial<UISettings>): Promise<UISettings> => {
    const res = await fetch("/api/settings", {
      method: "PUT",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify(patch),
    });
    if (!res.ok) throw new Error(`PUT settings: ${res.status} ${await res.text()}`);
    return res.json();
  },
};
