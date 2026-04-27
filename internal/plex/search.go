package plex

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"sort"
)

// SearchDiscover hits plex.tv's /library/search endpoint for cross-catalog
// movie + TV results. Distinct from per-server (*Client).Search which only
// finds items physically in a user's libraries — discover search returns
// items from Plex's full catalog whether the user owns them or not.
//
// Wire shape (Session 6.5 DevTools capture):
//
//	MediaContainer.SearchResults[]   // groups: "plex"/"Free On Demand" + "external"/"More Ways To Watch"
//	  └── SearchResult[]             // singular noun, plural slice
//	        └── { Metadata, score }  // Metadata is the standard wire shape
//
// Empty groups are common (e.g. Free On Demand returns size:0 for items not
// available on AVOD); we flatten everything across all non-empty groups,
// sort by score desc, and return a flat []Item. Score is the relevance float
// Plex itself computes — higher = better match.
//
// Query param contract — every value below is required to mirror Plex Web's
// capture; dropping any of them silently changes what Plex returns:
//   - searchTypes=movies,tv: result type filter (categories/people excluded)
//   - searchProviders=discover,plexAVOD: tells Plex to query both the
//     discover catalog AND Plex AVOD. Without this, the response can come
//     back with empty groups even for terms that should match.
//   - includeMetadata=1: fattens the Metadata blob (without it, only ratingKey + title)
//   - filterPeople=1: drops cast/crew matches that wouldn't navigate cleanly.
func (c *Client) SearchDiscover(query, accountToken string) ([]Item, error) {
	if query == "" {
		return nil, nil
	}
	qs := url.Values{
		"query":           []string{query},
		"limit":           []string{"30"},
		"searchTypes":     []string{"movies,tv"},
		"searchProviders": []string{"discover,plexAVOD"},
		"includeMetadata": []string{"1"},
		"filterPeople":    []string{"1"},
	}
	u := fmt.Sprintf("%s/library/search?%s", c.discoverBase, qs.Encode())
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
		return nil, fmt.Errorf("discover search %q: status %d", query, resp.StatusCode)
	}
	var env discoverSearchEnvelope
	if err := json.NewDecoder(resp.Body).Decode(&env); err != nil {
		return nil, err
	}
	// Flatten all non-empty groups, preserve score for relevance sort.
	type scored struct {
		item  Item
		score float64
	}
	var hits []scored
	for _, group := range env.MediaContainer.SearchResults {
		if group.Size == 0 {
			continue
		}
		for _, hit := range group.SearchResult {
			items := metadataSliceToItems([]metadataWire{hit.Metadata})
			if len(items) == 0 {
				continue
			}
			hits = append(hits, scored{item: items[0], score: hit.Score})
		}
	}
	sort.SliceStable(hits, func(i, j int) bool {
		return hits[i].score > hits[j].score
	})
	out := make([]Item, 0, len(hits))
	for _, h := range hits {
		out = append(out, h.item)
	}
	return out, nil
}

// discoverSearchEnvelope mirrors plex.tv's /library/search response. Two
// levels deep: groups → items. Captured Session 6.5 against query=hokum.
type discoverSearchEnvelope struct {
	MediaContainer struct {
		Size          int                   `json:"size"`
		SearchResults []discoverSearchGroup `json:"SearchResults"`
	} `json:"MediaContainer"`
}

// discoverSearchGroup is one source bucket — typically "plex" (Free On Demand)
// or "external" (More Ways To Watch). Size==0 means the bucket exists in the
// envelope but has no items for this query (skip it).
type discoverSearchGroup struct {
	ID           string              `json:"id"`
	Title        string              `json:"title"`
	Size         int                 `json:"size"`
	SearchResult []discoverSearchHit `json:"SearchResult"`
}

// discoverSearchHit pairs each metadata blob with Plex's relevance score.
// Score sits as a sibling of Metadata, NOT inside it — easy to mismatch
// when fixturing without a real capture.
type discoverSearchHit struct {
	Metadata metadataWire `json:"Metadata"`
	Score    float64      `json:"score"`
}
