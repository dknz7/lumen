package server

import (
	"net/http"
	"testing"

	"lumen/internal/config"
)

func TestHubsRouteValidatesNamespace(t *testing.T) {
	cfg := &config.Config{Plex: config.PlexConfig{AccountToken: "tok"}}
	s := New(cfg, nil, "127.0.0.1:0")

	// Note: empty-string namespace produces path "/api/hubs//whatever" which Go's
	// ServeMux cleans via 307 redirect — a non-200, non-hub-data response, which is
	// still correct behaviour. We accept 307 for that edge case.
	for _, ns := range []string{"bogus", "random"} {
		path := "/api/hubs/" + ns + "/whatever"
		req, _ := http.NewRequest("GET", path, nil)
		w := newResponseRecorder()
		s.mux.ServeHTTP(w, req)
		if w.status != 400 {
			t.Errorf("ns=%q: status %d, want 400", ns, w.status)
		}
	}
	// Empty namespace: double-slash path gets 307 redirect from ServeMux — acceptable.
	req, _ := http.NewRequest("GET", "/api/hubs//whatever", nil)
	w := newResponseRecorder()
	s.mux.ServeHTTP(w, req)
	if w.status == 200 {
		t.Errorf("empty ns: got 200, want non-200")
	}
}

func TestHubsRequiresAccountToken(t *testing.T) {
	cfg := &config.Config{} // no AccountToken
	s := New(cfg, nil, "127.0.0.1:0")

	req, _ := http.NewRequest("GET", "/api/hubs/home/trending-plex", nil)
	w := newResponseRecorder()
	s.mux.ServeHTTP(w, req)
	if w.status != 401 {
		t.Errorf("status %d, want 401", w.status)
	}
}
