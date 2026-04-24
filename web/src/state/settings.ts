import { createRoot, createSignal } from "solid-js";
import { settingsAPI, UISettings } from "../api/settings";
import { applyTheme, themeByID } from "../themes";

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
    root.style.setProperty("zoom", String(s.zoom / 100));
    root.setAttribute("data-card-size", s.cardSize);
    root.style.setProperty("--font-size", `${s.fontSize}px`);
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
    const current = settings();
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
