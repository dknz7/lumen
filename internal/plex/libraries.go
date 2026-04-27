package plex

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"strings"
)

// mediaContainer is the envelope Plex wraps all library responses in.
type mediaContainer struct {
	MediaContainer struct {
		Directory []directoryWire `json:"Directory"`
		Metadata  []metadataWire  `json:"Metadata"`
	} `json:"MediaContainer"`
}

type directoryWire struct {
	Key   string `json:"key"`
	Title string `json:"title"`
	Type  string `json:"type"`
}

type metadataWire struct {
	RatingKey string `json:"ratingKey"`
	GUID      string `json:"guid"`
	// GuidArray absorbs Plex's capital-"Guid" array of external IDs (imdb/tmdb/tvdb)
	// so Go's case-insensitive json matching doesn't spill it into the GUID string
	// field and fail to unmarshal. Parsed in Session 5 for OMDB IMDB lookup.
	GuidArray []struct {
		ID string `json:"id"`
	} `json:"Guid"`
	Title                string `json:"title"`
	Type                 string `json:"type"`
	Year                 int    `json:"year"`
	Summary              string `json:"summary"`
	Thumb                string `json:"thumb"`
	Art                  string `json:"art"`
	Duration             int64  `json:"duration"`
	ViewOffset           int64  `json:"viewOffset"`
	AddedAt              int64  `json:"addedAt"`
	LastViewedAt         int64  `json:"lastViewedAt"`
	Index                int    `json:"index"`
	ParentIndex          int    `json:"parentIndex"`
	ParentTitle          string `json:"parentTitle"`
	ParentThumb          string `json:"parentThumb"`
	ParentRatingKey      string `json:"parentRatingKey"`
	GrandparentTitle     string `json:"grandparentTitle"`
	GrandparentThumb     string `json:"grandparentThumb"`
	GrandparentArt       string `json:"grandparentArt"`
	GrandparentRatingKey string `json:"grandparentRatingKey"`
	// PrimaryGuid is Plex's hub-clip pointer at the parent movie/show
	// (e.g. "plex://show/6424..."). Hub clip items don't populate a literal
	// parentRatingKey — hubs.go parses the trailing segment of this guid as
	// a fallback so DiscoverTile navigation lands on the parent's detail page.
	PrimaryGuid           string `json:"primaryGuid,omitempty"`
	ViewCount             int    `json:"viewCount"`
	ContentRating         string `json:"contentRating"`
	Tagline               string `json:"tagline"`
	OriginallyAvailableAt string `json:"originallyAvailableAt"`
	// StudioString absorbs Plex's lowercase "studio" string. Server-local
	// /library/metadata responses may also emit a capital-"Studio" array
	// (same case-collision pattern as guid/Guid handled above). Without
	// this absorber pair, Go's case-insensitive json matching could spill
	// the array into a Studio string field and fail to decode. Hubs only
	// emit the lowercase form in practice, but metadataWire is shared
	// between hub and server-local paths so we handle both shapes.
	StudioString string `json:"studio"`
	Studio       []struct {
		Tag string `json:"tag"`
	} `json:"Studio"`
	Media []Media `json:"Media"`
	Role  []struct {
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
	Extras struct {
		Metadata []struct {
			ExtraType int     `json:"extraType"` // 1 = trailer
			Title     string  `json:"title"`
			GUID      string  `json:"guid"` // e.g. "youtube://abc123"
			Media     []Media `json:"Media"`
		} `json:"Metadata"`
	} `json:"Extras"`
}

// GetLibraries returns all top-level library sections on the server.
func (c *Client) GetLibraries(s *Server) ([]Library, error) {
	mc, err := c.serverGet(s, "/library/sections", nil)
	if err != nil {
		return nil, err
	}
	out := make([]Library, 0, len(mc.MediaContainer.Directory))
	for _, d := range mc.MediaContainer.Directory {
		out = append(out, Library{
			ID:    d.Key,
			Key:   d.Key,
			Title: d.Title,
			Type:  d.Type,
		})
	}
	return out, nil
}

// GetItems lists items in a library section. libraryID is the section's numeric key.
func (c *Client) GetItems(s *Server, libraryID string, q ItemQuery) ([]Item, error) {
	qs := url.Values{}
	if q.Sort != "" {
		qs.Set("sort", q.Sort)
	}
	for k, v := range q.Filters {
		qs.Set(k, v)
	}
	if q.Size > 0 {
		qs.Set("X-Plex-Container-Start", strconv.Itoa(q.Start))
		qs.Set("X-Plex-Container-Size", strconv.Itoa(q.Size))
	}
	path := fmt.Sprintf("/library/sections/%s/all", libraryID)
	if len(qs) > 0 {
		path += "?" + qs.Encode()
	}
	mc, err := c.serverGet(s, path, nil)
	if err != nil {
		return nil, err
	}
	return metadataSliceToItems(mc.MediaContainer.Metadata), nil
}

// GetRecentlyAdded lists items using Plex's native recently-added ordering,
// which is what Plex Web's Home page uses. Distinct from GetItems sorted by
// addedAt — /recentlyAdded is a curated feed that may filter out certain items
// (collections, trailers, etc.) and respects Plex's own freshness heuristics.
// Used by spec §12.1's "Recently Released" Home shelves.
func (c *Client) GetRecentlyAdded(s *Server, libraryID string, size int) ([]Item, error) {
	path := fmt.Sprintf("/library/sections/%s/recentlyAdded", libraryID)
	if size > 0 {
		path += fmt.Sprintf("?X-Plex-Container-Start=0&X-Plex-Container-Size=%d", size)
	}
	mc, err := c.serverGet(s, path, nil)
	if err != nil {
		return nil, err
	}
	return metadataSliceToItems(mc.MediaContainer.Metadata), nil
}

// GetItem fetches a single item by ratingKey.
func (c *Client) GetItem(s *Server, ratingKey string) (Item, error) {
	mc, err := c.serverGet(s, "/library/metadata/"+ratingKey, nil)
	if err != nil {
		return Item{}, err
	}
	items := metadataSliceToItems(mc.MediaContainer.Metadata)
	if len(items) == 0 {
		return Item{}, fmt.Errorf("item %s not found", ratingKey)
	}
	return items[0], nil
}

// Search performs a cross-library search on a single server.
func (c *Client) Search(s *Server, query string) ([]Item, error) {
	path := "/search?" + url.Values{"query": []string{query}}.Encode()
	mc, err := c.serverGet(s, path, nil)
	if err != nil {
		return nil, err
	}
	return metadataSliceToItems(mc.MediaContainer.Metadata), nil
}

// serverGet issues a GET to the server with the per-server token applied, parsing
// the MediaContainer envelope.
func (c *Client) serverGet(s *Server, path string, extraHeaders http.Header) (*mediaContainer, error) {
	if s.BaseURL == "" {
		return nil, fmt.Errorf("server %q has no BaseURL — call PickConnection first", s.Name)
	}
	req, err := c.NewRequest("GET", s.BaseURL+path, nil)
	if err != nil {
		return nil, err
	}
	c.SetToken(req, s.AccessToken)
	for k, vals := range extraHeaders {
		for _, v := range vals {
			req.Header.Add(k, v)
		}
	}
	resp, err := c.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("%s %s: status %d", req.Method, path, resp.StatusCode)
	}
	var mc mediaContainer
	if err := json.NewDecoder(resp.Body).Decode(&mc); err != nil {
		return nil, err
	}
	return &mc, nil
}

