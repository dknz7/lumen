// Lumen — personal Windows Plex companion. CLI entrypoint.
package main

import (
	"fmt"
	"os"
)

const version = "0.1.0-dev"

func main() {
	if len(os.Args) < 2 {
		usage()
		os.Exit(2)
	}
	switch os.Args[1] {
	case "auth":
		runAuth(os.Args[2:])
	case "list":
		runList(os.Args[2:])
	case "probe-hubs":
		runProbeHubs(os.Args[2:])
	case "serve":
		runServe(os.Args[2:])
	case "rename":
		runRename(os.Args[2:])
	case "version", "--version", "-v":
		fmt.Printf("lumen %s\n", version)
	default:
		fmt.Fprintf(os.Stderr, "unknown subcommand: %s\n\n", os.Args[1])
		usage()
		os.Exit(2)
	}
}

func usage() {
	fmt.Fprint(os.Stderr, `usage: lumen <subcommand> [args]

subcommands:
  auth         Run Plex PIN flow and store account token
  list         List connected Plex servers and their libraries
  probe-hubs   Probe Plex Discover hub slugs (diagnostic)
  serve        Start the Lumen web app (HTTP server on 127.0.0.1:7832)
  rename       Set a local display name for a server (e.g. rename <machineID> "Stargaze")
  version      Print lumen version
`)
}
