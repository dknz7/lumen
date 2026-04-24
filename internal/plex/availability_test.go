package plex

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestGetAvailabilityBuildsURLAndParses(t *testing.T) {
	var gotQuery string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotQuery = r.URL.RawQuery
		w.Write([]byte(`{"MediaContainer":{"Metadata":[
			{"ratingKey":"100","guid":"plex://movie/abc","title":"Dune","Media":[
				{"container":"mkv","videoResolution":"2160","bitrate":25000,
				 "Part":[{"key":"/library/parts/1/1/file.mkv","size":9876543210,"container":"mkv"}]}
			]}
		]}}`))
	}))
	defer srv.Close()

	c := NewClient("id", "1.0.0")
	s := &Server{BaseURL: srv.URL, AccessToken: "srv-tok", Name: "Test"}
	matches, err := c.GetAvailability(s, "plex://movie/abc")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(gotQuery, "guid=plex") {
		t.Errorf("query: %q", gotQuery)
	}
	if len(matches) != 1 {
		t.Fatalf("len=%d", len(matches))
	}
	m := matches[0]
	if m.RatingKey != "100" || m.Resolution != "2160" || m.Container != "mkv" || m.Size != 9876543210 {
		t.Errorf("match: %+v", m)
	}
}
