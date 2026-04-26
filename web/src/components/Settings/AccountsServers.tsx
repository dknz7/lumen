import { createResource, createSignal, For, Show } from "solid-js";
import Section from "./Section";
import { api } from "../../api/client";
import ReAuthModal from "../Modal/ReAuthModal";

interface Account {
  username: string;
  email: string;
}

export default function AccountsServers() {
  const [account] = createResource<Account>(async () => {
    const res = await fetch("/api/user");
    if (!res.ok) throw new Error(`${res.status}`);
    return res.json();
  });
  const [servers, { refetch: refetchServers }] = createResource(() => api.servers());
  const [refreshing, setRefreshing] = createSignal(false);
  const [refreshStatus, setRefreshStatus] = createSignal("");
  const [omdbKey, setOmdbKey] = createSignal("");
  const [omdbError, setOmdbError] = createSignal("");
  const [tmdbKey, setTmdbKey] = createSignal("");
  const [tmdbError, setTmdbError] = createSignal("");
  const [reAuthOpen, setReAuthOpen] = createSignal(false);

  async function renameServer(machineID: string, newName: string) {
    const res = await fetch(`/api/servers/${encodeURIComponent(machineID)}/rename`, {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({ displayName: newName }),
    });
    if (!res.ok) throw new Error(`${res.status} ${await res.text()}`);
    refetchServers();
  }

  async function refreshConnections() {
    setRefreshing(true);
    setRefreshStatus("Re-discovering…");
    try {
      const res = await fetch("/api/servers/refresh", { method: "POST" });
      if (!res.ok) throw new Error(`${res.status}`);
      refetchServers();
      setRefreshStatus("Done.");
    } catch (e) {
      setRefreshStatus(`Failed: ${(e as Error).message}`);
    } finally {
      setRefreshing(false);
    }
  }

  function saveOMDB() {
    fetch("/api/settings/omdb", {
      method: "PUT",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({ key: omdbKey() }),
    });
  }

  function validateAndSaveOMDB() {
    const k = omdbKey().trim();
    if (k === "") {
      setOmdbError("");
      saveOMDB();
      return;
    }
    // OMDB keys are exactly 8 hex chars.
    if (!/^[a-f0-9]{8}$/i.test(k)) {
      setOmdbError("Expected 8 hexadecimal characters (e.g. 1a2b3c4d).");
      return;
    }
    setOmdbError("");
    saveOMDB();
  }

  function saveTMDB() {
    fetch("/api/settings/tmdb", {
      method: "PUT",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({ key: tmdbKey() }),
    });
  }

  function validateAndSaveTMDB() {
    const k = tmdbKey().trim();
    if (k === "") {
      setTmdbError("");
      saveTMDB();
      return;
    }
    // TMDB v3 keys are 32 hex chars.
    if (!/^[a-f0-9]{32}$/i.test(k)) {
      setTmdbError("Expected 32 hexadecimal characters.");
      return;
    }
    setTmdbError("");
    saveTMDB();
  }

  return (
    <Section title="Accounts & Servers" description="Plex account, server names, and OMDB integration.">
      <div class="settings-row">
        <label>Plex account</label>
        <div class="settings-control">
          <Show when={account()} fallback={<span>Loading…</span>}>
            {(a) => (
              <div style={{ "display": "flex", "gap": "12px", "align-items": "center" }}>
                <strong>{(a() as Account).username}</strong>
                <span style={{ "color": "var(--text-muted)", "font-size": "12px" }}>{(a() as Account).email}</span>
              </div>
            )}
          </Show>
        </div>
      </div>

      <div class="settings-row">
        <label>Re-authenticate</label>
        <div class="settings-control">
          <button class="settings-btn" onClick={() => setReAuthOpen(true)}>
            Re-authenticate
          </button>
          <ReAuthModal
            open={reAuthOpen()}
            onClose={() => setReAuthOpen(false)}
            onLinked={() => { /* token persisted server-side; nothing more to do */ }}
          />
        </div>
      </div>

      <div class="settings-row">
        <label>Servers</label>
        <div class="settings-control">
          <Show when={servers()}>
            {(srvs) => (
              <div style={{ "display": "flex", "flex-direction": "column", "gap": "8px" }}>
                <For each={srvs() as any[]}>
                  {(srv) => (
                    <div style={{ "display": "grid", "grid-template-columns": "1fr 160px 80px", "gap": "8px", "align-items": "center" }}>
                      <input
                        type="text"
                        value={srv.displayName}
                        onChange={(e) => renameServer(srv.machineIdentifier, e.currentTarget.value).catch((err) => alert(err.message))}
                      />
                      <span style={{ "color": srv.status === "connected" ? "var(--status-online)" : "var(--status-offline)", "font-size": "12px" }}>
                        {srv.status}
                      </span>
                      <span style={{ "color": "var(--text-muted)", "font-size": "11px", "overflow": "hidden", "text-overflow": "ellipsis" }}>
                        {srv.machineIdentifier.slice(0, 8)}…
                      </span>
                    </div>
                  )}
                </For>
                <button class="settings-btn" disabled={refreshing()} onClick={refreshConnections}>
                  {refreshing() ? "Refreshing…" : "Refresh connections"}
                </button>
                {refreshStatus() && (
                  <div style={{ "color": "var(--text-muted)", "font-size": "12px" }}>{refreshStatus()}</div>
                )}
              </div>
            )}
          </Show>
        </div>
      </div>

      <div class="settings-row">
        <label for="omdbKey">OMDB API key</label>
        <div class="settings-control">
          <input
            id="omdbKey"
            type="password"
            placeholder="8-char hex key (powers IMDB ratings on Item Detail)"
            value={omdbKey()}
            onInput={(e) => setOmdbKey(e.currentTarget.value)}
            onBlur={validateAndSaveOMDB}
            aria-invalid={omdbError() !== ""}
            aria-describedby={omdbError() ? "omdbError" : undefined}
          />
          {omdbError() && (
            <div id="omdbError" role="alert" style={{ "margin-top": "4px", "color": "#f07878", "font-size": "12px" }}>
              {omdbError()}
            </div>
          )}
          <div style={{ "margin-top": "4px", "font-size": "11px" }}>
            <a href="https://www.omdbapi.com/apikey.aspx" target="_blank" rel="noreferrer" style={{ "color": "var(--text-muted)" }}>
              Get a free key →
            </a>
          </div>
        </div>
      </div>

      <div class="settings-row">
        <label for="tmdbKey">TMDB API key</label>
        <div class="settings-control">
          <input
            id="tmdbKey"
            type="password"
            placeholder="32-char hex key (powers Play Trailer button)"
            value={tmdbKey()}
            onInput={(e) => setTmdbKey(e.currentTarget.value)}
            onBlur={validateAndSaveTMDB}
            aria-invalid={tmdbError() !== ""}
            aria-describedby={tmdbError() ? "tmdbError" : undefined}
          />
          {tmdbError() && (
            <div id="tmdbError" role="alert" style={{ "margin-top": "4px", "color": "#f07878", "font-size": "12px" }}>
              {tmdbError()}
            </div>
          )}
          <div style={{ "margin-top": "4px", "font-size": "11px" }}>
            <a href="https://www.themoviedb.org/settings/api" target="_blank" rel="noreferrer" style={{ "color": "var(--text-muted)" }}>
              Get a free key →
            </a>
          </div>
        </div>
      </div>
    </Section>
  );
}
