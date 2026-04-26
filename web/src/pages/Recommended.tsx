import { createResource, For, Show } from "solid-js";
import { A } from "@solidjs/router";
import type { HubItem } from "../api/types";
import { api } from "../api/client";
import Skeleton from "../components/Skeleton";
import { refetchOnFocus } from "../util/focusRefetch";
import "./Recommended.css";

// Per spec §12.3 (revised — Pick Up Again dropped during Session 5 Phase 2).
// Four watchlist-namespace Plex Discover hubs. Lumen's Home pins Continue
// Watching at the top, so the original "Pick Up Again" shelf was redundant
// and upstream data was unreliable (stale watchlist entries).
const SHELVES: { title: string; slug: string }[] = [
  { title: "Recently Aired Episodes", slug: "new-episodes" },
  { title: "Coming Soon", slug: "coming-soon" },
  { title: "New Trailers from Your Watchlist", slug: "new-trailers" },
  { title: "Recently Added", slug: "recently-added" },
];

export default function Recommended() {
  return (
    <div class="recommended-page">
      <For each={SHELVES}>
        {(shelf) => <RecommendedShelf title={shelf.title} slug={shelf.slug} />}
      </For>
    </div>
  );
}

function RecommendedShelf(props: { title: string; slug: string }) {
  const [items, { refetch }] = createResource<HubItem[]>(() =>
    api.hub("watchlist", props.slug).catch(() => [])
  );
  refetchOnFocus(refetch);
  return (
    <section class="recommended-shelf">
      <h2 class="recommended-shelf-title">{props.title}</h2>
      <Show when={items()} fallback={<div class="recommended-shelf-row"><Skeleton kind="card" count={6} /></div>}>
        <Show when={items()!.length > 0} fallback={<div class="recommended-empty">Nothing here yet.</div>}>
          <ul class="recommended-shelf-row">
            <For each={items()}>
              {(it) => (
                <li class="recommended-card">
                  <A href={`/discover-item/${encodeURIComponent(it.ratingKey)}`} class="recommended-card-link">
                    <div class="recommended-poster" />
                    <div class="recommended-card-meta">
                      <div class="recommended-card-title">{it.title}</div>
                      <Show when={it.year}>
                        <div class="recommended-card-sub">{it.year}</div>
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
