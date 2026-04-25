import { useParams } from "@solidjs/router";
import { createResource, createSignal, For, Show } from "solid-js";
import { api } from "../api/client";
import type { Item, Match } from "../api/types";
import Skeleton from "../components/Skeleton";
import ResumeRestartModal from "../components/Modal/ResumeRestartModal";
import "./ItemDetail.css";

export default function ItemDetail() {
  const params = useParams();
  const [item, { refetch: refetchItem }] = createResource(
    () => ({ server: params.serverID!, rk: params.ratingKey! }),
    ({ server, rk }) => api.item(server, rk)
  );
  const [availability] = createResource(
    () => item()?.guid,
    (guid) => (guid ? api.availability(guid) : Promise.resolve([] as Match[]))
  );

  const [resumeOpen, setResumeOpen] = createSignal(false);

  async function handlePlay() {
    const it = item();
    if (!it) return;
    const offset = it.viewOffset ?? 0;
    const dur = it.duration ?? 0;
    if (offset > 0 && dur > 0 && offset < dur * 0.9) {
      setResumeOpen(true);
      return;
    }
    await playFromStart();
  }

  async function playFromStart() {
    const it = item();
    if (!it) return;
    try {
      await api.play(params.serverID!, it.ratingKey);
    } catch (e) {
      console.error("play failed:", e);
      alert(`Play failed: ${(e as Error).message}`);
    }
  }

  async function playResume() {
    const it = item();
    if (!it) return;
    try {
      await api.play(params.serverID!, it.ratingKey, it.viewOffset);
    } catch (e) {
      console.error("play resume failed:", e);
      alert(`Play failed: ${(e as Error).message}`);
    }
  }

  async function handleMarkWatched() {
    const it = item();
    if (!it) return;
    try {
      await api.scrobble(params.serverID!, it.ratingKey);
      refetchItem();
    } catch (e) {
      alert(`Mark watched failed: ${(e as Error).message}`);
    }
  }

  async function handleMarkUnwatched() {
    const it = item();
    if (!it) return;
    try {
      await api.unscrobble(params.serverID!, it.ratingKey);
      refetchItem();
    } catch (e) {
      alert(`Mark unwatched failed: ${(e as Error).message}`);
    }
  }

  return (
    <div class="item-detail">
      <Show when={item()} fallback={<div class="item-loading"><Skeleton kind="line" count={4} /></div>}>
        {(it) => (
          <>
            <Hero item={it() as Item} serverID={params.serverID!} />
            <nav class="action-row">
              <button class="btn-primary" onClick={handlePlay}>
                ▶ {(it() as Item).viewOffset && (it() as Item).viewOffset! > 0 ? "Resume" : "Play"}
              </button>
              <select class="btn-subtitle" disabled>
                <option>Subtitle: Default</option>
                <option>Off</option>
              </select>
              <button class="btn" disabled title="Session 5">Play Trailer</button>
              <button class="btn" onClick={handleMarkWatched}>Mark as Watched</button>
              <button class="btn" onClick={handleMarkUnwatched}>Mark as Unwatched</button>
              <button class="btn" disabled title="Session 5">Add to Watchlist</button>
            </nav>
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
      <ResumeRestartModal
        open={resumeOpen()}
        resumeOffsetMs={item()?.viewOffset ?? 0}
        onResume={() => { setResumeOpen(false); playResume(); }}
        onRestart={() => { setResumeOpen(false); playFromStart(); }}
        onCancel={() => setResumeOpen(false)}
      />
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
  // Backdrop preference order:
  //  1. item.art (movie/show backdrop)
  //  2. item.grandparentArt (episode → show backdrop)
  //  3. item.thumb (poster as last resort)
  const backdropPath = () =>
    props.item.art || props.item.grandparentArt || props.item.thumb;
  return (
    <header class="hero">
      <Show when={backdropPath()}>
        <div
          class="hero-backdrop"
          style={{ "background-image": `url(${api.image(props.serverID, backdropPath()!)})` }}
        />
        <div class="hero-fade" />
      </Show>
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

function formatBytes(n: number): string {
  if (!n) return "";
  const units = ["B", "KB", "MB", "GB", "TB"];
  let u = 0;
  let v = n;
  while (v >= 1024 && u < units.length - 1) { v /= 1024; u++; }
  return `${v.toFixed(1)} ${units[u]}`;
}
