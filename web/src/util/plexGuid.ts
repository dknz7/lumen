// extractPlexTvRatingKey pulls the discover-namespace ratingKey from a
// Plex GUID like "plex://movie/abc123" → "abc123". Returns "" for
// any non-plex:// guid (e.g. server-local "local://..." or unknown).
export function extractPlexTvRatingKey(guid: string | undefined | null): string {
  if (!guid) return "";
  const m = /^plex:\/\/(?:movie|show|episode|season)\/([\w-]+)$/.exec(guid);
  return m ? m[1] : "";
}
