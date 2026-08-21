package plex

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// TestGetCollectionsBuildsPathAndParses locks the path shape Plex Web
// uses (/library/sections/<id>/collections) and the basic decode shape.
// Real Stargaze captures: collections appear as type:"collection" entries
// inside the standard MediaContainer.Metadata slice.
func TestGetCollectionsBuildsPathAndParses(t *testing.T) {
	var gotPath, gotToken string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		gotToken = r.Header.Get("X-Plex-Token")
		w.Write([]byte(`{"MediaContainer":{"Metadata":[
			{"ratingKey":"490489","title":"Trending Movies","type":"collection"},
			{"ratingKey":"490500","title":"Marvel Universe","type":"collection"}
		]}}`))
	}))
	defer srv.Close()

	c := NewClient("id", "1.0.0")
	plexSrv := &Server{
		Name:        "Stargaze",
		BaseURL:     srv.URL,
		AccessToken: "stargaze-tok",
	}

	cols, err := c.GetCollections(plexSrv, "1")
	if err != nil {
		t.Fatal(err)
	}
	if gotPath != "/library/sections/1/collections" {
		t.Errorf("path: %q, want /library/sections/1/collections", gotPath)
	}
	if gotToken != "stargaze-tok" {
		t.Errorf("token header: %q, want stargaze-tok", gotToken)
	}
	if len(cols) != 2 {
		t.Fatalf("collections: %+v", cols)
	}
	if cols[0].RatingKey != "490489" || cols[0].Title != "Trending Movies" || cols[0].Type != "collection" {
		t.Errorf("cols[0] = %+v", cols[0])
	}
	if cols[1].Title != "Marvel Universe" {
		t.Errorf("cols[1].Title = %q", cols[1].Title)
	}
}

// TestGetCollectionItemsBuildsPathAndDecodes locks the path shape (Stargaze
// DevTools URL captured from a real server: /library/collections/<rk>/children)
// and the size-cap query params. Items decode through the shared
// metadataSliceToItems helper so collection items inherit every existing
// case-collision absorber + Item field surface.
func TestGetCollectionItemsBuildsPathAndDecodes(t *testing.T) {
	var gotPath, gotQuery string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		gotQuery = r.URL.RawQuery
		w.Write([]byte(`{"MediaContainer":{"Metadata":[
			{"ratingKey":"100","guid":"plex://movie/abc","title":"Trending Pick A","type":"movie","year":2026,"thumb":"/library/metadata/100/thumb/1"},
			{"ratingKey":"101","guid":"plex://movie/def","title":"Trending Pick B","type":"movie","year":2025}
		]}}`))
	}))
	defer srv.Close()

	c := NewClient("id", "1.0.0")
	plexSrv := &Server{BaseURL: srv.URL, AccessToken: "tok"}

	items, err := c.GetCollectionItems(plexSrv, "490489", 20)
	if err != nil {
		t.Fatal(err)
	}
	if gotPath != "/library/collections/490489/children" {
		t.Errorf("path: %q", gotPath)
	}
	if !strings.Contains(gotQuery, "X-Plex-Container-Size=20") {
		t.Errorf("query missing size cap: %q", gotQuery)
	}
	if len(items) != 2 {
		t.Fatalf("items: %+v", items)
	}
	if items[0].Title != "Trending Pick A" || items[0].Year != 2026 || items[0].RatingKey != "100" {
		t.Errorf("items[0] = %+v", items[0])
	}
	if items[1].Title != "Trending Pick B" {
		t.Errorf("items[1].Title = %q", items[1].Title)
	}
}

// TestGetCollectionItemsNoSizeCap covers the size==0 path (let Plex use
// its default container size). Used when callers want the full collection.
func TestGetCollectionItemsNoSizeCap(t *testing.T) {
	var gotQuery string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotQuery = r.URL.RawQuery
		w.Write([]byte(`{"MediaContainer":{"Metadata":[]}}`))
	}))
	defer srv.Close()

	c := NewClient("id", "1.0.0")
	plexSrv := &Server{BaseURL: srv.URL, AccessToken: "tok"}

	if _, err := c.GetCollectionItems(plexSrv, "490489", 0); err != nil {
		t.Fatal(err)
	}
	if gotQuery != "" {
		t.Errorf("expected empty query when size=0, got %q", gotQuery)
	}
}
