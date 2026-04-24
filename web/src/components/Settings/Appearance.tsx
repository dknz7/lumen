import { For, Show } from "solid-js";
import Section from "./Section";
import { store } from "../../state/settings";
import { THEMES } from "../../themes";

export default function Appearance() {
  const s = store.settings;
  const patch = store.patch;

  return (
    <Section title="Appearance" description="Theme, card sizing, layout. Changes apply instantly.">
      <Show when={s()} fallback={<p>Loading…</p>}>
        {(settings) => (
          <>
            <div class="settings-row">
              <label for="theme">Theme</label>
              <div class="settings-control">
                <select id="theme" value={settings().theme} onChange={(e) => patch({ theme: e.currentTarget.value })}>
                  <For each={THEMES}>
                    {(t) => <option value={t.id}>{t.name}</option>}
                  </For>
                </select>
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
