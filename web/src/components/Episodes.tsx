import { createEffect, createMemo, createResource, createSignal, For, Show } from "solid-js";
import { A, useNavigate } from "@solidjs/router";
import { api } from "../api/client";
import type { Item, Season } from "../api/types";
import Skeleton from "./Skeleton";
import { refetchOnFocus } from "../util/focusRefetch";
import "./Episodes.css";

export default function Episodes(props: {
  serverID: string;
  showRatingKey: string;
  initialSeasonIndex?: number;
  /** When true, clicking a season pill navigates to that season's first
   *  episode instead of just filtering the inline episode list. Used on
   *  episode-detail pages where staying on the original episode while
   *  toggling pills is confusing — Byron's spec call. Show-detail pages
   *  leave this false so users can browse seasons without leaving the show. */
  navigateOnSeasonChange?: boolean;
}) {
  const navigate = useNavigate();

  const [seasons, { refetch: refetchSeasons }] = createResource(
    () => props.showRatingKey,
    (k) => api.seasons(props.serverID, k)
  );

  const realSeasons = createMemo(() => (seasons() ?? []).filter((s) => s.index > 0));

  const [activeKey, setActiveKey] = createSignal<string | null>(null);
  // pendingNavigate flips to true on a "navigate-mode" pill click and stays
  // until the corresponding episodes resource resolves — at which point the
  // effect below jumps to the season's first episode. Decouples the click
  // (synchronous) from the navigate (waits on async fetch).
  const [pendingNavigate, setPendingNavigate] = createSignal(false);

  createMemo(() => {
    if (activeKey()) return;
    const list = realSeasons();
    if (list.length === 0) return;
    if (props.initialSeasonIndex) {
      const match = list.find((s: Season) => s.index === props.initialSeasonIndex);
      if (match) { setActiveKey(match.ratingKey); return; }
    }
    setActiveKey(list[0].ratingKey);
  });

  const [episodes, { refetch: refetchEpisodes }] = createResource(
    () => activeKey(),
    (key) => (key ? api.seasonEpisodes(props.serverID, key) : Promise.resolve([] as Item[]))
  );

  // Pick up viewCount changes from within-Lumen mutations (Mark Watched on a
  // parent show, Pot Player close, etc.) so the teal episode-watched tick
  // updates without a manual refresh.
  refetchOnFocus(() => {
    refetchSeasons();
    refetchEpisodes();
  });

  // Navigate-mode: when the episodes for the newly-activated season resolve,
  // jump to ep[0]. Empty seasons just clear the flag and stay put — the user
  // sees the empty list, no jarring redirect. Clears on each fire so a later
  // season-change click gets a fresh wait.
  createEffect(() => {
    if (!pendingNavigate()) return;
    if (episodes.loading) return;
    const eps = episodes();
    if (!eps || eps.length === 0) {
      setPendingNavigate(false);
      return;
    }
    setPendingNavigate(false);
    navigate(`/item/${encodeURIComponent(props.serverID)}/${encodeURIComponent(eps[0].ratingKey)}`);
  });

  function handleSeasonClick(seasonRatingKey: string) {
    // No-op when clicking the already-active pill — avoids spurious refetch
    // and (in navigate mode) re-navigating to the current page.
    if (seasonRatingKey === activeKey()) return;
    setActiveKey(seasonRatingKey);
    if (props.navigateOnSeasonChange) {
      setPendingNavigate(true);
    }
  }

  return (
    <section class="episodes">
      <h3>Episodes</h3>
      <Show when={realSeasons().length > 0} fallback={<Skeleton kind="line" count={2} />}>
        <div class="season-tabs">
          <For each={realSeasons()}>
            {(s) => (
              <button
                class="season-tab"
                classList={{ active: activeKey() === s.ratingKey }}
                onClick={() => handleSeasonClick(s.ratingKey)}
              >
                Season {s.index}
              </button>
            )}
          </For>
        </div>
      </Show>
      <Show when={!episodes.loading} fallback={<Skeleton kind="line" count={6} />}>
        <ul class="episode-list">
          <For each={episodes() ?? []}>
            {(ep) => (
              <li class="episode-row">
                <A href={`/item/${props.serverID}/${ep.ratingKey}`} class="episode-link">
                  <Show when={ep.thumb}>
                    <img class="episode-thumb" src={api.image(props.serverID, ep.thumb!)} alt="" />
                  </Show>
                  <div class="episode-meta">
                    <div class="episode-line1">
                      <span class="episode-num">E{ep.index}</span>
                      <span class="episode-title">{ep.title}</span>
                      <Show when={(ep.viewCount ?? 0) > 0}>
                        <span class="episode-watched" title="Watched">✓</span>
                      </Show>
                    </div>
                    <Show when={ep.summary}>
                      <div class="episode-summary">{ep.summary}</div>
                    </Show>
                    <div class="episode-line3">
                      <Show when={ep.duration}>
                        <span>{Math.round((ep.duration ?? 0) / 60_000)} min</span>
                      </Show>
                      <Show when={ep.originallyAvailableAt}>
                        <span class="episode-date"> · {ep.originallyAvailableAt}</span>
                      </Show>
                    </div>
                  </div>
                </A>
              </li>
            )}
          </For>
        </ul>
      </Show>
    </section>
  );
}
