package plex

import (
	"fmt"
	"net/url"
	"time"
)

// TimelineReport carries the data sent in a single POST /:/timeline.
type TimelineReport struct {
	RatingKey string
	State     string // "playing" | "paused" | "stopped"
	Position  time.Duration
	Duration  time.Duration
}

// ReportTimeline POSTs /:/timeline so Plex updates viewOffset, lastViewedAt,
// and (server-side) watched state. Caller decides cadence — typically every
// 10 s while playing.
func (c *Client) ReportTimeline(s *Server, r TimelineReport) error {
	if r.State != "playing" && r.State != "paused" && r.State != "stopped" {
		return fmt.Errorf("ReportTimeline: invalid state %q", r.State)
	}
	q := url.Values{
		"ratingKey":    []string{r.RatingKey},
		"state":        []string{r.State},
		"time":         []string{fmt.Sprintf("%d", r.Position.Milliseconds())},
		"duration":     []string{fmt.Sprintf("%d", r.Duration.Milliseconds())},
		"X-Plex-Token": []string{s.AccessToken},
	}
	u := s.BaseURL + "/:/timeline?" + q.Encode()
	req, err := c.NewRequest("POST", u, nil)
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
		return fmt.Errorf("timeline %s: status %d", r.RatingKey, resp.StatusCode)
	}
	return nil
}
