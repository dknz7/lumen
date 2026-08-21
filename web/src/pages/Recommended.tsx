import { createMemo, createResource, createSignal, For } from "solid-js";
import { Show } from "solid-js";
import type { HubItem, WatchlistItem } from "../api/types";
import { api } from "../api/client";
import Shelf from "../components/Shelf";
import Skeleton from "../components/Skeleton";
import DiscoverTile, { DiscoverTileProvider } from "../components/DiscoverTile";
import TrailerModal from "../components/Modal/TrailerModal";
import HLSTrailerModal from "../components/Modal/HLSTrailerModal";
import { refetchOnFocus } from "../util/focusRefetch";
import { stableArrayByKey } from "../util/stableArray";
import "./Recommended.css";
import { store } from "../state/settings";

// Per spec — Phase 4 Task 10. New shelf order (the user's call): Coming Soon
// leads, then trailers, then recently added, then aired episodes. Watchlist
// namespace.
const SHELVES: { id: string; title: string; slug: string }[] = [
  { id: "rec-coming-soon", title: "Coming Soon", slug: "coming-soon" },
  { id: "rec-new-trailers", title: "New Trailers From Your Watchlist", slug: "new-trailers" },
  { id: "rec-recently-added", title: "Recently Added", slug: "recently-added" },
  { id: "rec-new-episodes", title: "Recently Aired Episodes", slug: "new-episodes" },
];

// Appearance > Rows per shelf. Every call site used to pass a literal 2,
// which is why the setting appeared to do nothing.
const shelfRows = () => store.settings()?.rowsPerShelf ?? 2;

export default function Recommended() {
  // Page-level watchlist resource powers the +/✓ state on every clip card
  // across all four shelves. refetchOnFocus picks up lumen:data-invalidated
  // dispatched by DiscoverTile's toggle handler so other tiles' state syncs.
  const [watchlist, { refetch: refetchWatchlist }] = createResource<WatchlistItem[]>(() =>
    api.watchlist().catch(() => [] as WatchlistItem[]),
  );
  refetchOnFocus(refetchWatchlist);
  const watchlistSet = createMemo(
    () => new Set((watchlist() ?? []).map((w) => w.ratingKey)),
  );

  // Page-level TrailerModal — DiscoverTile's clip variant resolves a YouTube
  // ID then calls openTrailer() via the DiscoverTileProvider context.
  const [trailerOpen, setTrailerOpen] = createSignal(false);
  const [trailerYouTubeID, setTrailerYouTubeID] = createSignal<string | undefined>(undefined);
  const [trailerTitle, setTrailerTitle] = createSignal<string>("");

  function openTrailer(youtubeID: string, title: string) {
    setTrailerYouTubeID(youtubeID);
    setTrailerTitle(title);
    setTrailerOpen(true);
  }

  // Page-level HLSTrailerModal — DiscoverTile's clip variant routes here
  // when the hub item carries its own Media[].Part[].key HLS URL (Trending
  // Trailers / New Trailers). Distinct from the YouTube modal above.
  const [hlsOpen, setHlsOpen] = createSignal(false);
  const [hlsUrl, setHlsUrl] = createSignal<string | undefined>(undefined);
  const [hlsTitle, setHlsTitle] = createSignal<string>("");

  function openHLSTrailer(url: string, title: string) {
    setHlsUrl(url);
    setHlsTitle(title);
    setHlsOpen(true);
  }

  return (
    <DiscoverTileProvider value={{ inWatchlistSet: watchlistSet, openTrailer, openHLSTrailer }}>
      <div class="recommended-page">
        <For each={SHELVES}>
          {(s) => <RecommendedShelfHost id={s.id} title={s.title} slug={s.slug} />}
        </For>
      </div>
      <TrailerModal
        open={trailerOpen()}
        onClose={() => {
          setTrailerOpen(false);
          setTrailerYouTubeID(undefined);
        }}
        youtubeID={trailerYouTubeID()}
        title={trailerTitle()}
      />
      <HLSTrailerModal
        open={hlsOpen()}
        onClose={() => {
          setHlsOpen(false);
          setHlsUrl(undefined);
        }}
        hlsUrl={hlsUrl()}
        title={hlsTitle()}
      />
    </DiscoverTileProvider>
  );
}

function RecommendedShelfHost(props: { id: string; title: string; slug: string }) {
  const [items, { refetch }] = createResource<HubItem[]>(() =>
    api.hub("watchlist", props.slug).catch(() => [] as HubItem[]),
  );
  refetchOnFocus(refetch);

  const stable = stableArrayByKey(() => items() ?? [], (it) => it.ratingKey);

  // Critical: do NOT short-circuit on items.loading — see Discover.tsx
  // for the full explanation. Loading-true during refetch caused
  // itemList → undefined → grid destroyed → click-loss on focus refetch.
  const itemList = () => {
    if (items.error || !items()) return undefined;
    const list = stable();
    return list.length > 0 ? list : undefined;
  };

  return (
    <Shelf
      id={props.id}
      title={props.title}
      rowsPerPage={shelfRows()}
      sortable={false}
      items={itemList()}
      renderItem={(it: HubItem) => <DiscoverTile item={it} />}
    >
      <Show when={items.loading}>
        <Skeleton kind="card" count={6} />
      </Show>
      <Show when={!items.loading && (items() ?? []).length === 0}>
        <div class="shelf-stub">Nothing here yet.</div>
      </Show>
    </Shelf>
  );
}
