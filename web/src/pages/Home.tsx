import { createResource, For, Show } from "solid-js";
import { api } from "../api/client";
import type { Item, Library, Server } from "../api/types";
import Group from "../components/Group";
import Shelf from "../components/Shelf";
import Card from "../components/Card";
import "./Home.css";

// Shelf definitions — one entry per shelf on the Home page (spec §12.1).
// Session 2 stubs shelves that require Plex Collections (Trending Movies / Trending TV Shows).

type ShelfDef =
  | { kind: "ondeck-merged"; id: string; title: string }
  | { kind: "server-recent"; id: string; title: string; serverName: string; libraryName: string }
  | { kind: "stub"; id: string; title: string; reason: string };

const STARGAZE_SHELVES: ShelfDef[] = [
  { kind: "stub", id: "stargaze-trending-movies", title: "Trending Movies", reason: "Plex Collections — Session 5" },
  { kind: "server-recent", id: "stargaze-recent-movies", title: "Recently Released Movies", serverName: "Stargaze", libraryName: "Movies" },
  { kind: "server-recent", id: "stargaze-recent-movies-4k", title: "Recently Released Movies (4K)", serverName: "Stargaze", libraryName: "Movies - 4K" },
  { kind: "stub", id: "stargaze-trending-tv", title: "Trending TV Shows", reason: "Plex Collections — Session 5" },
  { kind: "server-recent", id: "stargaze-recent-episodes", title: "Recently Released Episodes", serverName: "Stargaze", libraryName: "TV Shows" },
  { kind: "server-recent", id: "stargaze-recent-episodes-4k", title: "Recently Released Episodes (4K)", serverName: "Stargaze", libraryName: "TV Shows - 4K" },
  { kind: "server-recent", id: "stargaze-recent-anime", title: "Recently Released Anime Episodes", serverName: "Stargaze", libraryName: "Anime" },
];

const DKNZPLEX_SHELVES: ShelfDef[] = [
  { kind: "server-recent", id: "dknzplex-recent-movies", title: "Recently Released Movies", serverName: "DKNZPLEX", libraryName: "Movies" },
  { kind: "server-recent", id: "dknzplex-recent-movies-4k", title: "Recently Released Movies (4K)", serverName: "DKNZPLEX", libraryName: "Movies - 4K UHD" },
  { kind: "server-recent", id: "dknzplex-recent-anime-movies", title: "Recently Released Anime Movies", serverName: "DKNZPLEX", libraryName: "Movies - Anime" },
  { kind: "server-recent", id: "dknzplex-recent-episodes", title: "Recently Released Episodes", serverName: "DKNZPLEX", libraryName: "TV Shows" },
  { kind: "server-recent", id: "dknzplex-recent-episodes-4k", title: "Recently Released Episodes (4K)", serverName: "DKNZPLEX", libraryName: "TV Shows - 4K HDR" },
  { kind: "server-recent", id: "dknzplex-recent-anime-episodes", title: "Recently Released Anime Episodes", serverName: "DKNZPLEX", libraryName: "TV Shows - Anime" },
];

export default function Home() {
  const [servers] = createResource(() => api.servers());
  return (
    <div class="home-page">
      <Show when={servers()}>
        {(srvs) => (
          <>
            <ContinueWatching servers={srvs() as Server[]} />
            {/* Stargaze group — resolve displayName match, not hardcoded */}
            <ServerGroup srvs={srvs() as Server[]} logicalName="Stargaze" shelves={STARGAZE_SHELVES} />
            <ServerGroup srvs={srvs() as Server[]} logicalName="DKNZPLEX" shelves={DKNZPLEX_SHELVES} />
          </>
        )}
      </Show>
    </div>
  );
}

