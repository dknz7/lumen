import { createSignal, onCleanup, onMount, Show } from "solid-js";
import { useLocation, useNavigate } from "@solidjs/router";
import { ArrowLeft, Home, Maximize2, Minimize2, Search, Sparkles, X } from "./icons";
import { store as settingsStore } from "../state/settings";
import { api } from "../api/client";
import CloseConfirmModal from "./CloseConfirmModal";
import SearchFlydown from "./SearchFlydown";
import "./TopBar.css";

export default function TopBar() {
  const navigate = useNavigate();
  const location = useLocation();
  const [query, setQuery] = createSignal("");
  const [closeOpen, setCloseOpen] = createSignal(false);
  const [flydownOpen, setFlydownOpen] = createSignal(false);
  let searchFormRef: HTMLFormElement | undefined;

  // Click-outside the search form closes the flydown. Listener attaches
  // for the lifetime of TopBar (always mounted). Doesn't interfere with
  // clicks INSIDE the form (input, flydown rows) — those propagate normally.
  function onDocumentClick(e: MouseEvent) {
    if (!flydownOpen()) return;
    if (!searchFormRef) return;
    if (!searchFormRef.contains(e.target as Node)) {
      setFlydownOpen(false);
    }
  }
  document.addEventListener("mousedown", onDocumentClick);
  onCleanup(() => document.removeEventListener("mousedown", onDocumentClick));

  // Zoom is global (state/settings.ts sets --card-width on :root), so cards
  // across all pages respond to the slider. Top bar / left menu don't use
  // --card-width and stay at fixed sizes regardless. The slider is hidden on
  // routes without card grids (Item Detail) — show it on Home, Watchlist,
  // Recommended, Discover, and Library.
  const zoom = () => settingsStore.settings()?.zoom ?? 100;
  const showZoom = () => {
    const p = location.pathname;
    return p === "/"
      || p === "/watchlist"
      || p === "/recommended"
      || p === "/discover"
      || p.startsWith("/library/");
  };

  function onSearch(e: SubmitEvent) {
    e.preventDefault();
    const q = query().trim();
    if (q.length < 2) return;
    // Enter navigates to the full results page and closes the flydown.
    setFlydownOpen(false);
    navigate(`/search?q=${encodeURIComponent(q)}`);
  }

  function onSearchInput(e: InputEvent & { currentTarget: HTMLInputElement }) {
    const val = e.currentTarget.value;
    setQuery(val);
    // Open the flydown as soon as the query crosses the min-length threshold.
    // SearchFlydown handles its own debounce internally before firing XHRs.
    setFlydownOpen(val.trim().length >= 2);
  }

  function onSearchFocus() {
    if (query().trim().length >= 2) setFlydownOpen(true);
  }

  // Browser fullscreen toggle — same effect as the user pressing F11.
  // Tracks state via the fullscreenchange event so the icon + label flip
  // even if the user exits via Esc / F11 / dev-tools instead of the button.
  const [isFullscreen, setIsFullscreen] = createSignal(false);
  onMount(() => {
    const sync = () => setIsFullscreen(!!document.fullscreenElement);
    sync();
    document.addEventListener("fullscreenchange", sync);
    onCleanup(() => document.removeEventListener("fullscreenchange", sync));
  });

  async function toggleFullscreen() {
    try {
      if (document.fullscreenElement) {
        await document.exitFullscreen();
      } else {
        await document.documentElement.requestFullscreen();
      }
    } catch (err) {
      // Browser denied (e.g. not triggered by a user gesture, or unsupported).
      // Silently log; the icon state stays in sync via fullscreenchange.
      console.warn("Fullscreen toggle failed:", err);
    }
  }

  function applyZoom(v: number) {
    settingsStore.patch({ zoom: v });
  }

  function confirmClose() {
    // Don't await — server tears down right after responding so the fetch
    // promise may not resolve cleanly. Fire-and-forget is the right shape.
    api.quit().catch(() => {});
    setCloseOpen(false);
    // Brief delay lets the modal exit animation play before the tab vanishes.
    setTimeout(() => window.close(), 200);
  }

  return (
    <header class="top-bar">
      <div class="top-bar-pill">
        <div class="tb-group tb-brand">
          <span class="logo"><Sparkles size={18} /></span>
          <span class="wordmark">Lumen</span>
        </div>
        <div class="tb-divider" />
        <div class="tb-group tb-nav">
          <button class="icon-btn" title="Back" aria-label="Back" onClick={() => navigate(-1)}>
            <ArrowLeft size={16} />
          </button>
          <button class="icon-btn" title="Home" aria-label="Home" onClick={() => navigate("/")}>
            <Home size={16} />
          </button>
        </div>
        <div class="tb-divider" />
        <form class="tb-search" onSubmit={onSearch} ref={searchFormRef}>
          <input
            type="search"
            placeholder="Search across servers and Discover..."
            value={query()}
            onInput={onSearchInput}
            onFocus={onSearchFocus}
            aria-label="Search"
            autocomplete="off"
          />
          <Show when={flydownOpen()}>
            <SearchFlydown query={query()} onClose={() => setFlydownOpen(false)} />
          </Show>
        </form>
        <div class="tb-divider" />
        <div class="tb-group tb-fullscreen">
          <button
            class="icon-btn"
            title={isFullscreen() ? "Exit fullscreen" : "Enter fullscreen"}
            aria-label={isFullscreen() ? "Exit fullscreen" : "Enter fullscreen"}
            onClick={toggleFullscreen}
          >
            <Show when={isFullscreen()} fallback={<Maximize2 size={16} />}>
              <Minimize2 size={16} />
            </Show>
          </button>
        </div>
        <Show when={showZoom()}>
          <div class="tb-divider" />
          <div class="tb-group tb-zoom">
            <span class="zoom-icon" aria-hidden="true"><Search size={12} /></span>
            <input
              type="range"
              min="80"
              max="150"
              value={zoom()}
              class="zoom-slider"
              title={`Card zoom: ${zoom()}%`}
              onInput={(e) => applyZoom(Number(e.currentTarget.value))}
            />
          </div>
        </Show>
        <div class="tb-divider" />
        <div class="tb-group tb-close">
          <button class="icon-btn" title="Close Lumen" aria-label="Close" onClick={() => setCloseOpen(true)}>
            <X size={16} />
          </button>
        </div>
      </div>
      <CloseConfirmModal
        open={closeOpen()}
        onCancel={() => setCloseOpen(false)}
        onConfirm={confirmClose}
      />
    </header>
  );
}
