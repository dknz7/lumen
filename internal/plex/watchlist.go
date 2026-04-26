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
			// GuidArray absorbs Plex's capital-"Guid" array of external IDs
			// (imdb/tmdb/tvdb) so Go's case-insensitive json matching doesn't
			// spill it into the GUID string field and fail to unmarshal.
			// Session 3 critical gotcha #6.
			GuidArray []struct {
				ID string `json:"id"`
			} `json:"Guid"`
			Title string `json:"title"`
			Type  string `json:"type"`
			Year  int    `json:"year"`
			Thumb string `json:"thumb"`
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

// AddToWatchlist adds a Discover-namespace ratingKey to the user's
// plex.tv watchlist. The ratingKey here is plex.tv's metadata rating
// key (NOT a server-local one). Caller must resolve that via the
// item's plex.tv GUID before invoking. Endpoint shape confirmed via
// Plex Web DevTools capture in Session 5 (PUT /actions/addToWatchlist
// with header-only X-Plex-Token, ratingKey via URL query, empty body,
// 200 response with {"MediaContainer":{"size":0}}).
func (c *Client) AddToWatchlist(accountToken, plexTvRatingKey string) error {
	return c.watchlistAction(accountToken, plexTvRatingKey, "addToWatchlist")
}

// RemoveFromWatchlist removes a Discover-namespace ratingKey from the
// watchlist. Same ratingKey semantics as AddToWatchlist. Symmetric
// shape to Add (not directly DevTools-captured but action-verb-only
// difference is the conventional Plex pattern; see scrobble.go for
// precedent).
func (c *Client) RemoveFromWatchlist(accountToken, plexTvRatingKey string) error {
	return c.watchlistAction(accountToken, plexTvRatingKey, "removeFromWatchlist")
}

func (c *Client) watchlistAction(accountToken, plexTvRatingKey, action string) error {
	u := c.discoverBase + "/actions/" + action + "?ratingKey=" + plexTvRatingKey
	req, err := c.NewRequest("PUT", u, nil)
	if err != nil {
		return err
	}
	c.SetToken(req, accountToken)
	resp, err := c.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("%s %s: status %d", action, plexTvRatingKey, resp.StatusCode)
	}
	return nil
}
