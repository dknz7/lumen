import type { Library, Server } from "../api/types";

// Home layout derivation.
//
// Home's shelves used to be compile-time constants naming the original
// author's own servers and library titles. On any other installation every
// shelf resolved to a "not found in servers" stub, which made the app's main
// page useless to everyone except its author.
//
// The layout is now derived from what the backend actually reports: a group
// per server, a Recently Added shelf per library. Ordering, hiding and
// collapsing still persist per user through the existing shelfState machinery,
// keyed by the ids generated here.
//
// Ids are built from machineIdentifier + library key rather than titles, so
// renaming a server or a library in Plex doesn't orphan a user's saved layout.

export type ShelfDef =
  | {
      kind: "server-recent";
      id: string;
      title: string;
      serverID: string;
      libraryKey: string;
      libraryTitle: string;
      libraryType: string;
    }
  | {
      kind: "server-collection";
      id: string;
      title: string;
      serverID: string;
      libraryKey: string;
      libraryTitle: string;
      collectionTitle: string;
    };

export interface GroupDef {
  /** Stable id — the server's machine identifier. */
  id: string;
  /** Display name, from the user's local override or the server's own name. */
  title: string;
  shelves: ShelfDef[];
}

/**
 * shelfTitle picks a label that reads naturally for the library type.
 *
 * Plex's own library titles are already descriptive ("Movies", "TV Shows",
 * "Anime", "Movies - 4K UHD"), so the library name carries the meaning and the
 * shelf just needs a verb.
 */
function shelfTitle(lib: Library): string {
  return `Recently Added — ${lib.title}`;
}

export function shelfIDFor(serverID: string, libraryKey: string): string {
  return `recent:${serverID}:${libraryKey}`;
}

export function collectionShelfIDFor(
  serverID: string,
  libraryKey: string,
  collectionTitle: string,
): string {
  return `collection:${serverID}:${libraryKey}:${collectionTitle}`;
}

/**
 * Parses one persisted "serverID:libraryKey:collection title" selection.
 *
 * Only the first two colons are separators. Collection titles routinely
 * contain one ("Marvel: Phase One"), and a naive split would truncate them —
 * so the remainder after the second colon is taken whole.
 */
export function parseCollectionSelection(
  entry: string,
): { serverID: string; libraryKey: string; collectionTitle: string } | null {
  const first = entry.indexOf(":");
  if (first < 1) return null;
  const second = entry.indexOf(":", first + 1);
  if (second < 0 || second === first + 1) return null;
  const collectionTitle = entry.slice(second + 1);
  if (!collectionTitle) return null;
  return {
    serverID: entry.slice(0, first),
    libraryKey: entry.slice(first + 1, second),
    collectionTitle,
  };
}

/**
 * deriveHomeLayout builds the default Home layout.
 *
 * Libraries the user has hidden are omitted entirely rather than rendered as
 * "(library hidden)" stubs — hiding a library in the left menu should remove
 * it from Home, not leave a placeholder behind.
 */
export function deriveHomeLayout(
  servers: Server[],
  librariesByServer: Record<string, Library[]>,
  hiddenLibraries: Set<string>,
  collectionShelves: string[] = [],
): GroupDef[] {
  return servers.map((srv) => {
    const libs = librariesByServer[srv.machineIdentifier] ?? [];
    const visible = libs
      .filter((lib) => !hiddenLibraries.has(`${srv.machineIdentifier}:${lib.key}`))
      // Only the library types Lumen can render as cards. A photo or music
      // library would produce an empty shelf.
      .filter((lib) => lib.type === "movie" || lib.type === "show");

    const shelves: ShelfDef[] = visible.map((lib) => ({
      kind: "server-recent" as const,
      id: shelfIDFor(srv.machineIdentifier, lib.key),
      title: shelfTitle(lib),
      serverID: srv.machineIdentifier,
      libraryKey: lib.key,
      libraryTitle: lib.title,
      libraryType: lib.type,
    }));

    // Collection shelves the user has opted into, in the order they were
    // saved. Unlike Recently Added there is no derivable rule for which of a
    // server's collections deserve a shelf — one library here has thirty — so
    // they are an explicit choice made in Settings rather than something
    // inferred. Selections naming a library that is hidden, gone, or not a
    // video library are skipped rather than rendered as stubs.
    const byKey = new Map(visible.map((lib) => [lib.key, lib]));
    for (const entry of collectionShelves) {
      const sel = parseCollectionSelection(entry);
      if (!sel || sel.serverID !== srv.machineIdentifier) continue;
      const lib = byKey.get(sel.libraryKey);
      if (!lib) continue;
      shelves.push({
        kind: "server-collection",
        id: collectionShelfIDFor(sel.serverID, sel.libraryKey, sel.collectionTitle),
        title: sel.collectionTitle,
        serverID: sel.serverID,
        libraryKey: sel.libraryKey,
        libraryTitle: lib.title,
        collectionTitle: sel.collectionTitle,
      });
    }

    return {
      id: srv.machineIdentifier,
      title: srv.displayName || srv.name,
      shelves,
    };
  });
}

/**
 * applyPersistedOrder reconciles a saved order with the ids that currently
 * exist.
 *
 * Saved ids that no longer resolve are dropped (a library was removed), and
 * ids that are new are appended (a library was added) — so a user's arrangement
 * survives their Plex server changing underneath it instead of being reset.
 */
export function applyPersistedOrder(current: string[], persisted?: string[]): string[] {
  if (!persisted || persisted.length === 0) return current;
  const valid = persisted.filter((id) => current.includes(id));
  const missing = current.filter((id) => !valid.includes(id));
  return [...valid, ...missing];
}
