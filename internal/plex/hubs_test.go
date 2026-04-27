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
				"Guid":[{"id":"imdb://tt12345"},{"id":"tmdb://999"}],
				"Media":[{"id":1,"Part":[{"id":"abc-def","key":"/library/metadata/123/extras/456/parts/hls.m3u8"}]}]
			},
			{
				"ratingKey":"fixture-clip-rk-2",
				"title":"Trailer Without ParentRatingKey",
				"type":"clip",
				"primaryGuid":"plex://show/parent-show-rk-from-primaryguid"
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
	if len(items) != 2 {
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
	// HLS URL — Media[0].Part[0].key qualified to absolute URL with the
	// account token applied. discoverBase points at the test server here, so
	// the URL is built against srv.URL.
	wantHLS := srv.URL + "/library/metadata/123/extras/456/parts/hls.m3u8?X-Plex-Token=acct-tok"
	if got.HLSUrl != wantHLS {
		t.Errorf("HLSUrl = %q, want %q", got.HLSUrl, wantHLS)
	}

	// Second fixture clip — no literal parentRatingKey, only primaryGuid.
	// Locks the fallback path: hubs.go parses the trailing segment of
	// "plex://show/parent-show-rk-from-primaryguid" into ParentRatingKey
	// so DiscoverTile navigation resolves to the parent show's detail page.
	got2 := items[1]
	if got2.ParentRatingKey != "parent-show-rk-from-primaryguid" {
		t.Errorf("ParentRatingKey (primaryGuid fallback) = %q, want parent-show-rk-from-primaryguid", got2.ParentRatingKey)
	}
}

// TestGetHubFiltersPlaceholdersAndSurfacesSeasonFields covers Session 6.5 —
// Plex's Coming Soon hub injects "type":"placeholder" ad slots, and season
// items carry parentTitle/parentIndex/index/grandparentTitle that
// DiscoverTile needs to render parentTitle / title / date per Plex Web's
// MediaContainer.Meta.DisplayFields directive. Confirmed against a real
// home/coming-soon DevTools capture in Session 6.5.
func TestGetHubFiltersPlaceholdersAndSurfacesSeasonFields(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`{"MediaContainer":{"Metadata":[
			{"ratingKey":"ad-1","type":"placeholder","url":"https://example.com/ad/tile?slot=1"},
			{
				"ratingKey":"season-rk",
				"guid":"plex://season/abc",
				"title":"Season 9",
				"type":"season",
				"year":2026,
				"index":9,
				"originallyAvailableAt":"2026-05-24",
				"parentTitle":"Rick and Morty",
				"parentRatingKey":"show-rk",
				"parentGuid":"plex://show/parent",
				"parentThumb":"https://example.com/p.jpg"
			},
			{
				"ratingKey":"episode-rk",
				"guid":"plex://episode/xyz",
				"title":"The One With Bottle Episodes",
				"type":"episode",
				"year":2026,
				"index":5,
				"parentIndex":2,
				"originallyAvailableAt":"2026-06-01",
				"grandparentTitle":"Show Name",
				"grandparentRatingKey":"gp-rk"
			},
			{"ratingKey":"ad-2","type":"placeholder","url":"https://example.com/ad/tile?slot=2"}
		]}}`))
	}))
	defer srv.Close()

	c := NewClient("id", "1.0.0")
	c.discoverBase = srv.URL

	items, err := c.GetHub("home", "coming-soon", "acct-tok")
	if err != nil {
		t.Fatal(err)
	}
	// Two placeholders dropped, two real items remain.
	if len(items) != 2 {
		t.Fatalf("placeholder filter: got %d items, want 2: %+v", len(items), items)
	}

	season := items[0]
	if season.Type != "season" {
		t.Errorf("season.Type = %q, want season", season.Type)
	}
	if season.ParentTitle != "Rick and Morty" {
		t.Errorf("season.ParentTitle = %q, want Rick and Morty", season.ParentTitle)
	}
	if season.Title != "Season 9" {
		t.Errorf("season.Title = %q, want Season 9", season.Title)
	}
	if season.Index != 9 {
		t.Errorf("season.Index = %d, want 9", season.Index)
	}
	if season.OriginallyAvailableAt != "2026-05-24" {
		t.Errorf("season.OriginallyAvailableAt = %q, want 2026-05-24", season.OriginallyAvailableAt)
	}

	ep := items[1]
	if ep.Type != "episode" {
		t.Errorf("ep.Type = %q, want episode", ep.Type)
	}
	if ep.GrandparentTitle != "Show Name" {
		t.Errorf("ep.GrandparentTitle = %q, want Show Name", ep.GrandparentTitle)
	}
	if ep.ParentIndex != 2 {
		t.Errorf("ep.ParentIndex = %d, want 2", ep.ParentIndex)
	}
	if ep.Index != 5 {
		t.Errorf("ep.Index = %d, want 5", ep.Index)
	}
}
