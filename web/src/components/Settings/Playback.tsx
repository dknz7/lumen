import { Show } from "solid-js";
import Section from "./Section";
import { store } from "../../state/settings";

export default function Playback() {
  const s = store.settings;
  const patch = store.patch;

  return (
    <Section title="Playback" description="Pot Player integration. Playback itself lands in Session 4.">
      <Show when={s()} fallback={<p>Loading…</p>}>
        {(settings) => (
          <>
            <div class="settings-row">
              <label for="potPath">Pot Player path</label>
              <div class="settings-control">
                <input
                  id="potPath"
                  type="text"
                  placeholder="Leave blank to auto-detect (Session 4)"
                  value={settings().playback.potPlayerPath}
                  onChange={(e) =>
                    patch({ playback: { ...settings().playback, potPlayerPath: e.currentTarget.value } })
                  }
                />
              </div>
            </div>

            <div class="settings-row">
              <label>Direct-play timeout</label>
              <div class="settings-control">
                <span style={{ "color": "var(--text-muted)" }}>10 seconds (fixed policy)</span>
              </div>
            </div>
          </>
        )}
      </Show>
    </Section>
  );
}
