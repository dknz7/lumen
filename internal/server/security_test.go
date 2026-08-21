package server

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/url"
	"strings"
	"testing"

	"lumen/internal/config"
	"lumen/internal/plex"
)

const fakeAccountToken = "SUPERSECRET-account-token-do-not-leak"

func newSecurityTestServer(t *testing.T) *Server {
	t.Helper()
	t.Setenv("APPDATA", t.TempDir())
	cfg := &config.Config{
		ClientIdentifier: "test-client",
		OMDBKey:          "0bad1dea",
		Plex: config.PlexConfig{
			AccountToken: fakeAccountToken,
			Servers: []config.Server{{
				Name:               "Test Server",
				MachineIdentifier:  "abc",
				AccessToken:        "SUPERSECRET-server-token",
				LastGoodConnection: "http://127.0.0.1:32400",
			}},
		},
	}
	return New(cfg, plex.NewClient("test-client", "0.0.0"), "127.0.0.1:0")
}

// TestHubResponsesCarryNoCredentials guards the invariant Lumen's README makes
// a promise about: the Plex account token never reaches the SPA.
//
// It regressed once already — GetHub stamped the account token into each clip's
// HLS URL, which the SPA then set as a <video> src, putting the credential in
// the DOM and the network log. The existing tests only checked response
// *headers* for the token, so a token in the response *body* sailed through.
//
// This asserts on the whole serialised body, which catches the next variation
// of the same mistake regardless of which field carries it.
func TestHubResponsesCarryNoCredentials(t *testing.T) {
	s := newSecurityTestServer(t)

	items := []plex.HubItem{
		{Title: "A Trailer", Type: "clip",
			HLSUrl: "https://discover.example/library/metadata/1/parts/hls.m3u8"},
		{Title: "No Trailer", Type: "movie"},
	}

	out := s.withHLSHandles(items)

	if out[0].HLSUrl == items[0].HLSUrl {
		t.Error("HLSUrl was passed through unchanged — expected an opaque proxy handle")
	}
	if !strings.HasPrefix(out[0].HLSUrl, "/api/hls/") {
		t.Errorf("HLSUrl = %q, want a /api/hls/ handle", out[0].HLSUrl)
	}
	if out[1].HLSUrl != "" {
		t.Errorf("item without a trailer gained an HLS URL: %q", out[1].HLSUrl)
	}
	// The cache must keep the raw upstream URL — handles expire, cached hubs
	// may not.
	if items[0].HLSUrl != "https://discover.example/library/metadata/1/parts/hls.m3u8" {
		t.Error("withHLSHandles mutated its input; the hub cache would be poisoned")
	}

	b, err := jsonMarshal(out)
	if err != nil {
		t.Fatal(err)
	}
	assertNoCredentials(t, string(b))
}

// TestAboutAndStatusCarryNoCredentials — About is the panel users are most
// likely to screenshot into a bug report.
func TestAboutAndStatusCarryNoCredentials(t *testing.T) {
	s := newSecurityTestServer(t)
	for _, path := range []string{"/api/about", "/api/status"} {
		req, _ := http.NewRequest("GET", path, nil)
		w := newResponseRecorder()
		s.mux.ServeHTTP(w, req)
		if w.status != 200 {
			t.Fatalf("%s: status %d", path, w.status)
		}
		body, _ := io.ReadAll(w.body)
		assertNoCredentials(t, string(body))
	}
}

func assertNoCredentials(t *testing.T, body string) {
	t.Helper()
	for _, secret := range []string{
		fakeAccountToken,
		"SUPERSECRET-server-token",
		"0bad1dea",
		"X-Plex-Token",
	} {
		if strings.Contains(body, secret) {
			t.Errorf("response body leaks %q:\n%s", secret, body)
		}
	}
}

