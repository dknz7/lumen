import { createResource, createSignal, For, Show } from "solid-js";
import Section from "./Section";
import { aboutAPI, diagnosticsText, type AboutInfo } from "../../api/about";
import { toast, errorMessage } from "../Toast";
import { ExternalLink } from "../icons";
import "./About.css";

export default function About() {
  const [info] = createResource<AboutInfo>(() => aboutAPI.get());
  const [copied, setCopied] = createSignal(false);

  async function copyDiagnostics(a: AboutInfo) {
    try {
      await navigator.clipboard.writeText(diagnosticsText(a));
      setCopied(true);
      setTimeout(() => setCopied(false), 2000);
    } catch (e) {
      toast.error(errorMessage(e, "Couldn't copy to the clipboard"));
    }
  }

  async function copyPath(path: string) {
    try {
      await navigator.clipboard.writeText(path);
      toast.success("Path copied");
    } catch (e) {
      toast.error(errorMessage(e, "Couldn't copy to the clipboard"));
    }
  }

  return (
    <Section
      title="About"
      description="Version, file locations, licences, and everything you need for a bug report."
    >
      {/* Read .error BEFORE .latest/(): in Solid, reading an errored resource
          THROWS, which would take the whole Settings modal down rather than
          showing a message. */}
      <Show
        when={!info.error}
        fallback={
          <p class="about-error">
            Couldn't load version information — {errorMessage(info.error)}
          </p>
        }
      >
        <Show when={info()} fallback={<p>Loading…</p>}>
          {(a) => (
            <>
              <div class="about-hero">
                <div class="about-mark" aria-hidden="true">
                  {/* Lucide "Sparkles" — the same mark as the top bar and the
                      app icon. Inline rather than imported so it can be sized
                      independently of the icon set's stroke conventions. */}
                  <svg
                    width="44" height="44" viewBox="0 0 24 24" fill="none"
                    stroke="currentColor" stroke-width="1.5"
                    stroke-linecap="round" stroke-linejoin="round"
                  >
                    <path d="M11.017 2.814a1 1 0 0 1 1.966 0l1.051 5.558a2 2 0 0 0 1.594 1.594l5.558 1.051a1 1 0 0 1 0 1.966l-5.558 1.051a2 2 0 0 0-1.594 1.594l-1.051 5.558a1 1 0 0 1-1.966 0l-1.051-5.558a2 2 0 0 0-1.594-1.594l-5.558-1.051a1 1 0 0 1 0-1.966l5.558-1.051a2 2 0 0 0 1.594-1.594z" />
                    <path d="M20 2v4" />
                    <path d="M22 4h-4" />
                    <circle cx="4" cy="20" r="2" />
                  </svg>
                </div>
                <div class="about-hero-text">
                  <h3>Lumen</h3>
                  <p class="about-version">
                    Version {a().version}
                    <Show when={a().commit}>
                      {" "}<span class="about-commit">({a().commit})</span>
                    </Show>
                  </p>
                  <p class="about-tagline">
                    A Windows desktop client for Plex that plays through PotPlayer.
                  </p>
                </div>
              </div>

              <div class="settings-row">
                <label>Links</label>
                <div class="settings-control about-links">
                  <a href={a().repository} target="_blank" rel="noreferrer">
                    Source on GitHub <ExternalLink size={12} />
                  </a>
                  <a href={a().issues} target="_blank" rel="noreferrer">
                    Report an issue <ExternalLink size={12} />
                  </a>
                  <a href={`${a().repository}/releases`} target="_blank" rel="noreferrer">
                    Releases <ExternalLink size={12} />
                  </a>
                </div>
              </div>

              <div class="settings-row">
                <label>Licence</label>
                <div class="settings-control">
                  <span class="about-value">{a().license}</span>
                  <p class="about-hint">
                    Free to use, modify and share. Not affiliated with Plex Inc.
                    or the PotPlayer team.
                  </p>
                </div>
              </div>

              <div class="settings-row">
                <label>Runtime</label>
                <div class="settings-control">
                  <span class="about-value">
                    {a().platform} · {a().goVersion}
                  </span>
                </div>
              </div>

              <div class="settings-row">
                <label>PotPlayer</label>
                <div class="settings-control">
                  <Show
                    when={a().potPlayer.detected}
                    fallback={
                      <span class="about-value about-value--warn">
                        Not detected — set the path in Playback
                      </span>
                    }
                  >
                    <code class="about-path">{a().potPlayer.path}</code>
                  </Show>
                </div>
              </div>

              <div class="settings-row about-row--top">
                <label>File locations</label>
                <div class="settings-control about-paths">
                  <For
                    each={[
                      ["Settings", a().paths.config],
                      ["Cache", a().paths.cache],
                      ["Logs", a().paths.logs],
                    ] as const}
                  >
                    {([label, path]) => (
                      <div class="about-path-row">
                        <span class="about-path-label">{label}</span>
                        <code class="about-path">{path}</code>
                        <button
                          class="about-copy-btn"
                          onClick={() => copyPath(path)}
                          aria-label={`Copy ${label.toLowerCase()} path`}
                        >
                          Copy
                        </button>
                      </div>
                    )}
                  </For>
                </div>
              </div>

              <div class="settings-row">
                <label>Bug reports</label>
                <div class="settings-control">
                  <button class="settings-btn" onClick={() => copyDiagnostics(a())}>
                    {copied() ? "Copied" : "Copy diagnostics"}
                  </button>
                  <p class="about-hint">
                    Copies your version and system details for pasting into a
                    GitHub issue. Contains no Plex tokens, API keys or details
                    about your library.
                  </p>
                </div>
              </div>

              <div class="settings-row about-row--top">
                <label>Built with</label>
                <div class="settings-control about-deps">
                  <For each={a().dependencies}>
                    {(d) => (
                      <a
                        class="about-dep"
                        href={d.url}
                        target="_blank"
                        rel="noreferrer"
                      >
                        <span class="about-dep-name">{d.name}</span>
                        <span class="about-dep-license">{d.license}</span>
                      </a>
                    )}
                  </For>
                </div>
              </div>
            </>
          )}
        </Show>
      </Show>
    </Section>
  );
}
