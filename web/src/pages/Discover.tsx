import { createMemo, createResource, createSignal, For, Show } from "solid-js";
import type { HubItem, WatchlistItem } from "../api/types";
import { api } from "../api/client";
import Shelf from "../components/Shelf";
import Skeleton from "../components/Skeleton";
import DiscoverTile, { DiscoverTileProvider } from "../components/DiscoverTile";
import TrailerModal from "../components/Modal/TrailerModal";
import HLSTrailerModal from "../components/Modal/HLSTrailerModal";
import { refetchOnFocus } from "../util/focusRefetch";
import "./Discover.css";

// Per spec §12.4 — eight Byron-curated shelves from the home namespace.
// Order kept as-is per Byron's call (Phase 4 Task 11).
const SHELVES: { id: string; title: string; slug: string }[] = [
  { id: "disc-coming-soon", title: "Coming Soon", slug: "coming-soon" },
  { id: "disc-new-trailers", title: "New Trailers", slug: "recently-released-trailers" },
  { id: "disc-trending-trailers", title: "Trending Trailers", slug: "trending-trailers" },
  { id: "disc-top-watchlisted", title: "Most Watchlisted This Week", slug: "top_watchlisted" },
  { id: "disc-trending-plex", title: "Trending on Plex", slug: "trending-plex" },
  { id: "disc-blockbusters", title: "Upcoming Blockbusters", slug: "blockbuster-trailers" },
  { id: "disc-anticipated", title: "Highly Anticipated", slug: "highly-anticipated-movies" },
  { id: "disc-apple-tv", title: "Trending on Apple TV", slug: "trend-apple-itunes" },
];

export default function Discover() {
  // Page-level watchlist resource — same pattern as Recommended. Each page
  // gets its own copy; the API layer caches /api/watchlist for 5 min so
  // mounting both pages in quick succession is cheap.
  const [watchlist, { refetch: refetchWatchlist }] = createResource<WatchlistItem[]>(() =>
    api.watchlist().catch(() => [] as WatchlistItem[]),
  );
  refetchOnFocus(refetchWatchlist);
  const watchlistSet = createMemo(
    () => new Set((watchlist() ?? []).map((w) => w.ratingKey)),
  );

  const [trailerOpen, setTrailerOpen] = createSignal(false);
  const [trailerYouTubeID, setTrailerYouTubeID] = createSignal<string | undefined>(undefined);
  const [trailerTitle, setTrailerTitle] = createSignal<string>("");

  function openTrailer(youtubeID: string, title: string) {
    setTrailerYouTubeID(youtubeID);
    setTrailerTitle(title);
    setTrailerOpen(true);
  }

  // Page-level HLSTrailerModal — clip items on Trending Trailers carry
  // their own Media[].Part[].key HLS playback URL (Phase 4.6 Task 12.8).
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
      <div class="discover-page">
        <For each={SHELVES}>
          {(s) => <DiscoverShelfHost id={s.id} title={s.title} slug={s.slug} />}
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

function DiscoverShelfHost(props: { id: string; title: string; slug: string }) {
  const [items, { refetch }] = createResource<HubItem[]>(() =>
    api.hub("home", props.slug).catch(() => [] as HubItem[]),
  );
  refetchOnFocus(refetch);

  const itemList = () => {
    if (items.loading || items.error) return undefined;
    const list = items() ?? [];
    return list.length > 0 ? list : undefined;
  };

  return (
    <Shelf
      id={props.id}
      title={props.title}
      rowsPerPage={2}
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
