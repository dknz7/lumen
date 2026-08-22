/** One theme file as it came off disk, before validation. */
export interface RawTheme {
  file?: string;
  id?: string;
  name?: string;
  extends?: string;
  tokens?: Record<string, unknown>;
}

export interface ThemesResponse {
  /** Absolute path of the themes folder, shown in Settings. */
  dir: string;
  themes: RawTheme[];
  /** Files the backend could not read or parse at all. */
  errors: { file: string; error: string }[];
}

export const themesAPI = {
  list: async (): Promise<ThemesResponse> => {
    const res = await fetch("/api/themes");
    if (!res.ok) throw new Error(`GET /api/themes: ${res.status} ${await res.text()}`);
    return res.json();
  },

  /** Opens the themes folder in Explorer. */
  reveal: async (): Promise<{ dir: string }> => {
    const res = await fetch("/api/themes/reveal", { method: "POST" });
    if (!res.ok) throw new Error(`POST /api/themes/reveal: ${res.status} ${await res.text()}`);
    return res.json();
  },

  /**
   * Writes a theme to the themes folder as a complete, editable starting
   * point. Sends fully resolved tokens so the result needs no "extends".
   */
  export: async (theme: {
    id: string;
    name: string;
    tokens: Record<string, string>;
  }): Promise<{ path: string; file: string }> => {
    const res = await fetch("/api/themes/export", {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify(theme),
    });
    if (!res.ok) throw new Error(`POST /api/themes/export: ${res.status} ${await res.text()}`);
    return res.json();
  },
};
