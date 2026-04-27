import { createSignal } from "solid-js";
import Section from "./Section";

// Shortcuts — one-click Windows .lnk that points at lumen.exe with the
// `serve` arg. Running it starts the HTTP server in the background and
// opens the user's default browser to http://127.0.0.1:7832 (handled by
// cmd/lumen/serve.go via pkg/browser).
export default function Shortcuts() {
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
      title="Shortcuts"
      description="Drop a Lumen.lnk on your Desktop. Double-clicking it starts the server and opens Lumen in your default browser."
    >
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
    </Section>
  );
}
