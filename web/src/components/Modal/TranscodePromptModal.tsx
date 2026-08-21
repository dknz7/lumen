import ModalShell from "./ModalShell";
import { playbackStore } from "../../state/playback";
import { api } from "../../api/client";
import "./TranscodePromptModal.css";
import { toast, errorMessage } from "../Toast";

export default function TranscodePromptModal() {
  const info = playbackStore.transcodePrompt;
  const close = () => playbackStore.dismissTranscodePrompt();

  async function confirm() {
    const i = info();
    if (!i) return;
    close();
    try {
      await api.playTranscode(i.serverID, i.ratingKey);
    } catch (e) {
      console.error("playTranscode failed:", e);
      toast.error(`Couldn't start the transcode — ${errorMessage(e)}`);
    }
  }

  return (
    <ModalShell open={info() !== null} onCancel={close} ariaLabel="Direct play failed">
      <h2 class="tpm-title">Direct play failed</h2>
      <p class="tpm-body">
        Pot Player couldn't play <strong>{info()?.title}</strong> directly. Reason: {info()?.reason}.
        <br/><br/>
        Try transcoding? This asks the Plex server to re-encode the file at 1080p H.264.
      </p>
      <div class="tpm-actions">
        <button class="tpm-cancel" onClick={close}>Cancel</button>
        <button class="tpm-confirm" onClick={confirm}>Try Transcode</button>
      </div>
    </ModalShell>
  );
}
