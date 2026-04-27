package plex

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
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

// summaryField accepts either a plain string ("summary": "...") OR an
// array form ("summary": [...]) on Plex discover.provider.plex.tv
// responses. Plex emits both shapes depending on the item — we don't
// have a documented schema, so we try the most common shapes and
// fall back to empty string on anything we don't recognise (the
// boundary error log will dump the raw body so we can iterate).
//
// Known shapes that produce useful text:
//   - string                       → use directly
//   - []string                     → join with double newline
//   - []struct{ Text string }      → join the .Text values
//   - []struct{ Summary string }   → join the .Summary values
//
// Anything else: silently return empty. The page renders without a
// summary rather than 502'ing — better degraded UX.
type summaryField string

func (s *summaryField) UnmarshalJSON(data []byte) error {
	raw := bytes.TrimSpace(data)
	if len(raw) == 0 || bytes.Equal(raw, []byte("null")) {
		return nil
	}
	switch raw[0] {
	case '"':
		var str string
		if err := json.Unmarshal(raw, &str); err != nil {
			return err
		}
		*s = summaryField(str)
		return nil
	case '[':
		// Try the most common array shapes in turn.
		var asStrings []string
		if err := json.Unmarshal(raw, &asStrings); err == nil && len(asStrings) > 0 {
			*s = summaryField(strings.Join(asStrings, "\n\n"))
			return nil
		}
		var asTextObjects []struct {
			Text string `json:"text"`
		}
		if err := json.Unmarshal(raw, &asTextObjects); err == nil {
			parts := make([]string, 0, len(asTextObjects))
			for _, e := range asTextObjects {
				if e.Text != "" {
					parts = append(parts, e.Text)
				}
			}
			if len(parts) > 0 {
				*s = summaryField(strings.Join(parts, "\n\n"))
				return nil
			}
		}
		var asSummaryObjects []struct {
			Summary string `json:"summary"`
		}
		if err := json.Unmarshal(raw, &asSummaryObjects); err == nil {
			parts := make([]string, 0, len(asSummaryObjects))
			for _, e := range asSummaryObjects {
				if e.Summary != "" {
					parts = append(parts, e.Summary)
				}
			}
			if len(parts) > 0 {
				*s = summaryField(strings.Join(parts, "\n\n"))
				return nil
			}
		}
		// Empty array, or array of an unknown shape. Log nothing here
		// (we don't have logger context); rely on the boundary scrub +
		// raw-body dump (Fix #2) for diagnosis. Return nil so the
		// overall decode succeeds and the page renders.
		return nil
	default:
		// Number, bool, object — unexpected. Same fallback: empty.
		return nil
	}
}

type discoverItemWire struct {
	MediaContainer struct {
		Metadata []struct {
			RatingKey             string       `json:"ratingKey"`
			GUID                  string       `json:"guid"`
			Title                 string       `json:"title"`
			Type                  string       `json:"type"`
			Year                  int          `json:"year"`
			Summary               summaryField `json:"summary"`
			Tagline               string       `json:"tagline"`
			ContentRating         string       `json:"contentRating"`
			OriginallyAvailableAt string       `json:"originallyAvailableAt"`
			Duration              int          `json:"duration"`
			Thumb                 string       `json:"thumb"`
			Art                   string       `json:"art"`
			AddedAt               int64        `json:"addedAt"`
			PublicPagesURL        string       `json:"publicPagesURL"`

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

			// RatingScalar absorbs Plex's lowercase "rating" float
			// (duplicates the Rotten Tomatoes critic value from
			// Rating[]). Same Session 3 / 5 / 6 case-insensitive json
			// absorber pattern as guid/Guid and studio/Studio: without
			// this sink, Go's case-insensitive matcher slots the scalar
			// into the Rating array below and the whole Metadata decode
			// fails. We don't use the scalar — just absorb it.
			RatingScalar float64 `json:"rating"`

			Rating []struct {
				Image string  `json:"image"`
				Type  string  `json:"type"`
				Value float64 `json:"value"`
			} `json:"Rating"`

			// Cast/Crew IDs are UUID-shaped strings on the discover.provider.plex.tv
			// surface — server-local Plex returns int IDs (and the existing helpers
			// personsFromRole/personsFromCrew in libraries.go expect that shape).
			// We can't reuse those helpers; declare a string-id local shape and
			// inline-adapt below. Same Session 5 Phase A.5/A.6 lesson as Part.ID.
			Role []struct {
				ID    string `json:"id"`
				Tag   string `json:"tag"`
				Role  string `json:"role"`
				Thumb string `json:"thumb"`
			} `json:"Role"`
			Director []struct {
				ID    string `json:"id"`
				Tag   string `json:"tag"`
				Thumb string `json:"thumb"`
			} `json:"Director"`
			Writer []struct {
				ID    string `json:"id"`
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
	body, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20)) // 1 MiB safety cap
	if err != nil {
		return nil, fmt.Errorf("discover item %s: read body: %w", plexTvRatingKey, err)
	}
	var w discoverItemWire
	if err := json.Unmarshal(body, &w); err != nil {
		// Surface the first 512 bytes of the upstream response in the error
		// string so the boundary log scrub (handler) shows the wire shape
		// that broke the decode. Plex's polymorphic fields keep surprising us;
		// having the raw bytes in the log eliminates DevTools roundtrips
		// when iterating on wire fixes.
		snippet := body
		if len(snippet) > 512 {
			snippet = snippet[:512]
		}
		return nil, fmt.Errorf("discover item %s: decode failed: %w (body[0:%d]: %q)",
			plexTvRatingKey, err, len(snippet), snippet)
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

	// Person.ID is int (server-local convention); discover IDs are UUID
	// strings. We don't surface Person.ID in the SPA, so dropping it (zero
	// value) is safe. Inline-adapt rather than reuse personsFromRole /
	// personsFromCrew which expect int-id source types.
	cast := make([]Person, 0, len(m.Role))
	for _, p := range m.Role {
		cast = append(cast, Person{
			Name:  p.Tag,
			Tag:   p.Role,
			Thumb: p.Thumb,
		})
	}
	directors := make([]Person, 0, len(m.Director))
	for _, p := range m.Director {
		directors = append(directors, Person{Name: p.Tag, Thumb: p.Thumb})
	}
	writers := make([]Person, 0, len(m.Writer))
	for _, p := range m.Writer {
		writers = append(writers, Person{Name: p.Tag, Thumb: p.Thumb})
	}

	return &DiscoverItem{
		RatingKey:             m.RatingKey,
		GUID:                  m.GUID,
		IMDBID:                imdbID,
		Title:                 m.Title,
		Type:                  m.Type,
		Year:                  m.Year,
		Summary:               string(m.Summary),
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
		Cast:                  cast,
		Directors:             directors,
		Writers:               writers,
	}, nil
}
