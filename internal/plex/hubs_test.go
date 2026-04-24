package plex

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestGetHubBuildsCorrectURLAndParses(t *testing.T) {
	var gotPath, gotQuery, gotToken string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		gotQuery = r.URL.RawQuery
		gotToken = r.Header.Get("X-Plex-Token")
		w.Write([]byte(`{"MediaContainer":{"Metadata":[
			{"ratingKey":"9","guid":"plex://show/xyz","title":"Show One","type":"show","year":2024}
		]}}`))
	}))
	defer srv.Close()

	c := NewClient("id", "1.0.0")
	c.discoverBase = srv.URL

	items, err := c.GetHub("home", "trending-plex", "acct-tok")
	if err != nil {
		t.Fatal(err)
	}
	if gotPath != "/hubs/sections/home/trending-plex" {
		t.Errorf("path: %q", gotPath)
	}
	if !strings.Contains(gotQuery, "contentDirectoryID=home") {
		t.Errorf("query missing contentDirectoryID: %q", gotQuery)
	}
	if gotToken != "acct-tok" {
		t.Errorf("token header: %q", gotToken)
	}
	if len(items) != 1 || items[0].Title != "Show One" {
		t.Fatalf("items: %+v", items)
	}
}
