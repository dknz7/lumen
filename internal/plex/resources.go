package plex

import (
	"encoding/json"
	"fmt"
	"net/http"
)

type resourceWire struct {
	Name             string           `json:"name"`
	ClientIdentifier string           `json:"clientIdentifier"`
	AccessToken      string           `json:"accessToken"`
	Product          string           `json:"product"`
	Connections      []connectionWire `json:"connections"`
}

type connectionWire struct {
	URI      string `json:"uri"`
	Address  string `json:"address"`
	Port     int    `json:"port"`
	Local    bool   `json:"local"`
	Relay    bool   `json:"relay"`
	Protocol string `json:"protocol"`
	IPv6     bool   `json:"IPv6"`
}

// DiscoverServers calls plex.tv/api/v2/resources and returns every Plex Media Server
// the account has access to — both owned and shared.
func (c *Client) DiscoverServers(accountToken string) ([]*Server, error) {
	u := c.plexTVBase + "/api/v2/resources?includeHttps=1&includeRelay=1"
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
		return nil, fmt.Errorf("resources: status %d", resp.StatusCode)
	}

	var wires []resourceWire
	if err := json.NewDecoder(resp.Body).Decode(&wires); err != nil {
		return nil, err
	}

	var out []*Server
	for _, w := range wires {
		if w.Product != "Plex Media Server" {
			continue
		}
		s := &Server{
			Name:              w.Name,
			MachineIdentifier: w.ClientIdentifier,
			AccessToken:       w.AccessToken,
		}
		for _, cw := range w.Connections {
			s.Connections = append(s.Connections, Connection{
				URI:      cw.URI,
				Address:  cw.Address,
				Port:     cw.Port,
				Local:    cw.Local,
				Relay:    cw.Relay,
				Protocol: cw.Protocol,
				IPv6:     cw.IPv6,
			})
		}
		out = append(out, s)
	}
	return out, nil
}
