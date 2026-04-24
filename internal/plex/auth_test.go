package plex

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestCreatePINParsesResponse(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "POST" || !strings.HasSuffix(r.URL.Path, "/pins") {
			http.Error(w, "wrong path/method", 404)
			return
		}
		if r.Header.Get("X-Plex-Client-Identifier") == "" {
			http.Error(w, "missing identity header", 400)
			return
		}
		_ = json.NewEncoder(w).Encode(map[string]any{
			"id":   42,
			"code": "ABCD",
		})
	}))
	defer srv.Close()

	c := NewClient("id", "1.0.0")
	c.plexTVBase = srv.URL // overridable for tests
	pin, err := c.CreatePIN()
	if err != nil {
		t.Fatal(err)
	}
	if pin.ID != 42 || pin.Code != "ABCD" {
		t.Errorf("pin: got %+v", pin)
	}
}

func TestPollPINReturnsTokenWhenClaimed(t *testing.T) {
	calls := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		// First two polls: unclaimed. Third: claimed.
		resp := map[string]any{"id": 42, "code": "ABCD"}
		if calls >= 3 {
			resp["authToken"] = "live-token-xyz"
		}
		_ = json.NewEncoder(w).Encode(resp)
	}))
	defer srv.Close()

	c := NewClient("id", "1.0.0")
	c.plexTVBase = srv.URL
	c.pinPollInterval = 10 * time.Millisecond // speed the test up
	token, err := c.PollPIN(PIN{ID: 42}, 2*time.Second)
	if err != nil {
		t.Fatal(err)
	}
	if token != "live-token-xyz" {
		t.Errorf("token: got %q", token)
	}
	if calls < 3 {
		t.Errorf("expected at least 3 polls, got %d", calls)
	}
}

func TestPollPINTimesOut(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{"id": 42})
	}))
	defer srv.Close()

	c := NewClient("id", "1.0.0")
	c.plexTVBase = srv.URL
	c.pinPollInterval = 10 * time.Millisecond

	_, err := c.PollPIN(PIN{ID: 42}, 50*time.Millisecond)
	if err == nil {
		t.Fatal("expected timeout error, got nil")
	}
}
