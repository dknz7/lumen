// Lumen — Windows desktop client for Plex. Entrypoint.
//
// The binary is linked with -H windowsgui, so it is a GUI-subsystem process
// with no console. Launching it with no arguments opens the app window; the
// subcommands below stay available for scripting and debugging, and reattach
// themselves to the calling terminal so their output is visible.
package main

import (
	"fmt"
	"os"

	"lumen/internal/shell"
)

// Stamped at build time, e.g.
//
//	-ldflags "-X main.version=1.0.0 -X main.commit=abc1234"
var (
	version   = "0.0.0-dev"
	commit    = ""
	buildDate = ""
)

func main() {
	// Before anything else, and specifically before any window exists: opt in
	// to per-monitor DPI awareness. Windows latches a process's awareness the
	// moment it creates its first HWND, so this genuinely has to be first.
	// Without it every window is rendered at 96 DPI and bitmap-stretched to
	// the monitor's scale — 25% oversized and visibly soft on a 125% display.
	shell.EnablePerMonitorDPI()

	// No arguments: the normal double-click / Start-menu path. Straight to GUI.
	if len(os.Args) < 2 {
		runApp(appOptions{})
		return
	}

	switch os.Args[1] {
	// --- GUI ---
	case "--tray", "-tray":
		// Used by the "start with Windows" shortcut: boot to the tray only.
		runApp(appOptions{StartHidden: true})
	case "serve":
		// Kept as a subcommand so shortcuts created by older versions still
		// work. Defaults to the window; --browser restores the old behaviour.
		runServe(os.Args[2:])

	// --- CLI. These print, so borrow the parent terminal's console. ---
	case "auth":
		attachParentConsole()
		runAuth(os.Args[2:])
	case "list":
		attachParentConsole()
		runList(os.Args[2:])
	case "probe-hubs":
		attachParentConsole()
		runProbeHubs(os.Args[2:])
	case "rename":
		attachParentConsole()
		runRename(os.Args[2:])
	case "install-shortcut":
		attachParentConsole()
		runInstallShortcut(os.Args[2:])
	case "version", "--version", "-v":
		attachParentConsole()
		fmt.Printf("lumen %s\n", version)
	case "help", "--help", "-h", "/?":
		attachParentConsole()
		usage(os.Stdout)
	default:
		attachParentConsole()
		fmt.Fprintf(os.Stderr, "unknown subcommand: %s\n\n", os.Args[1])
		usage(os.Stderr)
		os.Exit(2)
	}
}

func usage(w *os.File) {
	fmt.Fprintf(w, `Lumen %s — a Windows desktop client for Plex.

usage: lumen [subcommand] [flags]

Running lumen with no arguments opens the app.

subcommands:
  (none)            Open the Lumen window
  --tray            Start minimised to the system tray
  serve             Open the Lumen window (alias, kept for old shortcuts)
    --browser         ... in your default browser instead of a native window
    --addr <addr>     ... on a different address (default 127.0.0.1:7832)
  auth              Link a Plex account via the PIN flow
  list              List connected Plex servers and their libraries
  rename            Set a local display name for a server:
                      lumen rename <machineID> "Living Room"
  install-shortcut  Create a Lumen shortcut on your Desktop
  probe-hubs        Probe Plex Discover hub slugs (diagnostic)
  version           Print the version
`, version)
}
