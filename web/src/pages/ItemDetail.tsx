import { useParams, A } from "@solidjs/router";
import { createMemo, createResource, createSignal, For, Show } from "solid-js";
import { api } from "../api/client";
import type { Item, Match } from "../api/types";
import Skeleton from "../components/Skeleton";
import Episodes from "../components/Episodes";
import ResumeRestartModal from "../components/Modal/ResumeRestartModal";
import TrailerModal from "../components/Modal/TrailerModal";
import { refetchOnFocus } from "../util/focusRefetch";
import { extractPlexTvRatingKey } from "../util/plexGuid";
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

  // Local servers list — used to resolve the SPA's display-name override
  // (set via `lumen rename` or Settings → Accounts & Servers) for the
  // MORE WAYS TO WATCH rows. Plex's wire `serverName` is empty for some
  // shared-to-you servers (e.g. Stargaze — Session 1 finding).
  const [servers] = createResource(() => api.servers());
  const displayName = (machineID: string): string => {
    const list = servers();
    if (!list) return "";
    const found = list.find((s) => s.machineIdentifier === machineID);
    return found?.displayName ?? "";
  };

  // Pick up viewCount/viewOffset changes made elsewhere (Plex Web, Plex Desktop)
  // when the user switches back to the Lumen tab.
  refetchOnFocus(() => refetchItem());

  const [resumeOpen, setResumeOpen] = createSignal(false);
  const [trailerOpen, setTrailerOpen] = createSignal(false);

  // Optimistic — flips immediately on click, reverts if the API call fails.
  // Plex propagates Watchlist add/remove cross-device after a brief lag.
  const [inWatchlistOverride, setInWatchlistOverride] = createSignal<boolean | null>(null);
  const [watchlist] = createResource(() => api.watchlist().catch(() => []));
  const isInWatchlist = createMemo(() => {
    const override = inWatchlistOverride();
    if (override !== null) return override;
    const rk = extractPlexTvRatingKey(item()?.guid);
    if (!rk) return false;
    return (watchlist() ?? []).some((w) => w.ratingKey === rk);
  });

  async function toggleWatchlist() {
    const it = item();
    if (!it) return;
    const rk = extractPlexTvRatingKey(it.guid);
    if (!rk) {
      alert("This item doesn't have a plex.tv GUID — can't toggle watchlist.");
      return;
    }
    const wasIn = isInWatchlist();
    setInWatchlistOverride(!wasIn);
    try {
      if (wasIn) await api.watchlistRemove(rk); else await api.watchlistAdd(rk);
      // Brief Plex propagation delay before broadcasting invalidation so
      // any focus-refetched resources see fresh state.
      setTimeout(() => window.dispatchEvent(new CustomEvent("lumen:data-invalidated")), 350);
    } catch (e) {
      setInWatchlistOverride(wasIn);
      alert(`Watchlist toggle failed: ${(e as Error).message}`);
    }
  }

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
              <button
                class="btn"
                disabled={!(it() as Item).trailer?.youtubeID}
                title={
                  (it() as Item).trailer?.youtubeID
                    ? "Play trailer"
                    : (it() as Item).trailer?.plexKey
                      ? "Plex-hosted trailer (not supported in v1.0)"
                      : "No trailer available"
                }
                onClick={() => setTrailerOpen(true)}
              >
                Play Trailer
              </button>
              <button class="btn" onClick={handleMarkWatched}>
                {((it() as Item).viewCount ?? 0) > 0 ? "✓ Watched" : "Mark as Watched"}
              </button>
              <button class="btn" onClick={handleMarkUnwatched}>Mark as Unwatched</button>
              <button
                class="btn"
                disabled={!extractPlexTvRatingKey((it() as Item).guid)}
                onClick={toggleWatchlist}
                title={
                  extractPlexTvRatingKey((it() as Item).guid)
                    ? (isInWatchlist() ? "Remove from Watchlist" : "Add to Watchlist")
                    : "No plex.tv GUID — Watchlist unavailable for this item"
                }
              >
                {isInWatchlist() ? "Remove from Watchlist" : "Add to Watchlist"}
              </button>
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
                          <A href={`/item/${m.machineIdentifier}/${m.ratingKey}`} class="availability-link">
                            <span class="availability-server">
                              <strong>{displayName(m.machineIdentifier) || m.serverName || m.machineIdentifier}</strong>
                              <Show when={m.machineIdentifier === params.serverID && m.ratingKey === params.ratingKey}>
                                <span class="availability-current-led" title="Currently viewing" aria-label="Currently viewing" />
                              </Show>
                            </span>
                            <span class="availability-lib">{m.libraryName}</span>
                            <span class="availability-quality">{m.resolution}p · {m.codec ?? m.container}</span>
                            <span class="availability-size">{formatBytes(m.size)}</span>
                          </A>
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
            <CastCrew item={it() as Item} serverID={params.serverID!} />
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
      <TrailerModal
        open={trailerOpen()}
        onClose={() => setTrailerOpen(false)}
        youtubeID={item()?.trailer?.youtubeID}
        title={`${item()?.title ?? ""} — Trailer`}
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
          style={{ "background-image": `url(${api.image(props.serverID, backdropPath()!, "hero")})` }}
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
          <IMDBPill imdbId={props.item.imdbId} />
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

