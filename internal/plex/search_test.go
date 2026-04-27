package plex

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// TestSearchDiscoverDecodesAndSortsByScore is fixture-grounded against the
// real plex.tv /library/search response captured Session 6.5 (query=hokum).
// Locks the doubly-nested envelope shape (SearchResults groups → SearchResult
// items → Metadata + score), the empty-group skip behaviour, and score-desc
// sorting across groups.
func TestSearchDiscoverDecodesAndSortsByScore(t *testing.T) {
	var gotPath, gotQuery, gotToken string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		gotQuery = r.URL.RawQuery
		gotToken = r.Header.Get("X-Plex-Token")
		// Trimmed-down version of the real hokum response: one empty group
		// (Free On Demand) + one populated group (More Ways To Watch) with
		// out-of-order scores so we can lock the score-desc sort.
		w.Write([]byte(`{
			"MediaContainer": {
				"size": 2,
				"SearchResults": [
					{"id":"plex","title":"Free On Demand","size":0},
					{"id":"external","title":"More Ways To Watch","size":3,"SearchResult":[
						{"score":0.37,"Metadata":{"ratingKey":"low","guid":"plex://movie/low","title":"Old Hokum Bucket","type":"movie","year":1931}},
						{"score":0.79,"Metadata":{"ratingKey":"high","guid":"plex://movie/high","title":"Hokum (2026)","type":"movie","year":2026}},
						{"score":0.66,"Metadata":{"ratingKey":"mid","guid":"plex://movie/mid","title":"Hokum (1936)","type":"movie","year":1936}}
					]}
				]
			}
		}`))
	}))
	defer srv.Close()

	c := NewClient("id", "1.0.0")
	c.discoverBase = srv.URL

	items, err := c.SearchDiscover("hokum", "acct-tok")
	if err != nil {
		t.Fatal(err)
	}
	if gotPath != "/library/search" {
		t.Errorf("path = %q, want /library/search", gotPath)
	}
	for _, want := range []string{"query=hokum", "limit=30", "searchTypes=movies%2Ctv", "searchProviders=discover%2CplexAVOD", "includeMetadata=1", "filterPeople=1"} {
		if !strings.Contains(gotQuery, want) {
			t.Errorf("query missing %q: %q", want, gotQuery)
		}
	}
	if gotToken != "acct-tok" {
		t.Errorf("token header: %q, want acct-tok", gotToken)
	}
	if len(items) != 3 {
		t.Fatalf("expected 3 items (empty group skipped), got %d: %+v", len(items), items)
	}
	// Score-desc order: high (0.79) → mid (0.66) → low (0.37).
	wantOrder := []string{"high", "mid", "low"}
	for i, want := range wantOrder {
		if items[i].RatingKey != want {
			t.Errorf("items[%d].RatingKey = %q, want %q", i, items[i].RatingKey, want)
		}
	}
}

// TestSearchDiscoverEmptyQuery is a defensive guard — empty query strings
// shouldn't hit the network at all (debounce safety net).
func TestSearchDiscoverEmptyQuery(t *testing.T) {
	c := NewClient("id", "1.0.0")
	// No httptest server bound — if the function tries to fetch, the URL
	// will be unreachable and we'd get an error. Empty query short-circuits.
	items, err := c.SearchDiscover("", "tok")
	if err != nil {
		t.Fatalf("empty query should be a no-op, got err: %v", err)
	}
	if items != nil {
		t.Errorf("empty query should return nil items, got: %+v", items)
	}
}

// TestSearchDiscoverEmptyResults exercises the "no matches anywhere" path —
// both groups present but size 0. Should return empty slice (not nil), no error.
func TestSearchDiscoverEmptyResults(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`{"MediaContainer":{"size":2,"SearchResults":[
			{"id":"plex","title":"Free On Demand","size":0},
			{"id":"external","title":"More Ways To Watch","size":0}
		]}}`))
	}))
	defer srv.Close()

	c := NewClient("id", "1.0.0")
	c.discoverBase = srv.URL

	items, err := c.SearchDiscover("xyznonexistent", "tok")
	if err != nil {
		t.Fatal(err)
	}
	if len(items) != 0 {
		t.Errorf("expected empty results, got: %+v", items)
	}
}
