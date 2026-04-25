package plex

import (
	"fmt"
	"net/url"
)

// Scrobble marks a rating key as fully watched on the given server. Sets
// viewCount to 1 and advances the watched state — the item enters the user's
// watch history. This is the "Mark as Watched" action; one of its side effects
// is removing the item from Continue Watching (Plex excludes watched items
// from onDeck).
//
// Endpoint: GET /:/scrobble?key=<ratingKey>&identifier=com.plexapp.plugins.library
func (c *Client) Scrobble(s *Server, ratingKey string) error {
	return c.scrobbleAction(s, ratingKey, "scrobble")
}

// Unscrobble resets a rating key to "never watched, no progress". Sets
// viewCount=0 AND clears viewOffset on most Plex server versions — the item
// disappears from Continue Watching without polluting the user's watch
// history. This is the "Remove from Continue Watching" action.
//
// Endpoint: GET /:/unscrobble?key=<ratingKey>&identifier=com.plexapp.plugins.library
func (c *Client) Unscrobble(s *Server, ratingKey string) error {
	return c.scrobbleAction(s, ratingKey, "unscrobble")
}

// scrobbleAction is the shared transport for the (un)scrobble endpoints —
// they share path/auth shape, only the verb differs.
func (c *Client) scrobbleAction(s *Server, ratingKey string, action string) error {
	q := url.Values{
		"key":        []string{ratingKey},
		"identifier": []string{"com.plexapp.plugins.library"},
	}
	u := s.BaseURL + "/:/" + action + "?" + q.Encode()
	req, err := c.NewRequest("GET", u, nil)
	if err != nil {
		return err
	}
	c.SetToken(req, s.AccessToken)
	resp, err := c.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		return fmt.Errorf("%s %s: status %d", action, ratingKey, resp.StatusCode)
	}
	return nil
}
