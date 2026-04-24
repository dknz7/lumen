import { useParams } from "@solidjs/router";
import { createResource, For, Show } from "solid-js";
import { api } from "../api/client";
import type { Item, Match } from "../api/types";
import Skeleton from "../components/Skeleton";
import "./ItemDetail.css";

export default function ItemDetail() {
  const params = useParams();
  const [item] = createResource(
    () => ({ server: params.serverID!, rk: params.ratingKey! }),
    ({ server, rk }) => api.item(server, rk)
  );
  const [availability] = createResource(
    () => item()?.guid,
    (guid) => (guid ? api.availability(guid) : Promise.resolve([] as Match[]))
  );

  return (
    <div class="item-detail">
      <Show when={item()} fallback={<div class="item-loading"><Skeleton kind="line" count={4} /></div>}>
        {(it) => (
          <>
            <Hero item={it() as Item} serverID={params.serverID!} />
            <ActionRow item={it() as Item} serverID={params.serverID!} />
            <section class="overview">
              <h3>Overview</h3>
              <p>{(it() as Item).summary ?? "No synopsis available."}</p>
            </section>
            <section class="availability">
              <h3>More Ways to Watch</h3>
              <Show when={availability()} fallback={<div class="availability-loading">Checking other servers…</div>}>
                {(matches) => (
                  <ul>
                    <For each={matches() as Match[]}>
                      {(m) => (
                        <li class="availability-row">
                          <strong>{m.serverName || m.machineIdentifier}</strong>
                          <span class="availability-lib">{m.libraryName}</span>
                          <span class="availability-quality">{m.resolution}p · {m.codec ?? m.container}</span>
                          <span class="availability-size">{formatBytes(m.size)}</span>
                        </li>
                      )}
                    </For>
                    <Show when={(matches() as Match[]).length === 0}>
                      <li class="availability-empty">Not available on any connected server.</li>
                    </Show>
                  </ul>
                )}
              </Show>
            </section>
          </>
        )}
      </Show>
    </div>
  );
}

function Hero(props: { item: Item; serverID: string }) {
  const isEpisode = () => props.item.type === "episode";
  const showTitle = () => (isEpisode() ? props.item.grandparentTitle : props.item.title);
  const episodeLabel = () => {
    if (!isEpisode()) return null;
    const season = props.item.parentIndex ?? 0;
    const ep = props.item.index ?? 0;
    const se = season && ep ? `S${season} · E${ep}` : "";
    const title = props.item.title;
    return se + (title && se ? ` · ${title}` : title ?? "");
  };
  return (
    <header class="hero">
      <div class="hero-meta">
        <h1>{showTitle() ?? props.item.title}</h1>
        {isEpisode() && <div class="hero-episode">{episodeLabel()}</div>}
        <div class="meta-pills">
          {props.item.year && <span class="pill">{props.item.year}</span>}
          {props.item.type && <span class="pill">{props.item.type}</span>}
          {/* IMDB rating pill lands Session 5 with OMDB integration */}
        </div>
      </div>
    </header>
  );
}

function ActionRow(props: { item: Item; serverID: string }) {
  return (
    <nav class="action-row">
      <button class="btn-primary" onClick={() => launchPlayback(props.item, props.serverID)}>
        ▶ Play
      </button>
      <select class="btn-subtitle" disabled>
        <option>Subtitle: Default</option>
        <option>Off</option>
      </select>
      <button class="btn" disabled title="Session 5">Play Trailer</button>
      <button class="btn" disabled title="Session 4">Mark as Watched</button>
      <button class="btn" disabled title="Session 4">Mark as Unwatched</button>
      <button class="btn" disabled title="Session 5">Add to Watchlist</button>
    </nav>
  );
}

async function launchPlayback(item: Item, serverID: string) {
  console.log("play stub — Session 4 will wire this", { item, serverID });
  try {
    const res = await fetch("/api/play", { method: "POST" });
    console.log("server said:", res.status);
  } catch (e) {
    console.error(e);
  }
}

function formatBytes(n: number): string {
  if (!n) return "";
  const units = ["B", "KB", "MB", "GB", "TB"];
  let u = 0;
  let v = n;
  while (v >= 1024 && u < units.length - 1) { v /= 1024; u++; }
  return `${v.toFixed(1)} ${units[u]}`;
}
