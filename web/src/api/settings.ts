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
  kiosk: { enableOnStartup: boolean; browser: "edge" | "chrome" | "system" };
  playback: { potPlayerPath: string };
  hiddenLibraries: string[];
  shelfState: Record<string, PageShelfState>;
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
