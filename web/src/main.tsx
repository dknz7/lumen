import { render } from "solid-js/web";
import { Router, Route } from "@solidjs/router";
import App from "./App";
import Home from "./pages/Home";
import Library from "./pages/Library";
import ItemDetail from "./pages/ItemDetail";
import Placeholder from "./pages/Placeholder";
import "./theme.css";

render(() => (
  <Router root={App}>
    <Route path="/" component={Home} />
    <Route path="/library/:serverID/:libraryID" component={Library} />
    <Route path="/item/:serverID/:ratingKey" component={ItemDetail} />
    <Route path="/watchlist"   component={() => <Placeholder name="Watchlist"   session="Session 5" />} />
    <Route path="/recommended" component={() => <Placeholder name="Recommended" session="Session 5" />} />
    <Route path="/discover"    component={() => <Placeholder name="Discover"    session="Session 5" />} />
    <Route path="/settings"    component={() => <Placeholder name="Settings"    session="Session 3" />} />
  </Router>
), document.getElementById("root")!);
