package plex

import (
	"context"
	"fmt"
	"net/http"
	"sort"
	"strings"
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

// sortConnections returns a copy of the input sorted by preference (lower score wins):
//  1. plex.direct non-relay IPv4      (score 0)
//  2. plex.direct non-relay IPv6      (score 1)
//  3. custom-domain non-relay IPv4    (score 10)
//  4. custom-domain non-relay IPv6    (score 11)
//  5. relay (plex.direct or otherwise) (score 100+)
//
// plex.direct URLs are preferred because some custom-domain connections sit
// behind CDNs that only whitelist a subset of Plex API paths — /identity works
// but /library/metadata/*/thumb/* may 404 (observed against one server's
// Level 3 edge). plex.direct URLs bypass that by routing directly to the PMS.
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
		s += 100
	}
	if c.IPv6 {
		s += 1
	}
	if !strings.Contains(c.URI, ".plex.direct") && !c.Relay {
		s += 10
	}
	return s
}
