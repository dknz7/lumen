import { createMemo, createResource, createSignal, For, JSX, Show } from "solid-js";
import { DragDropProvider, DragDropSensors, SortableProvider, closestCenter } from "@thisbeyond/solid-dnd";
import { api } from "../api/client";
import type { Collection, Item, Library, Server } from "../api/types";
import Group from "../components/Group";
import Shelf from "../components/Shelf";
import Card from "../components/Card";
import Skeleton from "../components/Skeleton";
import { Cat, Film, Flame, Play, Server as ServerIcon, Tv } from "../components/icons";
import { store as settingsStore } from "../state/settings";
import { librariesFor } from "../state/libraries";
import {
  applyPersistedOrder,
  deriveHomeLayout,
  type GroupDef,
  type ShelfDef,
} from "../util/homeLayout";
import ResourceView from "../components/ResourceView";
import { toast, errorMessage } from "../components/Toast";
import { refetchOnFocus } from "../util/focusRefetch";
import { stableArrayByKey } from "../util/stableArray";
import "./Home.css";

// Icon dispatch for Home shelves and groups. Keyed off the library type and
// title rather than a hardcoded shelf id, so it works for any user's libraries.
function iconForShelf(def: ShelfDef): JSX.Element {
  const t = def.libraryTitle.toLowerCase();
  if (t.includes("anime")) return <Cat size={18} />;
  if (def.kind === "server-collection") return <Flame size={18} />;
  return def.libraryType === "show" ? <Tv size={18} /> : <Film size={18} />;
}

function iconForGroup(): JSX.Element {
  return <ServerIcon size={20} />;
}

// Appearance > Rows per shelf. Every call site used to pass a literal 2,
// which is why the setting appeared to do nothing.
const shelfRows = () => settingsStore.settings()?.rowsPerShelf ?? 2;

