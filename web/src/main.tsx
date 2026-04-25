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
import Placeholder from "./pages/Placeholder";
import { store as settingsStore } from "./state/settings";
import "./theme.css";

// Fire-and-forget — settings load populates the store and applies theme.
// The UI renders defaults until the load resolves; no blocking splash.
settingsStore.load().catch((e) => console.error("initial settings load failed:", e));

render(() => (
  <Router root={App}>
    <Route path="/" component={Home} />
    <Route path="/library/:serverID/:libraryID" component={Library} />
    <Route path="/item/:serverID/:ratingKey" component={ItemDetail} />
    <Route path="/watchlist"   component={() => <Placeholder name="Watchlist"   session="Session 5" />} />
    <Route path="/recommended" component={() => <Placeholder name="Recommended" session="Session 5" />} />
    <Route path="/discover"    component={() => <Placeholder name="Discover"    session="Session 5" />} />
    <Route path="/settings"    component={() => <Placeholder name="Settings"    session="opens modal instead" />} />
  </Router>
), document.getElementById("root")!);
