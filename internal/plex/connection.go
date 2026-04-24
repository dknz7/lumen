package plex

import (
	"context"
	"fmt"
	"net/http"
	"sort"
	"time"
)

// PickConnection probes the server's candidate URIs in preference order and returns
// the first one that responds to HEAD /identity within 2 s. Writes the winner to
// Server.BaseURL as a side-effect so subsequent calls can use it.
func (c *Client) PickConnection(s *Server) (Connection, error) {
	if len(s.Connections) == 0 {
		return Connection{}, fmt.Errorf("server %q has no connections", s.Name)
	}
	ordered := sortConnections(s.Connections)
	for _, conn := range ordered {
		if c.probe(conn.URI, 2*time.Second) {
			s.BaseURL = conn.URI
			return conn, nil
		}
	}
	return Connection{}, fmt.Errorf("no reachable connection for server %q", s.Name)
}

func (c *Client) probe(baseURL string, timeout time.Duration) bool {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, "HEAD", baseURL+"/identity", nil)
	if err != nil {
		return false
	}
	resp, err := c.http.Do(req)
	if err != nil {
		return false
	}
	resp.Body.Close()
	return resp.StatusCode < 400
}

// sortConnections returns a copy of the input sorted by preference:
//  1. non-relay IPv4
//  2. non-relay IPv6
//  3. relay IPv4
//  4. relay IPv6
func sortConnections(in []Connection) []Connection {
	out := make([]Connection, len(in))
	copy(out, in)
	sort.SliceStable(out, func(i, j int) bool {
		return score(out[i]) < score(out[j])
	})
	return out
}

func score(c Connection) int {
	s := 0
	if c.Relay {
		s += 10
	}
	if c.IPv6 {
		s += 1
	}
	return s
}
