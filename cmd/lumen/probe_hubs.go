package main

import (
	"encoding/json"
	"fmt"
	"os"

	"lumen/internal/config"
	"lumen/internal/plex"
)

// Candidate slugs for the "Pick Up Again" hub (spec §20).
var pickUpAgainCandidates = []string{
	"continue-watching",
	"on-deck",
	"pick-up-again",
	"in-progress",
	"continue",
	"resume",
	"keep-watching",
	"pick-up-where-you-left-off",
}

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

	// Phase 1: try each candidate slug under both namespaces.
	fmt.Println("=== Slug candidates ===")
	for _, ns := range []string{"home", "watchlist"} {
		for _, slug := range pickUpAgainCandidates {
			items, err := c.GetHub(ns, slug, cfg.Plex.AccountToken)
			if err != nil {
				fmt.Printf("  %-9s / %-30s  ERROR: %v\n", ns, slug, err)
				continue
			}
			sample := "<none>"
			if len(items) > 0 {
				sample = items[0].Title
			}
			fmt.Printf("  %-9s / %-30s  items=%d  first=%q\n", ns, slug, len(items), sample)
		}
	}

	// Phase 2: dump every hub identifier Plex's Discover returns for this account.
	fmt.Println("\n=== Live hub dump (discover.provider.plex.tv/hubs) ===")
	dumpHubIndex(c, cfg.Plex.AccountToken)
}

// dumpHubIndex calls the top-level /hubs endpoint and prints every hub's
// identifier/title/size so we can see what Plex actually exposes for the account.
func dumpHubIndex(c *plex.Client, accountToken string) {
	u := "https://discover.provider.plex.tv/hubs"
	req, err := c.NewRequest("GET", u, nil)
	if err != nil {
		fmt.Printf("  build request: %v\n", err)
		return
	}
	c.SetToken(req, accountToken)
	resp, err := c.Do(req)
	if err != nil {
		fmt.Printf("  GET /hubs: %v\n", err)
		return
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		fmt.Printf("  GET /hubs: status %d\n", resp.StatusCode)
		return
	}

	var envelope struct {
		MediaContainer struct {
			Hub []struct {
				HubKey     string `json:"hubKey"`
				Identifier string `json:"hubIdentifier"`
				Title      string `json:"title"`
				Type       string `json:"type"`
				Style      string `json:"style"`
				Size       int    `json:"size"`
				Context    string `json:"context"`
			} `json:"Hub"`
		} `json:"MediaContainer"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&envelope); err != nil {
		fmt.Printf("  decode: %v\n", err)
		return
	}
	for _, h := range envelope.MediaContainer.Hub {
		fmt.Printf("  identifier=%-40s  title=%-30s  size=%-3d  hubKey=%s\n",
			h.Identifier, truncate(h.Title, 30), h.Size, h.HubKey)
	}
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n]
}