function IMDBPill(props: { imdbId?: string }) {
  const [rating] = createResource(
    () => props.imdbId,
    async (id) => (id ? api.imdb(id) : null)
  );
  return (
    <span class="pill pill-imdb">
      <span class="pill-imdb-label">IMDB</span>
      <Show when={rating()} fallback={<span class="pill-imdb-value">—</span>}>
        {(r) => <span class="pill-imdb-value">{(r() as import("../api/types").OMDBRating).imdbRating ?? "—"}</span>}
      </Show>
    </span>
  );
}

function CastCrew(props: { item: Item; serverID: string }) {
  const cast = () => props.item.roles ?? [];
  const directors = () => props.item.directors ?? [];
  const writers = () => props.item.writers ?? [];
  const hasCrew = () => directors().length > 0 || writers().length > 0;
  return (
    <Show when={cast().length > 0 || hasCrew()}>
      <section class="cast-crew">
        <Show when={cast().length > 0}>
          <h3>Cast</h3>
          <ul class="people-grid">
            <For each={cast()}>
              {(p) => <PersonCard person={p} serverID={props.serverID} />}
            </For>
          </ul>
        </Show>
        <Show when={hasCrew()}>
          <h3>Crew</h3>
          <ul class="people-grid">
            <For each={directors()}>
              {(p) => <PersonCard person={{ ...p, tag: p.tag || "Director" }} serverID={props.serverID} />}
            </For>
            <For each={writers()}>
              {(p) => <PersonCard person={{ ...p, tag: p.tag || "Writer" }} serverID={props.serverID} />}
            </For>
          </ul>
        </Show>
      </section>
    </Show>
  );
}

function PersonCard(props: { person: import("../api/types").Person; serverID: string }) {
  const fallbackThumb = "data:image/svg+xml;base64,PHN2ZyB4bWxucz0iaHR0cDovL3d3dy53My5vcmcvMjAwMC9zdmciIHZpZXdCb3g9IjAgMCAyNCAyNCIgZmlsbD0ibm9uZSIgc3Ryb2tlPSIjNjY2IiBzdHJva2Utd2lkdGg9IjEuNSI+PGNpcmNsZSBjeD0iMTIiIGN5PSI4IiByPSI0Ii8+PHBhdGggZD0iTTQgMjBjMC00IDQtNyA4LTdzOCAzIDggNyIvPjwvc3ZnPg==";
  const src = () =>
    props.person.thumb
      ? api.image(props.serverID, props.person.thumb!, "person")
      : fallbackThumb;
  return (
    <li class="person-card">
      <img src={src()} alt={props.person.name} class="person-thumb"
           onError={(e) => { (e.currentTarget as HTMLImageElement).src = fallbackThumb; }} />
      <div class="person-name">{props.person.name}</div>
      <Show when={props.person.tag}>
        <div class="person-tag">{props.person.tag}</div>
      </Show>
    </li>
  );
}
