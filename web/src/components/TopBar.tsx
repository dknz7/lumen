import { createSignal } from "solid-js";
import { useNavigate } from "@solidjs/router";
import { ArrowLeft, Home, Maximize2, Search, Sparkles, X } from "./icons";
import "./TopBar.css";

export default function TopBar() {
  const navigate = useNavigate();
  const [query, setQuery] = createSignal("");
  const [zoom, setZoom] = createSignal(100);

  function onSearch(e: SubmitEvent) {
    e.preventDefault();
    // Session 5 wires real search; Session 2 just logs.
    console.log("search:", query());
  }

  function applyZoom(v: number) {
    setZoom(v);
    // CSS zoom on :root scales the whole viewport. Session 3 persists this.
    document.documentElement.style.setProperty("zoom", String(v / 100));
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
        <div class="tb-group tb-zoom">
          <button class="icon-btn" title="Kiosk mode (Session 5)" aria-label="Kiosk mode">
            <Maximize2 size={16} />
          </button>
          <span class="zoom-icon" aria-hidden="true"><Search size={12} /></span>
          <input
            type="range"
            min="80"
            max="150"
            value={zoom()}
            class="zoom-slider"
            title={`Viewport zoom: ${zoom()}%`}
            onInput={(e) => applyZoom(Number(e.currentTarget.value))}
          />
        </div>
        <div class="tb-divider" />
        <div class="tb-group tb-close">
          <button class="icon-btn" title="Close Lumen" aria-label="Close" onClick={() => window.close()}>
            <X size={16} />
          </button>
        </div>
      </div>
    </header>
  );
}
