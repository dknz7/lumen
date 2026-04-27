// Body font — Saira (geometric, multi-weight, multi-purpose)
import "@fontsource/saira/400.css";
import "@fontsource/saira/500.css";
import "@fontsource/saira/600.css";
import "@fontsource/saira/700.css";
// Headline font — Rajdhani (Agency-FB-flavoured, used for titles + wordmark)
import "@fontsource/rajdhani/500.css";
import "@fontsource/rajdhani/600.css";
import "@fontsource/rajdhani/700.css";
import { render } from "solid-js/web";
import { Router, Route } from "@solidjs/router";
import App from "./App";
import Home from "./pages/Home";
import Library from "./pages/Library";
import ItemDetail from "./pages/ItemDetail";
import DiscoverItem from "./pages/DiscoverItem";
import Placeholder from "./pages/Placeholder";
import Watchlist from "./pages/Watchlist";
import Recommended from "./pages/Recommended";
import Discover from "./pages/Discover";
import SearchResults from "./pages/SearchResults";
import { store as settingsStore } from "./state/settings";
import { playbackStore } from "./state/playback";
import "./theme.css";

// Fire-and-forget — settings load populates the store and applies theme.
// The UI renders defaults until the load resolves; no blocking splash.
settingsStore.load().catch((e) => console.error("initial settings load failed:", e));
playbackStore.connect();

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
    <Route path="/settings"    component={() => <Placeholder name="Settings"    session="opens modal instead" />} />
  </Router>
), document.getElementById("root")!);
