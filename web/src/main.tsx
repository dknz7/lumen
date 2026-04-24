import { render } from "solid-js/web";
import { Router, Route } from "@solidjs/router";
import App from "./App";
import Home from "./pages/Home";
import Library from "./pages/Library";
import ItemDetail from "./pages/ItemDetail";
import "./theme.css";

render(() => (
  <Router root={App}>
    <Route path="/" component={Home} />
    <Route path="/library/:serverID/:libraryID" component={Library} />
    <Route path="/item/:serverID/:ratingKey" component={ItemDetail} />
  </Router>
), document.getElementById("root")!);
