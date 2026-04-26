package plex

import (
	"io"
	"net/http"
	"time"
)

// Client is a thin wrapper around http.Client that stamps every outbound request
// with Lumen's stable identity headers.
type Client struct {
	http             *http.Client
	clientIdentifier string
	version          string
	plexTVBase       string        // override for tests; default set in NewClient
	pinPollInterval  time.Duration // override for tests; default 2 s
	discoverBase     string        // overridable for tests
}

// NewClient builds a Plex-aware HTTP client. clientIdentifier should be the stable
// UUID loaded from config (Task 3); version is Lumen's semver.
func NewClient(clientIdentifier, version string) *Client {
	c := &Client{
		http: &http.Client{
			Timeout: 15 * time.Second,
		},
		clientIdentifier: clientIdentifier,
		version:          version,
		plexTVBase:       "https://plex.tv",
		pinPollInterval:  2 * time.Second,
		discoverBase:     "https://discover.provider.plex.tv",
	}
	return c
}

// NewRequest constructs an http.Request with Lumen's identity headers applied.
// method is "GET", "POST", etc.; url is absolute; body may be nil.
func (c *Client) NewRequest(method, url string, body io.Reader) (*http.Request, error) {
	req, err := http.NewRequest(method, url, body)
	if err != nil {
		return nil, err
	}
	req.Header.Set("X-Plex-Product", "Lumen")
	req.Header.Set("X-Plex-Version", c.version)
	req.Header.Set("X-Plex-Platform", "Windows")
	req.Header.Set("X-Plex-Device", "PC")
	req.Header.Set("X-Plex-Client-Identifier", c.clientIdentifier)
	req.Header.Set("Accept", "application/json")
	return req, nil
}

// SetToken stamps the X-Plex-Token header on a request. Supply either an
// account-level token (plex.tv calls) or a per-server token (server calls).
func (c *Client) SetToken(req *http.Request, token string) {
	req.Header.Set("X-Plex-Token", token)
}

// Do executes the request via the underlying http.Client.
func (c *Client) Do(req *http.Request) (*http.Response, error) {
	return c.http.Do(req)
}
