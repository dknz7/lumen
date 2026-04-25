import { createEffect, createSignal, onCleanup } from "solid-js";
import ModalShell from "./ModalShell";
import "./ResumeRestartModal.css";

export interface ResumeRestartProps {
  open: boolean;
  resumeOffsetMs: number;
  onResume: () => void;
  onRestart: () => void;
  onCancel: () => void;
}

const COUNTDOWN_MS = 5000;
const TICK_MS = 100;

export default function ResumeRestartModal(props: ResumeRestartProps) {
  const [remaining, setRemaining] = createSignal(COUNTDOWN_MS);
  let timer: number | undefined;

  createEffect(() => {
    if (!props.open) return;
    setRemaining(COUNTDOWN_MS);
    timer = window.setInterval(() => {
      setRemaining((r) => {
        const next = r - TICK_MS;
        if (next <= 0) {
          window.clearInterval(timer);
          props.onResume();
          return 0;
        }
        return next;
      });
    }, TICK_MS);
    onCleanup(() => window.clearInterval(timer));
  });

  const fmtOffset = () => {
    const sec = Math.floor(props.resumeOffsetMs / 1000);
    const m = Math.floor(sec / 60);
    const s = sec % 60;
    return `${m}:${String(s).padStart(2, "0")}`;
  };

  const pct = () => 100 - (remaining() / COUNTDOWN_MS) * 100;

  return (
    <ModalShell open={props.open} onCancel={props.onCancel} ariaLabel="Resume or restart">
      <h2 class="rrm-title">Resume from {fmtOffset()}?</h2>
      <p class="rrm-body">Auto-resuming in {Math.ceil(remaining() / 1000)}s. Click Start Over to play from the beginning, or Cancel to stop.</p>
      <div class="rrm-progress">
        <div class="rrm-progress-fill" style={{ width: `${pct()}%` }} />
      </div>
      <div class="rrm-actions">
        <button class="rrm-cancel" onClick={() => { window.clearInterval(timer); props.onCancel(); }}>Cancel</button>
        <button class="rrm-restart" onClick={() => { window.clearInterval(timer); props.onRestart(); }}>Start Over</button>
        <button class="rrm-resume" onClick={() => { window.clearInterval(timer); props.onResume(); }}>Resume</button>
      </div>
    </ModalShell>
  );
}
