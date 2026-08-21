import { createResource, Show } from "solid-js";
import Section from "./Section";
import { store } from "../../state/settings";
import { aboutAPI, type AboutInfo } from "../../api/about";
import { errorMessage } from "../Toast";
import { ExternalLink } from "../icons";
import "./Playback.css";

const POTPLAYER_URL = "https://potplayer.daum.net/";

export default function Playback() {
  const s = store.settings;
  const patch = store.patch;

  // /api/about carries the resolved PotPlayer path, so the panel can show
  // whether detection actually worked rather than leaving the user to guess.
  const [info, { refetch }] = createResource<AboutInfo>(() => aboutAPI.get());

  return (
    <Section
      title="Playback"
      description="Lumen doesn't decode video itself — it hands the stream to PotPlayer, which direct-plays almost anything without transcoding."
    >
      <Show when={s()} fallback={<p>Loading…</p>}>
        {(settings) => (
          <>
            <div class="settings-row">
              <label>PotPlayer</label>
              <div class="settings-control">
                <Show
                  when={!info.error}
                  fallback={
                    <span class="pb-status pb-status--unknown">
                      Couldn't check — {errorMessage(info.error)}
                    </span>
                  }
                >
                  <Show when={info()} fallback={<span class="pb-status pb-status--unknown">Checking…</span>}>
                    {(a) => (
                      <Show
                        when={a().potPlayer.detected}
                        fallback={
                          <div class="pb-missing">
                            <span class="pb-status pb-status--missing">Not found</span>
                            <p class="pb-hint">
                              Lumen needs PotPlayer to play anything. Install it,
                              then use Re-check below — or set the path manually
                              if you installed it somewhere unusual.
                            </p>
                            <a
                              class="pb-download"
                              href={POTPLAYER_URL}
                              target="_blank"
                              rel="noreferrer"
                            >
                              Download PotPlayer <ExternalLink size={12} />
                            </a>
                          </div>
                        }
                      >
                        <div class="pb-found">
                          <span class="pb-status pb-status--found">Ready</span>
                          <code class="pb-path">{a().potPlayer.path}</code>
                        </div>
                      </Show>
                    )}
                  </Show>
                </Show>
              </div>
            </div>

            <div class="settings-row">
              <label for="potPath">Path override</label>
              <div class="settings-control">
                <input
                  id="potPath"
                  type="text"
                  placeholder="Leave blank to detect automatically"
                  value={settings().playback.potPlayerPath}
                  onChange={(e) => {
                    patch({
                      playback: { ...settings().playback, potPlayerPath: e.currentTarget.value },
                    });
                    // Give the debounced PUT time to land before re-reading the
                    // resolved path from the server.
                    setTimeout(refetch, 500);
                  }}
                />
                <p class="pb-hint">
                  Full path to <code>PotPlayerMini64.exe</code>. Only needed if
                  Lumen can't find it on its own — it checks the registry and the
                  usual install locations first.
                </p>
                <button class="settings-btn pb-recheck" onClick={() => refetch()}>
                  Re-check
                </button>
              </div>
            </div>

            <div class="settings-row">
              <label>Direct play</label>
              <div class="settings-control">
                <span class="about-value">Preferred, always</span>
                <p class="pb-hint">
                  Lumen asks your server for the original file first. If the
                  server insists on transcoding, it'll tell you before starting
                  rather than silently re-encoding a 4K remux.
                </p>
              </div>
            </div>

            <div class="settings-row">
              <label>Get PotPlayer</label>
              <div class="settings-control">
                <a
                  class="pb-download"
                  href={POTPLAYER_URL}
                  target="_blank"
                  rel="noreferrer"
                >
                  potplayer.daum.net <ExternalLink size={12} />
                </a>
                <p class="pb-hint">
                  Free, and Lumen expects the 64-bit build. Not affiliated with
                  Lumen — it's just very good at playing files.
                </p>
              </div>
            </div>
          </>
        )}
      </Show>
    </Section>
  );
}
