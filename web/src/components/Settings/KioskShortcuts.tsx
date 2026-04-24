import { createSignal, Show } from "solid-js";
import Section from "./Section";
import { store } from "../../state/settings";

export default function KioskShortcuts() {
  const s = store.settings;
  const patch = store.patch;
  const [shortcutStatus, setShortcutStatus] = createSignal<string>("");

  async function createShortcut() {
    setShortcutStatus("Creating…");
    try {
      const res = await fetch("/api/shortcut", { method: "POST" });
      if (!res.ok) throw new Error(`${res.status} ${await res.text()}`);
      const body = await res.json();
      setShortcutStatus(`Created at ${body.path}`);
    } catch (e) {
      setShortcutStatus(`Failed: ${(e as Error).message}`);
    }
  }

  return (
    <Section
      title="Kiosk & Shortcuts"
      description="Launch behaviour. Kiosk mode actually starts Lumen in Session 5; these toggles save the preference."
    >
      <Show when={s()} fallback={<p>Loading…</p>}>
        {(settings) => (
          <>
            <div class="settings-row">
              <label for="kioskOnStart">Launch in kiosk mode on startup</label>
              <div class="settings-control">
                <input
                  id="kioskOnStart"
                  type="checkbox"
                  checked={settings().kiosk.enableOnStartup}
                  onChange={(e) =>
                    patch({ kiosk: { ...settings().kiosk, enableOnStartup: e.currentTarget.checked } })
                  }
                />
              </div>
            </div>

            <div class="settings-row">
              <label for="kioskBrowser">Kiosk browser</label>
              <div class="settings-control">
                <select
                  id="kioskBrowser"
                  value={settings().kiosk.browser}
                  onChange={(e) =>
                    patch({ kiosk: { ...settings().kiosk, browser: e.currentTarget.value as any } })
                  }
                >
                  <option value="edge">Microsoft Edge</option>
                  <option value="chrome">Google Chrome</option>
                  <option value="system">System default</option>
                </select>
              </div>
            </div>

            <div class="settings-row">
              <label>Desktop shortcut</label>
              <div class="settings-control">
                <button class="settings-btn" onClick={createShortcut}>Create Desktop Shortcut</button>
                {shortcutStatus() && (
                  <div style={{ "margin-top": "8px", "color": "var(--text-muted)", "font-size": "12px" }}>
                    {shortcutStatus()}
                  </div>
                )}
              </div>
            </div>
          </>
        )}
      </Show>
    </Section>
  );
}
