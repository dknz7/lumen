import { createEffect, createMemo, createResource, createSignal, For, Show, onCleanup } from "solid-js";
import { useNavigate } from "@solidjs/router";
import { api } from "../api/client";
import type { Item } from "../api/types";
import { ImageOff } from "./icons";
import "./SearchFlydown.css";

// Per-source cap in the flydown — Plex Web shows a similar count.
// Full /search results page renders the unrestricted set.
const FLYDOWN_PER_SOURCE_CAP = 5;

// Min query length before we hit the network. Keeps single-letter
// keystrokes from triggering 100+ irrelevant matches and a cluttered
// flydown. Hidden until the user crosses the threshold.
const MIN_QUERY_LEN = 2;

export interface SearchFlydownProps {
  query: string;
  onClose: () => void;
}

// SearchFlydown — debounced live results panel anchored under the TopBar
// search input. Renders three sections (one per Plex server, plus Discover),
// each capped at FLYDOWN_PER_SOURCE_CAP items. Clicking an item routes to
// the right detail page based on source: server items route directly to
// the server's item detail; discover items go through availability lookup
// (in-library match → server item detail; otherwise → discover item detail).
export default function SearchFlydown(props: SearchFlydownProps) {
  const navigate = useNavigate();

  // Debounced query — updated 300ms after props.query stops changing.
  // The resource below keys off this signal so it only refetches when
  // the debounced value flips, not on every keystroke.
  const [debounced, setDebounced] = createSignal(props.query);
  let debounceTimer: ReturnType<typeof setTimeout> | undefined;

  createEffect(() => {
    const q = props.query;
    if (debounceTimer) clearTimeout(debounceTimer);
    debounceTimer = setTimeout(() => setDebounced(q), 300);
  });

  onCleanup(() => {
    if (debounceTimer) clearTimeout(debounceTimer);
  });

  const [results] = createResource(
    () => {
      const q = debounced().trim();
      if (q.length < MIN_QUERY_LEN) return null;
      return q;
    },
    (q) => api.search(q)
  );

  // Total non-empty buckets — when zero AND not loading, render the
  // empty-state. Buckets stay in the response even when items[]==[], so
  // we count by content rather than by presence.
  const totalHits = createMemo(() => {
    const r = results();
    if (!r) return 0;
    return (
      r.servers.reduce((sum, b) => sum + b.items.length, 0) + r.discover.length
    );
  });

  function navigateToServerItem(serverID: string, ratingKey: string) {
    navigate(`/item/${encodeURIComponent(serverID)}/${encodeURIComponent(ratingKey)}`);
    props.onClose();
  }

  // Discover-item routing mirrors Watchlist's pattern: lazy availability
  // check on click, pick the highest-resolution match if any, fall back
  // to the discover item detail page if not in any library.
  async function navigateToDiscoverItem(it: Item) {
    if (!it.guid) {
      navigate(`/discover-item/${encodeURIComponent(it.ratingKey)}`);
      props.onClose();
      return;
    }
    try {
      const matches = (await api.availability(it.guid)) ?? [];
      if (matches.length > 0) {
        // Highest-resolution match (Stargaze 1080p preferred over DKNZPLEX 720p).
        const best = matches.reduce((a, b) =>
          parseResolution(a.resolution) >= parseResolution(b.resolution) ? a : b,
        );
        navigate(`/item/${encodeURIComponent(best.machineIdentifier)}/${encodeURIComponent(best.ratingKey)}`);
      } else {
        navigate(`/discover-item/${encodeURIComponent(it.ratingKey)}`);
      }
    } catch {
      // Availability lookup failed — fall back to discover item detail.
      navigate(`/discover-item/${encodeURIComponent(it.ratingKey)}`);
    }
    props.onClose();
  }

  return (
    <div class="search-flydown" role="listbox" aria-label="Search results">
      <Show when={!results.loading && totalHits() === 0 && (debounced().trim().length >= MIN_QUERY_LEN)}>
        <div class="search-flydown-empty">No matches for "{debounced()}"</div>
      </Show>
      <Show when={results.loading}>
        <div class="search-flydown-empty">Searching…</div>
      </Show>
      <Show when={results() && totalHits() > 0}>
        {(_) => (
          <>
            <For each={results()!.servers}>
              {(bucket) => (
                <Show when={bucket.items.length > 0}>
                  <SearchSection
                    title={bucket.displayName}
                    items={bucket.items.slice(0, FLYDOWN_PER_SOURCE_CAP)}
                    totalCount={bucket.items.length}
                    thumbFor={(it) => serverThumb(bucket.machineIdentifier, it.thumb)}
                    onPick={(it) => navigateToServerItem(bucket.machineIdentifier, it.ratingKey)}
                  />
                </Show>
              )}
            </For>
            <Show when={results()!.discover.length > 0}>
              <SearchSection
                title="Discover"
                items={results()!.discover.slice(0, FLYDOWN_PER_SOURCE_CAP)}
                totalCount={results()!.discover.length}
                thumbFor={(it) => it.thumb ?? ""}
                onPick={navigateToDiscoverItem}
              />
            </Show>
            <button
              type="button"
              class="search-flydown-see-all"
              onClick={() => {
                navigate(`/search?q=${encodeURIComponent(debounced().trim())}`);
                props.onClose();
              }}
            >
              See all results for "{debounced()}"
            </button>
          </>
        )}
      </Show>
    </div>
  );
}

function SearchSection(props: {
  title: string;
  items: Item[];
  totalCount: number;
  thumbFor: (it: Item) => string;
  onPick: (it: Item) => void;
}) {
  return (
    <div class="search-flydown-section">
      <div class="search-flydown-section-header">
        <span class="search-flydown-section-title">{props.title}</span>
        <Show when={props.totalCount > props.items.length}>
          <span class="search-flydown-section-count">{props.totalCount} total</span>
        </Show>
      </div>
      <ul class="search-flydown-list">
        <For each={props.items}>
          {(it) => {
            const thumb = props.thumbFor(it);
            return (
              <li>
                <button
                  type="button"
                  class="search-flydown-row"
                  onClick={() => props.onPick(it)}
                >
                  <div class="search-flydown-thumb">
                    <Show when={thumb} fallback={<ImageOff size={16} strokeWidth={1.5} />}>
                      <img src={thumb} alt="" referrerpolicy="no-referrer" />
                    </Show>
                  </div>
                  <div class="search-flydown-meta">
                    <div class="search-flydown-title">{it.title}</div>
                    <div class="search-flydown-sub">
                      {it.type}
                      <Show when={it.year}> · {it.year}</Show>
                    </div>
                  </div>
                </button>
              </li>
            );
          }}
        </For>
      </ul>
    </div>
  );
}

// Server-local thumb URL via the existing image proxy. Server-local thumbs
// are relative paths (/library/metadata/.../thumb/...) that require the
// per-server token applied by the proxy. Discover thumbs are absolute
// URLs and bypass this helper entirely.
function serverThumb(serverID: string, path?: string): string {
  if (!path) return "";
  return `/api/image-proxy?server=${encodeURIComponent(serverID)}&path=${encodeURIComponent(path)}&w=64&h=96`;
}

// "1080" → 1080, "4k" → 2160, "" → 0. Lifted from Watchlist's same logic.
function parseResolution(r?: string): number {
  if (!r) return 0;
  const lower = r.toLowerCase();
  if (lower.includes("4k")) return 2160;
  const n = parseInt(lower, 10);
  return Number.isNaN(n) ? 0 : n;
}
