import { createMemo, createResource, createSignal, For, Show } from "solid-js";
import { A } from "@solidjs/router";
import { api } from "../api/client";
import type { Item, Season } from "../api/types";
import Skeleton from "./Skeleton";
import { refetchOnFocus } from "../util/focusRefetch";
import "./Episodes.css";

export default function Episodes(props: {
  serverID: string;
  showRatingKey: string;
  initialSeasonIndex?: number;
}) {
  const [seasons, { refetch: refetchSeasons }] = createResource(
    () => props.showRatingKey,
    (k) => api.seasons(props.serverID, k)
  );

  const realSeasons = createMemo(() => (seasons() ?? []).filter((s) => s.index > 0));

  const [activeKey, setActiveKey] = createSignal<string | null>(null);

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
                onClick={() => setActiveKey(s.ratingKey)}
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
