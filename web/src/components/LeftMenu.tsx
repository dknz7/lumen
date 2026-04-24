import { createMemo, createResource, createSignal, For, Show } from "solid-js";
import { A } from "@solidjs/router";
import { api } from "../api/client";
import type { Library, Server } from "../api/types";
import { store as settingsStore } from "../state/settings";
import { ChevronDown, ChevronRight, Eye, EyeOff, Settings } from "./icons";
import "./LeftMenu.css";

export default function LeftMenu(props: { onOpenSettings: () => void }) {
  const [servers] = createResource(() => api.servers());
  return (
    <nav class="left-menu">
      <ul class="menu-top">
        <li><A href="/" activeClass="active" end>Home</A></li>
        <li><A href="/watchlist" activeClass="active">Watchlist</A></li>
        <li><A href="/recommended" activeClass="active">Recommended</A></li>
        <li><A href="/discover" activeClass="active">Discover</A></li>
      </ul>
      <div class="libraries-section">
        <div class="libraries-label">LIBRARIES</div>
        <Show when={servers()}>
          {(srvs) => (
            <>
              <For each={srvs()}>
                {(srv) => <ServerLibraries server={srv} />}
              </For>
              <HiddenLibraries servers={srvs() as Server[]} />
            </>
          )}
        </Show>
      </div>
      <div class="menu-spacer" />
      <ul class="menu-bottom">
        <li>
          <button class="menu-settings-btn" onClick={props.onOpenSettings}>
            <Settings size={14} /> Settings
          </button>
        </li>
      </ul>
    </nav>
  );
}

function ServerLibraries(props: { server: Server }) {
  const [libs] = createResource(() => api.libraries(props.server.machineIdentifier));
  const [expanded, setExpanded] = createSignal(true);
  const hiddenSet = () => new Set(settingsStore.settings()?.hiddenLibraries ?? []);
  const key = (libKey: string) => `${props.server.machineIdentifier}:${libKey}`;

  function hideLibrary(libKey: string) {
    const set = new Set(hiddenSet());
    set.add(key(libKey));
    settingsStore.patch({ hiddenLibraries: Array.from(set) });
  }

  // Filter out hidden libraries — spec §10.2 + Byron's preference: hidden
  // libraries disappear entirely from the menu, managed via the restore
  // section at the bottom rather than sitting here greyed-out.
  const visibleLibs = createMemo(() => {
    const all = (libs() ?? []) as Library[];
    const hidden = hiddenSet();
    return all.filter((l) => !hidden.has(key(l.key)));
  });

  return (
    <div class="server-group">
      <button class="server-group-header" onClick={() => setExpanded(!expanded())}>
        <span class="caret">{expanded() ? <ChevronDown size={10} /> : <ChevronRight size={10} />}</span>
        <span>{props.server.displayName}</span>
        <span class="server-status" data-status={props.server.status} />
      </button>
      <Show when={expanded() && libs()}>
        <ul class="library-list">
          <For each={visibleLibs()}>
            {(l) => (
              <li class="library-row">
                <A
                  href={`/library/${props.server.machineIdentifier}/${l.key}`}
                  activeClass="active"
                >
                  {l.title}
                </A>
                <button
                  class="library-eye"
                  onClick={(e) => { e.preventDefault(); e.stopPropagation(); hideLibrary(l.key); }}
                  title="Hide library"
                  aria-label="Hide library"
                >
                  <Eye size={12} />
                </button>
              </li>
            )}
          </For>
        </ul>
      </Show>
    </div>
  );
}

/**
 * Hidden Libraries restore section — only rendered when at least one library
 * has been hidden. Starts collapsed so it doesn't visually crowd the menu.
 */
function HiddenLibraries(props: { servers: Server[] }) {
  const [expanded, setExpanded] = createSignal(false);
  const hiddenKeys = () => settingsStore.settings()?.hiddenLibraries ?? [];

  // Resolve each "serverID:libKey" to { server, lib.title } by querying libs
  // from every server. For v1.0 we just display the key string; a future
  // enhancement could cache all libs cross-server for real title lookup.
  // For now, display "<DisplayName> · <libKey>" — good enough for restore UX.
  const entries = () => {
    const srvByID = new Map(props.servers.map((s) => [s.machineIdentifier, s]));
    return hiddenKeys().map((k) => {
      const [serverID, libKey] = k.split(":");
      const srv = srvByID.get(serverID);
      return {
        fullKey: k,
        serverID,
        libKey,
        serverName: srv?.displayName ?? serverID,
      };
    });
  };

  function restore(fullKey: string) {
    const next = hiddenKeys().filter((k) => k !== fullKey);
    settingsStore.patch({ hiddenLibraries: next });
  }

  return (
    <Show when={entries().length > 0}>
      <div class="hidden-libraries">
        <button class="hidden-libraries-header" onClick={() => setExpanded(!expanded())}>
          <span class="caret">{expanded() ? <ChevronDown size={10} /> : <ChevronRight size={10} />}</span>
          <span>Hidden ({entries().length})</span>
        </button>
        <Show when={expanded()}>
          <ul class="library-list">
            <For each={entries()}>
              {(e) => (
                <li class="library-row hidden-row">
                  <span class="hidden-label">
                    <span class="hidden-server">{e.serverName}</span>
                    <span class="hidden-sep"> · </span>
                    <span>{e.libKey}</span>
                  </span>
                  <button
                    class="library-eye"
                    onClick={() => restore(e.fullKey)}
                    title="Show library"
                    aria-label="Restore library"
                  >
                    <EyeOff size={12} />
                  </button>
                </li>
              )}
            </For>
          </ul>
        </Show>
      </div>
    </Show>
  );
}
