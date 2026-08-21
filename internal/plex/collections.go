package plex

import (
	"fmt"
	"net/url"
	"strconv"
)

// Collection is one curated list inside a Plex library section. Powers the
// custom Home shelves such as "Trending Movies" / "Trending Shows".
// Plex emits collections via /library/sections/<id>/collections; the items
// themselves come from /library/collections/<rk>/children.
type Collection struct {
	RatingKey string `json:"ratingKey"`
	Title     string `json:"title"`
	Type      string `json:"type"` // typically "collection"
}

// GetCollections lists all collections in a given library section. Used
// by Home.tsx's "server-collection" shelf to resolve a collection by title
// without hardcoding ratingKeys (admin-rename tolerant).
func (c *Client) GetCollections(s *Server, libraryID string) ([]Collection, error) {
	mc, err := c.serverGet(s, fmt.Sprintf("/library/sections/%s/collections", libraryID), nil)
	if err != nil {
		return nil, err
	}
	out := make([]Collection, 0, len(mc.MediaContainer.Metadata))
	for _, m := range mc.MediaContainer.Metadata {
		out = append(out, Collection{
			RatingKey: m.RatingKey,
			Title:     m.Title,
			Type:      m.Type,
		})
	}
	return out, nil
}

// GetCollectionItems lists items inside a collection. size caps the
// returned slice; pass 0 for Plex's default. Returns standard Items —
// collections contain the same wire shape as /library/sections/<id>/all
// so metadataSliceToItems handles the conversion uniformly.
func (c *Client) GetCollectionItems(s *Server, collectionRatingKey string, size int) ([]Item, error) {
	path := "/library/collections/" + url.PathEscape(collectionRatingKey) + "/children"
	if size > 0 {
		path += "?X-Plex-Container-Start=0&X-Plex-Container-Size=" + strconv.Itoa(size)
	}
	mc, err := c.serverGet(s, path, nil)
	if err != nil {
		return nil, err
	}
	return metadataSliceToItems(mc.MediaContainer.Metadata), nil
}
