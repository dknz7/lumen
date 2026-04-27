package plex

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
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
		TotalSize int `json:"totalSize"`
		Metadata  []struct {
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
// Endpoint: GET https://discover.provider.plex.tv/library/sections/watchlist/all
// Authentication: X-Plex-Token header (account token).
//
// Endpoint host + request shape confirmed via Plex Web DevTools capture
// (Session 5 post-smoke). Plex Web pages 50→100→100→…; sizes above ~100
// are rejected with 400. We page in 100-item chunks until totalSize is
// reached or we've collected 1000 items (a generous cap that covers
// almost any user; Byron's 471 fits comfortably).
func (c *Client) GetWatchlist(accountToken string) ([]WatchlistItem, error) {
	const pageSize = 100
	const maxItems = 1000
	var all []WatchlistItem
	for start := 0; start < maxItems; start += pageSize {
		page, total, err := c.getWatchlistPage(accountToken, start, pageSize)
		if err != nil {
			return nil, err
		}
		all = append(all, page...)
		if len(page) < pageSize || start+pageSize >= total {
			break
		}
	}
	return all, nil
}

// getWatchlistPage returns one page of the watchlist along with the total
// container size reported by Plex (used by the caller to terminate paging).
func (c *Client) getWatchlistPage(accountToken string, start, size int) ([]WatchlistItem, int, error) {
	u := fmt.Sprintf(
		"%s/library/sections/watchlist/all?includeMeta=1&X-Plex-Container-Start=%d&X-Plex-Container-Size=%d",
		c.discoverBase, start, size,
	)
	req, err := c.NewRequest("GET", u, nil)
	if err != nil {
		return nil, 0, err
	}
	c.SetToken(req, accountToken)
	resp, err := c.Do(req)
	if err != nil {
		return nil, 0, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, 0, fmt.Errorf("watchlist: status %d", resp.StatusCode)
	}
	var w watchlistWire
	if err := json.NewDecoder(resp.Body).Decode(&w); err != nil {
		return nil, 0, err
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
	return out, w.MediaContainer.TotalSize, nil
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

// AddItemToWatchlist takes a server-local ratingKey and adds the corresponding
// plex.tv catalog entry to the user's watchlist. For TV episodes/seasons it
// rolls up to the parent show — Plex's watchlist is keyed at the show level,
// not per-episode. Movies and shows pass through directly.
//
// Two-hop resolution for episode/season: fetch the item, walk up to the show
// via parentRatingKey/grandparentRatingKey, fetch the show, extract the
// plex.tv ratingKey from its `plex://show/<rk>` GUID, then PUT to the
// addToWatchlist action.
//
// Returns an error if the resolved target item has no plex://-style GUID
// (older library entries with com.plexapp.agents.* GUIDs aren't supported —
// caller should refresh metadata to upgrade the agent).
func (c *Client) AddItemToWatchlist(s *Server, accountToken, ratingKey string) error {
	target, err := c.resolveWatchlistTarget(s, ratingKey)
	if err != nil {
		return err
	}
	plexTvRatingKey := extractPlexTvRatingKey(target.GUID)
	if plexTvRatingKey == "" {
		return fmt.Errorf("watchlist add: item %q has no plex.tv GUID (got %q) — refresh metadata in Plex to upgrade the agent", target.RatingKey, target.GUID)
	}
	return c.AddToWatchlist(accountToken, plexTvRatingKey)
}

// RemoveItemFromWatchlist mirrors AddItemToWatchlist for the remove path —
// rolls up TV episodes/seasons to the parent show before removing, since
// Plex's watchlist is keyed at the show level. Movies and shows pass through.
func (c *Client) RemoveItemFromWatchlist(s *Server, accountToken, ratingKey string) error {
	target, err := c.resolveWatchlistTarget(s, ratingKey)
	if err != nil {
		return err
	}
	plexTvRatingKey := extractPlexTvRatingKey(target.GUID)
	if plexTvRatingKey == "" {
		return fmt.Errorf("watchlist remove: item %q has no plex.tv GUID (got %q) — refresh metadata in Plex to upgrade the agent", target.RatingKey, target.GUID)
	}
	return c.RemoveFromWatchlist(accountToken, plexTvRatingKey)
}

// resolveWatchlistTarget fetches the item and, for episode/season types,
// walks up to the parent show. Returns the show-level Item (or the original
// item for movies/shows). Errors only on transport / decode failures —
// missing parent ratingKey is left for the caller to detect via empty GUID.
func (c *Client) resolveWatchlistTarget(s *Server, ratingKey string) (Item, error) {
	item, err := c.GetItem(s, ratingKey)
	if err != nil {
		return Item{}, fmt.Errorf("watchlist add: fetch item %q: %w", ratingKey, err)
	}
	switch item.Type {
	case "movie", "show":
		return item, nil
	case "season":
		if item.ParentRatingKey == "" {
			return Item{}, fmt.Errorf("watchlist add: season %q has no parent show ratingKey", ratingKey)
		}
		show, err := c.GetItem(s, item.ParentRatingKey)
		if err != nil {
			return Item{}, fmt.Errorf("watchlist add: fetch parent show %q: %w", item.ParentRatingKey, err)
		}
		return show, nil
	case "episode":
		if item.GrandparentRatingKey == "" {
			return Item{}, fmt.Errorf("watchlist add: episode %q has no grandparent show ratingKey", ratingKey)
		}
		show, err := c.GetItem(s, item.GrandparentRatingKey)
		if err != nil {
			return Item{}, fmt.Errorf("watchlist add: fetch grandparent show %q: %w", item.GrandparentRatingKey, err)
		}
		return show, nil
	default:
		// Unknown types (clip, person, collection, etc.) fall through with
		// the original item — caller's GUID extraction either works or
		// errors out cleanly.
		return item, nil
	}
}

// extractPlexTvRatingKey pulls the trailing segment from a Plex GUID.
// `plex://movie/abc123` → `abc123`. Returns empty string for non-plex://
// GUIDs (com.plexapp.agents.imdb://, com.plexapp.agents.thetvdb://, etc.)
// since those don't directly map to plex.tv catalog ratingKeys.
func extractPlexTvRatingKey(guid string) string {
	if !strings.HasPrefix(guid, "plex://") {
		return ""
	}
	i := strings.LastIndex(guid, "/")
	if i < 0 || i >= len(guid)-1 {
		return ""
	}
	rk := guid[i+1:]
	// Strip any query suffix (`plex://show/abc?lang=en` → `abc`).
	if q := strings.IndexByte(rk, '?'); q >= 0 {
		rk = rk[:q]
	}
	return rk
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