export default function Home() {
  const [servers, { refetch: refetchServers }] = createResource(() => api.servers());
  const pageKey = "home";

  // One libraries fetch per server for the whole page. Every Recently Added
  // shelf used to run its own, which is how a single Home load produced 17
  // requests for the same two library lists.
  const [libsByServer] = createResource(
    () => (servers.error ? undefined : (servers() as Server[] | undefined)),
    async (srvs) => {
      const entries = await Promise.all(
        srvs.map(async (s) => {
          try {
            return [s.machineIdentifier, await librariesFor(s.machineIdentifier)] as const;
          } catch (e) {
            // One unreachable server shouldn't blank the whole page; it just
            // contributes no shelves.
            console.error(`libraries failed for ${s.displayName}:`, e);
            return [s.machineIdentifier, [] as Library[]] as const;
          }
        }),
      );
      return Object.fromEntries(entries) as Record<string, Library[]>;
    },
  );

  const hiddenSet = () => new Set(settingsStore.settings()?.hiddenLibraries ?? []);

  // The Home layout, derived from whatever this user's account can actually
  // see. Previously a pair of hardcoded constants naming one person's servers.
  const groups = createMemo<GroupDef[]>(() => {
    if (servers.error || libsByServer.error) return [];
    const srvs = servers() as Server[] | undefined;
    const libs = libsByServer();
    if (!srvs || !libs) return [];
    return deriveHomeLayout(
      srvs,
      libs,
      hiddenSet(),
      settingsStore.settings()?.collectionShelves ?? [],
    );
  });

  const groupOrder = () =>
    applyPersistedOrder(
      groups().map((g) => g.id),
      settingsStore.settings()?.shelfState?.[pageKey]?.groupOrder,
    );

  function onGroupDragEnd(e: any) {
    const { draggable, droppable } = e;
    if (!draggable || !droppable) return;
    const current = groupOrder();
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
      {/* ResourceView reads .error before the value. Reading an errored resource
          in Solid THROWS, so the old <Show when={servers()}> turned a failed
          /api/servers into a blank page plus an uncaught error. */}
      <ResourceView
        resource={servers}
        errorTitle="Couldn't reach your Plex servers"
        onRetry={refetchServers}
        loading={<Skeleton kind="card" count={12} />}
        empty={
          <div class="home-empty">
            <h2>No servers yet</h2>
            <p>
              Lumen shows everything your Plex account can reach. If you've just
              signed in, refresh your servers in Settings &rsaquo; Accounts &amp;
              Servers.
            </p>
          </div>
        }
      >
        {(srvs) => (
          <>
            <ContinueWatching servers={srvs as Server[]} />
            <DragDropProvider onDragEnd={onGroupDragEnd} collisionDetector={closestCenter}>
              <DragDropSensors />
              <SortableProvider ids={groupOrder()}>
                <For each={groupOrder()}>
                  {(id) => {
                    const def = () => groups().find((g) => g.id === id);
                    return (
                      <Show when={def()}>
                        {(g) => <ServerGroup group={g()} />}
                      </Show>
                    );
                  }}
                </For>
              </SortableProvider>
            </DragDropProvider>
          </>
        )}
      </ResourceView>
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

  // CWItem is per-server-scoped — same ratingKey can exist on both servers,
  // so the stable key includes the serverID to keep items distinct.
  const stableDecks = stableArrayByKey<CWItem>(
    () => (decksData() ?? []) as CWItem[],
    (it) => `${it.serverID}:${it.ratingKey}`,
  );

  const [localItems, setLocalItems] = createSignal<CWItem[] | null>(null);
  const visibleItems = () => localItems() ?? stableDecks();

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
      toast.error(`Couldn't ${label} — ${errorMessage(e)}`, {
        label: "Retry",
        run: () => void applyCWAction(item, apiCall, label),
      });
    }
  }

  const markWatched = (item: CWItem) => applyCWAction(item, api.scrobble, "mark as watched");
  const removeItem = (item: CWItem) => applyCWAction(item, api.removeFromCW, "remove from Continue Watching");

  // Critical: do NOT short-circuit on decksData.loading — that flag is
  // true during refetch as well as initial fetch, and switching cwItems
  // to undefined during refetch flips Shelf's isPaginated() false →
  // grid destroyed and rebuilt → click-loss when window-focus refetch
  // lands during a click (a real regression). decksData() preserves
  // the previous value during refetch; we keep the grid visible.
  const cwItems = () => {
    if (decksData.error || !decksData()) return undefined;
    const items = visibleItems();
    return items.length > 0 ? (items as CWItem[]) : undefined;
  };

  return (
    <Shelf
      id="continue-watching"
      title="Continue Watching"
      sortable={false}
      rowsPerPage={shelfRows()}
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

function ServerGroup(props: { group: GroupDef }) {
  const pageKey = "home";
  const groupID = () => props.group.id;

  const shelfOrder = () =>
    applyPersistedOrder(
      props.group.shelves.map((sh) => sh.id),
      settingsStore.settings()?.shelfState?.[pageKey]?.shelfOrder?.[groupID()],
    );

  function onShelfDragEnd(e: any) {
    const { draggable, droppable } = e;
    if (!draggable || !droppable) return;
    const current = shelfOrder();
    const from = current.indexOf(draggable.id as string);
    const to = current.indexOf(droppable.id as string);
    if (from === -1 || to === -1 || from === to) return;
    const next = [...current];
    next.splice(from, 1);
    next.splice(to, 0, draggable.id as string);

    const state = settingsStore.settings()?.shelfState ?? {};
    const pageState = state[pageKey] ?? {};
    const order = { ...(pageState.shelfOrder ?? {}), [groupID()]: next };
    settingsStore.patch({
      shelfState: { ...state, [pageKey]: { ...pageState, shelfOrder: order } },
    });
  }

  // Group's id MUST match the ids handed to <SortableProvider> above, which
  // are the raw group ids. A "group-" prefix here meant createSortable
  // registered "group-<machineID>" while the provider — and onGroupDragEnd's
  // indexOf(draggable.id) — only ever knew "<machineID>", so every group drop
  // resolved to index -1 and returned early. Reordering servers had silently
  // done nothing for as long as the prefix existed; the tell was that no
  // groupOrder key had ever been written to config.json.
  return (
    <Group id={groupID()} title={props.group.title} icon={iconForGroup()}>
      <Show
        when={props.group.shelves.length > 0}
        fallback={
          <div class="shelf-stub">
            No movie or TV libraries on this server — or they're all hidden.
          </div>
        }
      >
        <DragDropProvider onDragEnd={onShelfDragEnd} collisionDetector={closestCenter}>
          <DragDropSensors />
          <SortableProvider ids={shelfOrder()}>
            <For each={shelfOrder()}>
              {(id) => {
                const def = () => props.group.shelves.find((sh) => sh.id === id);
                return (
                  <Show when={def()}>
                    {(d) => <ShelfForDef def={d()} />}
                  </Show>
                );
              }}
            </For>
          </SortableProvider>
        </DragDropProvider>
      </Show>
    </Group>
  );
}

// ShelfForDef picks the renderer for a shelf definition. Two kinds exist:
// Recently Added, derived automatically for every video library, and
// collection shelves the user opted into in Settings.
function ShelfForDef(props: { def: ShelfDef }) {
  return (
    <Show
      when={props.def.kind === "server-collection"}
      fallback={<RecentShelf def={props.def as Extract<ShelfDef, { kind: "server-recent" }>} />}
    >
      <CollectionShelf def={props.def as Extract<ShelfDef, { kind: "server-collection" }>} />
    </Show>
  );
}

// CollectionShelf renders one Plex collection as a Home shelf.
//
// The collection is resolved by TITLE, never by ratingKey. Server admins
// rebuild collections and a rebuild issues fresh ratingKeys while keeping the
// title, so a key-based reference would quietly go dead with no way to tell a
// deleted collection from a rebuilt one. Costs one extra request per shelf to
// list the library's collections; that response is small.
function CollectionShelf(props: { def: Extract<ShelfDef, { kind: "server-collection" }> }) {
  const [cols] = createResource(
    () => `${props.def.serverID}:${props.def.libraryKey}`,
    () => api.collections(props.def.serverID, props.def.libraryKey),
  );

  const collection = createMemo(() => {
    if (cols.error || !cols()) return null;
    return (cols() as Collection[]).find((c) => c.title === props.def.collectionTitle) ?? null;
  });

  const [items, { refetch: refetchItems }] = createResource(
    () => collection()?.ratingKey,
    (rk) => api.collectionItems(props.def.serverID, rk, 20),
  );

  refetchOnFocus(() => refetchItems());

  // Declared below `items` deliberately: stableArrayByKey is a createMemo and
  // Solid runs a memo body on creation, so reading `items` from above it would
  // land in its temporal dead zone.
  const stable = stableArrayByKey<Item>(
    () => (items.error ? [] : ((items() as Item[] | undefined) ?? [])),
    (it) => it.ratingKey,
  );

  const itemList = () => {
    if (items.error || !items()) return undefined;
    const list = stable();
    return list.length > 0 ? list : undefined;
  };

  return (
    <Shelf
      id={props.def.id}
      title={props.def.title}
      rowsPerPage={shelfRows()}
      icon={iconForShelf(props.def)}
      items={itemList()}
      renderItem={(it: Item) => <Card item={it} serverID={props.def.serverID} />}
    >
      <Show when={cols.error}>
        <div class="shelf-stub shelf-stub--error">
          Couldn't load collections for {props.def.libraryTitle} — {errorMessage(cols.error)}
        </div>
      </Show>
      <Show when={!cols.error && !!cols() && !collection()}>
        <div class="shelf-stub">
          "{props.def.collectionTitle}" is no longer in {props.def.libraryTitle}. Remove it in
          Settings &rsaquo; Home Shelves.
        </div>
      </Show>
      <Show when={!cols.error && (cols.loading || (!!collection() && items.loading && !items()))}>
        <Skeleton kind="card" count={6} />
      </Show>
      <Show when={!cols.error && !!collection() && !items.loading && !itemList()}>
        <div class="shelf-stub">Nothing in {props.def.title} yet.</div>
      </Show>
    </Shelf>
  );
}

// RecentShelf renders Plex's native /library/sections/<id>/recentlyAdded feed.
//
// It now receives the library key directly from the derived layout, so it does
// not resolve a library by NAME and does not fetch the server's library list
// itself — which is what made a single Home load issue seventeen identical
// /libraries requests.
function RecentShelf(props: { def: Extract<ShelfDef, { kind: "server-recent" }> }) {
  const [items, { refetch: refetchItems }] = createResource(
    () => `${props.def.serverID}:${props.def.libraryKey}`,
    () => api.recentlyAdded(props.def.serverID, props.def.libraryKey, 20),
  );

  refetchOnFocus(() => refetchItems());

  const stable = stableArrayByKey<Item>(
    () => (items.error ? [] : ((items() as Item[] | undefined) ?? [])),
    (it) => it.ratingKey,
  );

  // Guarded: reading items() while errored would throw. Keeps the previous
  // value during a refetch so the grid isn't destroyed and rebuilt mid-click.
  const itemList = () => {
    if (items.error || !items()) return undefined;
    const list = stable();
    return list.length > 0 ? list : undefined;
  };

  return (
    <Shelf
      id={props.def.id}
      title={props.def.title}
      rowsPerPage={shelfRows()}
      icon={iconForShelf(props.def)}
      items={itemList()}
      renderItem={(it: Item) => <Card item={it} serverID={props.def.serverID} />}
    >
      <Show when={items.error}>
        <div class="shelf-stub shelf-stub--error">
          Couldn't load {props.def.libraryTitle} — {errorMessage(items.error)}{" "}
          <button class="shelf-retry" onClick={() => refetchItems()}>Retry</button>
        </div>
      </Show>
      <Show when={!items.error && items.loading && !items()}>
        <Skeleton kind="card" count={6} />
      </Show>
      <Show when={!items.error && !items.loading && !itemList()}>
        <div class="shelf-stub">Nothing added to {props.def.libraryTitle} yet.</div>
      </Show>
    </Shelf>
  );
}
