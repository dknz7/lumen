import { createSignal, Show } from "solid-js";
import Section from "./Section";
import { store } from "../../state/settings";
import { toast, errorMessage } from "../Toast";
import "./WindowTray.css";

// Window & Tray — how Lumen behaves as a desktop application.
//
// Replaces the old "Shortcuts" section, which held a single button. These are
// the same concerns (how you launch it, how you dismiss it) and they read
// better together than as a one-item panel.
export default function WindowTray() {
  const s = store.settings;
  const patch = store.patch;
  const [shortcutStatus, setShortcutStatus] = createSignal("");
  const [creating, setCreating] = createSignal(false);

  async function createShortcut() {
    setCreating(true);
    setShortcutStatus("");
    try {
      const res = await fetch("/api/shortcut", { method: "POST" });
      if (!res.ok) throw new Error(`${res.status} ${await res.text()}`);
      const body = await res.json();
      setShortcutStatus(body.path);
      toast.success("Desktop shortcut created");
    } catch (e) {
      toast.error(errorMessage(e, "Couldn't create the shortcut"));
    } finally {
      setCreating(false);
    }
  }

  return (
    <Section
      title="Window & Tray"
      description="How Lumen behaves as a desktop app — what closing it does, and how you get it back."
    >
      <Show when={s()} fallback={<p>Loading…</p>}>
        {(settings) => (
          <>
            <div class="settings-row">
              <label for="closeAction">When I close the window</label>
              <div class="settings-control">
                <select
                  id="closeAction"
                  value={settings().window?.closeAction ?? "tray"}
                  onChange={(e) =>
                    patch({
                      window: {
                        ...(settings().window ?? { minimizeToTray: false, startHidden: false }),
                        closeAction: e.currentTarget.value as "tray" | "quit",
                      },
                    })
                  }
                >
                  <option value="tray">Keep running in the system tray</option>
                  <option value="quit">Quit Lumen completely</option>
                </select>
                <p class="wt-hint">
                  Staying in the tray keeps your servers connected and the cache
                  warm, so reopening is instant. You can always quit from the
                  tray icon's right-click menu.
                </p>
              </div>
            </div>

            <div class="settings-row">
              <label for="minimizeToTray">When I minimise</label>
              <div class="settings-control">
                <label class="wt-check">
                  <input
                    id="minimizeToTray"
                    type="checkbox"
                    checked={settings().window?.minimizeToTray ?? false}
                    onChange={(e) =>
                      patch({
                        window: {
                          ...(settings().window ?? { closeAction: "tray", startHidden: false }),
                          minimizeToTray: e.currentTarget.checked,
                        },
                      })
                    }
                  />
                  <span>Hide to the tray instead of the taskbar</span>
                </label>
                <p class="wt-hint">
                  Off by default — minimising behaves like any other window and
                  stays in the taskbar.
                </p>
              </div>
            </div>

            <div class="settings-row">
              <label for="startHidden">On startup</label>
              <div class="settings-control">
                <label class="wt-check">
                  <input
                    id="startHidden"
                    type="checkbox"
                    checked={settings().window?.startHidden ?? false}
                    onChange={(e) =>
                      patch({
                        window: {
                          ...(settings().window ?? { closeAction: "tray", minimizeToTray: false }),
                          startHidden: e.currentTarget.checked,
                        },
                      })
                    }
                  />
                  <span>Start minimised to the tray</span>
                </label>
                <p class="wt-hint">
                  Useful with "start with Windows", which the installer can set
                  up for you — Lumen boots quietly and waits in the tray.
                </p>
              </div>
            </div>

            <div class="settings-row">
              <label>Desktop shortcut</label>
              <div class="settings-control">
                <button class="settings-btn" disabled={creating()} onClick={createShortcut}>
                  {creating() ? "Creating…" : "Create desktop shortcut"}
                </button>
                <Show when={shortcutStatus()}>
                  <code class="wt-path">{shortcutStatus()}</code>
                </Show>
                <p class="wt-hint">
                  If you installed Lumen with the installer you already have
                  Start-menu and desktop entries; this is for portable copies.
                </p>
              </div>
            </div>
          </>
        )}
      </Show>
    </Section>
  );
}
