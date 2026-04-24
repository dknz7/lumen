package plex

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestStandardHeadersAppliedOnRequest(t *testing.T) {
	var got http.Header
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		got = r.Header.Clone()
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	c := NewClient("test-client-id", "1.2.3")
	req, err := c.NewRequest("GET", srv.URL+"/ping", nil)
	if err != nil {
		t.Fatal(err)
	}
	resp, err := c.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()

	checks := map[string]string{
		"X-Plex-Product":           "Lumen",
		"X-Plex-Version":           "1.2.3",
		"X-Plex-Platform":          "Windows",
		"X-Plex-Device":            "PC",
		"X-Plex-Client-Identifier": "test-client-id",
		"Accept":                   "application/json",
	}
	for k, want := range checks {
		if g := got.Get(k); g != want {
			t.Errorf("header %s: got %q want %q", k, g, want)
		}
	}
}

func TestClientPropagatesTokenHeader(t *testing.T) {
	var got http.Header
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		got = r.Header.Clone()
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	c := NewClient("id", "0.0.1")
	req, _ := c.NewRequest("GET", srv.URL, nil)
	c.SetToken(req, "token-abc")
	resp, _ := c.Do(req)
	resp.Body.Close()

	if got.Get("X-Plex-Token") != "token-abc" {
		t.Fatalf("X-Plex-Token: got %q want %q", got.Get("X-Plex-Token"), "token-abc")
	}
}
