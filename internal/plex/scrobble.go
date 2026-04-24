package plex

import (
	"fmt"
	"net/url"
)

// Scrobble marks a rating key as fully watched on the given server. This is the
// universally-supported way to remove an item from Continue Watching — the
// trade-off is it also advances the episode's watched state, which is what most
// users expect when they dismiss something from CW.
//
// Endpoint: GET /:/scrobble?key=<ratingKey>&identifier=com.plexapp.plugins.library
func (c *Client) Scrobble(s *Server, ratingKey string) error {
	q := url.Values{
		"key":        []string{ratingKey},
		"identifier": []string{"com.plexapp.plugins.library"},
	}
	u := s.BaseURL + "/:/scrobble?" + q.Encode()
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
		return fmt.Errorf("scrobble %s: status %d", ratingKey, resp.StatusCode)
	}
	return nil
}
