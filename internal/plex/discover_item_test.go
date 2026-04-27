package plex

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// Trimmed Mario fixture — keeps the wire-shape ground truth (Studio[],
// Genre[], Guid[], Rating[], Role/Director/Writer) but drops the 19 RT
// reviews and 46 cast entries down to 1-2 each. Anything we add here
// becomes a regression target if Plex ever drifts the JSON.
const marioDiscoverFixture = `{
  "MediaContainer": {
    "identifier": "tv.plex.provider.discover",
    "size": 1,
    "Metadata": [{
      "ratingKey": "64dd290c84713e6f8ba2874b",
      "guid": "plex://movie/64dd290c84713e6f8ba2874b",
      "title": "The Super Mario Galaxy Movie",
      "type": "movie",
      "year": 2026,
      "summary": "Having thwarted Bowser's previous plot...",
      "tagline": "The galaxy awaits.",
      "contentRating": "PG",
      "rating": 4.3,
      "originallyAvailableAt": "2026-04-01",
      "duration": 5880000,
      "thumb": "https://metadata-static.plex.tv/5/gracenote/x.jpg",
      "art": "https://metadata-static.plex.tv/6/gracenote/y.jpg",
      "addedAt": 1775001600,
      "publicPagesURL": "https://watch.plex.tv/movie/the-super-mario-galaxy-movie",
      "studio": "Illumination",
      "Studio": [{"tag": "Illumination"}, {"tag": "Universal Pictures"}, {"tag": "Nintendo"}],
      "Genre": [{"tag": "Animation"}, {"tag": "Adventure"}],
      "Guid": [
        {"id": "imdb://tt28650488"},
        {"id": "tmdb://1226863"},
        {"id": "tvdb://354713"}
      ],
      "Rating": [
        {"image": "imdb://image.rating", "type": "audience", "value": 6.4},
        {"image": "rottentomatoes://image.rating.rotten", "type": "critic", "value": 4.3},
        {"image": "rottentomatoes://image.rating.upright", "type": "audience", "value": 8.9}
      ],
      "Role": [
        {"id": "5d7768328718ba001e313dcc", "tag": "Chris Pratt", "role": "Mario (voice)", "thumb": "https://x/cp.jpg"},
        {"id": "5d776b54ad5437001f79bb69", "tag": "Anya Taylor-Joy", "role": "Peach (voice)", "thumb": "https://x/atj.jpg"}
      ],
      "Director": [{"id": "5d776d6d47dd6e001f6f31a8", "tag": "Aaron Horvath", "thumb": "https://x/ah.jpg"}],
      "Writer": [{"id": "5d77686deb5d26001f1eb0c4", "tag": "Matthew Fogel", "thumb": "https://x/mf.jpg"}]
    }]
  }
}`

func TestGetDiscoverItem_Decodes(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("X-Plex-Token") != "tok" {
			http.Error(w, "missing token header", http.StatusUnauthorized)
			return
		}
		// Token must be header-only; never on the URL query.
		if r.URL.Query().Get("X-Plex-Token") != "" {
			http.Error(w, "token must be header-only", http.StatusBadRequest)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(marioDiscoverFixture))
	}))
	defer srv.Close()

	c := NewClient("client-id", "test")
	c.discoverBase = srv.URL

	item, err := c.GetDiscoverItem("tok", "64dd290c84713e6f8ba2874b")
	if err != nil {
		t.Fatalf("GetDiscoverItem: %v", err)
	}
	if item.Title != "The Super Mario Galaxy Movie" {
		t.Errorf("title = %q", item.Title)
	}
	if item.IMDBID != "tt28650488" {
		t.Errorf("imdbId = %q, want tt28650488", item.IMDBID)
	}
	if item.Tagline != "The galaxy awaits." {
		t.Errorf("tagline = %q", item.Tagline)
	}
	if len(item.Studios) != 3 || item.Studios[0] != "Illumination" {
		t.Errorf("studios = %v, want [Illumination, Universal Pictures, Nintendo]", item.Studios)
	}
	if len(item.Genres) != 2 || item.Genres[0] != "Animation" {
		t.Errorf("genres = %v", item.Genres)
	}
	if len(item.Ratings) != 3 {
		t.Errorf("ratings count = %d, want 3", len(item.Ratings))
	}
	if len(item.Cast) != 2 || item.Cast[0].Name != "Chris Pratt" || item.Cast[0].Tag != "Mario (voice)" {
		t.Errorf("cast = %+v", item.Cast)
	}
	if len(item.Directors) != 1 || item.Directors[0].Name != "Aaron Horvath" {
		t.Errorf("directors = %+v", item.Directors)
	}
	if len(item.Writers) != 1 || item.Writers[0].Name != "Matthew Fogel" {
		t.Errorf("writers = %+v", item.Writers)
	}
}

const summaryArrayFixture = `{
  "MediaContainer": {
    "identifier": "tv.plex.provider.discover",
    "size": 1,
    "Metadata": [{
      "ratingKey": "fixture-array-summary",
      "guid": "plex://show/fixture",
      "title": "Test Show",
      "type": "show",
      "year": 2020,
      "summary": ["Paragraph one.", "Paragraph two."],
      "contentRating": "TV-14",
      "rating": 7.5,
      "Studio": [{"tag": "Test"}],
      "Genre": [{"tag": "Drama"}],
      "Guid": [{"id": "imdb://tt1111111"}],
      "Rating": [{"image": "imdb://image.rating", "type": "audience", "value": 7.5}],
      "Role": [],
      "Director": [],
      "Writer": []
    }]
  }
}`

func TestGetDiscoverItem_SummaryArray(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(summaryArrayFixture))
	}))
	defer srv.Close()

	c := NewClient("client-id", "test")
	c.discoverBase = srv.URL
	item, err := c.GetDiscoverItem("tok", "fixture-array-summary")
	if err != nil {
		t.Fatalf("GetDiscoverItem: %v", err)
	}
	if !strings.Contains(item.Summary, "Paragraph one.") || !strings.Contains(item.Summary, "Paragraph two.") {
		t.Errorf("Summary did not absorb array form: %q", item.Summary)
	}
}

func TestGetDiscoverItem_EmptyMetadata(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"MediaContainer":{"size":0,"Metadata":[]}}`))
	}))
	defer srv.Close()

	c := NewClient("client-id", "test")
	c.discoverBase = srv.URL
	if _, err := c.GetDiscoverItem("tok", "deadbeef"); err == nil {
		t.Fatalf("expected error on empty Metadata, got nil")
	}
}
