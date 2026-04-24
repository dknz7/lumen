import { createResource, createSignal, For, Show } from "solid-js";
import { DragDropProvider, DragDropSensors, SortableProvider, closestCenter } from "@thisbeyond/solid-dnd";
import { api } from "../api/client";
import type { Item, Library, Server } from "../api/types";
import Group from "../components/Group";
import Shelf from "../components/Shelf";
import Card from "../components/Card";
import Skeleton from "../components/Skeleton";
import { store as settingsStore } from "../state/settings";
import "./Home.css";

// Shelf definitions — one entry per shelf on the Home page (spec §12.1).
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

const GROUP_DEFS = [
  { id: "stargaze", logicalName: "Stargaze", shelves: STARGAZE_SHELVES },
  { id: "dknzplex", logicalName: "DKNZPLEX", shelves: DKNZPLEX_SHELVES },
];

export default function Home() {
  const [servers] = createResource(() => api.servers());
  const pageKey = "home";

  const persistedGroupOrder = () => {
    const persisted = settingsStore.settings()?.shelfState?.[pageKey]?.groupOrder;
    if (!persisted || persisted.length === 0) return GROUP_DEFS.map((g) => g.id);
    const missing = GROUP_DEFS.map((g) => g.id).filter((id) => !persisted.includes(id));
    return [...persisted.filter((id) => GROUP_DEFS.some((g) => g.id === id)), ...missing];
  };

  function onGroupDragEnd(e: any) {
    const { draggable, droppable } = e;
    if (!draggable || !droppable) return;
    const current = persistedGroupOrder();
    const from = current.indexOf(draggable.id as string);
    const to = current.indexOf(droppable.id as string);
    if (from === -1 || to === -1 || from === to) return;
    const next = [...current];
    next.splice(from, 1);
    next.splice(to, 0, draggable.id as string);

    const state = settingsStore.settings()?.shelfState ?? {};
    const pageState = state[pageKey] ?? {};
    settingsStore.patch({
      shelfState: { ...state, [pageKey]: { ...pageState, groupOrder: next } },
    });
  }

  return (
    <div class="home-page">
      <Show when={servers()}>
        {(srvs) => (
          <>
            <ContinueWatching servers={srvs() as Server[]} />
            <DragDropProvider onDragEnd={onGroupDragEnd} collisionDetector={closestCenter}>
              <DragDropSensors />
              <SortableProvider ids={persistedGroupOrder()}>
                <For each={persistedGroupOrder()}>
                  {(id) => {
                    const def = GROUP_DEFS.find((g) => g.id === id)!;
                    return <ServerGroup srvs={srvs() as Server[]} groupID={def.id} logicalName={def.logicalName} shelves={def.shelves} />;
                  }}
                </For>
              </SortableProvider>
            </DragDropProvider>
          </>
        )}
      </Show>
    </div>
  );
}

type CWItem = Item & { serverID: string };

function ContinueWatching(props: { servers: Server[] }) {
  const machineIDs = props.servers.map((s) => s.machineIdentifier);

  const [decksData] = createResource(
    () => machineIDs.join(","),
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
      const merged = results.flatMap((r) =>
        r.items.map((it) => ({ ...it, serverID: r.id } as CWItem))
      );
      merged.sort((a, b) => {
        const lvDiff = (b.lastViewedAt ?? 0) - (a.lastViewedAt ?? 0);
        if (lvDiff !== 0) return lvDiff;
        return (b.addedAt ?? 0) - (a.addedAt ?? 0);
      });
      return merged;
    }
  );

  const [localItems, setLocalItems] = createSignal<CWItem[] | null>(null);
  const visibleItems = () => localItems() ?? (decksData() ?? []);

  async function removeItem(item: CWItem) {
    const current = visibleItems();
    setLocalItems(current.filter((x) => !(x.ratingKey === item.ratingKey && x.serverID === item.serverID)));
    try {
      await api.removeFromOnDeck(item.serverID, item.ratingKey);
    } catch (e) {
      console.error("removeFromOnDeck failed:", e);
      setLocalItems(current);
      alert(`Failed to remove: ${(e as Error).message}`);
    }
  }

  return (
    <Shelf id="continue-watching" title="Continue Watching">
      <Show
        when={!decksData.loading}
        fallback={<Skeleton kind="card" count={6} />}
      >
        <Show
          when={decksData.error}
          fallback={
            <Show
              when={visibleItems().length > 0}
              fallback={<div class="shelf-stub">Nothing in progress across your servers.</div>}
            >
              <For each={visibleItems() as CWItem[]}>
                {(it) => (
                  <Card
                    item={it}
                    serverID={it.serverID}
                    onRemove={() => removeItem(it)}
                  />
                )}
              </For>
            </Show>
          }
        >
          <div class="shelf-stub">Continue Watching failed: {(decksData.error as Error)?.message}</div>
        </Show>
      </Show>
    </Shelf>
  );
}

