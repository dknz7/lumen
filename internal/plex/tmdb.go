package plex

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"time"
)

// TMDBClient is a thin wrapper around the TMDB v3 API. Distinct from
// plex.Client (no Plex identity headers) and OMDBClient (different host
// + endpoint family). Used by the Play Trailer feature on Item Detail
// to find a YouTube trailer ID for any item that exposes an IMDB id.
type TMDBClient struct {
	http   *http.Client
	apiKey string
	base   string // overridable for tests; default https://api.themoviedb.org
}

func NewTMDBClient(apiKey string) *TMDBClient {
	return &TMDBClient{
		http:   &http.Client{Timeout: 8 * time.Second},
		apiKey: apiKey,
		base:   "https://api.themoviedb.org",
	}
}

// findResponse is the shape of /3/find/{external_id}?external_source=imdb_id.
type findResponse struct {
	MovieResults []struct {
		ID int `json:"id"`
	} `json:"movie_results"`
	TVResults []struct {
		ID int `json:"id"`
	} `json:"tv_results"`
}

// videosResponse is the shape of /3/movie/{id}/videos and /3/tv/{id}/videos.
type videosResponse struct {
	Results []videoEntry `json:"results"`
}

type videoEntry struct {
	Key      string `json:"key"`
	Name     string `json:"name"`
	Site     string `json:"site"`
	Type     string `json:"type"` // "Trailer" | "Teaser" | "Clip" | "Featurette" | ...
	Official bool   `json:"official"`
}

// LookupTrailerByIMDBID resolves an IMDB id to TMDB and returns the best
// YouTube trailer key it can find. Returns ("", nil) when there's no
// matching TMDB entry or the entry has no YouTube trailers — caller
// renders the Plex Extras fallback. mediaType is "movie" or "show" (we
// translate "show" → TMDB's "tv" path internally).
//
// Selection order: official YouTube Trailer > unofficial YouTube Trailer
// > official YouTube Teaser > unofficial YouTube Teaser. Anything else
// is dropped.
func (c *TMDBClient) LookupTrailerByIMDBID(imdbID, mediaType string) (string, error) {
	if c.apiKey == "" {
		return "", fmt.Errorf("tmdb: no api key configured")
	}
	if imdbID == "" {
		return "", nil
	}
	if mediaType != "movie" && mediaType != "show" {
		return "", fmt.Errorf("tmdb: invalid mediaType %q (want 'movie' or 'show')", mediaType)
	}

	// Step 1 — IMDB → TMDB id via /3/find.
	findURL := fmt.Sprintf("%s/3/find/%s?external_source=imdb_id&api_key=%s",
		c.base, url.PathEscape(imdbID), url.QueryEscape(c.apiKey))
	resp, err := c.http.Get(findURL)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("tmdb find: status %d", resp.StatusCode)
	}
	var find findResponse
	if err := json.NewDecoder(resp.Body).Decode(&find); err != nil {
		return "", err
	}

	var tmdbID int
	var videosPath string
	if mediaType == "movie" {
		if len(find.MovieResults) == 0 {
			return "", nil
		}
		tmdbID = find.MovieResults[0].ID
		videosPath = "/3/movie/"
	} else {
		if len(find.TVResults) == 0 {
			return "", nil
		}
		tmdbID = find.TVResults[0].ID
		videosPath = "/3/tv/"
	}

	// Step 2 — TMDB id → videos.
	videosURL := fmt.Sprintf("%s%s%d/videos?api_key=%s",
		c.base, videosPath, tmdbID, url.QueryEscape(c.apiKey))
	resp2, err := c.http.Get(videosURL)
	if err != nil {
		return "", err
	}
	defer resp2.Body.Close()
	if resp2.StatusCode != http.StatusOK {
		return "", fmt.Errorf("tmdb videos: status %d", resp2.StatusCode)
	}
	var vids videosResponse
	if err := json.NewDecoder(resp2.Body).Decode(&vids); err != nil {
		return "", err
	}
	return pickBestTrailer(vids.Results), nil
}

// pickBestTrailer returns the YouTube key of the preferred trailer per the
// official-first ordering. Empty string when no YouTube trailer/teaser is
// present.
func pickBestTrailer(in []videoEntry) string {
	type rank struct {
		key   string
		score int // lower is better
	}
	best := rank{score: 9999}
	for _, v := range in {
		if v.Site != "YouTube" {
			continue
		}
		var s int
		switch v.Type {
		case "Trailer":
			s = 0
			if !v.Official {
				s = 1
			}
		case "Teaser":
			s = 2
			if !v.Official {
				s = 3
			}
		default:
			continue
		}
		if s < best.score {
			best = rank{key: v.Key, score: s}
		}
	}
	if best.score == 9999 {
		return ""
	}
	return best.key
}