function ContinueWatching(props: { servers: Server[] }) {
  // Take a stable snapshot of machine IDs at mount. props.servers rarely changes
  // within a single page view, but the reactive accessor was causing the resource
  // to refetch or never settle.
  const machineIDs = props.servers.map((s) => s.machineIdentifier);

  const [decks] = createResource(
    () => machineIDs.join(","), // stable primitive source — refetch only if IDs change
    async () => {
      const results = await Promise.all(machineIDs.map(async (id) => {
        try {
          const items = await api.onDeck(id);
          return { id, items, error: null as string | null };
        } catch (e) {
          console.error(`onDeck failed for server ${id}:`, e);
          return { id, items: [], error: (e as Error).message };
        }
      }));
      const totalErrors = results.filter((r) => r.error !== null).length;
      if (totalErrors === results.length && results.length > 0) {
        throw new Error(`onDeck failed for all ${results.length} servers`);
      }
      return results.flatMap((r) => r.items.map((it) => ({ ...it, serverID: r.id })));
    }
  );
  return (
    <Shelf id="continue-watching" title="Continue Watching">
      <Show
        when={!decks.loading}
        fallback={<div class="shelf-loading">Loading Continue Watching…</div>}
      >
        <Show
          when={decks.error}
          fallback={
            <Show
              when={(decks() ?? []).length > 0}
              fallback={<div class="shelf-stub">Nothing in progress across your servers.</div>}
            >
              <For each={(decks() ?? []) as (Item & { serverID: string })[]}>
                {(it) => (
                  <Card
                    title={it.title}
                    year={it.year}
                    ratingKey={it.ratingKey}
                    serverID={it.serverID}
                  />
                )}
              </For>
            </Show>
          }
        >
          <div class="shelf-stub">Continue Watching failed: {(decks.error as Error)?.message}</div>
        </Show>
      </Show>
    </Shelf>
  );
}

function ServerGroup(props: { srvs: Server[]; logicalName: string; shelves: ShelfDef[] }) {
  // Find the server whose displayName contains the logical name (case-insensitive).
  // This gracefully handles Stargaze's empty-name fallback to machineIdentifier.
  const matched = () =>
    props.srvs.find((s) =>
      s.displayName.toLowerCase().includes(props.logicalName.toLowerCase())
    ) ?? props.srvs.find((s) => s.name.toLowerCase() === props.logicalName.toLowerCase());

  return (
    <Group id={`group-${props.logicalName.toLowerCase()}`} title={props.logicalName}>
      <Show when={matched()} fallback={<div class="group-missing">{props.logicalName} not found in servers — run `lumen list`</div>}>
        {(srv) => (
          <For each={props.shelves}>
            {(def) => <ShelfLoader server={srv() as Server} def={def} />}
          </For>
        )}
      </Show>
    </Group>
  );
}

function ShelfLoader(props: { server: Server; def: ShelfDef }) {
  if (props.def.kind === "stub") {
    return (
      <Shelf id={props.def.id} title={props.def.title} initialCollapsed>
        <div class="shelf-stub">({props.def.reason})</div>
      </Shelf>
    );
  }
  if (props.def.kind === "ondeck-merged") {
    return null; // handled by <ContinueWatching />
  }
  // server-recent
  const [libs] = createResource(() => api.libraries(props.server.machineIdentifier));
  return (
    <Shelf id={props.def.id} title={props.def.title}>
      <Show when={libs()}>
        {(libList) => {
          const lib = (libList() as Library[]).find((l) => l.title === props.def.libraryName);
          if (!lib) {
            return <div class="shelf-stub">(library "{props.def.libraryName}" not found on {props.server.displayName})</div>;
          }
          return <LibraryCards server={props.server} libraryKey={lib.key} />;
        }}
      </Show>
    </Shelf>
  );
}

function LibraryCards(props: { server: Server; libraryKey: string }) {
  const [items] = createResource(() =>
    api.items(props.server.machineIdentifier, props.libraryKey, { sort: "addedAt:desc", size: 20 })
  );
  return (
    <Show when={items()} fallback={<div class="shelf-loading">Loading…</div>}>
      {(list) => (
        <For each={list() as Item[]}>
          {(it) => (
            <Card
              title={it.title}
              year={it.year}
              ratingKey={it.ratingKey}
              serverID={props.server.machineIdentifier}
            />
          )}
        </For>
      )}
    </Show>
  );
}
