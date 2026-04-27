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

// TestGetHubSurfacesExtendedFields covers Task 12 — verifies that the
// HubItem wire surfaces parentRatingKey / grandparentRatingKey / imdbId /
// contentRating / studio / tagline / addedAt / originallyAvailableAt for
// clip items so DiscoverTile can navigate to the parent show/movie's
// detail page and resolve trailers via TMDB.
func TestGetHubSurfacesExtendedFields(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`{"MediaContainer":{"Metadata":[
			{
				"ratingKey":"fixture-clip-rk",
				"title":"Trailer",
				"type":"clip",
				"thumb":"https://example.com/t.jpg",
				"parentRatingKey":"parent-show-rk",
				"grandparentRatingKey":"grandparent-show-rk",
				"contentRating":"PG-13",
				"studio":"Test Studio",
				"tagline":"A tagline.",
				"addedAt":1700000000,
				"originallyAvailableAt":"2026-04-01",
				"Guid":[{"id":"imdb://tt12345"},{"id":"tmdb://999"}]
			}
		]}}`))
	}))
	defer srv.Close()

	c := NewClient("id", "1.0.0")
	c.discoverBase = srv.URL

	items, err := c.GetHub("home", "trending-trailers", "acct-tok")
	if err != nil {
		t.Fatal(err)
	}
	if len(items) != 1 {
		t.Fatalf("items: %+v", items)
	}
	got := items[0]
	if got.ParentRatingKey != "parent-show-rk" {
		t.Errorf("ParentRatingKey = %q, want parent-show-rk", got.ParentRatingKey)
	}
	if got.GrandparentRatingKey != "grandparent-show-rk" {
		t.Errorf("GrandparentRatingKey = %q, want grandparent-show-rk", got.GrandparentRatingKey)
	}
	if got.IMDBID != "tt12345" {
		t.Errorf("IMDBID = %q, want tt12345", got.IMDBID)
	}
	if got.ContentRating != "PG-13" {
		t.Errorf("ContentRating = %q, want PG-13", got.ContentRating)
	}
	if got.Studio != "Test Studio" {
		t.Errorf("Studio = %q, want Test Studio", got.Studio)
	}
	if got.Tagline != "A tagline." {
		t.Errorf("Tagline = %q, want A tagline.", got.Tagline)
	}
	if got.AddedAt != 1700000000 {
		t.Errorf("AddedAt = %d, want 1700000000", got.AddedAt)
	}
	if got.OriginallyAvailableAt != "2026-04-01" {
		t.Errorf("OriginallyAvailableAt = %q, want 2026-04-01", got.OriginallyAvailableAt)
	}
}
