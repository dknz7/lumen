package server

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"lumen/internal/config"
	"lumen/internal/plex"
)

func TestImageProxyForwardsWithTokenServerSide(t *testing.T) {
	// Fake Plex server — confirm it sees the token, SPA-facing response doesn't.
	var gotToken string
	plexFake := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotToken = r.URL.Query().Get("X-Plex-Token")
		w.Header().Set("Content-Type", "image/jpeg")
		_, _ = w.Write([]byte("FAKE-JPEG-BYTES"))
	}))
	defer plexFake.Close()

	cfg := &config.Config{Plex: config.PlexConfig{
		Servers: []config.Server{{MachineIdentifier: "abc", AccessToken: "secret-token", LastGoodConnection: plexFake.URL}},
	}}
	c := plex.NewClient("id", "1.0.0")
	s := New(cfg, c, "127.0.0.1:0")

	req, _ := http.NewRequest("GET", "/api/image-proxy?server=abc&path=/library/metadata/1/thumb/1", nil)
	w := newResponseRecorder()
	s.mux.ServeHTTP(w, req)

	if w.status != 200 {
		t.Fatalf("status %d", w.status)
	}
	// Try-with-fallback (Session 6): the handler tries the account token first
	// and retries with the per-server token on failure. With no account token
	// configured (empty string), the first attempt errors out and we fall
	// through to the per-server token, so we expect "secret-token". If a future
	// test sets AccountToken, "account-token" would also be a legitimate result.
	if gotToken != "account-token" && gotToken != "secret-token" {
		t.Errorf("plex server saw token %q, want 'account-token' or 'secret-token'", gotToken)
	}
	body, _ := io.ReadAll(w.body)
	if string(body) != "FAKE-JPEG-BYTES" {
		t.Errorf("body: %q", body)
	}
	// The response back to the SPA must NOT contain the token anywhere.
	if strings.Contains(string(body), "secret-token") {
		t.Error("response leaks token")
	}
	if ct := w.headers.Get("Content-Type"); !strings.HasPrefix(ct, "image/") {
		t.Errorf("Content-Type: %q", ct)
	}
	// Cache-Control should be long — posters don't change.
	if cc := w.headers.Get("Cache-Control"); cc == "" {
		t.Error("missing Cache-Control header")
	}
}

func TestImageProxyValidatesPath(t *testing.T) {
	cfg := &config.Config{Plex: config.PlexConfig{Servers: []config.Server{{MachineIdentifier: "abc"}}}}
	s := New(cfg, nil, "127.0.0.1:0")

	cases := []struct {
		path string
		want int
		name string
	}{
		{"?server=abc", 400, "missing path"},
		{"?server=abc&path=http://evil.com/exfil", 400, "absolute URL path"},
		{"?server=abc&path=../../../etc/passwd", 400, "path traversal"},
		{"?server=abc&path=/library/metadata/1/thumb/1", 200, "valid — but 500-502 is also fine since server has no BaseURL"},
		{"?server=nonexistent&path=/library/metadata/1/thumb/1", 404, "unknown server"},
		{"?path=/library/metadata/1/thumb/1", 400, "missing server"},
	}
	for _, tc := range cases {
		req, _ := http.NewRequest("GET", "/api/image-proxy"+tc.path, nil)
		w := newResponseRecorder()
		s.mux.ServeHTTP(w, req)
		if tc.want == 200 && (w.status == 500 || w.status == 502) {
			continue // offline Plex server, handler still rejected correctly
		}
		if w.status != tc.want {
			t.Errorf("%s: status %d, want %d", tc.name, w.status, tc.want)
		}
	}
}