// extractIMDBId picks the imdb:// id (e.g. "tt0111161") out of Plex's
// capital-Guid array. Returns empty string when no imdb entry is present.
func extractIMDBId(arr []struct{ ID string }) string {
	for _, g := range arr {
		if strings.HasPrefix(g.ID, "imdb://") {
			return strings.TrimPrefix(g.ID, "imdb://")
		}
	}
	return ""
}

func personsFromRole(in []struct {
	ID    int    `json:"id"`
	Tag   string `json:"tag"`
	Role  string `json:"role"`
	Thumb string `json:"thumb"`
}) []Person {
	out := make([]Person, 0, len(in))
	for _, p := range in {
		out = append(out, Person{ID: p.ID, Name: p.Tag, Tag: p.Role, Thumb: p.Thumb})
	}
	return out
}

func personsFromCrew(in []struct {
	ID    int    `json:"id"`
	Tag   string `json:"tag"`
	Thumb string `json:"thumb"`
}) []Person {
	out := make([]Person, 0, len(in))
	for _, p := range in {
		out = append(out, Person{ID: p.ID, Name: p.Tag, Thumb: p.Thumb})
	}
	return out
}

// trailerFromExtras returns the first trailer (extraType == 1) Plex
// reports for the item, normalising youtube:// guids and plex-hosted
// part keys into TrailerInfo. Returns nil when no trailer is present.
func trailerFromExtras(in []struct {
	ExtraType int     `json:"extraType"`
	Title     string  `json:"title"`
	GUID      string  `json:"guid"`
	Media     []Media `json:"Media"`
}) *TrailerInfo {
	for _, e := range in {
		if e.ExtraType != 1 {
			continue
		}
		ti := &TrailerInfo{Title: e.Title}
		if strings.HasPrefix(e.GUID, "youtube://") {
			ti.YouTubeID = strings.TrimPrefix(e.GUID, "youtube://")
			return ti
		}
		if len(e.Media) > 0 && len(e.Media[0].Part) > 0 {
			ti.PlexKey = e.Media[0].Part[0].Key
			return ti
		}
	}
	return nil
}

// toIDOnly strips json tags so extractIMDBId's tag-less parameter type
// can accept the result. Same semantic content; just a different
// anonymous-struct shape.
func toIDOnly(in []struct {
	ID string `json:"id"`
}) []struct{ ID string } {
	out := make([]struct{ ID string }, 0, len(in))
	for _, g := range in {
		out = append(out, struct{ ID string }{ID: g.ID})
	}
	return out
}

func metadataSliceToItems(mw []metadataWire) []Item {
	out := make([]Item, 0, len(mw))
	for _, m := range mw {
		out = append(out, Item{
			RatingKey:             m.RatingKey,
			GUID:                  m.GUID,
			Title:                 m.Title,
			Type:                  m.Type,
			Year:                  m.Year,
			Summary:               m.Summary,
			Thumb:                 m.Thumb,
			Art:                   m.Art,
			Duration:              m.Duration,
			ViewOffset:            m.ViewOffset,
			AddedAt:               m.AddedAt,
			LastViewedAt:          m.LastViewedAt,
			Index:                 m.Index,
			ParentIndex:           m.ParentIndex,
			ParentTitle:           m.ParentTitle,
			ParentThumb:           m.ParentThumb,
			GrandparentTitle:      m.GrandparentTitle,
			GrandparentThumb:      m.GrandparentThumb,
			GrandparentArt:        m.GrandparentArt,
			GrandparentRatingKey:  m.GrandparentRatingKey,
			ViewCount:             m.ViewCount,
			OriginallyAvailableAt: m.OriginallyAvailableAt,
			Media:                 m.Media,
			IMDBId:                extractIMDBId(toIDOnly(m.GuidArray)),
			Roles:                 personsFromRole(m.Role),
			Directors:             personsFromCrew(m.Director),
			Writers:               personsFromCrew(m.Writer),
			Trailer:               trailerFromExtras(m.Extras.Metadata),
		})
	}
	return out
}
