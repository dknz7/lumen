package server

import (
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"testing"

	"lumen/internal/config"
)

func TestAPIServersReturnsConfiguredServers(t *testing.T) {
	cfg := &config.Config{
		ClientIdentifier: "x",
		Plex: config.PlexConfig{
			Servers: []config.Server{
				{Name: "Stargaze", MachineIdentifier: "abc", LastGoodConnection: "https://a.example"},
				{Name: "", MachineIdentifier: "def", LastGoodConnection: "https://b.example"}, // empty name → falls through
			},
		},
	}
	s := New(cfg, nil, "127.0.0.1:0")
	req, _ := http.NewRequest("GET", "/api/servers", nil)
	w := newResponseRecorder()
	s.mux.ServeHTTP(w, req)

	if w.status != 200 {
		t.Fatalf("status: %d", w.status)
	}
	var got []map[string]any
	body, _ := io.ReadAll(w.body)
	if err := json.Unmarshal(body, &got); err != nil {
		t.Fatal(err)
	}
	if len(got) != 2 {
		t.Fatalf("got %d servers, want 2", len(got))
	}
	if got[0]["name"] != "Stargaze" || got[0]["status"] != "connected" {
		t.Errorf("server 0: %+v", got[0])
	}
	// Empty name should fall through to machineIdentifier — Session 1 finding.
	if got[1]["displayName"] != "def" {
		t.Errorf("server 1 displayName fallback failed: %+v", got[1])
	}
}

// Small in-package httptest replacement so we don't need httptest.Server for unit tests.
type responseRecorder struct {
	status  int
	body    *byteBuffer
	headers http.Header
}

type byteBuffer struct{ b []byte }

func (b *byteBuffer) Write(p []byte) (int, error) { b.b = append(b.b, p...); return len(p), nil }
func (b *byteBuffer) Read(p []byte) (int, error) {
	n := copy(p, b.b)
	b.b = b.b[n:]
	if n == 0 {
		return 0, io.EOF
	}
	return n, nil
}

func newResponseRecorder() *responseRecorder {
	return &responseRecorder{status: 200, body: &byteBuffer{}, headers: http.Header{}}
}
func (r *responseRecorder) Header() http.Header         { return r.headers }
func (r *responseRecorder) Write(p []byte) (int, error) { return r.body.Write(p) }
func (r *responseRecorder) WriteHeader(s int)           { r.status = s }

func TestLibrariesAndItemsRoutesNeedLiveClient(t *testing.T) {
	// We can't exercise the real plex.Client without a live server, so this test
	// confirms the router routes these paths and returns 500/404 deterministically
	// rather than crashing. Live integration happens in Task 23.

	cfg := &config.Config{Plex: config.PlexConfig{Servers: []config.Server{{MachineIdentifier: "abc", LastGoodConnection: "https://offline.example", AccessToken: "tok"}}}}
	s := New(cfg, nil, "127.0.0.1:0")

	// Unknown server → 404.
	req, _ := http.NewRequest("GET", "/api/servers/nonexistent/libraries", nil)
	w := newResponseRecorder()
	s.mux.ServeHTTP(w, req)
	if w.status != 404 {
		t.Errorf("unknown server: status %d, want 404", w.status)
	}

	// Invalid sub-path → 404.
	req, _ = http.NewRequest("GET", "/api/servers/abc/something-weird", nil)
	w = newResponseRecorder()
	s.mux.ServeHTTP(w, req)
	if w.status != 404 {
		t.Errorf("bogus sub-path: status %d, want 404", w.status)
	}

	// Known server, libraries path — will attempt to hit live Plex and fail with 502.
	// That's fine for this unit test. Integration test does the real run.
	req, _ = http.NewRequest("GET", "/api/servers/abc/libraries", nil)
	w = newResponseRecorder()
	s.mux.ServeHTTP(w, req)
	if !(w.status == 500 || w.status == 502) {
		t.Errorf("live-call attempt: status %d, want 500 or 502", w.status)
	}
	body := string(w.body.b)
	if !strings.Contains(body, "error") {
		t.Errorf("expected error payload, got %q", body)
	}
}
