import { createResource, createSignal, For, Show } from "solid-js";
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
            <For each={srvs()}>
              {(srv) => <ServerLibraries server={srv} />}
            </For>
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
  const isHidden = (libKey: string) => hiddenSet().has(key(libKey));

  function toggleHidden(libKey: string) {
    const set = new Set(hiddenSet());
    const k = key(libKey);
    if (set.has(k)) set.delete(k); else set.add(k);
    settingsStore.patch({ hiddenLibraries: Array.from(set) });
  }

  return (
    <div class="server-group">
      <button class="server-group-header" onClick={() => setExpanded(!expanded())}>
        <span class="caret">{expanded() ? <ChevronDown size={10} /> : <ChevronRight size={10} />}</span>
        <span>{props.server.displayName}</span>
        <span class="server-status" data-status={props.server.status} />
      </button>
      <Show when={expanded() && libs()}>
        {(libList) => (
          <ul class="library-list">
            <For each={libList() as Library[]}>
              {(l) => (
                <li class="library-row" classList={{ "is-hidden": isHidden(l.key) }}>
                  <A
                    href={`/library/${props.server.machineIdentifier}/${l.key}`}
                    activeClass="active"
                  >
                    {l.title}
                  </A>
                  <button
                    class="library-eye"
                    onClick={(e) => { e.preventDefault(); e.stopPropagation(); toggleHidden(l.key); }}
                    title={isHidden(l.key) ? "Show library" : "Hide library"}
                    aria-label="Toggle library visibility"
                  >
                    {isHidden(l.key) ? <EyeOff size={12} /> : <Eye size={12} />}
                  </button>
                </li>
              )}
            </For>
          </ul>
        )}
      </Show>
    </div>
  );
}
