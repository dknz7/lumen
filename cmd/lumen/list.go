package main

import (
	"fmt"
	"os"
	"sync"

	"lumen/internal/config"
	"lumen/internal/plex"
)

func runList(args []string) {
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
	servers, err := c.DiscoverServers(cfg.Plex.AccountToken)
	if err != nil {
		fmt.Fprintf(os.Stderr, "discover servers: %v\n", err)
		os.Exit(1)
	}
	if len(servers) == 0 {
		fmt.Fprintln(os.Stderr, "no Plex Media Servers accessible on this account")
		os.Exit(1)
	}

	// Pick connections in parallel — one slow server shouldn't block the other.
	var wg sync.WaitGroup
	errs := make([]error, len(servers))
	for i, s := range servers {
		wg.Add(1)
		go func(i int, s *plex.Server) {
			defer wg.Done()
			if _, err := c.PickConnection(s); err != nil {
				errs[i] = err
			}
		}(i, s)
	}
	wg.Wait()

	// Print report + collect persisted server state.
	var persisted []config.Server
	for i, s := range servers {
		fmt.Printf("=== %s ===\n", s.Name)
		if errs[i] != nil {
			fmt.Printf("  connection: OFFLINE (%v)\n", errs[i])
			continue
		}
		fmt.Printf("  connection: %s\n", s.BaseURL)
		libs, err := c.GetLibraries(s)
		if err != nil {
			fmt.Printf("  libraries: ERROR — %v\n", err)
			continue
		}
		fmt.Printf("  libraries:\n")
		for _, l := range libs {
			fmt.Printf("    [%s] %s (%s)\n", l.Key, l.Title, l.Type)
		}
		persisted = append(persisted, config.Server{
			Name:               s.Name,
			MachineIdentifier:  s.MachineIdentifier,
			AccessToken:        s.AccessToken,
			LastGoodConnection: s.BaseURL,
		})
	}

	cfg.Plex.Servers = persisted
	if err := cfg.Save(); err != nil {
		fmt.Fprintf(os.Stderr, "save config: %v\n", err)
		os.Exit(1)
	}

	// Fail the overall command only if EVERY server was offline.
	allDown := true
	for _, e := range errs {
		if e == nil {
			allDown = false
			break
		}
	}
	if allDown {
		os.Exit(1)
	}
}
