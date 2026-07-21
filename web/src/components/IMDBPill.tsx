import { createResource, Show } from "solid-js";
import { api } from "../api/client";
import type { OMDBRating } from "../api/types";

// The one IMDB pill, shared by ItemDetail and DiscoverItem heroes. Fetches
// its own OMDB rating; renders as a link to imdb.com when an imdbId is
// present, and a plain non-link span when not (no imdbId → nothing to link
// to). The link never depends on the rating fetch — unrated/unreleased
// titles show "—" but still link.
export default function IMDBPill(props: { imdbId?: string }) {
  const [rating] = createResource(
    () => props.imdbId,
    async (id) => (id ? api.imdb(id) : null)
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
