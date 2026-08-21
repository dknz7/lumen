import { createSignal, ParentProps } from "solid-js";
import TopBar from "./components/TopBar";
import NowPlaying from "./components/NowPlaying";
import LeftMenu from "./components/LeftMenu";
import SettingsModal from "./components/Settings/SettingsModal";
import TranscodePromptModal from "./components/Modal/TranscodePromptModal";
import NextEpisodeModal from "./components/Modal/NextEpisodeModal";
import AppErrorBoundary from "./components/AppErrorBoundary";
import ToastHost from "./components/Toast";

export default function App(props: ParentProps) {
  const [settingsOpen, setSettingsOpen] = createSignal(false);
  return (
    <div class="app-shell">
      <TopBar />
      <NowPlaying />
      <div class="app-body">
        <LeftMenu onOpenSettings={() => setSettingsOpen(true)} />
        <main class="content">
          {/* Per-route boundary: a thrown render in one page shouldn't take
              the shell (top bar, menu, now-playing) down with it. */}
          <AppErrorBoundary>{props.children}</AppErrorBoundary>
        </main>
      </div>
      <SettingsModal open={settingsOpen()} onClose={() => setSettingsOpen(false)} />
      <TranscodePromptModal />
      <NextEpisodeModal />
      <ToastHost />
    </div>
  );
}
