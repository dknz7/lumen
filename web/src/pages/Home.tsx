import { createMemo, createResource, createSignal, For, JSX, Show } from "solid-js";
import { DragDropProvider, DragDropSensors, SortableProvider, closestCenter } from "@thisbeyond/solid-dnd";
import { api } from "../api/client";
import type { Item, Library, Server } from "../api/types";
import Group from "../components/Group";
import Shelf from "../components/Shelf";
import Card from "../components/Card";
import Skeleton from "../components/Skeleton";
import { Cat, Film, Flame, Play, Server as ServerIcon, Star, Tv } from "../components/icons";
import { store as settingsStore } from "../state/settings";
import { refetchOnFocus } from "../util/focusRefetch";
import "./Home.css";

// Icon dispatch for Home shelves + groups. Keyed off shelf id / group name so
// new shelves added later can be tagged via the same naming pattern.
function iconForShelf(id: string): JSX.Element {
  if (id.includes("trending")) return <Flame size={18} />;
  if (id.includes("anime")) return <Cat size={18} />;
  if (id.includes("episodes")) return <Tv size={18} />;
  if (id.includes("movies")) return <Film size={18} />;
  return <Film size={18} />;
}

function iconForGroup(logicalName: string): JSX.Element {
  if (logicalName.toLowerCase() === "stargaze") return <Star size={20} />;
  return <ServerIcon size={20} />;
}

// Shelf definitions — one entry per shelf on the Home page (spec §12.1).
type ShelfDef =
  | { kind: "ondeck-merged"; id: string; title: string }
  | { kind: "server-recent"; id: string; title: string; serverName: string; libraryName: string }
  | { kind: "stub"; id: string; title: string; reason: string };

const STARGAZE_SHELVES: ShelfDef[] = [
  { kind: "stub", id: "stargaze-trending-movies", title: "Trending Movies", reason: "Plex Collections — deferred to a later session" },
  { kind: "server-recent", id: "stargaze-recent-movies", title: "Recently Released Movies", serverName: "Stargaze", libraryName: "Movies" },
  { kind: "server-recent", id: "stargaze-recent-movies-4k", title: "Recently Released Movies (4K)", serverName: "Stargaze", libraryName: "Movies - 4K" },
  { kind: "stub", id: "stargaze-trending-tv", title: "Trending TV Shows", reason: "Plex Collections — deferred to a later session" },
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

  const [decksData, { refetch: refetchDecks }] = createResource(
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

  // When the user comes back from Plex Web/Desktop, pick up any state
  // changes made there. Clear localItems so the freshly fetched server
  // data wins over any optimistic-removed entries from earlier.
  refetchOnFocus(() => {
    setLocalItems(null);
    refetchDecks();
  });

  // Tick (mark watched) and bin (remove from CW) share the same optimistic-
  // remove + rollback-on-error flow; they only differ in which Plex endpoint
  // they hit (scrobble vs unscrobble).
  async function applyCWAction(
    item: CWItem,
    apiCall: (serverID: string, ratingKey: string) => Promise<unknown>,
    label: string,
  ) {
    const current = visibleItems();
    setLocalItems(current.filter((x) => !(x.ratingKey === item.ratingKey && x.serverID === item.serverID)));
    try {
      await apiCall(item.serverID, item.ratingKey);
    } catch (e) {
      console.error(`${label} failed:`, e);
      setLocalItems(current);
      alert(`Failed to ${label}: ${(e as Error).message}`);
    }
  }

  const markWatched = (item: CWItem) => applyCWAction(item, api.scrobble, "mark as watched");
  const removeItem = (item: CWItem) => applyCWAction(item, api.removeFromCW, "remove from Continue Watching");

  // Items are passed to Shelf only when loaded successfully and non-empty;
  // otherwise children render the skeleton / error / empty stub instead.
  const cwItems = () => {
    if (decksData.loading || decksData.error) return undefined;
    const items = visibleItems();
    return items.length > 0 ? (items as CWItem[]) : undefined;
  };

  return (
    <Shelf
      id="continue-watching"
      title="Continue Watching"
      sortable={false}
      rowsPerPage={3}
      icon={<Play size={18} />}
      items={cwItems()}
      renderItem={(it: CWItem) => (
        <Card
          item={it}
          serverID={it.serverID}
          onMarkWatched={() => markWatched(it)}
          onRemove={() => removeItem(it)}
        />
      )}
    >
      <Show
        when={!decksData.loading}
        fallback={<Skeleton kind="card" count={6} />}
      >
        <Show when={decksData.error}>
          <div class="shelf-stub">Continue Watching failed: {(decksData.error as Error)?.message}</div>
        </Show>
        <Show when={!decksData.error && visibleItems().length === 0}>
          <div class="shelf-stub">Nothing in progress across your servers.</div>
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
    <Group id={`group-${groupID}`} title={props.logicalName} icon={iconForGroup(props.logicalName)}>
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
      <Shelf
        id={props.def.id}
        title={props.def.title}
        initialCollapsed
        rowsPerPage={2}
        icon={iconForShelf(props.def.id)}
      >
        <div class="shelf-stub">({props.def.reason})</div>
      </Shelf>
    );
  }
  if (props.def.kind === "ondeck-merged") {
    return null;
  }
  // server-recent. Capture the narrowed def into a const so TypeScript's
  // discriminant narrowing persists across the inner closures below.
  const def = props.def;
  const hiddenSet = () => new Set(settingsStore.settings()?.hiddenLibraries ?? []);
  const [libs] = createResource(() => api.libraries(props.server.machineIdentifier));

  // Resolve the named library on this server. Null if the library list hasn't
  // loaded yet OR the named library doesn't exist on this server.
  const lib = createMemo(() => {
    const list = libs();
    if (!list) return null;
    return (list as Library[]).find((l) => l.title === def.libraryName) ?? null;
  });

  const isHidden = createMemo(() => {
    const l = lib();
    if (!l) return false;
    return hiddenSet().has(`${props.server.machineIdentifier}:${l.key}`);
  });

  // Fetch recentlyAdded only when we have a non-hidden library. Resource
  // re-runs whenever lib() changes (different libKey) or hidden state flips.
  const [items, { refetch: refetchItems }] = createResource(
    () => {
      const l = lib();
      if (!l || isHidden()) return null;
      return l.key;
    },
    (libKey) => api.recentlyAdded(props.server.machineIdentifier, libKey, 20)
  );

  // Refresh recently-added on focus so newly added items in Plex show up
  // when the user switches back to the Lumen tab.
  refetchOnFocus(() => refetchItems());

  // Items array is only passed to Shelf when fully loaded; otherwise the
  // children handle stub/skeleton states.
  const itemList = () => {
    const list = items();
    return list ? (list as Item[]) : undefined;
  };

  return (
    <Shelf
      id={def.id}
      title={def.title}
      rowsPerPage={2}
      icon={iconForShelf(def.id)}
      items={itemList()}
      renderItem={(it: Item) => (
        <Card item={it} serverID={props.server.machineIdentifier} />
      )}
    >
      <Show when={!libs()}>
        <Skeleton kind="card" count={6} />
      </Show>
      <Show when={libs() && !lib()}>
        <div class="shelf-stub">(library "{def.libraryName}" not found on {props.server.displayName})</div>
      </Show>
      <Show when={lib() && isHidden()}>
        <div class="shelf-stub">(library hidden — toggle in left menu)</div>
      </Show>
      <Show when={lib() && !isHidden() && !items()}>
        <Skeleton kind="card" count={6} />
      </Show>
    </Shelf>
  );
}
