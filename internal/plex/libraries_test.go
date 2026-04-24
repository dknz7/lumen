package plex

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// fakePlexServer returns a test server that responds to the four library
// endpoints with canned JSON.
func fakePlexServer(t *testing.T) *httptest.Server {
	t.Helper()
	mux := http.NewServeMux()
	mux.HandleFunc("/library/sections", func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("X-Plex-Token") != "srv-tok" {
			http.Error(w, "no token", 401)
			return
		}
		w.Write([]byte(`{"MediaContainer":{"Directory":[
			{"key":"1","title":"Movies","type":"movie"},
			{"key":"2","title":"TV Shows","type":"show"}
		]}}`))
	})
	mux.HandleFunc("/library/sections/1/all", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`{"MediaContainer":{"Metadata":[
			{"ratingKey":"100","guid":"plex://movie/abc","title":"Dune","type":"movie","year":2021}
		]}}`))
	})
	mux.HandleFunc("/library/metadata/100", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`{"MediaContainer":{"Metadata":[
			{"ratingKey":"100","guid":"plex://movie/abc","title":"Dune","type":"movie","year":2021,"summary":"Sand worm."}
		]}}`))
	})
	mux.HandleFunc("/search", func(w http.ResponseWriter, r *http.Request) {
		if !strings.Contains(r.URL.RawQuery, "query=dune") {
			http.Error(w, "missing query", 400)
			return
		}
		w.Write([]byte(`{"MediaContainer":{"Metadata":[{"ratingKey":"100","title":"Dune","type":"movie"}]}}`))
	})
	return httptest.NewServer(mux)
}

func TestGetLibraries(t *testing.T) {
	fake := fakePlexServer(t)
	defer fake.Close()

	c := NewClient("id", "1.0.0")
	s := &Server{BaseURL: fake.URL, AccessToken: "srv-tok"}
	libs, err := c.GetLibraries(s)
	if err != nil {
		t.Fatal(err)
	}
	if len(libs) != 2 || libs[0].Title != "Movies" {
		t.Fatalf("got %+v", libs)
	}
}

func TestGetItemsReturnsMetadata(t *testing.T) {
	fake := fakePlexServer(t)
	defer fake.Close()
	c := NewClient("id", "1.0.0")
	s := &Server{BaseURL: fake.URL, AccessToken: "srv-tok"}
	items, err := c.GetItems(s, "1", ItemQuery{})
	if err != nil {
		t.Fatal(err)
	}
	if len(items) != 1 || items[0].Title != "Dune" || items[0].Year != 2021 {
		t.Fatalf("got %+v", items)
	}
}

func TestGetItem(t *testing.T) {
	fake := fakePlexServer(t)
	defer fake.Close()
	c := NewClient("id", "1.0.0")
	s := &Server{BaseURL: fake.URL, AccessToken: "srv-tok"}
	it, err := c.GetItem(s, "100")
	if err != nil {
		t.Fatal(err)
	}
	if it.Summary != "Sand worm." {
		t.Fatalf("got %+v", it)
	}
}

func TestSearch(t *testing.T) {
	fake := fakePlexServer(t)
	defer fake.Close()
	c := NewClient("id", "1.0.0")
	s := &Server{BaseURL: fake.URL, AccessToken: "srv-tok"}
	items, err := c.Search(s, "dune")
	if err != nil {
		t.Fatal(err)
	}
	if len(items) != 1 || items[0].Title != "Dune" {
		t.Fatalf("got %+v", items)
	}
}
