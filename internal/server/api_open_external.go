package server

import (
	"encoding/json"
	"net/http"
	"net/url"

	"github.com/pkg/browser"
)

// handleOpenExternal hands a URL to the user's real default browser.
//
// In a browser tab, target="_blank" opens a tab. Inside WebView2 the same
// markup raises NewWindowRequested, and with no host handler attached the
// runtime answers it by spawning a second, chrome-less WebView2 window — an
// IMDB page in a bare frame with no address bar, no back button and no
// bookmarks. That is not a browser, and it is not where an external link
// should land.
//
// Handling the event at its source would mean reaching ICoreWebView2's
// add_NewWindowRequested, which go-webview2 neither wraps nor exposes: it
// keeps its edge.Chromium handle unexported and ships no event-handler
// binding for it. So the interception happens in the SPA (see
// web/src/util/externalLinks.ts) and terminates here.
func (s *Server) handleOpenExternal(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	var req struct {
		URL string `json:"url"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON body")
		return
	}

	// Scheme allow-list. originGuard already turns away other origins, but
	// this endpoint is a launcher — it asks Windows to run whatever is
	// registered for the URL's scheme. Without the check, anything able to
	// reach it could start a file:// open or fire a registered protocol
	// handler. Only the two schemes a web link can legitimately carry pass.
	u, err := url.Parse(req.URL)
	if err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Host == "" {
		writeError(w, http.StatusBadRequest, "only http and https URLs can be opened")
		return
	}

	if err := browser.OpenURL(u.String()); err != nil {
		writeError(w, http.StatusInternalServerError, "could not open your browser: "+err.Error())
		return
	}
	writeJSON(w, map[string]string{"status": "opened"})
}
