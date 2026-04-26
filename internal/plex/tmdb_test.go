package plex

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestTMDBLookupTrailerByIMDBID_Movie(t *testing.T) {
	hits := map[string]int{}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits[r.URL.Path]++
		switch r.URL.Path {
		case "/3/find/tt0111161":
			if r.URL.Query().Get("external_source") != "imdb_id" {
				http.Error(w, "missing external_source", http.StatusBadRequest)
				return
			}
			if r.URL.Query().Get("api_key") != "test-key" {
				http.Error(w, "missing api_key", http.StatusBadRequest)
				return
			}
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"movie_results":[{"id":278,"title":"The Shawshank Redemption"}],"tv_results":[]}`))
		case "/3/movie/278/videos":
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{
				"results": [
					{"key":"NmzuHjWmXOc","name":"Final Trailer","site":"YouTube","type":"Trailer","official":true},
					{"key":"6hB3S9bIaco","name":"Teaser","site":"YouTube","type":"Teaser","official":true}
				]
			}`))
		default:
			http.NotFound(w, r)
		}
	}))
	defer srv.Close()

	c := NewTMDBClient("test-key")
	c.base = srv.URL
	yt, err := c.LookupTrailerByIMDBID("tt0111161", "movie")
	if err != nil {
		t.Fatalf("LookupTrailerByIMDBID: %v", err)
	}
	if yt != "NmzuHjWmXOc" {
		t.Errorf("got %q, want %q (the official Trailer)", yt, "NmzuHjWmXOc")
	}
	if hits["/3/find/tt0111161"] != 1 || hits["/3/movie/278/videos"] != 1 {
		t.Errorf("expected exactly one hit per endpoint; got %v", hits)
	}
}

func TestTMDBLookupTrailerByIMDBID_Show(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/3/find/tt2861424":
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"movie_results":[],"tv_results":[{"id":61222,"name":"Rick and Morty"}]}`))
		case "/3/tv/61222/videos":
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{
				"results": [
					{"key":"abcDEF12345","name":"Season 7 Trailer","site":"YouTube","type":"Trailer","official":true}
				]
			}`))
		default:
			http.NotFound(w, r)
		}
	}))
	defer srv.Close()

	c := NewTMDBClient("test-key")
	c.base = srv.URL
	yt, err := c.LookupTrailerByIMDBID("tt2861424", "show")
	if err != nil {
		t.Fatalf("LookupTrailerByIMDBID: %v", err)
	}
	if yt != "abcDEF12345" {
		t.Errorf("got %q, want abcDEF12345", yt)
	}
}

func TestTMDBLookupTrailerByIMDBID_NoTrailer(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/3/find/tt9999999":
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"movie_results":[{"id":999}],"tv_results":[]}`))
		case "/3/movie/999/videos":
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"results": []}`))
		default:
			http.NotFound(w, r)
		}
	}))
	defer srv.Close()

	c := NewTMDBClient("test-key")
	c.base = srv.URL
	yt, err := c.LookupTrailerByIMDBID("tt9999999", "movie")
	if err != nil {
		t.Fatalf("expected nil err on no-trailer; got %v", err)
	}
	if yt != "" {
		t.Errorf("expected empty string on no trailer; got %q", yt)
	}
}
