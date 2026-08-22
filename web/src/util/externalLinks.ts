import { api } from "../api/client";

/**
 * External-link routing for the desktop shell.
 *
 * Lumen renders inside WebView2, which is a browser engine but not a browser:
 * no tabs, no address bar, no back button. An anchor carrying target="_blank"
 * therefore does not open a tab — WebView2 raises NewWindowRequested and,
 * with nothing handling it, answers by spawning a second bare WebView2 window.
 * An IMDB page in a frame with no chrome is worse than useless.
 *
 * The clean fix would be to handle that event on the Go side, but go-webview2
 * keeps its edge.Chromium handle unexported and ships no binding for
 * add_NewWindowRequested, so the interception lives here instead and
 * terminates at POST /api/open-external.
 *
 * Delegated once at the document level rather than wired per-anchor: there are
 * already a dozen external links across Settings, About, the auth modal and
 * the IMDB pills, and any added later inherit the behaviour for free.
 */

/** Returns the absolute URL when `raw` leaves Lumen's own origin, else null. */
function externalURL(raw: string): string | null {
  try {
    const u = new URL(raw, window.location.href);
    // Anything that isn't a plain web link — mailto:, file:, a protocol
    // handler — is left alone rather than handed to a launcher.
    if (u.protocol !== "http:" && u.protocol !== "https:") return null;
    if (u.origin === window.location.origin) return null;
    return u.href;
  } catch {
    return null;
  }
}

/**
 * Opens a URL in the user's default browser.
 *
 * On failure it falls back to navigating in place. That's a degraded outcome —
 * the user loses their spot in Lumen — but a dead link that does nothing at
 * all when clicked is worse, and reads as the app being broken.
 */
export function openExternal(raw: string): void {
  const href = externalURL(raw);
  if (!href) return;
  void api.openExternal(href).catch((e) => {
    console.error("open-external failed; falling back to in-app navigation:", e);
    window.location.href = href;
  });
}

/** Installs the document-level click interceptor. Call once, at boot. */
export function installExternalLinkHandler(): void {
  document.addEventListener(
    "click",
    (e) => {
      // Capture phase, so this runs before the router's own delegated handler
      // and before the browser starts the default navigation.
      if (e.defaultPrevented || e.button !== 0) return;
      const target = e.target;
      const anchor = target instanceof Element ? target.closest("a") : null;
      if (!anchor) return;
      // getAttribute, not .href: an SVG anchor's href is an
      // SVGAnimatedString, and the raw attribute is what we want to resolve.
      const raw = anchor.getAttribute("href");
      if (!raw) return;
      const url = externalURL(raw);
      if (!url) return;
      e.preventDefault();
      openExternal(url);
    },
    true,
  );
}
