package plex

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestGetWatchlistHeaderOnlyAuthAndShape(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("X-Plex-Token") != "tok" {
			http.Error(w, "missing token header", http.StatusUnauthorized)
			return
		}
		// Pagination params required by Plex Discover; reject token in query.
		if r.URL.Query().Get("X-Plex-Token") != "" {
			http.Error(w, "token must be header-only", http.StatusBadRequest)
			return
		}
		if r.URL.Query().Get("X-Plex-Container-Size") == "" {
			http.Error(w, "expected X-Plex-Container-Size", http.StatusBadRequest)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{
			"MediaContainer": {
				"totalSize": 2,
				"Metadata": [
					{"ratingKey":"5d77","title":"Dune","type":"movie","year":2021,"guid":"plex://movie/abc"},
					{"ratingKey":"7e88","title":"Severance","type":"show","year":2022,"guid":"plex://show/xyz"}
				]
			}
		}`))
	}))
	defer srv.Close()

	c := NewClient("client-id", "test")
	c.discoverBase = srv.URL
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

// TestAddItemToWatchlist_MoviePassThrough — movie items use their own GUID
// directly. One GetItem call, one AddToWatchlist call. Verifies the plex.tv
// ratingKey is correctly extracted from the trailing segment of the GUID.
func TestAddItemToWatchlist_MoviePassThrough(t *testing.T) {
	var addedRatingKey string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/library/metadata/movie-rk" {
			w.Write([]byte(`{"MediaContainer":{"Metadata":[
				{"ratingKey":"movie-rk","guid":"plex://movie/movie-plex-rk","title":"Pineapple Express","type":"movie","year":2008}
			]}}`))
			return
		}
		if r.URL.Path == "/actions/addToWatchlist" {
			addedRatingKey = r.URL.Query().Get("ratingKey")
			w.Write([]byte(`{"MediaContainer":{"size":0}}`))
			return
		}
		http.NotFound(w, r)
	}))
	defer srv.Close()

	c := NewClient("client-id", "test")
	c.discoverBase = srv.URL
	plexSrv := &Server{BaseURL: srv.URL, AccessToken: "server-tok"}

	if err := c.AddItemToWatchlist(plexSrv, "acct-tok", "movie-rk"); err != nil {
		t.Fatalf("AddItemToWatchlist: %v", err)
	}
	if addedRatingKey != "movie-plex-rk" {
		t.Errorf("addToWatchlist called with ratingKey=%q, want movie-plex-rk", addedRatingKey)
	}
}

// TestAddItemToWatchlist_EpisodeRollsUpToShow — episode items fetch the
// grandparent show and use its GUID. Two GetItem calls, one AddToWatchlist
// targeting the SHOW's plex.tv ratingKey (not the episode's).
func TestAddItemToWatchlist_EpisodeRollsUpToShow(t *testing.T) {
	var addedRatingKey string
	getCalls := map[string]int{}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/library/metadata/episode-rk":
			getCalls["episode"]++
			w.Write([]byte(`{"MediaContainer":{"Metadata":[
				{"ratingKey":"episode-rk","guid":"plex://episode/episode-plex-rk","title":"Pilot","type":"episode","grandparentRatingKey":"show-rk"}
			]}}`))
		case "/library/metadata/show-rk":
			getCalls["show"]++
			w.Write([]byte(`{"MediaContainer":{"Metadata":[
				{"ratingKey":"show-rk","guid":"plex://show/show-plex-rk","title":"The Show","type":"show"}
			]}}`))
		case "/actions/addToWatchlist":
			addedRatingKey = r.URL.Query().Get("ratingKey")
			w.Write([]byte(`{"MediaContainer":{"size":0}}`))
		default:
			http.NotFound(w, r)
		}
	}))
	defer srv.Close()

	c := NewClient("client-id", "test")
	c.discoverBase = srv.URL
	plexSrv := &Server{BaseURL: srv.URL, AccessToken: "server-tok"}

	if err := c.AddItemToWatchlist(plexSrv, "acct-tok", "episode-rk"); err != nil {
		t.Fatalf("AddItemToWatchlist: %v", err)
	}
	if getCalls["episode"] != 1 {
		t.Errorf("expected 1 GetItem for episode, got %d", getCalls["episode"])
	}
	if getCalls["show"] != 1 {
		t.Errorf("expected 1 GetItem for grandparent show, got %d", getCalls["show"])
	}
	if addedRatingKey != "show-plex-rk" {
		t.Errorf("addToWatchlist called with ratingKey=%q, want show-plex-rk (the SHOW's plex.tv rk, not episode's)", addedRatingKey)
	}
}

// TestAddItemToWatchlist_NonPlexGUIDErrors — items with legacy agent GUIDs
// (com.plexapp.agents.imdb://, com.plexapp.agents.thetvdb://, etc.) can't be
// directly added to the watchlist since those don't map to plex.tv catalog
// ratingKeys. Returns a clear error so the SPA can surface a useful message.
func TestAddItemToWatchlist_NonPlexGUIDErrors(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/library/metadata/legacy-rk" {
			w.Write([]byte(`{"MediaContainer":{"Metadata":[
				{"ratingKey":"legacy-rk","guid":"com.plexapp.agents.imdb://tt12345?lang=en","title":"Legacy Movie","type":"movie"}
			]}}`))
			return
		}
		http.NotFound(w, r)
	}))
	defer srv.Close()

	c := NewClient("client-id", "test")
	c.discoverBase = srv.URL
	plexSrv := &Server{BaseURL: srv.URL, AccessToken: "server-tok"}

	err := c.AddItemToWatchlist(plexSrv, "acct-tok", "legacy-rk")
	if err == nil {
		t.Fatal("expected error for non-plex:// GUID, got nil")
	}
	if !strings.Contains(err.Error(), "no plex.tv GUID") {
		t.Errorf("error message should explain the GUID issue, got: %v", err)
	}
}

// TestRemoveItemFromWatchlist_EpisodeRollsUpToShow — symmetric coverage of
// the remove path. Episode → walks to grandparent show → PUTs
// removeFromWatchlist?ratingKey=<showPlexTvRk>. Without this, ItemDetail's
// toggle silently sent the episode's plex.tv ratingKey to plex.tv and got
// 400 back.
func TestRemoveItemFromWatchlist_EpisodeRollsUpToShow(t *testing.T) {
	var removedRatingKey string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/library/metadata/episode-rk":
			w.Write([]byte(`{"MediaContainer":{"Metadata":[
				{"ratingKey":"episode-rk","guid":"plex://episode/episode-plex-rk","title":"Pilot","type":"episode","grandparentRatingKey":"show-rk"}
			]}}`))
		case "/library/metadata/show-rk":
			w.Write([]byte(`{"MediaContainer":{"Metadata":[
				{"ratingKey":"show-rk","guid":"plex://show/show-plex-rk","title":"The Show","type":"show"}
			]}}`))
		case "/actions/removeFromWatchlist":
			removedRatingKey = r.URL.Query().Get("ratingKey")
			w.Write([]byte(`{"MediaContainer":{"size":0}}`))
		default:
			http.NotFound(w, r)
		}
	}))
	defer srv.Close()

	c := NewClient("client-id", "test")
	c.discoverBase = srv.URL
	plexSrv := &Server{BaseURL: srv.URL, AccessToken: "server-tok"}

	if err := c.RemoveItemFromWatchlist(plexSrv, "acct-tok", "episode-rk"); err != nil {
		t.Fatalf("RemoveItemFromWatchlist: %v", err)
	}
	if removedRatingKey != "show-plex-rk" {
		t.Errorf("removeFromWatchlist called with ratingKey=%q, want show-plex-rk", removedRatingKey)
	}
}

// TestExtractPlexTvRatingKey — table-driven coverage of the GUID parser.
func TestExtractPlexTvRatingKey(t *testing.T) {
	cases := []struct {
		guid string
		want string
	}{
		{"plex://movie/abc123", "abc123"},
		{"plex://show/67a8bfbbc14f87b54f306104", "67a8bfbbc14f87b54f306104"},
		{"plex://show/abc?lang=en", "abc"},
		{"com.plexapp.agents.imdb://tt12345", ""},
		{"", ""},
		{"plex://", ""},
		{"plex://show/", ""},
	}
	for _, tc := range cases {
		got := extractPlexTvRatingKey(tc.guid)
		if got != tc.want {
			t.Errorf("extractPlexTvRatingKey(%q) = %q, want %q", tc.guid, got, tc.want)
		}
	}
}
