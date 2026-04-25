package main

import (
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/pkg/browser"

	"lumen/internal/config"
	"lumen/internal/plex"
	"lumen/internal/server"
)

func runServe(args []string) {
	cfg, err := config.Load()
	if err != nil {
		fmt.Fprintf(os.Stderr, "load config: %v\n", err)
		os.Exit(1)
	}
	if cfg.Plex.AccountToken == "" {
		fmt.Fprintln(os.Stderr, "no Plex account token — run `lumen auth` first")
		os.Exit(1)
	}
	if len(cfg.Plex.Servers) == 0 {
		fmt.Fprintln(os.Stderr, "no servers discovered — run `lumen list` first")
		os.Exit(1)
	}

	c := plex.NewClient(cfg.ClientIdentifier, version)
	s := server.New(cfg, c, "127.0.0.1:7832")

	errCh := make(chan error, 1)
	go func() { errCh <- s.ListenAndServe() }()

	// Give it 200 ms to bind, then open the browser.
	time.Sleep(200 * time.Millisecond)
	url := "http://127.0.0.1:7832"
	fmt.Printf("Lumen is serving at %s\n", url)
	if err := browser.OpenURL(url); err != nil {
		fmt.Fprintf(os.Stderr, "(couldn't open browser automatically: %v)\n", err)
	}

	// Trap SIGINT / SIGTERM for graceful shutdown. Also listen on s.Quit() so
	// the SPA's "Close Lumen" button can end the process.
	sig := make(chan os.Signal, 1)
	signal.Notify(sig, os.Interrupt, syscall.SIGTERM)
	select {
	case <-sig:
		fmt.Println("\nshutting down...")
		_ = s.Shutdown()
	case <-s.Quit():
		fmt.Println("\nquit requested via Lumen UI, shutting down...")
		_ = s.Shutdown()
	case err := <-errCh:
		fmt.Fprintf(os.Stderr, "server error: %v\n", err)
		os.Exit(1)
	}
}
