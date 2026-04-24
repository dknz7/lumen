package main

import (
	"fmt"
	"os"

	"lumen/internal/config"
	"lumen/internal/plex"
)

// Ordered list of candidate Pick Up Again slugs (spec §20).
var pickUpAgainCandidates = []string{
	"continue-watching",
	"on-deck",
	"pick-up-again",
	"in-progress",
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

	for _, slug := range pickUpAgainCandidates {
		items, err := c.GetHub("watchlist", slug, cfg.Plex.AccountToken)
		if err != nil {
			fmt.Printf("  watchlist/%-20s  ERROR: %v\n", slug, err)
			continue
		}
		sample := "<none>"
		if len(items) > 0 {
			sample = items[0].Title
		}
		fmt.Printf("  watchlist/%-20s  items=%d  first=%q\n", slug, len(items), sample)
	}
}
