import { createRoot, createSignal, untrack } from "solid-js";
import { settingsAPI, UISettings } from "../api/settings";
import { applyTheme, themeByID } from "../themes";

// Card-size base widths must mirror theme.css's --card-width-{s,m,l,xl}.
// Used to compute the zoom-scaled global --card-width override.
const CARD_WIDTH_BASE_PX: Record<"s" | "m" | "l" | "xl", number> = {
  s: 120, m: 160, l: 200, xl: 240,
};

// Debounce helper for PUT coalescing.
function debounce<T extends (...args: any[]) => any>(fn: T, ms: number): T {
  let t: number | undefined;
  return ((...args: Parameters<T>) => {
    if (t !== undefined) clearTimeout(t);
    t = setTimeout(() => fn(...args), ms) as unknown as number;
  }) as T;
}

function createSettingsStore() {
  const [settings, setSettings] = createSignal<UISettings | null>(null);
  const [loaded, setLoaded] = createSignal(false);

  function applyRootDerived(s: UISettings) {
    const root = document.documentElement;
    root.setAttribute("data-card-size", s.cardSize);
    root.style.setProperty("--font-size", `${s.fontSize}px`);
    // --card-width is set INLINE on :root as cardSize-base × zoom%. This wins
    // over the [data-card-size] CSS rules (same specificity but inline beats
    // selector). All cards across pages scale together. Top bar / left menu
    // don't reference --card-width, so they stay at fixed sizes regardless.
    const base = CARD_WIDTH_BASE_PX[s.cardSize] ?? CARD_WIDTH_BASE_PX.m;
    const scaled = Math.round(base * (s.zoom / 100));
    root.style.setProperty("--card-width", `${scaled}px`);
  }

  // Initial fetch. Called once at app boot from main.tsx.
  async function load() {
    const s = await settingsAPI.get();
    setSettings(s);
    setLoaded(true);
    applyTheme(themeByID(s.theme));
    applyRootDerived(s);
  }

  const flushDebounced = debounce(async (patch: Partial<UISettings>) => {
    try {
      const updated = await settingsAPI.put(patch);
      setSettings(updated);
    } catch (e) {
      console.error("settings PUT failed:", e);
    }
  }, 300);

  // Patch mutates the store locally (optimistic) AND schedules a server write.
  function patch(update: Partial<UISettings>) {
    // The reactive read of settings() inside patch() would otherwise subscribe
    // any caller in a reactive scope (createEffect, createMemo, createResource
    // source) to the settings signal, causing the surrounding scope to re-fire
    // on every patch — including the patch the caller just invoked, plus the
    // ~300ms debounced PUT response. Library.tsx's pagination snap-back regression
    // was traced to exactly this trap (see commit fb29b5d). Wrapping the read
    // in untrack() closes the bug class for all future callers. There's no
    // legitimate use case for "subscribe to settings while patching them" —
    // callers that want reactivity should call settings() directly.
    const current = untrack(() => settings());
    if (!current) return;
    const next = { ...current, ...update };
    setSettings(next);

    // Side-effects that should happen immediately (not wait for debounce):
    if (update.theme && update.theme !== current.theme) {
      applyTheme(themeByID(update.theme));
    }
    applyRootDerived(next);

    flushDebounced(update);
  }

  return { settings, loaded, load, patch };
}

export const store = createRoot(createSettingsStore);
