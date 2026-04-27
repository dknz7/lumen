import { useParams, A } from "@solidjs/router";
import { createMemo, createResource, createSignal, For, Show } from "solid-js";
import { api } from "../api/client";
import type { DiscoverItem, DiscoverRating, Match, OMDBRating, Person } from "../api/types";
import Skeleton from "../components/Skeleton";
import TrailerModal from "../components/Modal/TrailerModal";
import "./DiscoverItem.css";
// Reuse ItemDetail's CSS — .hero / .meta-pills / .btn / .availability /
// .people-grid / .person-card / .pill-imdb all live there. We only add
// DiscoverItem-specific rules in DiscoverItem.css.
import "./ItemDetail.css";

// DiscoverItem renders the plex.tv-source detail page. Both
// /discover-item/:ratingKey (Recommended/Discover tile click) and
// /watchlist/:ratingKey (Watchlist card click) resolve here — the wire
// shape and behaviour are identical, just different entry points.
//
// Distinct from ItemDetail (server-local /item/:server/:rk): the title may
// not be on any of your servers (or might be on several — surfaced via the
// MORE WAYS TO WATCH section, filtered to local servers only).
export default function DiscoverItem() {
  const params = useParams();
  const [item, { refetch }] = createResource(
    () => params.ratingKey ?? null,
    async (rk: string) => api.discoverItem(rk)
  );

  // Local servers list — same DisplayName resolution as ItemDetail.tsx so
  // MORE WAYS TO WATCH rows show the user's preferred name (set via Settings
  // → Accounts & Servers).
  const [servers] = createResource(() => api.servers());
  const displayName = (machineID: string): string => {
    const list = servers();
    if (!list) return "";
    const found = list.find((s) => s.machineIdentifier === machineID);
    return found?.displayName ?? "";
  };

  // Availability across local servers — uses item.guid (e.g. "plex://movie/abc").
  // Result is empty when none of the user's servers have this title.
  const [availability] = createResource(
    () => item()?.guid,
    (guid) => (guid ? api.availability(guid) : Promise.resolve([] as Match[]))
  );

  // OMDB IMDB rating pill — same lookup ItemDetail uses.
  const [imdbRating] = createResource(
    () => item()?.imdbId,
    async (id) => (id ? api.imdb(id) : null)
  );

  // Watchlist toggle — optimistic override + revert on error, identical to
  // ItemDetail/DiscoverTile. The plex.tv ratingKey IS the watchlist key
  // (Discover and Watchlist share the discover-namespace ratingKey space).
  const [watchlist] = createResource(() => api.watchlist().catch(() => []));
  const [inWatchlistOverride, setInWatchlistOverride] = createSignal<boolean | null>(null);
  const isInWatchlist = createMemo(() => {
    const override = inWatchlistOverride();
    if (override !== null) return override;
    const rk = item()?.ratingKey;
    if (!rk) return false;
    return (watchlist() ?? []).some((w) => w.ratingKey === rk);
  });

  async function toggleWatchlist() {
    const it = item();
    if (!it) return;
    const wasIn = isInWatchlist();
    setInWatchlistOverride(!wasIn);
    try {
      if (wasIn) await api.watchlistRemove(it.ratingKey);
      else await api.watchlistAdd(it.ratingKey);
      setTimeout(() => window.dispatchEvent(new CustomEvent("lumen:data-invalidated")), 350);
    } catch (e) {
      setInWatchlistOverride(wasIn);
      alert(`Watchlist toggle failed: ${(e as Error).message}`);
    }
  }

  // Trailer — TMDB-first via imdbId (same resolver pattern as ItemDetail).
  // Discover items don't carry Plex Extras, so there's no Plex fallback —
  // if TMDB returns nothing, the button stays disabled.
  const [trailerOpen, setTrailerOpen] = createSignal(false);
  const [resolvedTrailer] = createResource<
    string | null,
    { imdbId: string; mediaType: "movie" | "show" }
  >(
    () => {
      const it = item();
      if (!it || !it.imdbId) return null;
      const mediaType: "movie" | "show" =
        it.type === "show" || it.type === "episode" || it.type === "season" ? "show" : "movie";
      return { imdbId: it.imdbId, mediaType };
    },
    async (src) => {
      try {
        return await api.tmdbTrailer(src.imdbId, src.mediaType);
      } catch (e) {
        console.warn("tmdb trailer lookup failed", e);
        return null;
      }
    }
  );

  return (
    <div class="discover-item-page item-detail">
      <Show
        when={!item.error}
        fallback={
          <div class="discover-item-error">
            <h2>Could not load item</h2>
            <p class="discover-item-error-message">
              {item.error instanceof Error ? item.error.message : String(item.error)}
            </p>
            <button type="button" class="btn" onClick={() => refetch()}>
              Retry
            </button>
          </div>
        }
      >
        <Show
          when={item()}
          fallback={<div class="item-loading"><Skeleton kind="line" count={4} /></div>}
        >
          {(it) => (
          <>
            <Hero item={it() as DiscoverItem} imdbRating={imdbRating() ?? null} />
            <nav class="action-row">
              <button
                type="button"
                class="btn-primary"
                onClick={toggleWatchlist}
                title={isInWatchlist() ? "Remove from Watchlist" : "Add to Watchlist"}
              >
                {isInWatchlist() ? "✓ On Watchlist" : "+ Add to Watchlist"}
              </button>
              <button
                type="button"
                class="btn"
                disabled={!resolvedTrailer()}
                title={
                  resolvedTrailer()
                    ? "Play trailer"
                    : (it() as DiscoverItem).imdbId
                      ? "No trailer available (TMDB returned nothing)"
                      : "No IMDB id — Trailer unavailable"
                }
                onClick={() => setTrailerOpen(true)}
              >
                Play Trailer
              </button>
              <Show when={(it() as DiscoverItem).imdbId}>
                <a
                  class="btn"
                  href={`https://www.imdb.com/title/${(it() as DiscoverItem).imdbId}/`}
                  target="_blank"
                  rel="noreferrer"
                >
                  IMDB
                </a>
              </Show>
            </nav>
            <Show when={(it() as DiscoverItem).summary}>
              <section class="overview">
                <h3>Overview</h3>
                <p>{(it() as DiscoverItem).summary}</p>
              </section>
            </Show>
            <section class="availability">
              <h3>More Ways to Watch</h3>
              <Show
                when={!availability.error}
                fallback={
                  <ul>
                    <li class="availability-empty">
                      Couldn't check your servers — try again later.
                    </li>
                  </ul>
                }
              >
                <Show
                  when={availability()}
                  fallback={<div class="availability-loading">Checking your servers…</div>}
                >
                  {(matches) => (
                    <ul>
                      <For each={matches() as Match[]}>
                        {(m) => (
                          <li class="availability-row">
                            <A
                              href={`/item/${m.machineIdentifier}/${m.ratingKey}`}
                              class="availability-link"
                            >
                              <span class="availability-server">
                                <strong>
                                  {displayName(m.machineIdentifier) || m.serverName || m.machineIdentifier}
                                </strong>
                              </span>
                              <span class="availability-lib">{m.libraryName}</span>
                              <span class="availability-quality">
                                {m.resolution}p · {m.codec ?? m.container}
                              </span>
                              <span class="availability-size">{formatBytes(m.size)}</span>
                            </A>
                          </li>
                        )}
                      </For>
                      <Show when={(matches() as Match[]).length === 0}>
                        <li class="availability-empty">
                          Not available on any of your servers.
                        </li>
                      </Show>
                    </ul>
                  )}
                </Show>
              </Show>
            </section>
            <CastCrew item={it() as DiscoverItem} />
          </>
        )}
        </Show>
      </Show>
      <TrailerModal
        open={trailerOpen()}
        onClose={() => setTrailerOpen(false)}
        youtubeID={resolvedTrailer() ?? undefined}
        title={`${item()?.title ?? ""} — Trailer`}
      />
    </div>
  );
}

