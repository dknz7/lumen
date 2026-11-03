import type { LibraryPref, UISettings } from "../api/settings";

// Per-library sort/view memory. See Library.tsx and UIConfig.LibraryPrefs in
// internal/config/config.go.

type Prefs = Pick<UISettings, "libraryPrefs" | "typePrefs" | "defaultSort" | "defaultViewMode">;

/** "serverID:libraryKey", the same key format hiddenLibraries uses. */
export const libraryPrefKey = (serverID: string, libraryKey: string) => `${serverID}:${libraryKey}`;

/**
 * The sort and view a library opens with: its own pick, else the last pick on
 * any library of the same type, else the defaults.
 *
 * The sort default is always `defaultSort` (Date Added), deliberately NOT the
 * saved defaultSort setting: up to 1.3.1 every pick on any library was written
 * there, so on an upgraded install it means "the last pick anywhere", not a
 * chosen default. Honouring it would open every library (movies included) in
 * whatever was last picked on, say, a TV library, with no way to reset it.
 * defaultViewMode is still honoured: old code saved Shows as "", which config
 * loading turns back into "episodes", so it never carried a stale pick.
 */
export function resolveLibraryPrefs(
  s: Partial<Prefs> | null | undefined,
  key: string,
  type: string,
  defaultSort: string,
): { sort: string; viewMode: string } {
  const own = s?.libraryPrefs?.[key];
  const byType = type ? s?.typePrefs?.[type] : undefined;
  return {
    sort: own?.sort || byType?.sort || defaultSort,
    viewMode: own?.viewMode || byType?.viewMode || (s?.defaultViewMode ?? "episodes"),
  };
}

/**
 * The settings patch for a pick: it becomes this library's own pref AND its
 * type's. Returns complete maps, because the debounced save sends only the
 * last patch in its window.
 */
export function prefsAfterPick(
  s: Partial<Prefs>,
  key: string,
  type: string,
  update: LibraryPref,
): Pick<UISettings, "libraryPrefs" | "typePrefs"> {
  const libraryPrefs = { ...s.libraryPrefs, [key]: { ...s.libraryPrefs?.[key], ...update } };
  const typePrefs = type
    ? { ...s.typePrefs, [type]: { ...s.typePrefs?.[type], ...update } }
    : { ...s.typePrefs };
  return { libraryPrefs, typePrefs };
}
