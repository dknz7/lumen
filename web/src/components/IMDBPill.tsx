import { createResource, Show } from "solid-js";
import { api } from "../api/client";
import type { OMDBRating } from "../api/types";

// The one IMDB pill, shared by ItemDetail and DiscoverItem heroes. Fetches
// its own OMDB rating; renders as a link to imdb.com when an imdbId is
// present, and a plain non-link span when not (no imdbId → nothing to link
// to). The link never depends on the rating fetch — unrated/unreleased
// titles show "—" but still link.
export default function IMDBPill(props: { imdbId?: string }) {
  // OMDB legitimately 404s for unreleased titles, and api.imdb throws on any
  // non-2xx. An unhandled rejection here would make reading rating() re-throw
  // and take the whole detail page down over a missing rating — so "no rating"
  // is caught and rendered as "—", which is what it means.
  const [rating] = createResource(
    () => props.imdbId,
    async (id) => {
      if (!id) return null;
      try {
        return await api.imdb(id);
      } catch {
        return null;
      }
    },
  );
  const value = () => (
    <Show when={rating()} fallback={<span class="pill-imdb-value">—</span>}>
      {(r) => <span class="pill-imdb-value">{(r() as OMDBRating).imdbRating ?? "—"}</span>}
    </Show>
  );
  return (
    <Show
      when={props.imdbId}
      fallback={
        <span class="pill pill-imdb">
          <span class="pill-imdb-label">IMDB</span>
          {value()}
        </span>
      }
    >
      <a
        class="pill pill-imdb pill-imdb-link"
        href={`https://www.imdb.com/title/${props.imdbId}/`}
        target="_blank"
        rel="noreferrer"
        title="Open on IMDB"
      >
        <span class="pill-imdb-label">IMDB</span>
        {value()}
      </a>
    </Show>
  );
}