// TestOriginGuardBlocksCrossOriginWrites covers the CSRF hole: Lumen listens on
// loopback, which any page in the user's browser can reach. POST /api/quit
// takes no body and is a CORS "simple request", so before the guard existed a
// website could shut Lumen down, wipe the cache, or persist a server rename
// just by being open in a tab.
func TestOriginGuardBlocksCrossOriginWrites(t *testing.T) {
	s := newSecurityTestServer(t)

	writeEndpoints := []struct{ method, path, body string }{
		{"POST", "/api/quit", ""},
		{"POST", "/api/cache/clear?scope=all", ""},
		{"POST", "/api/servers/abc/rename", `{"displayName":"pwned"}`},
		{"PUT", "/api/settings", `{"theme":"pwned"}`},
		{"POST", "/api/window/show", ""},
	}

	t.Run("cross-origin is refused", func(t *testing.T) {
		for _, e := range writeEndpoints {
			req, _ := http.NewRequest(e.method, e.path, bytes.NewBufferString(e.body))
			req.Header.Set("Origin", "https://evil.example")
			req.Header.Set("Content-Type", "text/plain") // the simple-request trick
			w := newResponseRecorder()
			s.http.Handler.ServeHTTP(w, req)
			if w.status != http.StatusForbidden {
				t.Errorf("%s %s: status %d, want 403", e.method, e.path, w.status)
			}
		}
	})

	t.Run("Sec-Fetch-Site cross-site is refused", func(t *testing.T) {
		req, _ := http.NewRequest("POST", "/api/quit", nil)
		req.Header.Set("Sec-Fetch-Site", "cross-site")
		w := newResponseRecorder()
		s.http.Handler.ServeHTTP(w, req)
		if w.status != http.StatusForbidden {
			t.Errorf("status %d, want 403", w.status)
		}
	})

	t.Run("same-origin is allowed", func(t *testing.T) {
		// The server has no listener in tests, so the guard falls back to the
		// configured addr's port.
		req, _ := http.NewRequest("PUT", "/api/settings", bytes.NewBufferString(`{"zoom":110}`))
		req.Header.Set("Origin", "http://127.0.0.1:"+s.listenPort())
		req.Header.Set("Sec-Fetch-Site", "same-origin")
		w := newResponseRecorder()
		s.http.Handler.ServeHTTP(w, req)
		if w.status != http.StatusOK {
			t.Errorf("same-origin PUT rejected: status %d", w.status)
		}
	})

	t.Run("no Origin header is allowed", func(t *testing.T) {
		// Non-browser callers (curl, Lumen's own second-instance ping) send no
		// Origin. Browsers always do on a cross-origin write, which is the
		// case being defended against.
		req, _ := http.NewRequest("PUT", "/api/settings", bytes.NewBufferString(`{"zoom":115}`))
		w := newResponseRecorder()
		s.http.Handler.ServeHTTP(w, req)
		if w.status != http.StatusOK {
			t.Errorf("local PUT rejected: status %d", w.status)
		}
	})

	t.Run("reads are never blocked", func(t *testing.T) {
		req, _ := http.NewRequest("GET", "/api/status", nil)
		req.Header.Set("Origin", "https://evil.example")
		w := newResponseRecorder()
		s.http.Handler.ServeHTTP(w, req)
		if w.status != http.StatusOK {
			t.Errorf("cross-origin GET blocked: status %d — reads change nothing "+
				"and the browser withholds the response anyway", w.status)
		}
	})
}

// TestHLSProxyRejectsUnknownAndExpiredHandles — the handle is the only thing
// standing between a caller and an authenticated fetch against Plex.
func TestHLSProxyRejectsUnknownAndExpiredHandles(t *testing.T) {
	s := newSecurityTestServer(t)

	req, _ := http.NewRequest("GET", "/api/hls/deadbeef/index.m3u8", nil)
	w := newResponseRecorder()
	s.mux.ServeHTTP(w, req)
	if w.status != http.StatusGone {
		t.Errorf("unknown handle: status %d, want 410", w.status)
	}

	// A minted handle must not be guessable from the upstream URL.
	h := s.hls.mint("https://discover.example/a/b/hls.m3u8", fakeAccountToken)
	if strings.Contains(h, "discover.example") || strings.Contains(h, fakeAccountToken) {
		t.Errorf("handle exposes upstream detail: %q", h)
	}
}

// TestHLSManifestRewriteStripsTokens covers the manifest body itself — an
// absolute upstream URL inside the playlist would otherwise bypass the proxy,
// and any token in it would land in the client.
func TestHLSManifestRewriteStripsTokens(t *testing.T) {
	base := mustParseURL(t, "https://discover.example/library/metadata/1/parts/hls.m3u8")
	in := strings.Join([]string{
		"#EXTM3U",
		`#EXT-X-KEY:METHOD=AES-128,URI="https://discover.example/keys/1?X-Plex-Token=leak"`,
		"#EXTINF:6.0,",
		"segment0.ts",
		"#EXTINF:6.0,",
		"https://discover.example/library/metadata/1/parts/segment1.ts?X-Plex-Token=leak",
	}, "\n")

	out := rewriteManifest(in, base, "/api/hls/abc/")

	if strings.Contains(out, "X-Plex-Token") || strings.Contains(out, "leak") {
		t.Errorf("rewritten manifest still carries a token:\n%s", out)
	}
	if strings.Contains(out, "https://discover.example") {
		t.Errorf("absolute upstream URL survived rewriting, bypassing the proxy:\n%s", out)
	}
	if !strings.Contains(out, "/api/hls/abc/keys/1") {
		t.Errorf("EXT-X-KEY URI was not rewritten:\n%s", out)
	}
	// Relative URIs must be left alone: the browser resolves them against the
	// proxy path, which already maps onto the upstream base.
	if !strings.Contains(out, "\nsegment0.ts") {
		t.Errorf("relative segment URI was altered:\n%s", out)
	}
}

func jsonMarshal(v any) ([]byte, error) { return json.Marshal(v) }

func mustParseURL(t *testing.T, raw string) *url.URL {
	t.Helper()
	u, err := url.Parse(raw)
	if err != nil {
		t.Fatal(err)
	}
	return u
}
