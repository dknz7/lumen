// Fonts — latin subsets only.
//
// The unscoped imports pull every subset @fontsource ships: Devanagari,
// Vietnamese and latin-ext as well as latin, across both families. A browser
// only downloads the subsets it needs (they're split by unicode-range), so the
// runtime cost was nil — but all 44 files were embedded into lumen.exe,
// roughly 700 kB of binary for scripts Lumen's UI never renders. The three
// Rajdhani Devanagari faces alone were the largest assets in the build.
//
// Body font — Saira (geometric, multi-weight)
import "@fontsource/saira/latin-400.css";
import "@fontsource/saira/latin-500.css";
import "@fontsource/saira/latin-600.css";
import "@fontsource/saira/latin-700.css";
// Headline font — Rajdhani (used for titles and the wordmark)
import "@fontsource/rajdhani/latin-500.css";
import "@fontsource/rajdhani/latin-600.css";
import "@fontsource/rajdhani/latin-700.css";
import { render } from "solid-js/web";
import { Router, Route, useNavigate } from "@solidjs/router";
import App from "./App";
import Home from "./pages/Home";
import Library from "./pages/Library";
import ItemDetail from "./pages/ItemDetail";
import DiscoverItem from "./pages/DiscoverItem";
import NotFound from "./pages/NotFound";
import Watchlist from "./pages/Watchlist";
import Recommended from "./pages/Recommended";
import Discover from "./pages/Discover";
import SearchResults from "./pages/SearchResults";
import { store as settingsStore } from "./state/settings";
import { playbackStore } from "./state/playback";
import "./theme.css";

// Settings load populates the store and applies the theme. The UI renders
// defaults until it resolves; no blocking splash.
//
// It retries rather than giving up after one failure: when the load failed,
// settings() stayed null forever, which left Appearance and Playback showing
// "Loading…" permanently and made every patch() a silent no-op. A user with a
// slow first paint could end up with settings that simply never worked.
settingsStore.loadWithRetry();
playbackStore.connect();

// A route can't open App's modal directly, so bounce to Home — the Settings
// entry in the left menu is one click away and clearly labelled.
function SettingsRedirect() {
  const navigate = useNavigate();
  navigate("/", { replace: true });
  return null;
}

render(() => (
  <Router root={App}>
    <Route path="/" component={Home} />
    <Route path="/library/:serverID/:libraryID" component={Library} />
    <Route path="/item/:serverID/:ratingKey" component={ItemDetail} />
    <Route path="/discover-item/:ratingKey" component={DiscoverItem} />
    <Route path="/watchlist/:ratingKey" component={DiscoverItem} />
    <Route path="/watchlist" component={Watchlist} />
    <Route path="/recommended" component={Recommended} />
    <Route path="/discover" component={Discover} />
    <Route path="/search" component={SearchResults} />
    {/* Settings lives in a modal, but people type the URL. Send them Home;
        LeftMenu opens the modal. Previously this rendered a dev placeholder
        reading "This page lands in opens modal instead". */}
    <Route path="/settings" component={SettingsRedirect} />
    <Route path="*" component={NotFound} />
  </Router>
), document.getElementById("root")!);
