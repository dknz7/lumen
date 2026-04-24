import { createSignal, ParentProps } from "solid-js";
import TopBar from "./components/TopBar";
import LeftMenu from "./components/LeftMenu";
import SettingsModal from "./components/Settings/SettingsModal";

export default function App(props: ParentProps) {
  const [settingsOpen, setSettingsOpen] = createSignal(false);
  return (
    <div class="app-shell">
      <TopBar />
      <div class="app-body">
        <LeftMenu onOpenSettings={() => setSettingsOpen(true)} />
        <main class="content">{props.children}</main>
      </div>
      <SettingsModal open={settingsOpen()} onClose={() => setSettingsOpen(false)} />
    </div>
  );
}
