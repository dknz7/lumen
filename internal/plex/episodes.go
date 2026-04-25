package plex

import (
	"encoding/json"
	"fmt"
	"sort"
)

// allLeavesResponse mirrors /library/metadata/<showKey>/allLeaves.
type allLeavesResponse struct {
	MediaContainer struct {
		Metadata []Item `json:"Metadata"`
	} `json:"MediaContainer"`
}

// NextEpisode returns the episode that comes immediately after currentRatingKey
// in show showRatingKey, ordered by (season, episode-index). Returns nil
// (without error) when currentRatingKey is the last episode in the show.
func (c *Client) NextEpisode(s *Server, showRatingKey, currentRatingKey string) (*Item, error) {
	u := fmt.Sprintf("%s/library/metadata/%s/allLeaves", s.BaseURL, showRatingKey)
	req, err := c.NewRequest("GET", u, nil)
	if err != nil {
		return nil, err
	}
	c.SetToken(req, s.AccessToken)
	resp, err := c.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("allLeaves %s: status %d", showRatingKey, resp.StatusCode)
	}
	var body allLeavesResponse
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		return nil, err
	}

	eps := body.MediaContainer.Metadata
	sort.SliceStable(eps, func(i, j int) bool {
		if eps[i].ParentIndex != eps[j].ParentIndex {
			return eps[i].ParentIndex < eps[j].ParentIndex
		}
		return eps[i].Index < eps[j].Index
	})

	for i, ep := range eps {
		if ep.RatingKey == currentRatingKey {
			if i+1 < len(eps) {
				return &eps[i+1], nil
			}
			return nil, nil
		}
	}
	return nil, fmt.Errorf("episode %s not found in show %s", currentRatingKey, showRatingKey)
}

// Season is one season row returned by /library/metadata/<showKey>/children.
// LeafCount/ViewedLeafCount let the SPA show watched-progress dots without
// fetching every episode.
type Season struct {
	RatingKey       string `json:"ratingKey"`
	Index           int    `json:"index"`
	Title           string `json:"title"`
	LeafCount       int    `json:"leafCount"`
	ViewedLeafCount int    `json:"viewedLeafCount"`
	Thumb           string `json:"thumb"`
}

type seasonsResponse struct {
	MediaContainer struct {
		Metadata []Season `json:"Metadata"`
	} `json:"MediaContainer"`
}

// GetSeasons returns the season list for a show. The Plex endpoint also
// includes the synthetic "All Episodes" season at index 0 — callers should
// filter those out for the season-tabs UI.
func (c *Client) GetSeasons(s *Server, showRatingKey string) ([]Season, error) {
	u := fmt.Sprintf("%s/library/metadata/%s/children", s.BaseURL, showRatingKey)
	req, err := c.NewRequest("GET", u, nil)
	if err != nil {
		return nil, err
	}
	c.SetToken(req, s.AccessToken)
	resp, err := c.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("seasons %s: status %d", showRatingKey, resp.StatusCode)
	}
	var body seasonsResponse
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		return nil, err
	}
	return body.MediaContainer.Metadata, nil
}

// GetSeasonEpisodes returns episodes for a single season key. Reuses the same
// allLeavesResponse envelope as NextEpisode — the wire shape of season
// children matches show /allLeaves.
func (c *Client) GetSeasonEpisodes(s *Server, seasonRatingKey string) ([]Item, error) {
	u := fmt.Sprintf("%s/library/metadata/%s/children", s.BaseURL, seasonRatingKey)
	req, err := c.NewRequest("GET", u, nil)
	if err != nil {
		return nil, err
	}
	c.SetToken(req, s.AccessToken)
	resp, err := c.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("season episodes %s: status %d", seasonRatingKey, resp.StatusCode)
	}
	var body allLeavesResponse
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		return nil, err
	}
	return body.MediaContainer.Metadata, nil
}
