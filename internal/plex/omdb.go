package plex

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"time"
)

// OMDBClient is a thin wrapper around the OMDB HTTP API. Distinct from
// plex.Client because OMDB is unrelated to Plex auth/identity headers.
type OMDBClient struct {
	http   *http.Client
	apiKey string
}

func NewOMDBClient(apiKey string) *OMDBClient {
	return &OMDBClient{
		http:   &http.Client{Timeout: 8 * time.Second},
		apiKey: apiKey,
	}
}

// OMDBRating carries the subset of OMDB's response Lumen renders.
type OMDBRating struct {
	IMDBId     string `json:"imdbID"`
	Title      string `json:"title,omitempty"`
	Year       string `json:"year,omitempty"`
	Rated      string `json:"rated,omitempty"`      // MPAA
	IMDBRating string `json:"imdbRating,omitempty"` // "8.4" or "N/A"
	IMDBVotes  string `json:"imdbVotes,omitempty"`
}

// omdbWire is the raw OMDB response shape; field names are PascalCase.
type omdbWire struct {
	IMDBId     string `json:"imdbID"`
	Title      string `json:"Title"`
	Year       string `json:"Year"`
	Rated      string `json:"Rated"`
	IMDBRating string `json:"imdbRating"`
	IMDBVotes  string `json:"imdbVotes"`
	Response   string `json:"Response"` // "True" / "False"
	Error      string `json:"Error"`
}

// LookupByIMDBId calls OMDB by IMDB id (e.g. "tt0111161"). Returns nil
// (without error) when OMDB returns Response:"False" — a common case
// for items whose IMDB id Plex couldn't map.
func (c *OMDBClient) LookupByIMDBId(imdbId string) (*OMDBRating, error) {
	if c.apiKey == "" {
		return nil, fmt.Errorf("omdb: no api key configured")
	}
	q := url.Values{"i": []string{imdbId}, "apikey": []string{c.apiKey}}
	u := "https://www.omdbapi.com/?" + q.Encode()
	resp, err := c.http.Get(u)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("omdb: status %d", resp.StatusCode)
	}
	var w omdbWire
	if err := json.NewDecoder(resp.Body).Decode(&w); err != nil {
		return nil, err
	}
	if w.Response != "True" {
		return nil, nil
	}
	return &OMDBRating{
		IMDBId:     w.IMDBId,
		Title:      w.Title,
		Year:       w.Year,
		Rated:      w.Rated,
		IMDBRating: w.IMDBRating,
		IMDBVotes:  w.IMDBVotes,
	}, nil
}
