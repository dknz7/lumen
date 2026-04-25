/**
 * Format an epoch-seconds timestamp as either a relative duration
 * (when < 24 hours ago) or an ISO date prefixed with "Added"
 * (when ≥ 24 hours ago).
 *
 * - "Just added" if <60 sec
 * - "N minutes ago" if <60 min
 * - "N hours ago" if <24 hr
 * - "Added YYYY-MM-DD" otherwise
 *
 * Returns empty string if epochSec is 0/undefined (no data).
 */
export function formatAddedTimestamp(epochSec: number | undefined): string {
  if (!epochSec) return "";
  const nowSec = Math.floor(Date.now() / 1000);
  const deltaSec = nowSec - epochSec;
  if (deltaSec < 60) return "Just added";
  if (deltaSec < 3600) {
    const m = Math.floor(deltaSec / 60);
    return `${m} minute${m === 1 ? "" : "s"} ago`;
  }
  if (deltaSec < 86400) {
    const h = Math.floor(deltaSec / 3600);
    return `${h} hour${h === 1 ? "" : "s"} ago`;
  }
  // ISO date — Plex's addedAt is epoch SECONDS (not ms)
  const d = new Date(epochSec * 1000);
  const yyyy = d.getFullYear();
  const mm = String(d.getMonth() + 1).padStart(2, "0");
  const dd = String(d.getDate()).padStart(2, "0");
  return `Added ${yyyy}-${mm}-${dd}`;
}
