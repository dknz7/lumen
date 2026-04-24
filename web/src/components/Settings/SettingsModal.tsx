import { createSignal, For, onCleanup, onMount, Show } from "solid-js";
import { Motion, Presence } from "@motionone/solid";
import Appearance from "./Appearance";
import KioskShortcuts from "./KioskShortcuts";
import AccountsServers from "./AccountsServers";
import Playback from "./Playback";
import DataCache from "./DataCache";
import About from "./About";
import { X } from "../icons";
import "./SettingsModal.css";

const SECTIONS = [
  { id: "appearance",  label: "Appearance",         component: Appearance },
  { id: "kiosk",       label: "Kiosk & Shortcuts",  component: KioskShortcuts },
  { id: "accounts",    label: "Accounts & Servers", component: AccountsServers },
  { id: "playback",    label: "Playback",           component: Playback },
  { id: "cache",       label: "Data & Cache",       component: DataCache },
  { id: "about",       label: "About",              component: About },
] as const;

export default function SettingsModal(props: { open: boolean; onClose: () => void }) {
  const [activeID, setActiveID] = createSignal<string>(SECTIONS[0].id);

  function onKeyDown(e: KeyboardEvent) {
    if (e.key === "Escape") props.onClose();
  }

  onMount(() => {
    document.addEventListener("keydown", onKeyDown);
    onCleanup(() => document.removeEventListener("keydown", onKeyDown));
  });

  const activeSection = () => SECTIONS.find((s) => s.id === activeID())!;

  return (
    <Presence>
      <Show when={props.open}>
        <Motion.div
          class="settings-backdrop"
          onClick={props.onClose}
          role="presentation"
          initial={{ opacity: 0 }}
          animate={{ opacity: 1 }}
          exit={{ opacity: 0 }}
          transition={{ duration: 0.18, easing: [0.16, 1, 0.3, 1] }}
        >
          <Motion.div
            class="settings-modal"
            onClick={(e) => e.stopPropagation()}
            role="dialog"
            aria-label="Settings"
            initial={{ opacity: 0, scale: 0.96, y: 8 }}
            animate={{ opacity: 1, scale: 1, y: 0 }}
            exit={{ opacity: 0, scale: 0.96, y: 8 }}
            transition={{ duration: 0.24, easing: [0.22, 1, 0.36, 1] }}
          >
            <aside class="settings-nav">
              <header class="settings-nav-header">
                <span class="settings-nav-title">Settings</span>
                <button class="settings-close-btn" onClick={props.onClose} aria-label="Close settings">
                  <X size={14} />
                </button>
              </header>
              <nav>
                <For each={SECTIONS}>
                  {(s) => (
                    <button
                      class="settings-nav-item"
                      classList={{ active: activeID() === s.id }}
                      onClick={() => setActiveID(s.id)}
                    >
                      {s.label}
                    </button>
                  )}
                </For>
              </nav>
            </aside>
            <main class="settings-detail">
              {activeSection().component({})}
            </main>
          </Motion.div>
        </Motion.div>
      </Show>
    </Presence>
  );
}
