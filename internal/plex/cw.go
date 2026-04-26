package plex

import (
	"fmt"
	"net/url"
)

// RemoveFromContinueWatching tells Plex to remove the show containing this
// item from the user's Continue Watching shelf. Plex's internal logic
// handles "whole show removal" semantics when given any episode ratingKey
// from that show. Mirrors Plex Web's PUT /actions/removeFromContinueWatching
// call (header-only X-Plex-Token, ratingKey via query string).
//
// Returns nil on Plex 200, error otherwise. Removal syncs cross-device via
// Plex (Plex Web, mobile apps, smart TV apps all reflect it on next fetch).
func (c *Client) RemoveFromContinueWatching(s *Server, ratingKey string) error {
	q := url.Values{"ratingKey": []string{ratingKey}}
	u := fmt.Sprintf("%s/actions/removeFromContinueWatching?%s", s.BaseURL, q.Encode())
	req, err := c.NewRequest("PUT", u, nil)
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
		return fmt.Errorf("removeFromContinueWatching %s: status %d", ratingKey, resp.StatusCode)
	}
	return nil
}
