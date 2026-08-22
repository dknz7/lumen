import { createSignal } from "solid-js";
import { themesAPI, type RawTheme } from "../api/themes";

/**
 * TOKENS is the single source of truth for the theme contract: it names every
 * token and the CSS custom property it becomes.
 *
 * The token list used to exist twice — once as a TypeScript interface and,
 * once custom themes needed validating at runtime, again as an array of key
 * names. Two lists that must agree and nothing to make them. Deriving the
 * type from this map means adding a token is one line and the compiler
 * enforces the rest.
 */
export const TOKENS = {
  bg: "--bg",
  bgMenu: "--bg-menu",
  bgElevated: "--bg-elevated",
  bgInverse: "--bg-inverse",
  text: "--text",
  textMuted: "--text-muted",
  textInverse: "--text-inverse",
  menuIcon: "--menu-icon",
  border: "--border",
  borderSoft: "--border-soft",
  stroke: "--stroke",
  statusOnline: "--status-online",
  statusOffline: "--status-offline",
  shadow: "--shadow",

  // Accent and state. Added when a second theme arrived: the original
  // fourteen described surfaces, text and borders only, so every accent,
  // error, warning and success colour lived as a literal in component CSS
  // and no theme could reach any of them.
  accent: "--accent",
  accentContrast: "--accent-contrast",
  danger: "--danger",
  dangerStrong: "--danger-strong",
  warning: "--warning",
  success: "--success",
  overlay: "--overlay",
  surfaceSubtle: "--surface-subtle",
  cardEmpty: "--bg-card-empty",
  shelfOuter: "--bg-shelf-outer",
  shelfInner: "--bg-shelf-inner",
} as const;

export type TokenName = keyof typeof TOKENS;
export type ThemeTokens = Record<TokenName, string>;

export const TOKEN_NAMES = Object.keys(TOKENS) as TokenName[];

export interface Theme {
  id: string;
  name: string; // display label for the picker
  tokens: ThemeTokens;
  /** True for themes loaded from %APPDATA%\Lumen\themes. */
  custom?: boolean;
  /** Source file name — used when reporting a bad theme back to the user. */
  file?: string;
}

import { pureOled } from "./pure-oled";
import { tokyoNight } from "./tokyo-night";

/** Themes compiled into the app. Custom themes are appended at runtime. */
export const BUILTIN_THEMES: Theme[] = [pureOled, tokyoNight];

const [customThemes, setCustomThemes] = createSignal<Theme[]>([]);
const [themeErrors, setThemeErrors] = createSignal<ThemeLoadError[]>([]);
const [themesDir, setThemesDir] = createSignal<string>("");

export { customThemes, themeErrors, themesDir };

/** Built-ins first, then whatever the user has written. */
export function allThemes(): Theme[] {
  return [...BUILTIN_THEMES, ...customThemes()];
}

export function themeByID(id: string): Theme {
  return allThemes().find((t) => t.id === id) ?? pureOled;
}

export interface ThemeLoadError {
  file: string;
  error: string;
}

/**
 * isValidTokenValue gates a value before it reaches the DOM.
 *
 * A custom property is inert on its own, but these are interpolated into real
 * declarations — and `shadow` becomes a whole box-shadow, which is a property
 * that accepts a url(). Without a check, a downloaded theme could ship a
 * value that fetches from a remote host the moment it renders. Asking the
 * browser whether the value is valid *for the property it will be used as*
 * rejects that, and catches typos on the way through.
 */
