package main

import (
	"fmt"
	"os"
	"time"

	"github.com/pkg/browser"

	"lumen/internal/config"
	"lumen/internal/plex"
)

func runAuth(args []string) {
	cfg, err := config.Load()
	if err != nil {
		fmt.Fprintf(os.Stderr, "load config: %v\n", err)
		os.Exit(1)
	}
	c := plex.NewClient(cfg.ClientIdentifier, version)

	pin, err := c.CreatePIN()
	if err != nil {
		fmt.Fprintf(os.Stderr, "create PIN: %v\n", err)
		os.Exit(1)
	}

	link := plex.ForceBrowserURL(pin.Code)
	fmt.Printf("Enter this code at %s\n  Code: %s\n", plex.LinkURL(), pin.Code)
	if err := browser.OpenURL(link); err != nil {
		fmt.Fprintf(os.Stderr, "(couldn't open browser automatically: %v)\n", err)
	}
	fmt.Println("Waiting for you to link the PIN...")

	token, err := c.PollPIN(pin, 5*time.Minute)
	if err != nil {
		fmt.Fprintf(os.Stderr, "poll PIN: %v\n", err)
		os.Exit(1)
	}

	cfg.Plex.AccountToken = token
	if err := cfg.Save(); err != nil {
		fmt.Fprintf(os.Stderr, "save config: %v\n", err)
		os.Exit(1)
	}
	fmt.Println("Authentication successful.")
}
