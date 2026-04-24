export interface ThemeTokens {
  bg: string;
  bgMenu: string;
  bgElevated: string;
  bgInverse: string;
  text: string;
  textMuted: string;
  textInverse: string;
  menuIcon: string;
  border: string;
  borderSoft: string;
  stroke: string;
  statusOnline: string;
  statusOffline: string;
  shadow: string;
}

export interface Theme {
  id: string;
  name: string;      // display label for the picker
  tokens: ThemeTokens;
}

import { pureOled } from "./pure-oled";

// Registry — add new themes here. The picker reads this list.
export const THEMES: Theme[] = [pureOled];

export function themeByID(id: string): Theme {
  return THEMES.find((t) => t.id === id) ?? pureOled;
}

/**
 * Applies a theme's tokens to :root as CSS custom properties.
 * Call on boot (from the loaded settings) and again on every theme change.
 */
export function applyTheme(theme: Theme) {
  const root = document.documentElement;
  const t = theme.tokens;
  root.style.setProperty("--bg", t.bg);
  root.style.setProperty("--bg-menu", t.bgMenu);
  root.style.setProperty("--bg-elevated", t.bgElevated);
  root.style.setProperty("--bg-inverse", t.bgInverse);
  root.style.setProperty("--text", t.text);
  root.style.setProperty("--text-muted", t.textMuted);
  root.style.setProperty("--text-inverse", t.textInverse);
  root.style.setProperty("--menu-icon", t.menuIcon);
  root.style.setProperty("--border", t.border);
  root.style.setProperty("--border-soft", t.borderSoft);
  root.style.setProperty("--stroke", t.stroke);
  root.style.setProperty("--status-online", t.statusOnline);
  root.style.setProperty("--status-offline", t.statusOffline);
  root.style.setProperty("--shadow", t.shadow);
}
