package server

import (
	"net"
	"net/http"
	"strings"
)

// originGuard rejects state-changing requests that came from another origin.
//
// Lumen listens on loopback, which is often mistaken for "unreachable from the
// internet". It isn't: any page in the user's browser can send requests to
// http://127.0.0.1:7832. Several of Lumen's endpoints are CORS "simple
// requests" — no preflight, no opt-in needed — so before this guard existed,
// a page the user merely visited could:
//
//   - POST /api/quit             → shut Lumen down
//   - POST /api/cache/clear?...  → wipe the image and metadata cache
//   - POST /api/servers/x/rename → persist a change (JSON decoded without
//     checking Content-Type, so text/plain got through)
//
// It could never *read* the responses — no CORS headers are set, so the browser
// withholds them — meaning this is a state-change and denial-of-service issue,
// not data theft. Still worth closing.
//
// Reads pass through untouched: they change nothing and the browser blocks
// cross-origin reads of the response anyway.
func (s *Server) originGuard(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodGet, http.MethodHead, http.MethodOptions:
			next.ServeHTTP(w, r)
			return
		}

		// Sec-Fetch-Site is sent by every current browser and is the most
		// direct signal available. Non-browser callers omit it.
		switch r.Header.Get("Sec-Fetch-Site") {
		case "cross-site", "same-site":
			writeError(w, http.StatusForbidden, "cross-origin requests are not allowed")
			return
		}

		// An absent Origin means a non-browser client (curl, or Lumen's own
		// second-instance ping). Browsers always send it on a cross-origin
		// write, which is the case being defended against.
		origin := r.Header.Get("Origin")
		if origin != "" && !s.originAllowed(origin) {
			writeError(w, http.StatusForbidden, "cross-origin requests are not allowed")
			return
		}

		next.ServeHTTP(w, r)
	})
}

// originAllowed reports whether origin is Lumen's own. The listen port is
// configurable (--addr), so the port is taken from the bound address rather
// than hardcoded, and both loopback spellings are accepted because the user
// may type either into a browser.
func (s *Server) originAllowed(origin string) bool {
	want := s.listenPort()
	for _, prefix := range []string{"http://127.0.0.1:", "http://localhost:", "http://[::1]:"} {
		if strings.HasPrefix(origin, prefix) && strings.TrimPrefix(origin, prefix) == want {
			return true
		}
	}
	return false
}

// listenPort returns the port Lumen is actually bound to, falling back to the
// configured address before the listener exists.
func (s *Server) listenPort() string {
	// Via Addr(), which takes the lock: this runs on every request goroutine,
	// concurrently with Serve writing s.ln at startup.
	addr := s.Addr()
	if addr == "" {
		addr = s.http.Addr
	}
	if _, port, err := net.SplitHostPort(addr); err == nil {
		return port
	}
	return ""
}
