import { createMemo, createResource, For, Show } from "solid-js";
import { useNavigate, useSearchParams } from "@solidjs/router";
import { api } from "../api/client";
import type { Item } from "../api/types";
import Skeleton from "../components/Skeleton";
import { ImageOff } from "../components/icons";
import "./SearchResults.css";

// SearchResults — full /search?q=<query> page. Renders the same grouped
// shape as SearchFlydown but with all results per source (no slice cap)
// and bigger card-style tiles. Mirrors Plex Web's full-search layout.
export default function SearchResults() {
  const [searchParams] = useSearchParams();
  const navigate = useNavigate();

  const query = createMemo(() => (searchParams.q ?? "").toString().trim());

  const [results] = createResource(
    () => (query().length >= 2 ? query() : null),
    (q) => api.search(q),
  );

  const totalHits = createMemo(() => {
    // .error first — reading an errored resource re-throws, which would take
    // the page down instead of showing "search failed".
    if (results.error) return 0;
    const r = results();
    if (!r) return 0;
    return r.servers.reduce((sum, b) => sum + b.items.length, 0) + r.discover.length;
  });

  function navigateToServerItem(serverID: string, ratingKey: string) {
    navigate(`/item/${encodeURIComponent(serverID)}/${encodeURIComponent(ratingKey)}`);
  }

  async function navigateToDiscoverItem(it: Item) {
    if (!it.guid) {
      navigate(`/discover-item/${encodeURIComponent(it.ratingKey)}`);
      return;
    }
    try {
      const matches = (await api.availability(it.guid)) ?? [];
      if (matches.length > 0) {
        const best = matches.reduce((a, b) =>
          parseRes(a.resolution) >= parseRes(b.resolution) ? a : b,
        );
        navigate(`/item/${encodeURIComponent(best.machineIdentifier)}/${encodeURIComponent(best.ratingKey)}`);
      } else {
        navigate(`/discover-item/${encodeURIComponent(it.ratingKey)}`);
      }
    } catch {
      navigate(`/discover-item/${encodeURIComponent(it.ratingKey)}`);
    }
  }

  return (
    <div class="search-results-page">
      <header class="search-results-header">
        <h1 class="search-results-title">
          <Show when={query()} fallback={<>Search</>}>
            Results for "{query()}"
          </Show>
        </h1>
      </header>

      <Show when={query().length < 2}>
        <div class="search-results-empty">Type at least 2 characters in the search bar above.</div>
      </Show>

      <Show when={query().length >= 2 && results.loading}>
        <Skeleton kind="card" count={12} />
      </Show>

      <Show when={results() && totalHits() === 0 && !results.loading}>
        <div class="search-results-empty">No matches for "{query()}".</div>
      </Show>

      <Show when={results() && totalHits() > 0}>
        <For each={results()!.servers}>
          {(bucket) => (
            <Show when={bucket.items.length > 0}>
              <ResultGroup
                title={bucket.displayName}
                items={bucket.items}
                thumbFor={(it) => serverThumb(bucket.machineIdentifier, it.thumb)}
                onPick={(it) => navigateToServerItem(bucket.machineIdentifier, it.ratingKey)}
              />
            </Show>
          )}
        </For>
        <Show when={results()!.discover.length > 0}>
          <ResultGroup
            title="Discover"
            items={results()!.discover}
            thumbFor={(it) => it.thumb ?? ""}
            onPick={navigateToDiscoverItem}
          />
        </Show>
      </Show>
    </div>
  );
}

function ResultGroup(props: {
  title: string;
  items: Item[];
  thumbFor: (it: Item) => string;
  onPick: (it: Item) => void;
}) {
  return (
    <section class="search-results-section">
      <div class="search-results-section-header">
        <h2 class="search-results-section-title">{props.title}</h2>
        <span class="search-results-section-count">{props.items.length} {props.items.length === 1 ? "result" : "results"}</span>
      </div>
      <div class="search-results-grid">
        <For each={props.items}>
          {(it) => {
            const thumb = props.thumbFor(it);
            return (
              <button
                type="button"
                class="search-result-card"
                onClick={() => props.onPick(it)}
              >
                <div class="search-result-poster">
                  <Show when={thumb} fallback={<div class="search-result-poster-empty"><ImageOff size={32} strokeWidth={1.5} /></div>}>
                    <img src={thumb} alt="" loading="lazy" referrerpolicy="no-referrer" />
                  </Show>
                </div>
                <div class="search-result-meta">
                  <div class="search-result-title">{it.title}</div>
                  <div class="search-result-sub">
                    {it.type}
                    <Show when={it.year}> · {it.year}</Show>
                  </div>
                </div>
              </button>
            );
          }}
        </For>
      </div>
    </section>
  );
}

function serverThumb(serverID: string, path?: string): string {
  if (!path) return "";
  return `/api/image-proxy?server=${encodeURIComponent(serverID)}&path=${encodeURIComponent(path)}&w=240&h=360`;
}

function parseRes(r?: string): number {
  if (!r) return 0;
  const lower = r.toLowerCase();
  if (lower.includes("4k")) return 2160;
  const n = parseInt(lower, 10);
  return Number.isNaN(n) ? 0 : n;
}
