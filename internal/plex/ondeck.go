package plex

// GetOnDeck returns the server's in-progress items, matching Plex Web's Continue
// Watching row for that server. Used by spec §12.1's pinned shelf.
func (c *Client) GetOnDeck(s *Server) ([]Item, error) {
	mc, err := c.serverGet(s, "/library/onDeck", nil)
	if err != nil {
		return nil, err
	}
	return metadataSliceToItems(mc.MediaContainer.Metadata), nil
}
