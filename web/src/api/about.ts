export interface Dependency {
  name: string;
  license: string;
  url: string;
}

export interface AboutInfo {
  version: string;
  commit?: string;
  buildDate?: string;
  goVersion: string;
  platform: string;
  repository: string;
  license: string;
  issues: string;
  paths: { config: string; cache: string; logs: string };
  potPlayer: { detected: boolean; path?: string; override?: string };
  dependencies: Dependency[];
}

export interface StatusInfo {
  linked: boolean;
  hasServers: boolean;
  username?: string;
  version: string;
}

export const aboutAPI = {
  get: async (): Promise<AboutInfo> => {
    const res = await fetch("/api/about");
    if (!res.ok) throw new Error(`GET about: ${res.status}`);
    return res.json();
  },

  status: async (): Promise<StatusInfo> => {
    const res = await fetch("/api/status");
    if (!res.ok) throw new Error(`GET status: ${res.status}`);
    return res.json();
  },
};

export const windowAPI = {
  show: () => fetch("/api/window/show", { method: "POST" }),
  hide: () => fetch("/api/window/hide", { method: "POST" }),
};

/**
 * diagnosticsText builds the block behind About's "Copy diagnostics" button.
 *
 * Deliberately contains no tokens, no API keys and nothing about the user's
 * library — everything here comes from /api/about, which is tested to be
 * credential-free, precisely because the point of this button is to paste the
 * result into a public GitHub issue.
 */
export function diagnosticsText(a: AboutInfo): string {
  return [
    `Lumen ${a.version}${a.commit ? ` (${a.commit})` : ""}`,
    `Platform:   ${a.platform}`,
    `Go:         ${a.goVersion}`,
    `PotPlayer:  ${a.potPlayer.detected ? a.potPlayer.path : "not detected"}`,
    `Config:     ${a.paths.config}`,
    `Logs:       ${a.paths.logs}`,
    `User agent: ${navigator.userAgent}`,
  ].join("\n");
}
