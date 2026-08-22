import { createSignal, For, onMount, Show } from "solid-js";
import Section from "./Section";
import { store } from "../../state/settings";
import {
  BUILTIN_THEMES,
  customThemes,
  loadCustomThemes,
  themeByID,
  themeErrors,
  themesDir,
} from "../../themes";
import { themesAPI } from "../../api/themes";
import { toast, errorMessage } from "../Toast";
import "./Appearance.css";

export default function Appearance() {
  const s = store.settings;
  const patch = store.patch;

  const [busy, setBusy] = createSignal(false);

  // Re-read the folder whenever this panel opens. The list is otherwise only
  // populated at boot, so deleting or fixing a theme file left the picker and
  // the rejected-file list showing state that no longer existed on disk —
  // which reads as a bug rather than as staleness. It is a local directory
  // listing; the cost is nil.
  onMount(() => {
    void loadCustomThemes();
  });

  async function openFolder() {
    try {
      await themesAPI.reveal();
    } catch (e) {
      toast.error(`Couldn't open the themes folder — ${errorMessage(e)}`);
    }
  }

  // Exports the ACTIVE theme with its tokens fully resolved, so the file is
  // valid on its own and needs no "extends" to work. Nobody should have to
  // type twenty-five keys from a blank document to get started.
  async function exportCurrent() {
    const current = themeByID(s()?.theme ?? "");
    setBusy(true);
    try {
      const res = await themesAPI.export({
        id: `${current.id}-copy`,
        name: `${current.name} (copy)`,
        tokens: { ...current.tokens },
      });
      await loadCustomThemes();
      toast.success(`Wrote ${res.file} — edit it and hit Reload.`);
    } catch (e) {
      toast.error(`Couldn't export the theme — ${errorMessage(e)}`);
    } finally {
      setBusy(false);
    }
  }

  async function reload() {
    setBusy(true);
    try {
      await loadCustomThemes();
      const n = customThemes().length;
      toast.success(n === 0 ? "No custom themes found." : `Found ${n} custom theme${n === 1 ? "" : "s"}.`);
    } finally {
      setBusy(false);
    }
  }

  return (
    <Section title="Appearance" description="Theme, card sizing, layout. Changes apply instantly.">
      <Show when={s()} fallback={<p>Loading…</p>}>
        {(settings) => (
          <>
            <div class="settings-row">
              <label for="theme">Theme</label>
              <div class="settings-control">
                <select id="theme" value={settings().theme} onChange={(e) => patch({ theme: e.currentTarget.value })}>
                  <optgroup label="Built-in">
                    <For each={BUILTIN_THEMES}>
                      {(t) => <option value={t.id}>{t.name}</option>}
                    </For>
                  </optgroup>
                  <Show when={customThemes().length > 0}>
                    <optgroup label="Custom">
                      <For each={customThemes()}>
                        {(t) => <option value={t.id}>{t.name}</option>}
                      </For>
                    </optgroup>
                  </Show>
                </select>
              </div>
            </div>

            <div class="settings-row settings-row--stacked">
              <label>Custom themes</label>
              <div class="settings-control">
                <p class="theme-help">
                  Drop a <code>.json</code> theme into the themes folder and hit Reload — no
                  restart. Export the current theme to get a complete file to edit.
                </p>
                <div class="theme-actions">
                  <button class="settings-btn" onClick={openFolder}>Open themes folder</button>
                  <button class="settings-btn" disabled={busy()} onClick={exportCurrent}>
                    Export current theme
                  </button>
                  <button class="settings-btn" disabled={busy()} onClick={reload}>Reload</button>
                </div>
                <Show when={themesDir()}>
                  <p class="theme-path" title={themesDir()}>{themesDir()}</p>
                </Show>

                {/* A rejected theme is named with its reason. Silently dropping
                    it would look like the file was ignored, and the author
                    would have nothing to go on. */}
                <Show when={themeErrors().length > 0}>
                  <ul class="theme-errors">
                    <For each={themeErrors()}>
                      {(err) => (
                        <li>
                          <strong>{err.file}</strong> — {err.error}
                        </li>
                      )}
                    </For>
                  </ul>
                </Show>
              </div>
            </div>

            <div class="settings-row">
              <label for="cardSize">Card size</label>
              <div class="settings-control">
                <select id="cardSize" value={settings().cardSize} onChange={(e) => patch({ cardSize: e.currentTarget.value as any })}>
                  <option value="s">Small</option>
                  <option value="m">Medium</option>
                  <option value="l">Large</option>
                  <option value="xl">Extra Large</option>
                </select>
              </div>
            </div>

            <div class="settings-row">
              <label for="rowsPerShelf">Rows per shelf</label>
              <div class="settings-control">
                <select id="rowsPerShelf" value={String(settings().rowsPerShelf)} onChange={(e) => patch({ rowsPerShelf: Number(e.currentTarget.value) as any })}>
                  <option value="1">1</option>
                  <option value="2">2</option>
                  <option value="3">3</option>
                  <option value="4">4</option>
                </select>
              </div>
            </div>

            <div class="settings-row">
              <label for="cardDensity">Card density</label>
              <div class="settings-control">
                <input
                  id="cardDensity"
                  type="range"
                  min="0" max="100"
                  value={settings().cardDensity}
                  onInput={(e) => patch({ cardDensity: Number(e.currentTarget.value) })}
                />
              </div>
            </div>

            <div class="settings-row">
              <label for="cardLayout">Card layout</label>
              <div class="settings-control">
                <select id="cardLayout" value={settings().cardLayout} onChange={(e) => patch({ cardLayout: e.currentTarget.value as any })}>
                  <option value="poster">Poster (2:3)</option>
                  <option value="landscape">Landscape (16:9)</option>
                </select>
              </div>
            </div>

            <div class="settings-row">
              <label for="fontSize">Font size</label>
              <div class="settings-control">
                <input
                  id="fontSize"
                  type="range"
                  min="11" max="18"
                  value={settings().fontSize}
                  onInput={(e) => patch({ fontSize: Number(e.currentTarget.value) })}
                />
              </div>
            </div>
          </>
        )}
      </Show>
    </Section>
  );
}
