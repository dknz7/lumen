package main

import (
	"encoding/json"
	"fmt"
	"os"

	"lumen/internal/config"
	"lumen/internal/plex"
)

func runProbeHubs(args []string) {
	cfg, err := config.Load()
	if err != nil {
		fmt.Fprintf(os.Stderr, "load config: %v\n", err)
		os.Exit(1)
	}
	if cfg.Plex.AccountToken == "" {
		fmt.Fprintln(os.Stderr, "no Plex account token — run `lumen auth` first")
		os.Exit(1)
	}
	c := plex.NewClient(cfg.ClientIdentifier, version)

	// Phase A — dump the full hub index for BOTH namespaces.
	fmt.Println("=== Discover hub index: contentDirectoryID=home ===")
	dumpHubIndex(c, cfg.Plex.AccountToken, "home")

	fmt.Println("\n=== Discover hub index: contentDirectoryID=watchlist ===")
	dumpHubIndex(c, cfg.Plex.AccountToken, "watchlist")

	// Phase B — per-server onDeck. spec §12.1 Continue Watching row is sourced
	// from per-server /library/onDeck merged across servers. Pick Up Again on
	// Recommended is likely the same data, filtered to watchlisted items.
	fmt.Println("\n=== Per-server /library/onDeck ===")
	for _, s := range cfg.Plex.Servers {
		probeServerOnDeck(c, s)
	}
}

// dumpHubIndex calls /hubs?contentDirectoryID=<ns> and prints every hub Plex
// exposes for that namespace.
func dumpHubIndex(c *plex.Client, token, namespace string) {
	u := "https://discover.provider.plex.tv/hubs?contentDirectoryID=" + namespace
	req, err := c.NewRequest("GET", u, nil)
	if err != nil {
		fmt.Printf("  build request: %v\n", err)
		return
	}
	c.SetToken(req, token)
	resp, err := c.Do(req)
	if err != nil {
		fmt.Printf("  GET %s: %v\n", u, err)
		return
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		fmt.Printf("  status %d\n", resp.StatusCode)
		return
	}

	var envelope struct {
		MediaContainer struct {
			Hub []struct {
				HubKey     string `json:"hubKey"`
				Identifier string `json:"hubIdentifier"`
				Title      string `json:"title"`
				Type       string `json:"type"`
				Size       int    `json:"size"`
				Context    string `json:"context"`
			} `json:"Hub"`
		} `json:"MediaContainer"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&envelope); err != nil {
		fmt.Printf("  decode: %v\n", err)
		return
	}
	if len(envelope.MediaContainer.Hub) == 0 {
		fmt.Println("  (no hubs)")
		return
	}
	for _, h := range envelope.MediaContainer.Hub {
		fmt.Printf("  identifier=%-40s  size=%-3d  title=%s\n",
			h.Identifier, h.Size, h.Title)
	}
}

// probeServerOnDeck hits the per-server onDeck endpoint and reports count + first title.
func probeServerOnDeck(c *plex.Client, s config.Server) {
	if s.LastGoodConnection == "" || s.AccessToken == "" {
		fmt.Printf("  [%s] no cached connection/token — run `lumen list` first\n", s.Name)
		return
	}
	u := s.LastGoodConnection + "/library/onDeck"
	req, err := c.NewRequest("GET", u, nil)
	if err != nil {
		fmt.Printf("  [%s] build request: %v\n", s.Name, err)
		return
	}
	c.SetToken(req, s.AccessToken)
	resp, err := c.Do(req)
	if err != nil {
		fmt.Printf("  [%s] GET %s: %v\n", s.Name, u, err)
		return
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		fmt.Printf("  [%s] status %d\n", s.Name, resp.StatusCode)
		return
	}

	var envelope struct {
		MediaContainer struct {
			Size     int `json:"size"`
			Metadata []struct {
				RatingKey string `json:"ratingKey"`
				Title     string `json:"title"`
				Type      string `json:"type"`
				GrandparentTitle string `json:"grandparentTitle"`
			} `json:"Metadata"`
		} `json:"MediaContainer"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&envelope); err != nil {
		fmt.Printf("  [%s] decode: %v\n", s.Name, err)
		return
	}
	fmt.Printf("  [%s] onDeck items=%d\n", s.Name, envelope.MediaContainer.Size)
	for i, m := range envelope.MediaContainer.Metadata {
		if i >= 5 {
			fmt.Printf("    ... and %d more\n", envelope.MediaContainer.Size-5)
			break
		}
		label := m.Title
		if m.GrandparentTitle != "" {
			label = m.GrandparentTitle + " — " + m.Title
		}
		fmt.Printf("    %s (%s)\n", label, m.Type)
	}
}
