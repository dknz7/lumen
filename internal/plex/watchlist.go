package plex

import (
	"encoding/json"
	"fmt"
	"net/http"
)

// WatchlistItem is one entry on the user's plex.tv Watchlist. Distinct
// from HubItem (cast might overlap but the wire is different and shapes
// can diverge).
type WatchlistItem struct {
	RatingKey string `json:"ratingKey"`
	GUID      string `json:"guid,omitempty"`
	Title     string `json:"title"`
	Type      string `json:"type"`
	Year      int    `json:"year,omitempty"`
	Thumb     string `json:"thumb,omitempty"`
}

type watchlistWire struct {
	MediaContainer struct {
		Metadata []struct {
			RatingKey string `json:"ratingKey"`
			GUID      string `json:"guid"`
			Title     string `json:"title"`
			Type      string `json:"type"`
			Year      int    `json:"year"`
			Thumb     string `json:"thumb"`
		} `json:"Metadata"`
	} `json:"MediaContainer"`
}

// GetWatchlist returns the user's plex.tv watchlist (account-wide).
// Endpoint: GET https://metadata.provider.plex.tv/library/sections/watchlist/all
// Authentication: X-Plex-Token header (account token).
func (c *Client) GetWatchlist(accountToken string) ([]WatchlistItem, error) {
	u := c.metadataBase + "/library/sections/watchlist/all"
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
		return nil, fmt.Errorf("watchlist: status %d", resp.StatusCode)
	}
	var w watchlistWire
	if err := json.NewDecoder(resp.Body).Decode(&w); err != nil {
		return nil, err
	}
	out := make([]WatchlistItem, 0, len(w.MediaContainer.Metadata))
	for _, m := range w.MediaContainer.Metadata {
		out = append(out, WatchlistItem{
			RatingKey: m.RatingKey,
			GUID:      m.GUID,
			Title:     m.Title,
			Type:      m.Type,
			Year:      m.Year,
			Thumb:     m.Thumb,
		})
	}
	return out, nil
}
