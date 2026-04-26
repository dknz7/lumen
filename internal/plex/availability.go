package plex

import (
	"encoding/json"
	"fmt"
	"net/url"
)

// Match is what GetAvailability returns per server that has a matching GUID.
type Match struct {
	ServerName  string `json:"serverName"`
	MachineID   string `json:"machineIdentifier"`
	RatingKey   string `json:"ratingKey"`
	LibraryName string `json:"libraryName,omitempty"`
	Resolution  string `json:"resolution"` // "2160" | "1080" | "720" | ...
	Container   string `json:"container"`  // "mkv" | "mp4" | ...
	Bitrate     int    `json:"bitrate"`
	Size        int64  `json:"size"`
	Codec       string `json:"codec,omitempty"`
}

type availabilityWire struct {
	MediaContainer struct {
		Metadata []struct {
			RatingKey string `json:"ratingKey"`
			GUID      string `json:"guid"`
			GuidArray []struct {
				ID string `json:"id"`
			} `json:"Guid"` // absorbs the external-ID array — see metadataWire comment
			Title               string `json:"title"`
			LibrarySectionTitle string `json:"librarySectionTitle"`
			Media               []struct {
				Container       string `json:"container"`
				VideoResolution string `json:"videoResolution"`
				Bitrate         int    `json:"bitrate"`
				VideoCodec      string `json:"videoCodec"`
				Part            []struct {
					Size      int64  `json:"size"`
					Container string `json:"container"`
				} `json:"Part"`
			} `json:"Media"`
		} `json:"Metadata"`
	} `json:"MediaContainer"`
}

// GetAvailability queries one server's /library/all?guid=<guid> and returns zero
// or more Match entries. Normal case is 0 (not on this server) or 1 (one copy).
func (c *Client) GetAvailability(s *Server, guid string) ([]Match, error) {
	q := url.Values{"guid": []string{guid}}
	u := s.BaseURL + "/library/all?" + q.Encode()
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
		return nil, fmt.Errorf("plex: %s returned status %d", u, resp.StatusCode)
	}
	var mc availabilityWire
	if err := json.NewDecoder(resp.Body).Decode(&mc); err != nil {
		return nil, err
	}
	out := make([]Match, 0, len(mc.MediaContainer.Metadata))
	for _, m := range mc.MediaContainer.Metadata {
		match := Match{
			ServerName:  s.Name,
			MachineID:   s.MachineIdentifier,
			RatingKey:   m.RatingKey,
			LibraryName: m.LibrarySectionTitle,
		}
		if len(m.Media) > 0 {
			match.Container = m.Media[0].Container
			match.Resolution = m.Media[0].VideoResolution
			match.Bitrate = m.Media[0].Bitrate
			match.Codec = m.Media[0].VideoCodec
			if len(m.Media[0].Part) > 0 {
				match.Size = m.Media[0].Part[0].Size
			}
		}
		out = append(out, match)
	}
	return out, nil
}
