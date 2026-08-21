package main

import (
	"fmt"
	"os"

	"lumen/internal/config"
)

func runRename(args []string) {
	if len(args) != 2 {
		fmt.Fprintln(os.Stderr, `usage: lumen rename <machineIdentifier> "<display name>"

Sets a local display-name override for a server. Useful when Plex returns an
empty name for shared servers (the name shown in the Plex Friends UI isn't
always exposed via the API).

Example:
  lumen rename 4db54e45876c "Living Room"`)
		os.Exit(2)
	}
	machineID, displayName := args[0], args[1]

	cfg, err := config.Load()
	if err != nil {
		fmt.Fprintf(os.Stderr, "load config: %v\n", err)
		os.Exit(1)
	}

	var found *config.Server
	for i := range cfg.Plex.Servers {
		if cfg.Plex.Servers[i].MachineIdentifier == machineID {
			found = &cfg.Plex.Servers[i]
			break
		}
	}
	if found == nil {
		fmt.Fprintf(os.Stderr, "no server with machineIdentifier %q — run `lumen list` first\n", machineID)
		fmt.Fprintln(os.Stderr, "\nKnown servers:")
		for _, srv := range cfg.Plex.Servers {
			fmt.Fprintf(os.Stderr, "  %s  name=%q  display=%q\n", srv.MachineIdentifier, srv.Name, srv.DisplayName)
		}
		os.Exit(1)
	}

	old := found.DisplayName
	if old == "" {
		old = found.Name
	}
	if old == "" {
		old = found.MachineIdentifier
	}
	found.DisplayName = displayName

	if err := cfg.Save(); err != nil {
		fmt.Fprintf(os.Stderr, "save config: %v\n", err)
		os.Exit(1)
	}
	fmt.Printf("Renamed %s: %q → %q\n", machineID, old, displayName)
}