function Hero(props: { item: DiscoverItem; imdbRating: OMDBRating | null }) {
  // plex.tv image URLs are absolute (https://metadata-static.plex.tv/...);
  // render direct with referrerpolicy="no-referrer" — no image-proxy round
  // trip needed. Confirmed in HubItem comment + DiscoverTile usage.
  const backdrop = () => props.item.art || props.item.thumb || "";
  // Tomatometer + audience rating from ratings[]. We surface up to one
  // critic value (RT) and one audience value (RT preferred, IMDB fallback).
  const criticRT = () =>
    props.item.ratings?.find(
      (r) => r.type === "critic" && r.image.startsWith("rottentomatoes://"),
    );
  const audienceRT = () =>
    props.item.ratings?.find(
      (r) => r.type === "audience" && r.image.startsWith("rottentomatoes://"),
    );
  const runtimeText = () => {
    const ms = props.item.duration ?? 0;
    if (ms <= 0) return "";
    const min = Math.round(ms / 60000);
    const h = Math.floor(min / 60);
    const m = min % 60;
    if (h > 0) return `${h}h ${m}m`;
    return `${m}m`;
  };
  return (
    <header class="hero">
      <Show when={backdrop()}>
        <div
          class="hero-backdrop discover-hero-backdrop"
          style={{ "background-image": `url(${backdrop()})` }}
        />
        <div class="hero-fade" />
      </Show>
      <div class="hero-meta">
        <h1>{props.item.title}</h1>
        <Show when={props.item.tagline}>
          <div class="discover-tagline">{props.item.tagline}</div>
        </Show>
        <div class="meta-pills">
          <Show when={props.item.year}>
            <span class="pill">{props.item.year}</span>
          </Show>
          <Show when={props.item.contentRating}>
            <span class="pill">{props.item.contentRating}</span>
          </Show>
          <Show when={runtimeText()}>
            <span class="pill">{runtimeText()}</span>
          </Show>
          <Show when={props.item.imdbId}>
            <span class="pill pill-imdb">
              <span class="pill-imdb-label">IMDB</span>
              <span class="pill-imdb-value">
                {props.imdbRating?.imdbRating ?? "—"}
              </span>
            </span>
          </Show>
          <Show when={criticRT()}>
            {(r) => (
              <span class="pill pill-rt" title="Rotten Tomatoes — Critic">
                <span class="pill-rt-label">RT</span>
                <span class="pill-rt-value">
                  {formatRating((r() as DiscoverRating).value)}
                </span>
              </span>
            )}
          </Show>
          <Show when={audienceRT()}>
            {(r) => (
              <span class="pill pill-rt-audience" title="Rotten Tomatoes — Audience">
                <span class="pill-rt-label">Audience</span>
                <span class="pill-rt-value">
                  {formatRating((r() as DiscoverRating).value)}
                </span>
              </span>
            )}
          </Show>
        </div>
        <Show when={(props.item.genres?.length ?? 0) > 0}>
          <div class="discover-genres">{props.item.genres!.join(" · ")}</div>
        </Show>
      </div>
    </header>
  );
}

