package plex

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"time"
)

// PIN is the result of Client.CreatePIN. Code is what the user types at plex.tv/link.
type PIN struct {
	ID   int    `json:"id"`
	Code string `json:"code"`
}

// pinResponse matches the plex.tv /api/v2/pins JSON shape, with the authToken
// field populated once the user claims the PIN.
type pinResponse struct {
	ID        int    `json:"id"`
	Code      string `json:"code"`
	AuthToken string `json:"authToken"`
}

// CreatePIN asks plex.tv for a new PIN with a 4-char Code and numeric ID.
// Byron will enter the Code at https://plex.tv/link.
func (c *Client) CreatePIN() (PIN, error) {
	u := c.plexTVBase + "/api/v2/pins?strong=true"
	req, err := c.NewRequest("POST", u, nil)
	if err != nil {
		return PIN{}, err
	}
	resp, err := c.Do(req)
	if err != nil {
		return PIN{}, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusCreated && resp.StatusCode != http.StatusOK {
		return PIN{}, fmt.Errorf("create pin: status %d", resp.StatusCode)
	}
	var p pinResponse
	if err := json.NewDecoder(resp.Body).Decode(&p); err != nil {
		return PIN{}, err
	}
	return PIN{ID: p.ID, Code: p.Code}, nil
}

// PollPIN polls /api/v2/pins/<id> until authToken is populated or timeout elapses.
// Returns the account token on success.
func (c *Client) PollPIN(pin PIN, timeout time.Duration) (string, error) {
	deadline := time.Now().Add(timeout)
	u := fmt.Sprintf("%s/api/v2/pins/%d", c.plexTVBase, pin.ID)
	for {
		req, err := c.NewRequest("GET", u, nil)
		if err != nil {
			return "", err
		}
		resp, err := c.Do(req)
		if err != nil {
			return "", err
		}
		var p pinResponse
		_ = json.NewDecoder(resp.Body).Decode(&p)
		resp.Body.Close()
		if p.AuthToken != "" {
			return p.AuthToken, nil
		}
		if time.Now().After(deadline) {
			return "", fmt.Errorf("PIN poll timed out after %s", timeout)
		}
		time.Sleep(c.pinPollInterval)
	}
}

// LinkURL returns the user-visible URL Byron opens in a browser.
func LinkURL() string {
	return "https://plex.tv/link"
}

// ForceBrowserURL returns the URL with the code pre-filled as a query parameter.
// (Plex's /link page reads ?code=XXXX if present.)
func ForceBrowserURL(code string) string {
	return LinkURL() + "?" + url.Values{"code": []string{code}}.Encode()
}
