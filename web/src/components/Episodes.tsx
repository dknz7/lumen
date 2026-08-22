import { createEffect, createMemo, createResource, createSignal, For, on, Show } from "solid-js";
import { A, useNavigate } from "@solidjs/router";
import { api } from "../api/client";
import type { Item, Season } from "../api/types";
import Skeleton from "./Skeleton";
import { refetchOnFocus } from "../util/focusRefetch";
import { stableArrayByKey } from "../util/stableArray";
import "./Episodes.css";

export default function Episodes(props: {
  serverID: string;
  showRatingKey: string;
  initialSeasonIndex?: number;
  /** When true, clicking a season pill navigates to that season's first
   *  episode instead of just filtering the inline episode list. Used on
   *  episode-detail pages where staying on the original episode while
   *  toggling pills is confusing — a deliberate choice. Show-detail pages
   *  leave this false so users can browse seasons without leaving the show. */
  navigateOnSeasonChange?: boolean;
}) {
  const navigate = useNavigate();

  const [seasons, { refetch: refetchSeasons }] = createResource(
    () => props.showRatingKey,
    (k) => api.seasons(props.serverID, k)
  );

  const realSeasons = createMemo(() =>
    seasons.error ? [] : (seasons() ?? []).filter((s) => s.index > 0),
  );

  const [activeKey, setActiveKey] = createSignal<string | null>(null);
  // pendingNavigate flips to true on a "navigate-mode" pill click and stays
  // until the corresponding episodes resource resolves — at which point the
  // effect below jumps to the season's first episode. Decouples the click
  // (synchronous) from the navigate (waits on async fetch).
  const [pendingNavigate, setPendingNavigate] = createSignal(false);

  // Solid Router keeps this component mounted across /item/:s/:rk param
  // changes — clicking a "More ways to watch" row, a hero show-link, or a
  // search result while already on a detail page swaps props.showRatingKey
  // without remounting. activeKey then still held the PREVIOUS show's season,
  // the picker below bailed on `if (activeKey()) return`, and the episodes
  // resource never refetched: new show's season tabs above the old show's
  // episode list (or a 404 when the server changed too).
  //
  // defer:true so this doesn't clear the key on first render.
  createEffect(
    on(
      () => props.showRatingKey,
      () => {
        setActiveKey(null);
        setPendingNavigate(false);
      },
      { defer: true },
    ),
  );

  // A memo used purely for its side effect is really an effect; it only worked
  // because something happened to read it.
  createEffect(() => {
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

  // Episodes were the one list still iterated raw while refetchOnFocus was
  // attached: <For> keys by reference, so every alt-tab handed it new objects
  // and remounted every row — the click-loss/flicker class that
  // stableArrayByKey exists to prevent, fixed everywhere else.
  //
  // This MUST stay below the `episodes` resource. stableArrayByKey is a
  // createMemo, and Solid runs a memo's body the instant it is created — so
  // declaring it above `episodes` read a const inside its temporal dead zone
  // and threw "Cannot access 'episodes' before initialization" (minified to a
  // bare 'f') the moment any episode or show detail page mounted. Continue
  // Watching is almost all episodes, which is why that shelf looked cursed
  // while Recently Added — seasons and movies — was fine.
  const stableEpisodes = stableArrayByKey<Item>(
    () => (episodes.error ? [] : ((episodes() as Item[] | undefined) ?? [])),
    (ep) => ep.ratingKey,
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
          <For each={stableEpisodes()}>
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
