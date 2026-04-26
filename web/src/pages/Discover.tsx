import { createResource, For, Show } from "solid-js";
import { A } from "@solidjs/router";
import type { HubItem } from "../api/types";
import { api } from "../api/client";
import Skeleton from "../components/Skeleton";
import { refetchOnFocus } from "../util/focusRefetch";
import "./Discover.css";

// Per spec §12.4 — eight Byron-curated shelves from the home namespace.
const SHELVES: { title: string; slug: string }[] = [
  { title: "Coming Soon", slug: "coming-soon" },
  { title: "New Trailers", slug: "recently-released-trailers" },
  { title: "Trending Trailers", slug: "trending-trailers" },
  { title: "Most Watchlisted This Week", slug: "top_watchlisted" },
  { title: "Trending on Plex", slug: "trending-plex" },
  { title: "Upcoming Blockbusters", slug: "blockbuster-trailers" },
  { title: "Highly Anticipated", slug: "highly-anticipated-movies" },
  { title: "Trending on Apple TV", slug: "trend-apple-itunes" },
];

export default function Discover() {
  return (
    <div class="discover-page">
      <For each={SHELVES}>
        {(shelf) => <DiscoverShelf title={shelf.title} slug={shelf.slug} />}
      </For>
    </div>
  );
}

function DiscoverShelf(props: { title: string; slug: string }) {
  const [items, { refetch }] = createResource<HubItem[]>(() =>
    api.hub("home", props.slug).catch(() => [])
  );
  refetchOnFocus(refetch);
  return (
    <section class="discover-shelf">
      <h2 class="discover-shelf-title">{props.title}</h2>
      <Show when={items()} fallback={<div class="discover-shelf-row"><Skeleton kind="card" count={6} /></div>}>
        <Show when={items()!.length > 0} fallback={<div class="discover-empty">Nothing here yet.</div>}>
          <ul class="discover-shelf-row">
            <For each={items()}>
              {(it) => (
                <li class="discover-card">
                  <A href={`/discover-item/${encodeURIComponent(it.ratingKey)}`} class="discover-card-link">
                    <div class="discover-poster">
                      <Show when={it.thumb}>
                        <img src={it.thumb!} alt={it.title} referrerpolicy="no-referrer"
                             onError={(e) => { (e.currentTarget as HTMLImageElement).style.display = "none"; }} />
                      </Show>
                    </div>
                    <div class="discover-card-meta">
                      <div class="discover-card-title">{it.title}</div>
                      <Show when={it.year}>
                        <div class="discover-card-sub">{it.year}</div>
                      </Show>
                    </div>
                  </A>
                </li>
              )}
            </For>
          </ul>
        </Show>
      </Show>
    </section>
  );
}
