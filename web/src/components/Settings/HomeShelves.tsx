import { createResource, createSignal, For, Show } from "solid-js";
import Section from "./Section";
import { api } from "../../api/client";
import type { Collection, Library, Server } from "../../api/types";
import { store as settingsStore } from "../../state/settings";
import { librariesFor } from "../../state/libraries";
import { ChevronDown, ChevronRight } from "../icons";
import { errorMessage } from "../Toast";
import "./HomeShelves.css";

// Home Shelves — pick which Plex collections appear as shelves on Home.
//
// Recently Added needs no configuration: every video library gets one, which
// is derivable. Collections are not — this account can see eighty-four of
// them across two servers, and which ones are worth a shelf is a judgement
// only the user can make. So they are opted into here rather than guessed at.
//
// Collections are fetched per library on expand, not up front: eagerly loading
// every library's collections would be roughly thirty requests to render a
// settings panel the user may not even open.

function entryFor(serverID: string, libraryKey: string, title: string): string {
  return `${serverID}:${libraryKey}:${title}`;
}

export default function HomeShelves() {
  const [servers] = createResource(() => api.servers());

  const selected = () => settingsStore.settings()?.collectionShelves ?? [];

  function toggle(entry: string, on: boolean) {
    const current = selected();
    const next = on
      ? current.includes(entry)
        ? current
        : [...current, entry]
      : current.filter((e) => e !== entry);
    settingsStore.patch({ collectionShelves: next });
  }

  return (
    <Section
      title="Home Shelves"
      description="Every video library already gets a Recently Added shelf. Add collections from your servers here — they appear on Home and can be reordered by dragging."
    >
      <Show
        when={!servers.error}
        fallback={
          <p class="hs-error">Couldn't reach your servers — {errorMessage(servers.error)}</p>
        }
      >
        <Show when={servers()} fallback={<p>Loading…</p>}>
          {(list) => (
            <>
              <p class="hs-count">
                {selected().length === 0
                  ? "No collection shelves yet."
                  : `${selected().length} collection ${selected().length === 1 ? "shelf" : "shelves"} on Home.`}
              </p>
              <For each={list() as Server[]}>
                {(srv) => <ServerBlock server={srv} selected={selected} onToggle={toggle} />}
              </For>
            </>
          )}
        </Show>
      </Show>
    </Section>
  );
}

function ServerBlock(props: {
  server: Server;
  selected: () => string[];
  onToggle: (entry: string, on: boolean) => void;
}) {
  const [libs] = createResource(() => props.server.machineIdentifier, librariesFor);

  const hidden = () => new Set(settingsStore.settings()?.hiddenLibraries ?? []);

  // Mirrors deriveHomeLayout's filter exactly: a hidden library has no Home
  // group to hang a shelf on, and a music or photo library never gets one.
  // Offering them here would let the user tick something that can't appear.
  const videoLibs = () => {
    const all = (libs() ?? []) as Library[];
    const h = hidden();
    return all.filter(
      (l) =>
        (l.type === "movie" || l.type === "show") &&
        !h.has(`${props.server.machineIdentifier}:${l.key}`),
    );
  };

  return (
    <div class="hs-server">
      <h3 class="hs-server-name">{props.server.displayName || props.server.name}</h3>
      <Show when={libs.error}>
        <p class="hs-error">Couldn't load libraries — {errorMessage(libs.error)}</p>
      </Show>
      <Show when={!libs.error && libs() && videoLibs().length === 0}>
        <p class="hs-empty">No visible movie or TV libraries on this server.</p>
      </Show>
      <For each={videoLibs()}>
        {(lib) => (
          <LibraryRow
            serverID={props.server.machineIdentifier}
            library={lib}
            selected={props.selected}
            onToggle={props.onToggle}
          />
        )}
      </For>
    </div>
  );
}

function LibraryRow(props: {
  serverID: string;
  library: Library;
  selected: () => string[];
  onToggle: (entry: string, on: boolean) => void;
}) {
  const [open, setOpen] = createSignal(false);

  // Keyed on open() so nothing is requested until the user asks for it, and
  // the result is kept once fetched.
  const [cols] = createResource(
    () => (open() ? `${props.serverID}:${props.library.key}` : undefined),
    () => api.collections(props.serverID, props.library.key),
  );

  const chosenHere = () =>
    props.selected().filter((e) => e.startsWith(`${props.serverID}:${props.library.key}:`)).length;

  return (
    <div class="hs-library">
      <button class="hs-library-header" onClick={() => setOpen(!open())} aria-expanded={open()}>
        <span class="hs-caret">
          {open() ? <ChevronDown size={12} /> : <ChevronRight size={12} />}
        </span>
        <span class="hs-library-title">{props.library.title}</span>
        <Show when={chosenHere() > 0}>
          <span class="hs-badge">{chosenHere()}</span>
        </Show>
      </button>
      <Show when={open()}>
        <Show when={cols.error}>
          <p class="hs-error">Couldn't load collections — {errorMessage(cols.error)}</p>
        </Show>
        <Show when={!cols.error && cols.loading}>
          <p class="hs-loading">Loading collections…</p>
        </Show>
        <Show when={!cols.error && cols() && (cols() as Collection[]).length === 0}>
          <p class="hs-empty">This library has no collections.</p>
        </Show>
        <Show when={!cols.error && cols() && (cols() as Collection[]).length > 0}>
          <ul class="hs-collections">
            <For each={cols() as Collection[]}>
              {(c) => {
                const entry = entryFor(props.serverID, props.library.key, c.title);
                const on = () => props.selected().includes(entry);
                return (
                  <li>
                    <label class="hs-collection">
                      <input
                        type="checkbox"
                        checked={on()}
                        onChange={(e) => props.onToggle(entry, e.currentTarget.checked)}
                      />
                      <span>{c.title}</span>
                    </label>
                  </li>
                );
              }}
            </For>
          </ul>
        </Show>
      </Show>
    </div>
  );
}
