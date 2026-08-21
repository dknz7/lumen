import { createResource, createSignal, For, Show } from "solid-js";
import Section from "./Section";
import { api } from "../../api/client";
import type { Server } from "../../api/types";
import ReAuthModal from "../Modal/ReAuthModal";
import { toast, errorMessage } from "../Toast";
import { invalidateLibraries } from "../../state/libraries";
import { ExternalLink, RefreshCw } from "../icons";
import "./AccountsServers.css";

interface Account {
  username: string;
  email: string;
}

const PLEX_MANAGE_URL = "https://app.plex.tv/desktop/#!/settings/manage-library-access";

export default function AccountsServers() {
  const [account] = createResource<Account>(async () => {
    const res = await fetch("/api/user");
    if (!res.ok) throw new Error(`${res.status}`);
    return res.json();
  });
  const [servers, { refetch: refetchServers }] = createResource(() => api.servers());
  const [refreshing, setRefreshing] = createSignal(false);
  const [omdbKey, setOmdbKey] = createSignal("");
  const [omdbError, setOmdbError] = createSignal("");
  const [omdbSaved, setOmdbSaved] = createSignal(false);
  const [tmdbKey, setTmdbKey] = createSignal("");
  const [tmdbError, setTmdbError] = createSignal("");
  const [tmdbSaved, setTmdbSaved] = createSignal(false);
  const [reAuthOpen, setReAuthOpen] = createSignal(false);
  const [confirmForget, setConfirmForget] = createSignal<string | null>(null);

  async function renameServer(machineID: string, newName: string) {
    try {
      const res = await fetch(`/api/servers/${encodeURIComponent(machineID)}/rename`, {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({ displayName: newName }),
      });
      if (!res.ok) throw new Error(`${res.status} ${await res.text()}`);
      refetchServers();
      toast.success("Server renamed");
    } catch (e) {
      toast.error(errorMessage(e, "Couldn't rename the server"));
    }
  }

  async function forgetServer(machineID: string) {
    try {
      const res = await fetch(`/api/servers/${encodeURIComponent(machineID)}`, {
        method: "DELETE",
      });
      if (!res.ok) throw new Error(`${res.status} ${await res.text()}`);
      invalidateLibraries(machineID);
      setConfirmForget(null);
      refetchServers();
      toast.success("Server removed from Lumen");
    } catch (e) {
      toast.error(errorMessage(e, "Couldn't remove the server"));
    }
  }

  async function refreshConnections() {
    setRefreshing(true);
    try {
      const res = await fetch("/api/servers/refresh", { method: "POST" });
      if (!res.ok) throw new Error(`${res.status}`);
      const body = await res.json();
      // Discovery can add or remove servers, so the cached library lists are
      // no longer authoritative.
      invalidateLibraries();
      refetchServers();
      toast.success(`Found ${body.count} server${body.count === 1 ? "" : "s"}`);
    } catch (e) {
      toast.error(errorMessage(e, "Couldn't reach Plex to re-discover servers"));
    } finally {
      setRefreshing(false);
    }
  }

  async function saveKey(
    which: "omdb" | "tmdb",
    value: string,
    setErr: (s: string) => void,
    setSaved: (b: boolean) => void,
    clear: () => void,
  ) {
    try {
      const res = await fetch(`/api/settings/${which}`, {
        method: "PUT",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({ key: value }),
      });
      if (!res.ok) throw new Error(`${res.status}`);
      setErr("");
      clear();
      setSaved(true);
      toast.success(value === "" ? "Key cleared" : "Key saved");
    } catch (e) {
      setErr("Couldn't save — try again");
      toast.error(errorMessage(e, "Couldn't save the key"));
    }
  }

  function validateAndSave(
    which: "omdb" | "tmdb",
    raw: string,
    pattern: RegExp,
    hint: string,
    setErr: (s: string) => void,
    setSaved: (b: boolean) => void,
    clear: () => void,
  ) {
    const k = raw.trim();
    // Empty is a deliberate "remove my key", not a validation failure.
    if (k !== "" && !pattern.test(k)) {
      setErr(hint);
      return;
    }
    setErr("");
    saveKey(which, k, setErr, setSaved, clear);
  }

  return (
    <Section
      title="Accounts & Servers"
      description="Your Plex sign-in, the servers Lumen can see, and optional keys for ratings and trailers."
    >
      <div class="settings-row">
        <label>Plex account</label>
        <div class="settings-control">
          <Show
            when={!account.error}
            fallback={<span class="as-warn">Couldn't load your account — {errorMessage(account.error)}</span>}
          >
            <Show when={account()} fallback={<span>Loading…</span>}>
              {(a) => (
                <div class="as-account">
                  <strong>{a().username}</strong>
                  <span class="as-email">{a().email}</span>
                </div>
              )}
            </Show>
          </Show>
          <button class="settings-btn as-reauth" onClick={() => setReAuthOpen(true)}>
            Sign in again
          </button>
          <ReAuthModal
            open={reAuthOpen()}
            onClose={() => setReAuthOpen(false)}
            onLinked={() => {
              invalidateLibraries();
              refetchServers();
              toast.success("Plex account linked");
            }}
          />
          <p class="as-hint">
            Use this if your sign-in expires or you want to switch to a
            different Plex account.
          </p>
        </div>
      </div>

      <div class="settings-row as-row--top">
        <label>Servers</label>
        <div class="settings-control">
          {/* The honest explanation. Lumen cannot add a server, because access
              is granted on the Plex account, not in the client — saying so
              plainly is better than an "Add server" button that can't work. */}
          <p class="as-hint as-hint--lead">
            Lumen shows every server your Plex account can reach. It can't add
            one — sharing and access are managed on your Plex account, and
            anything you're given access to shows up here after a refresh.
          </p>

          <Show
            when={!servers.error}
            fallback={
              <div class="as-error">
                <span>Couldn't load your servers — {errorMessage(servers.error)}</span>
                <button class="settings-btn" onClick={() => refetchServers()}>Retry</button>
              </div>
            }
          >
            <Show when={servers()} fallback={<p>Loading…</p>}>
              {(srvs) => (
                <Show
                  when={(srvs() as Server[]).length > 0}
                  fallback={
                    <div class="as-empty">
                      <p>No servers yet.</p>
                      <p class="as-hint">
                        If you've just been granted access to one, refresh below.
                      </p>
                    </div>
                  }
                >
                  <div class="as-server-list">
                    <For each={srvs() as Server[]}>
                      {(srv) => (
                        <div class="as-server">
                          <div class="as-server-main">
                            <input
                              class="as-server-name"
                              type="text"
                              value={srv.displayName}
                              aria-label={`Display name for ${srv.name}`}
                              onChange={(e) => renameServer(srv.machineIdentifier, e.currentTarget.value)}
                            />
                            <span
                              class="as-server-status"
                              classList={{ "as-server-status--on": srv.status === "connected" }}
                            >
                              {srv.status}
                            </span>
                          </div>
                          <div class="as-server-meta">
                            <span class="as-server-id" title={srv.machineIdentifier}>
                              {srv.machineIdentifier.slice(0, 12)}…
                            </span>
                            <Show
                              when={confirmForget() === srv.machineIdentifier}
                              fallback={
                                <button
                                  class="as-forget"
                                  onClick={() => setConfirmForget(srv.machineIdentifier)}
                                >
                                  Forget
                                </button>
                              }
                            >
                              <span class="as-confirm">
                                Remove from Lumen?
                                <button
                                  class="as-forget as-forget--yes"
                                  onClick={() => forgetServer(srv.machineIdentifier)}
                                >
                                  Remove
                                </button>
                                <button class="as-forget" onClick={() => setConfirmForget(null)}>
                                  Cancel
                                </button>
                              </span>
                            </Show>
                          </div>
                        </div>
                      )}
                    </For>
                  </div>
                </Show>
              )}
            </Show>
          </Show>

          <div class="as-server-actions">
            <button class="settings-btn" disabled={refreshing()} onClick={refreshConnections}>
              <RefreshCw size={13} />
              {refreshing() ? "Refreshing…" : "Refresh servers"}
            </button>
            <a class="as-link" href={PLEX_MANAGE_URL} target="_blank" rel="noreferrer">
              Manage sharing on Plex <ExternalLink size={12} />
            </a>
          </div>
          <p class="as-hint">
            Renaming a server only changes what Lumen calls it. "Forget" removes
            it from Lumen's list — it doesn't revoke anything, and the server
            comes back on the next refresh if your account can still see it.
          </p>
        </div>
      </div>

      <div class="settings-row as-row--top">
        <label for="omdbKey">OMDB API key</label>
        <div class="settings-control">
          <input
            id="omdbKey"
            type="password"
            placeholder="8-character key"
            value={omdbKey()}
            onInput={(e) => { setOmdbKey(e.currentTarget.value); setOmdbSaved(false); }}
            aria-invalid={omdbError() !== ""}
            aria-describedby={omdbError() ? "omdbError" : "omdbHelp"}
          />
          <button
            type="button"
            class={omdbSaved() ? "settings-key-save settings-key-save--saved" : "settings-key-save"}
            onClick={() =>
              validateAndSave("omdb", omdbKey(), /^[a-f0-9]{8}$/i,
                "Expected 8 hexadecimal characters (e.g. 1a2b3c4d).",
                setOmdbError, setOmdbSaved, () => setOmdbKey(""))
            }
          >
            {omdbSaved() ? "Saved" : "Save"}
          </button>
          <Show when={omdbError()}>
            <div id="omdbError" role="alert" class="as-field-error">{omdbError()}</div>
          </Show>
          <p id="omdbHelp" class="as-hint">
            Optional — adds IMDB ratings to detail pages. Free.{" "}
            <a class="as-link as-link--inline" href="https://www.omdbapi.com/apikey.aspx" target="_blank" rel="noreferrer">
              Get a key <ExternalLink size={11} />
            </a>
          </p>
        </div>
      </div>

      <div class="settings-row as-row--top">
        <label for="tmdbKey">TMDB API key</label>
        <div class="settings-control">
          <input
            id="tmdbKey"
            type="password"
            placeholder="32-character key"
            value={tmdbKey()}
            onInput={(e) => { setTmdbKey(e.currentTarget.value); setTmdbSaved(false); }}
            aria-invalid={tmdbError() !== ""}
            aria-describedby={tmdbError() ? "tmdbError" : "tmdbHelp"}
          />
          <button
            type="button"
            class={tmdbSaved() ? "settings-key-save settings-key-save--saved" : "settings-key-save"}
            onClick={() =>
              validateAndSave("tmdb", tmdbKey(), /^[a-f0-9]{32}$/i,
                "Expected 32 hexadecimal characters.",
                setTmdbError, setTmdbSaved, () => setTmdbKey(""))
            }
          >
            {tmdbSaved() ? "Saved" : "Save"}
          </button>
          <Show when={tmdbError()}>
            <div id="tmdbError" role="alert" class="as-field-error">{tmdbError()}</div>
          </Show>
          <p id="tmdbHelp" class="as-hint">
            Optional — enables the Play Trailer button. Free.{" "}
            <a class="as-link as-link--inline" href="https://www.themoviedb.org/settings/api" target="_blank" rel="noreferrer">
              Get a key <ExternalLink size={11} />
            </a>
          </p>
          <p class="as-hint">
            Saved keys are stored encrypted on this machine and never shown
            again — leave a field blank and Save to remove one.
          </p>
        </div>
      </div>
    </Section>
  );
}
