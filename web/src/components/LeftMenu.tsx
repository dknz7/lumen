import { createMemo, createResource, createSignal, For, Show } from "solid-js";
import { A } from "@solidjs/router";
import { api } from "../api/client";
import type { Library, Server } from "../api/types";
import { store as settingsStore } from "../state/settings";
import {
  Bookmark,
  ChevronDown,
  ChevronRight,
  Compass,
  Eye,
  EyeOff,
  Home,
  Library as LibraryIcon,
  Server as ServerIcon,
  Settings,
  Sparkles,
  } from "./icons";
import type { JSX } from "solid-js";
import "./LeftMenu.css";
import { librariesFor } from "../state/libraries";

// Every server gets the same icon.
//
// This used to special-case one server by NAME — a substring match that handed
// a star to anything called "stargaze" and meant nothing on any other install.
function iconForServer(): JSX.Element {
  return <ServerIcon size={12} class="menu-link-icon" />;
}

export default function LeftMenu(props: { onOpenSettings: () => void }) {
  const [servers] = createResource(() => api.servers());
  return (
    <nav class="left-menu">
      <ul class="menu-top">
        <li>
          <A href="/" activeClass="active" end>
            <Home size={14} class="menu-link-icon" />
            <span>Home</span>
          </A>
        </li>
        <li>
          <A href="/watchlist" activeClass="active">
            <Bookmark size={14} class="menu-link-icon" />
            <span>Watchlist</span>
          </A>
        </li>
        <li>
          <A href="/recommended" activeClass="active">
            <Sparkles size={14} class="menu-link-icon" />
            <span>Recommended</span>
          </A>
        </li>
        <li>
          <A href="/discover" activeClass="active">
            <Compass size={14} class="menu-link-icon" />
            <span>Discover</span>
          </A>
        </li>
      </ul>
      <div class="libraries-section">
        <div class="libraries-label">
          <LibraryIcon size={12} class="menu-link-icon" />
          <span>LIBRARIES</span>
        </div>
        <Show when={servers.error}>
          <div class="libraries-error" role="alert">Couldn't load your libraries.</div>
        </Show>
        <Show when={!servers.error && servers()}>
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
      <div class="settings-pill-wrap">
        <button
          class="settings-pill"
          onClick={props.onOpenSettings}
          aria-label="Open Settings"
        >
          <span class="settings-led" aria-hidden="true" />
          <Settings size={14} />
          <span class="settings-pill-label">Settings</span>
        </button>
      </div>
    </nav>
  );
}

function ServerLibraries(props: { server: Server }) {
  const [libs] = createResource(() => librariesFor(props.server.machineIdentifier));
  const [expanded, setExpanded] = createSignal(true);
  const hiddenSet = () => new Set(settingsStore.settings()?.hiddenLibraries ?? []);
  const key = (libKey: string) => `${props.server.machineIdentifier}:${libKey}`;

  function hideLibrary(libKey: string) {
    const set = new Set(hiddenSet());
    set.add(key(libKey));
    settingsStore.patch({ hiddenLibraries: Array.from(set) });
  }

  // Filter out hidden libraries — spec §10.2 + the user's preference: hidden
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
        {iconForServer()}
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
 *
 * Fetches libraries from every server so we can resolve each hidden key's
 * actual title (e.g. "Movies - Kids") instead of showing the opaque numeric
 * libKey. The fetch is keyed on the server-id list so it re-runs only when
 * servers change; browser/server image-of-libs caching dedupes against the
 * per-server fetches in ServerLibraries.
 */
function HiddenLibraries(props: { servers: Server[] }) {
  const [expanded, setExpanded] = createSignal(false);

  // Every reactive derivation routes through these memos so the dependency
  // chain settings signal → hiddenKeys → hiddenCount + entries is single-rooted.
  // Earlier the count + Show subscribed via the entries memo, which sometimes
  // missed live updates on rapid successive hides; deriving count from
  // hiddenKeys directly removes that path.
  const hiddenKeys = createMemo<string[]>(() =>
    settingsStore.settings()?.hiddenLibraries ?? []
  );
  const hiddenCount = createMemo(() => hiddenKeys().length);

  const [titleMap] = createResource(
    () => props.servers.map((s) => s.machineIdentifier).join("|"),
    async () => {
      const map = new Map<string, string>();
      await Promise.all(
        props.servers.map(async (s) => {
          try {
            const libs = await librariesFor(s.machineIdentifier);
            for (const l of libs) {
              map.set(`${s.machineIdentifier}:${l.key}`, l.title);
            }
          } catch (e) {
            console.error(`HiddenLibraries: libraries fetch failed for ${s.displayName}`, e);
          }
        })
      );
      return map;
    }
  );

  const entries = createMemo(() => {
    const srvByID = new Map(props.servers.map((s) => [s.machineIdentifier, s]));
    const titles = titleMap();
    return hiddenKeys().map((k) => {
      const [serverID, libKey] = k.split(":");
      const srv = srvByID.get(serverID);
      return {
        fullKey: k,
        serverID,
        libKey,
        libTitle: titles?.get(k) ?? libKey,
        serverName: srv?.displayName ?? serverID,
      };
    });
  });

  function restore(fullKey: string) {
    const next = hiddenKeys().filter((k) => k !== fullKey);
    settingsStore.patch({ hiddenLibraries: next });
  }

  return (
    <Show when={hiddenCount() > 0}>
      <div class="hidden-libraries">
        <button class="hidden-libraries-header" onClick={() => setExpanded(!expanded())}>
          <span class="caret">{expanded() ? <ChevronDown size={10} /> : <ChevronRight size={10} />}</span>
          <span>Hidden ({hiddenCount()})</span>
        </button>
        <Show when={expanded()}>
          <ul class="library-list">
            <For each={entries()}>
              {(e) => (
                <li class="library-row hidden-row">
                  <span class="hidden-label">
                    <span class="hidden-title">{e.libTitle}</span>
                    <span class="hidden-server"> ({e.serverName})</span>
                  </span>
                  <button
                    class="library-eye"
                    onClick={() => restore(e.fullKey)}
                    title="Restore library"
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
