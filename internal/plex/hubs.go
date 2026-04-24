package plex

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
)

// GetHub calls the plex.tv Discover hubs endpoint for a given namespace + slug.
// Namespace is "home" or "watchlist".
func (c *Client) GetHub(namespace, slug, accountToken string) ([]HubItem, error) {
	qs := url.Values{"contentDirectoryID": []string{namespace}}
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
		out = append(out, HubItem{
			GUID:      m.GUID,
			RatingKey: m.RatingKey,
			Title:     m.Title,
			Type:      m.Type,
			Year:      m.Year,
		})
	}
	return out, nil
}
