package plex

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"
)

// GetHub calls the plex.tv Discover hubs endpoint for a given namespace + slug.
// Namespace is "home" or "watchlist".
//
// Request shape mirrors Plex Web's call (a real DevTools
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
		// Plex injects ad slots as "placeholder" items in some hubs (Coming
		// Soon being the confirmed example — see the captured fixture). They
		// carry no title/thumb and only a stub "url" pointing at /ad/tile.
		// Drop them so the SPA never tries to render an empty card.
		if m.Type == "placeholder" {
			continue
		}
		// IMDB id absorbed from the Guid[] array on each hub item. Same
		// helper movies/shows + the discover-item endpoint use.
		imdbID := extractIMDBId(toIDOnly(m.GuidArray))
		// HLS URL — clip items on home/trending-trailers carry their own
		// native HLS playback URL inside Media[0].Part[0].key (path-style,
		// e.g. /library/metadata/.../extras/.../parts/hls.m3u8). Qualify it
		// against the Discover host and return it WITHOUT credentials: the
		// server layer swaps it for an opaque /api/hls/<handle> before the SPA
		// ever sees it. Putting the account token in here used to hand the
		// browser Lumen's broadest-scoped credential.
		hlsURL := ""
		if len(m.Media) > 0 && len(m.Media[0].Part) > 0 {
			key := m.Media[0].Part[0].Key
			if key != "" {
				if strings.HasPrefix(key, "http://") || strings.HasPrefix(key, "https://") {
					hlsURL = key
				} else {
					hlsURL = c.discoverBase + key
				}
			}
		}
		// Plex's hub clip items (e.g. New Trailers / Trending Trailers) point
		// at their parent movie/show via primaryGuid (e.g. "plex://show/6424...")
		// rather than a literal parentRatingKey. Parse the trailing segment as
		// a fallback so DiscoverTile's href() cascade lands on the parent's
		// Discover Item Detail page instead of 404-ing on the clip's own rk.
		// Server-local items and clips that DO populate parentRatingKey are
		// unaffected — the fallback only fires when the field is empty.
		parentRk := m.ParentRatingKey
		if parentRk == "" && m.Type == "clip" && m.PrimaryGuid != "" {
			if i := strings.LastIndex(m.PrimaryGuid, "/"); i >= 0 && i < len(m.PrimaryGuid)-1 {
				parentRk = m.PrimaryGuid[i+1:]
			}
		}
		out = append(out, HubItem{
			GUID:                  m.GUID,
			RatingKey:             m.RatingKey,
			Title:                 m.Title,
			Type:                  m.Type,
			Year:                  m.Year,
			Thumb:                 m.Thumb,
			IMDBID:                imdbID,
			ParentRatingKey:       parentRk,
			GrandparentRatingKey:  m.GrandparentRatingKey,
			ContentRating:         m.ContentRating,
			Studio:                m.StudioString,
			Tagline:               m.Tagline,
			AddedAt:               m.AddedAt,
			OriginallyAvailableAt: m.OriginallyAvailableAt,
			ParentTitle:           m.ParentTitle,
			ParentIndex:           m.ParentIndex,
			Index:                 m.Index,
			GrandparentTitle:      m.GrandparentTitle,
			HLSUrl:                hlsURL,
		})
	}
	return out, nil
}
