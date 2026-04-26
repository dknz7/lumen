package plex

import (
	"encoding/json"
	"fmt"
	"net/http"
)

// DiscoverItem is the curated shape Lumen presents for a plex.tv-source item
// (movies/shows from Recommended/Discover/Watchlist that may not be on any
// local server). Distinct from server-local Item: no Media/Part chain (the
// title may not have been released yet) but richer marketing metadata —
// tagline, multiple Rating sources (Tomatometer + audience + IMDB),
// alternate image variants.
type DiscoverItem struct {
	RatingKey             string           `json:"ratingKey"`
	GUID                  string           `json:"guid"`
	IMDBID                string           `json:"imdbId,omitempty"`
	Title                 string           `json:"title"`
	Type                  string           `json:"type"`
	Year                  int              `json:"year,omitempty"`
	Summary               string           `json:"summary,omitempty"`
	Tagline               string           `json:"tagline,omitempty"`
	ContentRating         string           `json:"contentRating,omitempty"`
	Studios               []string         `json:"studios,omitempty"` // from Studio[].tag — richer than top-level
	OriginallyAvailableAt string           `json:"originallyAvailableAt,omitempty"`
	Duration              int              `json:"duration,omitempty"` // milliseconds
	Thumb                 string           `json:"thumb,omitempty"`
	Art                   string           `json:"art,omitempty"`
	AddedAt               int64            `json:"addedAt,omitempty"`
	PublicPagesURL        string           `json:"publicPagesURL,omitempty"`
	Genres                []string         `json:"genres,omitempty"`    // from Genre[].tag
	Ratings               []DiscoverRating `json:"ratings,omitempty"`   // from Rating[]
	Cast                  []Person         `json:"cast,omitempty"`      // from Role[]
	Directors             []Person         `json:"directors,omitempty"` // from Director[]
	Writers               []Person         `json:"writers,omitempty"`   // from Writer[]
}

// DiscoverRating is one of the Rating[] entries — typically Plex returns
// 3-4 (RT critic + RT audience + IMDB audience + TMDB audience). The image
// scheme tells the SPA which logo/badge to render.
type DiscoverRating struct {
	Type  string  `json:"type"`  // "critic" | "audience"
	Image string  `json:"image"` // "rottentomatoes://image.rating.rotten", "imdb://image.rating", etc.
	Value float64 `json:"value"`
}

type discoverItemWire struct {
	MediaContainer struct {
		Metadata []struct {
			RatingKey             string `json:"ratingKey"`
			GUID                  string `json:"guid"`
			Title                 string `json:"title"`
			Type                  string `json:"type"`
			Year                  int    `json:"year"`
			Summary               string `json:"summary"`
			Tagline               string `json:"tagline"`
			ContentRating         string `json:"contentRating"`
			OriginallyAvailableAt string `json:"originallyAvailableAt"`
			Duration              int    `json:"duration"`
			Thumb                 string `json:"thumb"`
			Art                   string `json:"art"`
			AddedAt               int64  `json:"addedAt"`
			PublicPagesURL        string `json:"publicPagesURL"`

			// Guid (capital) is the absorber array of external IDs (imdb/tmdb/tvdb).
			// Plex also emits a lowercase "guid" string at the same level — Go's
			// case-insensitive json matching means we must declare both fields
			// (above + here) so they don't collide; same Session 3 / 5 gotcha.
			Guid []struct {
				ID string `json:"id"`
			} `json:"Guid"`

			Genre []struct {
				Tag string `json:"tag"`
			} `json:"Genre"`

			// StudioString absorbs Plex's lowercase "studio" string so Go's
			// case-insensitive json matching doesn't try to slot it into
			// Studio (the rich array) and fail to decode. We only want the
			// array — same Session 3 / 5 trick used for Guid vs guid.
			StudioString string `json:"studio"`

			// Studio is the rich array form. Plex emits this AND the top-level
			// "studio" string above which duplicates the first element here.
			Studio []struct {
				Tag string `json:"tag"`
			} `json:"Studio"`

			Rating []struct {
				Image string  `json:"image"`
				Type  string  `json:"type"`
				Value float64 `json:"value"`
			} `json:"Rating"`

			// Cast/Crew shapes match server-local Plex Item structure so we can
			// reuse personsFromRole / personsFromCrew from libraries.go.
			Role []struct {
				ID    int    `json:"id"`
				Tag   string `json:"tag"`
				Role  string `json:"role"`
				Thumb string `json:"thumb"`
			} `json:"Role"`
			Director []struct {
				ID    int    `json:"id"`
				Tag   string `json:"tag"`
				Thumb string `json:"thumb"`
			} `json:"Director"`
			Writer []struct {
				ID    int    `json:"id"`
				Tag   string `json:"tag"`
				Thumb string `json:"thumb"`
			} `json:"Writer"`
		} `json:"Metadata"`
	} `json:"MediaContainer"`
}

