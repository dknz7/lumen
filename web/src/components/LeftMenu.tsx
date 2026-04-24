import { createResource, createSignal, For, Show } from "solid-js";
import { A } from "@solidjs/router";
import { api } from "../api/client";
import type { Library, Server } from "../api/types";
import "./LeftMenu.css";

export default function LeftMenu() {
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
        <li><A href="/settings" activeClass="active">⚙ Settings</A></li>
      </ul>
    </nav>
  );
}

function ServerLibraries(props: { server: Server }) {
  const [libs] = createResource(() => api.libraries(props.server.machineIdentifier));
  const [expanded, setExpanded] = createSignal(true);
  return (
    <div class="server-group">
      <button class="server-group-header" onClick={() => setExpanded(!expanded())}>
        <span class="caret">{expanded() ? "▾" : "▸"}</span>
        <span>{props.server.displayName}</span>
        <span class="server-status" data-status={props.server.status} />
      </button>
      <Show when={expanded() && libs()}>
        {(libList) => (
          <ul class="library-list">
            <For each={libList() as Library[]}>
              {(l) => (
                <li>
                  <A
                    href={`/library/${props.server.machineIdentifier}/${l.key}`}
                    activeClass="active"
                  >
                    {l.title}
                  </A>
                </li>
              )}
            </For>
          </ul>
        )}
      </Show>
    </div>
  );
}