function ServerGroup(props: { srvs: Server[]; groupID: string; logicalName: string; shelves: ShelfDef[] }) {
  const matched = () =>
    props.srvs.find((s) =>
      s.displayName.toLowerCase().includes(props.logicalName.toLowerCase())
    ) ?? props.srvs.find((s) => s.name.toLowerCase() === props.logicalName.toLowerCase());

  const pageKey = "home";
  const groupID = props.groupID;

  const persistedOrder = () => {
    const order = settingsStore.settings()?.shelfState?.[pageKey]?.shelfOrder?.[groupID];
    if (!order || order.length === 0) return props.shelves.map((s) => s.id);
    const missing = props.shelves.map((s) => s.id).filter((id) => !order.includes(id));
    return [...order.filter((id) => props.shelves.some((s) => s.id === id)), ...missing];
  };

  function onShelfDragEnd(e: any) {
    const { draggable, droppable } = e;
    if (!draggable || !droppable) return;
    const current = persistedOrder();
    const from = current.indexOf(draggable.id as string);
    const to = current.indexOf(droppable.id as string);
    if (from === -1 || to === -1 || from === to) return;
    const next = [...current];
    next.splice(from, 1);
    next.splice(to, 0, draggable.id as string);

    const state = settingsStore.settings()?.shelfState ?? {};
    const pageState = state[pageKey] ?? {};
    const shelfOrder = { ...(pageState.shelfOrder ?? {}), [groupID]: next };
    settingsStore.patch({
      shelfState: { ...state, [pageKey]: { ...pageState, shelfOrder } },
    });
  }

  return (
    <Group id={`group-${groupID}`} title={props.logicalName}>
      <Show when={matched()} fallback={<div class="group-missing">{props.logicalName} not found in servers — run `lumen list`</div>}>
        {(srv) => (
          <DragDropProvider onDragEnd={onShelfDragEnd} collisionDetector={closestCenter}>
            <DragDropSensors />
            <SortableProvider ids={persistedOrder()}>
              <For each={persistedOrder()}>
                {(id) => {
                  const def = props.shelves.find((s) => s.id === id);
                  return def ? <ShelfLoader server={srv() as Server} def={def} /> : null;
                }}
              </For>
            </SortableProvider>
          </DragDropProvider>
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
    return null;
  }
  // From here the discriminated union narrows to "server-recent" — bind the
  // required fields locally so the closures below can reference them without
  // TypeScript losing the narrowing inside async callback bodies.
  const def = props.def; // narrowed as { kind: "server-recent"; libraryName: string; ... }
  const hiddenSet = () => new Set(settingsStore.settings()?.hiddenLibraries ?? []);
  const [libs] = createResource(() => api.libraries(props.server.machineIdentifier));
  return (
    <Shelf id={def.id} title={def.title}>
      <Show when={libs()}>
        {(libList) => {
          const lib = (libList() as Library[]).find((l) => l.title === def.libraryName);
          if (!lib) {
            return <div class="shelf-stub">(library "{def.libraryName}" not found on {props.server.displayName})</div>;
          }
          const hidden = hiddenSet().has(`${props.server.machineIdentifier}:${lib.key}`);
          if (hidden) return <div class="shelf-stub">(library hidden — toggle in left menu)</div>;
          return <LibraryCards server={props.server} libraryKey={lib.key} />;
        }}
      </Show>
    </Shelf>
  );
}

function LibraryCards(props: { server: Server; libraryKey: string }) {
  const [items] = createResource(() =>
    api.recentlyAdded(props.server.machineIdentifier, props.libraryKey, 20)
  );
  return (
    <Show when={items()} fallback={<Skeleton kind="card" count={6} />}>
      {(list) => (
        <For each={list() as Item[]}>
          {(it) => <Card item={it} serverID={props.server.machineIdentifier} />}
        </For>
      )}
    </Show>
  );
}
