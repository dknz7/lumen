package plex

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

// helper spins up a test server that either answers /identity OK or sleeps forever
// to simulate unreachable.
func identityServer(t *testing.T, ok bool) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/identity" {
			http.Error(w, "wrong path", 404)
			return
		}
		if !ok {
			http.Error(w, "boom", 500)
			return
		}
		w.WriteHeader(http.StatusOK)
	}))
}

func TestPickConnectionPrefersNonRelay(t *testing.T) {
	good := identityServer(t, true)
	defer good.Close()
	relay := identityServer(t, true)
	defer relay.Close()

	s := &Server{Connections: []Connection{
		{URI: relay.URL, Relay: true},
		{URI: good.URL, Relay: false},
	}}

	c := NewClient("id", "1.0.0")
	conn, err := c.PickConnection(s)
	if err != nil {
		t.Fatal(err)
	}
	if conn.URI != good.URL {
		t.Errorf("picked %q, want non-relay %q", conn.URI, good.URL)
	}
	if s.BaseURL != good.URL {
		t.Errorf("BaseURL not cached: %q", s.BaseURL)
	}
}

func TestPickConnectionFallsBackToRelayWhenDirectFails(t *testing.T) {
	bad := identityServer(t, false)
	defer bad.Close()
	relay := identityServer(t, true)
	defer relay.Close()

	s := &Server{Connections: []Connection{
		{URI: bad.URL, Relay: false},
		{URI: relay.URL, Relay: true},
	}}
	c := NewClient("id", "1.0.0")
	conn, err := c.PickConnection(s)
	if err != nil {
		t.Fatal(err)
	}
	if conn.URI != relay.URL {
		t.Errorf("fallback failed: picked %q", conn.URI)
	}
}

func TestPickConnectionReturnsErrorWhenAllFail(t *testing.T) {
	a := identityServer(t, false)
	defer a.Close()
	b := identityServer(t, false)
	defer b.Close()

	s := &Server{Connections: []Connection{{URI: a.URL}, {URI: b.URL}}}
	c := NewClient("id", "1.0.0")
	_, err := c.PickConnection(s)
	if err == nil {
		t.Fatal("expected error when all candidates fail")
	}
}
