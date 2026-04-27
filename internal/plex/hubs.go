package plex

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
)

// GetHub calls the plex.tv Discover hubs endpoint for a given namespace + slug.
// Namespace is "home" or "watchlist".
//
// Request shape mirrors Plex Web's call (Session 5 post-smoke DevTools
// capture): includeMeta=1 enables richer metadata fields, and the
// X-Plex-Container-Size param is required for some clip-type hubs (e.g.
// home/trending-trailers) that returned empty without it.
func (c *Client) GetHub(namespace, slug, accountToken string) ([]HubItem, error) {
	qs := url.Values{
		"contentDirectoryID":     []string{namespace},
		"includeMeta":            []string{"1"},
		"X-Plex-Container-Start": []string{"0"},
		"X-Plex-Container-Size":  []string{"50"},
	}
	u := fmt.Sprintf("%s/hubs/sections/%s/%s?%s", c.discoverBase, namespace, slug, qs.Encode())
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
		return nil, fmt.Errorf("hub %s/%s: status %d", namespace, slug, resp.StatusCode)
	}
	var mc mediaContainer
	if err := json.NewDecoder(resp.Body).Decode(&mc); err != nil {
		return nil, err
	}
	out := make([]HubItem, 0, len(mc.MediaContainer.Metadata))
	for _, m := range mc.MediaContainer.Metadata {
		// IMDB id absorbed from the Guid[] array on each hub item. Same
		// helper movies/shows + the discover-item endpoint use.
		imdbID := extractIMDBId(toIDOnly(m.GuidArray))
		out = append(out, HubItem{
			GUID:                  m.GUID,
			RatingKey:             m.RatingKey,
			Title:                 m.Title,
			Type:                  m.Type,
			Year:                  m.Year,
			Thumb:                 m.Thumb,
			IMDBID:                imdbID,
			ParentRatingKey:       m.ParentRatingKey,
			GrandparentRatingKey:  m.GrandparentRatingKey,
			ContentRating:         m.ContentRating,
			Studio:                m.StudioString,
			Tagline:               m.Tagline,
			AddedAt:               m.AddedAt,
			OriginallyAvailableAt: m.OriginallyAvailableAt,
		})
	}
	return out, nil
}
