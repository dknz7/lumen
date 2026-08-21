import { createResource, createSignal, Show } from "solid-js";
import Section from "./Section";
import { toast, errorMessage } from "../Toast";

interface CacheSize {
  images: number;
  omdb: number;
  total: number;
}

function formatBytes(n: number): string {
  if (n === 0) return "0 B";
  const units = ["B", "KB", "MB", "GB"];
  let u = 0;
  let v = n;
  while (v >= 1024 && u < units.length - 1) { v /= 1024; u++; }
  return `${v.toFixed(u === 0 ? 0 : 1)} ${units[u]}`;
}

export default function DataCache() {
  const [size, { refetch }] = createResource<CacheSize>(async () => {
    const res = await fetch("/api/cache/size");
    if (!res.ok) throw new Error(`${res.status}`);
    return res.json();
  });
  const [clearingScope, setClearingScope] = createSignal<string | null>(null);

  async function clear(scope: "images" | "omdb" | "all") {
    setClearingScope(scope);
    try {
      const res = await fetch(`/api/cache/clear?scope=${scope}`, { method: "POST" });
      if (!res.ok) throw new Error(`${res.status}`);
      refetch();
    } catch (e) {
      toast.error(`Couldn't clear the cache — ${errorMessage(e)}`);
    } finally {
      setClearingScope(null);
    }
  }

  return (
    <Section title="Data & Cache" description="Proxied images and OMDB metadata cache.">
      <Show
        when={!size.error}
        fallback={<p class="as-warn">Couldn't read cache sizes — {errorMessage(size.error)}</p>}
      >
      <Show when={size()} fallback={<p>Loading…</p>}>
        {(cs) => (
          <>
            <div class="settings-row">
              <label>Image cache</label>
              <div class="settings-control" style={{ "display": "flex", "gap": "12px", "align-items": "center" }}>
                <strong>{formatBytes((cs() as CacheSize).images)}</strong>
                <button class="settings-btn settings-btn-danger" disabled={clearingScope() === "images"} onClick={() => clear("images")}>
                  {clearingScope() === "images" ? "Clearing…" : "Clear"}
                </button>
              </div>
            </div>

            <div class="settings-row">
              <label>Metadata cache (OMDB)</label>
              <div class="settings-control" style={{ "display": "flex", "gap": "12px", "align-items": "center" }}>
                <strong>{formatBytes((cs() as CacheSize).omdb)}</strong>
                <button class="settings-btn settings-btn-danger" disabled={clearingScope() === "omdb"} onClick={() => clear("omdb")}>
                  {clearingScope() === "omdb" ? "Clearing…" : "Clear"}
                </button>
              </div>
            </div>

            <div class="settings-row">
              <label>All caches</label>
              <div class="settings-control" style={{ "display": "flex", "gap": "12px", "align-items": "center" }}>
                <strong>{formatBytes((cs() as CacheSize).total)}</strong>
                <button class="settings-btn settings-btn-danger" disabled={clearingScope() === "all"} onClick={() => clear("all")}>
                  {clearingScope() === "all" ? "Clearing…" : "Clear All"}
                </button>
              </div>
            </div>
          </>
        )}
      </Show>
      </Show>
    </Section>
  );
}
