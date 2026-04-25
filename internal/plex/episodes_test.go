package plex

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestNextEpisode_FindsNextInSeries(t *testing.T) {
	episodes := []Item{
		{RatingKey: "ep1", ParentIndex: 1, Index: 1},
		{RatingKey: "ep2", ParentIndex: 1, Index: 2},
		{RatingKey: "ep3", ParentIndex: 1, Index: 3},
		{RatingKey: "ep4", ParentIndex: 2, Index: 1},
	}
	body := map[string]any{
		"MediaContainer": map[string]any{
			"Metadata": episodes,
		},
	}
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(body)
	}))
	defer ts.Close()

	c := NewClient("client-id", "0.1.0")
	srv := &Server{BaseURL: ts.URL, AccessToken: "secret"}

	next, err := c.NextEpisode(srv, "show-99", "ep2")
	if err != nil {
		t.Fatal(err)
	}
	if next == nil {
		t.Fatal("expected ep3, got nil")
	}
	if next.RatingKey != "ep3" {
		t.Errorf("next = %s, want ep3", next.RatingKey)
	}
}

func TestNextEpisode_LastEpisodeReturnsNil(t *testing.T) {
	episodes := []Item{{RatingKey: "ep1", ParentIndex: 1, Index: 1}}
	body := map[string]any{
		"MediaContainer": map[string]any{"Metadata": episodes},
	}
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(body)
	}))
	defer ts.Close()

	c := NewClient("client-id", "0.1.0")
	srv := &Server{BaseURL: ts.URL, AccessToken: "secret"}

	next, err := c.NextEpisode(srv, "show-1", "ep1")
	if err != nil {
		t.Fatal(err)
	}
	if next != nil {
		t.Errorf("expected nil for last episode, got %v", next)
	}
}