// GetDiscoverItem fetches rich metadata for a plex.tv ratingKey via
// discover.provider.plex.tv. plexTvRatingKey is the trailing segment of a
// `plex://movie/<id>` GUID (e.g. "64dd290c84713e6f8ba2874b"). Header-only
// auth — never put the token on the URL query for non-image API calls.
func (c *Client) GetDiscoverItem(accountToken, plexTvRatingKey string) (*DiscoverItem, error) {
	u := fmt.Sprintf("%s/library/metadata/%s?includeMeta=1", c.discoverBase, plexTvRatingKey)
	req, err := c.NewRequest("GET", u, nil)
	if err != nil {
		return nil, err
	}
	c.SetToken(req, accountToken)
	resp, err := c.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("discover item %s: status %d", plexTvRatingKey, resp.StatusCode)
	}
	var w discoverItemWire
	if err := json.NewDecoder(resp.Body).Decode(&w); err != nil {
		return nil, err
	}
	if len(w.MediaContainer.Metadata) == 0 {
		return nil, fmt.Errorf("discover item %s: no metadata", plexTvRatingKey)
	}
	m := w.MediaContainer.Metadata[0]

	genres := make([]string, 0, len(m.Genre))
	for _, g := range m.Genre {
		if g.Tag != "" {
			genres = append(genres, g.Tag)
		}
	}
	studios := make([]string, 0, len(m.Studio))
	for _, s := range m.Studio {
		if s.Tag != "" {
			studios = append(studios, s.Tag)
		}
	}
	// Drop fully-empty Rating rows (Plex occasionally emits placeholders).
	ratings := make([]DiscoverRating, 0, len(m.Rating))
	for _, r := range m.Rating {
		if r.Value > 0 || r.Image != "" {
			ratings = append(ratings, DiscoverRating{Type: r.Type, Image: r.Image, Value: r.Value})
		}
	}

	// IMDB id absorbed from the Guid[] array. Reuse the same helper movies/
	// shows go through; toIDOnly strips the json struct tags so the
	// helper's tag-less parameter type accepts the value.
	tagless := make([]struct{ ID string }, 0, len(m.Guid))
	for _, g := range m.Guid {
		tagless = append(tagless, struct{ ID string }{ID: g.ID})
	}
	imdbID := extractIMDBId(tagless)

	return &DiscoverItem{
		RatingKey:             m.RatingKey,
		GUID:                  m.GUID,
		IMDBID:                imdbID,
		Title:                 m.Title,
		Type:                  m.Type,
		Year:                  m.Year,
		Summary:               m.Summary,
		Tagline:               m.Tagline,
		ContentRating:         m.ContentRating,
		Studios:               studios,
		OriginallyAvailableAt: m.OriginallyAvailableAt,
		Duration:              m.Duration,
		Thumb:                 m.Thumb,
		Art:                   m.Art,
		AddedAt:               m.AddedAt,
		PublicPagesURL:        m.PublicPagesURL,
		Genres:                genres,
		Ratings:               ratings,
		Cast:                  personsFromRole(m.Role),
		Directors:             personsFromCrew(m.Director),
		Writers:               personsFromCrew(m.Writer),
	}, nil
}
