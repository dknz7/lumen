package server

import (
	"bytes"
	"fmt"
	"net/http"
	"sync"
	"testing"

	"lumen/internal/config"
)

// TestSettingsConcurrentAccessDoesNotCrash is a regression test for an
// unsynchronised-config crash.
//
// s.cfg used to be read and written by handler goroutines with no lock. Two
// overlapping PUT /api/settings both wrote cfg.UI.ShelfState — a map — which
// the Go runtime detects as "fatal error: concurrent map writes". That is a
// hard process abort: not a panic, not recoverable, and it takes the whole app
// down. Overlapping GETs made it worse by marshalling the same map concurrently
// ("concurrent map read and map write").
//
// The SPA autosaves shelf layout on every drag/collapse, so this was reachable
// in ordinary use, not just under an artificial hammering.
//
// Note this test needs no -race: the map checks are unconditional runtime
// checks, so it reproduces the original failure on a plain `go test`. It is
// also *stronger* under -race, which additionally catches the scalar field
// races (AccountToken, OMDBKey, the Servers slice header).
func TestSettingsConcurrentAccessDoesNotCrash(t *testing.T) {
	t.Setenv("APPDATA", t.TempDir())
	cfg, err := config.Load()
	if err != nil {
		t.Fatal(err)
	}
	s := New(cfg, nil, "127.0.0.1:0")

	const goroutines = 12
	const iterations = 40

	var wg sync.WaitGroup
	start := make(chan struct{})

	// Writers: each hammers shelfState with a distinct payload.
	for g := 0; g < goroutines; g++ {
		wg.Add(1)
		go func(g int) {
			defer wg.Done()
			<-start
			for i := 0; i < iterations; i++ {
				body := fmt.Sprintf(
					`{"theme":"pure-oled","zoom":%d,"shelfState":{"home":{"groupOrder":["g%d"],"groupCollapsed":{"g%d":true},"shelfOrder":{"g%d":["s%d"]}}}}`,
					100+g, g, g, g, i)
				req, _ := http.NewRequest("PUT", "/api/settings", bytes.NewBufferString(body))
				req.Header.Set("Content-Type", "application/json")
				s.mux.ServeHTTP(newResponseRecorder(), req)
			}
		}(g)
	}

	// Readers: GET marshals the same maps.
	for g := 0; g < goroutines; g++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			<-start
			for i := 0; i < iterations; i++ {
				req, _ := http.NewRequest("GET", "/api/settings", nil)
				s.mux.ServeHTTP(newResponseRecorder(), req)
			}
		}()
	}

	// Other config readers, exercising the accessor paths.
	for g := 0; g < 4; g++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			<-start
			for i := 0; i < iterations; i++ {
				_ = s.accountToken()
				_ = s.omdbKey()
				_ = s.tmdbKey()
				_ = s.serverList()
				_ = s.serverByID("nope")
				_ = s.potPlayerPath()
			}
		}()
	}

	close(start) // release everything at once to maximise overlap
	wg.Wait()

	// Surviving is the assertion — the old code aborted the test binary here.
	// Check the config is still coherent rather than half-written.
	b, err := s.marshalCfg(func(c *config.Config) any { return c.UI })
	if err != nil {
		t.Fatalf("config unmarshalable after concurrent access: %v", err)
	}
	if len(b) == 0 {
		t.Error("marshalled UI config is empty")
	}
	// Every writer sent a valid zoom in [100,111]; a torn write would land
	// outside that range or leave the field zeroed.
	if z := currentZoom(t, s); z < 100 || z > 100+goroutines {
		t.Errorf("zoom = %d after concurrent writes, want a value one writer actually sent", z)
	}
}

// TestSettingsPutIsAuthoritativeOverShelfState pins the reset-then-merge
// behaviour: encoding/json merges into an existing map, so without clearing it
// first a PUT could never remove a page's shelf state.
func TestSettingsPutIsAuthoritativeOverShelfState(t *testing.T) {
	t.Setenv("APPDATA", t.TempDir())
	cfg, err := config.Load()
	if err != nil {
		t.Fatal(err)
	}
	cfg.UI.ShelfState = map[string]config.PageShelfState{
		"stale": {GroupOrder: []string{"old"}},
	}
	s := New(cfg, nil, "127.0.0.1:0")

	body := `{"shelfState":{"home":{"groupOrder":["new"]}}}`
	req, _ := http.NewRequest("PUT", "/api/settings", bytes.NewBufferString(body))
	w := newResponseRecorder()
	s.mux.ServeHTTP(w, req)
	if w.status != 200 {
		t.Fatalf("status %d", w.status)
	}

	s.cfgMu.RLock()
	defer s.cfgMu.RUnlock()
	if _, ok := s.cfg.UI.ShelfState["stale"]; ok {
		t.Error(`"stale" page survived a PUT that did not mention it`)
	}
	if _, ok := s.cfg.UI.ShelfState["home"]; !ok {
		t.Error(`"home" page missing after PUT`)
	}
}

func currentZoom(t *testing.T, s *Server) int {
	t.Helper()
	s.cfgMu.RLock()
	defer s.cfgMu.RUnlock()
	return s.cfg.UI.Zoom
}
