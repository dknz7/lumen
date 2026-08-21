import { A } from "@solidjs/router";
import { useLocation } from "@solidjs/router";
import "./NotFound.css";

// Replaces the dev Placeholder component, which rendered lines like
// "This page lands in opens modal instead. Current route: /settings" — visible
// to anyone who typed a URL by hand.
export default function NotFound() {
  const location = useLocation();
  return (
    <div class="notfound">
      <h1 class="notfound-title">Nothing here</h1>
      <p class="notfound-detail">
        <code>{location.pathname}</code> isn't a page in Lumen.
      </p>
      <A class="notfound-link" href="/">Back to Home</A>
    </div>
  );
}
