package plex

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"
)

func TestReportTimeline_SendsExpectedQuery(t *testing.T) {
	var gotPath, gotQuery string
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		gotQuery = r.URL.RawQuery
		w.WriteHeader(http.StatusOK)
	}))
	defer ts.Close()

	c := NewClient("client-id", "0.1.0")
	srv := &Server{BaseURL: ts.URL, AccessToken: "secret"}

	err := c.ReportTimeline(srv, TimelineReport{
		RatingKey: "12345",
		State:     "playing",
		Position:  5 * time.Second,
		Duration:  60 * time.Second,
	})
	if err != nil {
		t.Fatal(err)
	}

	if gotPath != "/:/timeline" {
		t.Errorf("path = %q, want /:/timeline", gotPath)
	}
	q, _ := url.ParseQuery(gotQuery)
	if q.Get("ratingKey") != "12345" {
		t.Errorf("ratingKey = %q", q.Get("ratingKey"))
	}
	if q.Get("state") != "playing" {
		t.Errorf("state = %q", q.Get("state"))
	}
	if !strings.HasPrefix(q.Get("time"), "5000") {
		t.Errorf("time = %q, want 5000ms", q.Get("time"))
	}
	if q.Get("duration") != "60000" {
		t.Errorf("duration = %q, want 60000ms", q.Get("duration"))
	}
}