export function isValidTokenValue(name: TokenName, value: unknown): value is string {
  if (typeof value !== "string") return false;
  const v = value.trim();
  if (v === "" || v.length > 120) return false;
  if (typeof CSS === "undefined" || typeof CSS.supports !== "function") {
    // No CSS.supports to ask (non-browser test context) — fall back to
    // refusing anything that could terminate the declaration.
    return !/[;{}]|url\s*\(|expression\s*\(/i.test(v);
  }
  return CSS.supports(name === "shadow" ? "box-shadow" : "color", v);
}

/**
 * resolveCustomTheme turns one file off disk into a Theme, or explains why it
 * could not.
 *
 * `extends` names a built-in to inherit from, so a variant is the handful of
 * colours the author actually wants to change rather than all twenty-five.
 * It also means a token added in a later Lumen version inherits a sensible
 * value instead of invalidating every custom theme in existence.
 */
export function resolveCustomTheme(raw: RawTheme): { theme: Theme } | { error: string } {
  const id = (raw.id ?? "").trim();
  const name = (raw.name ?? "").trim();
  if (!id || !name) return { error: `needs both an "id" and a "name"` };
  if (BUILTIN_THEMES.some((t) => t.id === id)) {
    return { error: `"${id}" is a built-in theme id — pick another` };
  }

  let base: Partial<ThemeTokens> = {};
  if (raw.extends) {
    const parent = BUILTIN_THEMES.find((t) => t.id === raw.extends);
    if (!parent) {
      const known = BUILTIN_THEMES.map((t) => t.id).join(", ");
      return { error: `"extends": "${raw.extends}" is not a built-in theme (${known})` };
    }
    base = parent.tokens;
  }

  const incoming = raw.tokens ?? {};
  const unknown = Object.keys(incoming).filter((k) => !(k in TOKENS));
  if (unknown.length) {
    return { error: `unknown token${unknown.length > 1 ? "s" : ""}: ${unknown.join(", ")}` };
  }

  const merged: Record<string, string> = { ...base };
  for (const key of Object.keys(incoming)) {
    const value = incoming[key];
    if (!isValidTokenValue(key as TokenName, value)) {
      return { error: `"${key}" is not a colour the browser accepts: ${JSON.stringify(value)}` };
    }
    merged[key] = (value as string).trim();
  }

  const missing = TOKEN_NAMES.filter((k) => typeof merged[k] !== "string");
  if (missing.length) {
    const hint = raw.extends ? "" : ` — add "extends": "pure-oled" to inherit the rest`;
    return { error: `missing ${missing.length} token${missing.length > 1 ? "s" : ""}: ${missing.join(", ")}${hint}` };
  }

  return { theme: { id, name, tokens: merged as ThemeTokens, custom: true, file: raw.file } };
}

/**
 * loadCustomThemes reads %APPDATA%\Lumen\themes and populates the picker.
 *
 * A theme that fails validation is dropped and reported rather than applied
 * in part: a half-applied theme looks like a rendering bug, while a named
 * error tells the author which line to fix.
 */
export async function loadCustomThemes(): Promise<void> {
  try {
    const res = await themesAPI.list();
    const themes: Theme[] = [];
    const errors: ThemeLoadError[] = res.errors ?? [];
    for (const raw of res.themes ?? []) {
      const out = resolveCustomTheme(raw);
      if ("theme" in out) themes.push(out.theme);
      else errors.push({ file: raw.file ?? raw.id ?? "(unnamed)", error: out.error });
    }
    setThemesDir(res.dir ?? "");
    setCustomThemes(themes);
    setThemeErrors(errors);
  } catch (e) {
    console.error("custom themes could not be loaded:", e);
    setCustomThemes([]);
    setThemeErrors([{ file: ".", error: (e as Error).message }]);
  }
}

/**
 * Applies a theme's tokens to :root as CSS custom properties.
 * Call on boot (from the loaded settings) and again on every theme change.
 */
export function applyTheme(theme: Theme) {
  const root = document.documentElement;
  for (const name of TOKEN_NAMES) {
    const value = theme.tokens[name];
    // Built-ins are valid by construction and custom themes were validated at
    // load; anything failing here would be a bug, and skipping beats writing
    // a value the browser will discard anyway.
    if (!isValidTokenValue(name, value)) continue;
    root.style.setProperty(TOKENS[name], value);
  }
  // Kept as an alias: --led-teal is the name several rules already use for
  // the accent, and renaming every call site buys nothing.
  root.style.setProperty("--led-teal", theme.tokens.accent);
}
