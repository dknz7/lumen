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
