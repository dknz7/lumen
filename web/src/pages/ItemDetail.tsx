import { useParams, A } from "@solidjs/router";
import { createResource, createSignal, For, Show } from "solid-js";
import { api } from "../api/client";
import type { Item, Match } from "../api/types";
import Skeleton from "../components/Skeleton";
import Episodes from "../components/Episodes";
import ResumeRestartModal from "../components/Modal/ResumeRestartModal";
import { refetchOnFocus } from "../util/focusRefetch";
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

  // Pick up viewCount/viewOffset changes made elsewhere (Plex Web, Plex Desktop)
  // when the user switches back to the Lumen tab.
  refetchOnFocus(() => refetchItem());

  const [resumeOpen, setResumeOpen] = createSignal(false);

  async function handlePlay() {
    const it = item();
    if (!it) return;
    const offset = it.viewOffset ?? 0;
    const dur = it.duration ?? 0;
    // 95% matches the scrobble/end-of-file threshold in poller.go — past
    // that point the user is functionally done with the item, just play
    // from start instead of offering resume.
    if (offset > 0 && dur > 0 && offset < dur * 0.95) {
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
      // Plex's /library/metadata/<key> reflects the scrobble after a brief
      // server-side cache propagation (~100-300ms). Wait before refetching to
      // avoid getting stale viewCount/viewOffset.
      await new Promise((r) => setTimeout(r, 350));
      // Single dispatch triggers all refetchOnFocus subscribers including
      // ItemDetail's own refetchItem and any mounted Episodes resources.
      window.dispatchEvent(new CustomEvent("lumen:data-invalidated"));
    } catch (e) {
      alert(`Mark watched failed: ${(e as Error).message}`);
    }
  }

  // ms → "H:MM:SS" (or "M:SS" when <1h). Plex's viewOffset/duration on Item
  // are milliseconds; NowPlaying.tsx has a similar helper but operates on
  // nanoseconds — keep these inline until Phase 3's Now Playing rebuild
  // promotes them to util/time.ts.
  const fmtMs = (ms: number) => {
    const s = Math.floor(ms / 1000);
    const m = Math.floor(s / 60);
    const sec = s % 60;
    const h = Math.floor(m / 60);
    const min = m % 60;
    if (h > 0) return `${h}:${String(min).padStart(2, "0")}:${String(sec).padStart(2, "0")}`;
    return `${min}:${String(sec).padStart(2, "0")}`;
  };
  const remainingPct = (offset: number, dur: number) => Math.round(100 - (offset / dur) * 100);

  async function handleMarkUnwatched() {
    const it = item();
    if (!it) return;
    try {
      await api.unscrobble(params.serverID!, it.ratingKey);
      // Same Plex metadata-cache propagation race as handleMarkWatched.
      await new Promise((r) => setTimeout(r, 350));
      // Single dispatch triggers all refetchOnFocus subscribers including
      // ItemDetail's own refetchItem and any mounted Episodes resources.
      window.dispatchEvent(new CustomEvent("lumen:data-invalidated"));
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
              <button class="btn" onClick={handleMarkWatched}>
                {((it() as Item).viewCount ?? 0) > 0 ? "✓ Watched" : "Mark as Watched"}
              </button>
              <button class="btn" onClick={handleMarkUnwatched}>Mark as Unwatched</button>
              <button class="btn" disabled title="Session 5">Add to Watchlist</button>
            </nav>
            <Show when={(it() as Item).viewOffset && (it() as Item).viewOffset! > 0 && (it() as Item).duration && (it() as Item).duration! > 0}>
              <div class="resume-subtitle">
                Watched {fmtMs((it() as Item).viewOffset!)} of {fmtMs((it() as Item).duration!)}
                {" · "}
                {remainingPct((it() as Item).viewOffset!, (it() as Item).duration!)}% remaining
              </div>
            </Show>
            <section class="overview">
              <h3>Overview</h3>
              <p>{(it() as Item).summary ?? "No synopsis available."}</p>
            </section>
            <Show when={(it() as Item).type === "episode" || (it() as Item).type === "show"}>
              <Episodes
                serverID={params.serverID!}
                showRatingKey={(it() as Item).grandparentRatingKey ?? (it() as Item).ratingKey}
                initialSeasonIndex={(it() as Item).parentIndex}
              />
            </Show>
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
  const isSeason = () => props.item.type === "season";
  const linksToShow = () => isEpisode() || isSeason();
  const showTitle = () => (isEpisode() ? props.item.grandparentTitle : props.item.title);
  const showRatingKey = () => props.item.grandparentRatingKey;
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
        <h1>
          <Show
            when={linksToShow() && showRatingKey()}
            fallback={<>{showTitle() ?? props.item.title}</>}
          >
            <A class="hero-title-link" href={`/item/${props.serverID}/${showRatingKey()}`}>
              {showTitle() ?? props.item.title}
            </A>
          </Show>
        </h1>
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
