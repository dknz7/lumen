package plex

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestGetWatchlistHeaderOnlyAuthAndShape(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("X-Plex-Token") != "tok" {
			http.Error(w, "missing token header", http.StatusUnauthorized)
			return
		}
		if r.URL.RawQuery != "" {
			// The endpoint takes no query params; ensure we don't drift back to
			// query-string auth.
			http.Error(w, "unexpected query string", http.StatusBadRequest)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{
			"MediaContainer": {
				"Metadata": [
					{"ratingKey":"5d77","title":"Dune","type":"movie","year":2021,"guid":"plex://movie/abc"},
					{"ratingKey":"7e88","title":"Severance","type":"show","year":2022,"guid":"plex://show/xyz"}
				]
			}
		}`))
	}))
	defer srv.Close()

	c := NewClient("client-id", "test")
	c.metadataBase = srv.URL
	items, err := c.GetWatchlist("tok")
	if err != nil {
		t.Fatalf("GetWatchlist: %v", err)
	}
	if len(items) != 2 {
		t.Fatalf("got %d items, want 2", len(items))
	}
	if items[0].Title != "Dune" || items[1].Type != "show" {
		t.Errorf("unexpected items: %+v", items)
	}
}

func TestAddRemoveWatchlistShape(t *testing.T) {
	hits := map[string]int{} // path → count
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "PUT" {
			http.Error(w, "expected PUT", http.StatusBadRequest)
			return
		}
		if r.Header.Get("X-Plex-Token") != "tok" {
			http.Error(w, "missing token header", http.StatusUnauthorized)
			return
		}
		if r.URL.Query().Get("ratingKey") != "12345" {
			http.Error(w, "missing ratingKey query", http.StatusBadRequest)
			return
		}
		hits[r.URL.Path]++
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"MediaContainer":{"size":0}}`))
	}))
	defer srv.Close()

	c := NewClient("client-id", "test")
	c.discoverBase = srv.URL

	if err := c.AddToWatchlist("tok", "12345"); err != nil {
		t.Fatalf("AddToWatchlist: %v", err)
	}
	if err := c.RemoveFromWatchlist("tok", "12345"); err != nil {
		t.Fatalf("RemoveFromWatchlist: %v", err)
	}
	if hits["/actions/addToWatchlist"] != 1 {
		t.Errorf("addToWatchlist hit count: %d", hits["/actions/addToWatchlist"])
	}
	if hits["/actions/removeFromWatchlist"] != 1 {
		t.Errorf("removeFromWatchlist hit count: %d", hits["/actions/removeFromWatchlist"])
	}
}