// Rating values arrive as 0-10 (RT critic shows percentage / 10 in
// captures: 4.3 = 43%, 8.9 = 89%). Render as a percentage so the SPA
// matches Plex Web's pill display.
function formatRating(v: number): string {
  if (v <= 0) return "—";
  return `${Math.round(v * 10)}%`;
}

function formatBytes(n: number): string {
  if (!n) return "";
  const units = ["B", "KB", "MB", "GB", "TB"];
  let u = 0;
  let v = n;
  while (v >= 1024 && u < units.length - 1) {
    v /= 1024;
    u++;
  }
  return `${v.toFixed(1)} ${units[u]}`;
}

function CastCrew(props: { item: DiscoverItem }) {
  const cast = () => props.item.cast ?? [];
  const directors = () => props.item.directors ?? [];
  const writers = () => props.item.writers ?? [];
  const hasCrew = () => directors().length > 0 || writers().length > 0;
  return (
    <Show when={cast().length > 0 || hasCrew()}>
      <section class="cast-crew">
        <Show when={cast().length > 0}>
          <h3>Cast</h3>
          <ul class="people-grid">
            <For each={cast()}>{(p) => <PersonCard person={p} />}</For>
          </ul>
        </Show>
        <Show when={hasCrew()}>
          <h3>Crew</h3>
          <ul class="people-grid">
            <For each={directors()}>
              {(p) => <PersonCard person={{ ...p, tag: p.tag || "Director" }} />}
            </For>
            <For each={writers()}>
              {(p) => <PersonCard person={{ ...p, tag: p.tag || "Writer" }} />}
            </For>
          </ul>
        </Show>
      </section>
    </Show>
  );
}

function PersonCard(props: { person: Person }) {
  const fallbackThumb =
    "data:image/svg+xml;base64,PHN2ZyB4bWxucz0iaHR0cDovL3d3dy53My5vcmcvMjAwMC9zdmciIHZpZXdCb3g9IjAgMCAyNCAyNCIgZmlsbD0ibm9uZSIgc3Ryb2tlPSIjNjY2IiBzdHJva2Utd2lkdGg9IjEuNSI+PGNpcmNsZSBjeD0iMTIiIGN5PSI4IiByPSI0Ii8+PHBhdGggZD0iTTQgMjBjMC00IDQtNyA4LTdzOCAzIDggNyIvPjwvc3ZnPg==";
  // plex.tv person thumbs are absolute URLs (https://metadata-static.plex.tv/...);
  // direct render with referrerpolicy="no-referrer" — same as DiscoverTile poster.
  const src = () => props.person.thumb || fallbackThumb;
  return (
    <li class="person-card">
      <img
        src={src()}
        alt={props.person.name}
        class="person-thumb"
        referrerpolicy="no-referrer"
        onError={(e) => {
          (e.currentTarget as HTMLImageElement).src = fallbackThumb;
        }}
      />
      <div class="person-name">{props.person.name}</div>
      <Show when={props.person.tag}>
        <div class="person-tag">{props.person.tag}</div>
      </Show>
    </li>
  );
}
