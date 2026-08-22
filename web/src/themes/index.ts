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

  // --- Accent and state ------------------------------------------------
  // Added when a second theme arrived. The original fourteen described
  // surfaces, text and borders only, so every accent, error, warning and
  // success colour in the app was a literal in component CSS — roughly a
  // hundred of them across a dozen files. A theme could restyle the page and
  // not one of them would move, which is fine with a single theme and useless
  // with two.

  /** Lumen's highlight: status LEDs, progress fills, the Save button. */
  accent: string;
  /** Text and icons drawn on top of `accent`. */
  accentContrast: string;
  /** Error text. */
  danger: string;
  /** Error borders and solid destructive fills. */
  dangerStrong: string;
  /** Degraded-but-working states — transcoding, partial availability. */
  warning: string;
  /** Healthy states — reachable server, watched, direct play. */
  success: string;
  /** Full-screen scrim behind modals. */
  overlay: string;
  /** Faint raised fill: hover surfaces, the skeleton shimmer. */
  surfaceSubtle: string;
  /** Card poster background when there is no artwork. */
  cardEmpty: string;
  /** Shelf container background. */
  shelfOuter: string;
  /** Shelf row / page background. */
  shelfInner: string;
}

export interface Theme {
  id: string;
  name: string;      // display label for the picker
  tokens: ThemeTokens;
}

import { pureOled } from "./pure-oled";
import { tokyoNight } from "./tokyo-night";

// Registry — add new themes here. The picker reads this list.
export const THEMES: Theme[] = [pureOled, tokyoNight];

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

  root.style.setProperty("--accent", t.accent);
  root.style.setProperty("--accent-contrast", t.accentContrast);
  root.style.setProperty("--danger", t.danger);
  root.style.setProperty("--danger-strong", t.dangerStrong);
  root.style.setProperty("--warning", t.warning);
  root.style.setProperty("--success", t.success);
  root.style.setProperty("--overlay", t.overlay);
  root.style.setProperty("--surface-subtle", t.surfaceSubtle);
  root.style.setProperty("--bg-card-empty", t.cardEmpty);
  root.style.setProperty("--bg-shelf-outer", t.shelfOuter);
  root.style.setProperty("--bg-shelf-inner", t.shelfInner);
  // Kept as an alias: --led-teal is the name several rules already use for
  // the accent, and renaming every call site buys nothing.
  root.style.setProperty("--led-teal", t.accent);
}
