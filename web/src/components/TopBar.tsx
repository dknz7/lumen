import { createSignal, Show } from "solid-js";
import { useLocation, useNavigate } from "@solidjs/router";
import { ArrowLeft, Home, Maximize2, Search, Sparkles, X } from "./icons";
import { store as settingsStore } from "../state/settings";
import { api } from "../api/client";
import CloseConfirmModal from "./CloseConfirmModal";
import "./TopBar.css";

export default function TopBar() {
  const navigate = useNavigate();
  const location = useLocation();
  const [query, setQuery] = createSignal("");
  const [closeOpen, setCloseOpen] = createSignal(false);

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
    // Session 5 wires real search; Session 2 just logs.
    console.log("search:", query());
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
        <form class="tb-search" onSubmit={onSearch}>
          <input
            type="search"
            placeholder="Search across servers and Discover..."
            value={query()}
            onInput={(e) => setQuery(e.currentTarget.value)}
            aria-label="Search"
          />
        </form>
        <div class="tb-divider" />
        <div class="tb-group tb-kiosk">
          <button class="icon-btn" title="Kiosk mode (Session 5)" aria-label="Kiosk mode">
            <Maximize2 size={16} />
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
