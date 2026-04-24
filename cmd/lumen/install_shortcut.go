package main

import (
	"fmt"
	"os"

	"lumen/internal/shortcuts"
)

func runInstallShortcut(args []string) {
	exe, err := os.Executable()
	if err != nil {
		fmt.Fprintf(os.Stderr, "resolve exe path: %v\n", err)
		os.Exit(1)
	}
	path, err := shortcuts.CreateDesktop(exe, "serve")
	if err != nil {
		fmt.Fprintf(os.Stderr, "create shortcut: %v\n", err)
		os.Exit(1)
	}
	fmt.Printf("Created desktop shortcut at %s\n", path)
}
