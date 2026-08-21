package main

import (
	"os"
	"syscall"
)

var (
	kernel32          = syscall.NewLazyDLL("kernel32.dll")
	procAttachConsole = kernel32.NewProc("AttachConsole")
	procGetStdHandle  = kernel32.NewProc("GetStdHandle")
)

const (
	attachParentProcess = ^uintptr(0)      // (DWORD)-1
	stdOutputHandle     = ^uintptr(11) + 1 // (DWORD)-11
	invalidHandleValue  = ^uintptr(0)
)

// attachParentConsole makes CLI output visible when lumen.exe is run from a
// terminal.
//
// lumen.exe is linked with -H windowsgui so double-clicking it doesn't flash a
// console window. The side effect is that a GUI-subsystem process starts with
// no console of its own, so `lumen version` would print into the void.
// AttachConsole(ATTACH_PARENT_PROCESS) borrows the calling terminal's console.
//
// The important subtlety, found by testing rather than reading: when the caller
// has ALREADY given us usable std handles — `lumen version > out.txt`, or a
// shell capturing our output through a pipe — reopening CONOUT$ over the top of
// them throws that redirection away and writes to the console window instead.
// The first version of this function did exactly that and produced an empty
// out.txt. So: if a valid stdout handle already exists, leave everything alone.
//
// Known limitation, inherent to a single GUI-subsystem binary: the shell does
// not wait for a GUI process, so it returns to the prompt immediately and this
// output arrives after it. Use `Start-Process -Wait` (PowerShell) or
// `start /wait` (cmd) when the exit code matters.
func attachParentConsole() {
	if stdoutIsUsable() {
		return // already redirected or inherited — don't touch it
	}
	if r, _, _ := procAttachConsole.Call(attachParentProcess); r == 0 {
		return // no parent console (launched from Explorer) — nothing to attach
	}
	// Go cached the std handles at startup, when they pointed at nothing.
	// Re-open them against the console we just attached to.
	if f, err := os.OpenFile("CONOUT$", os.O_WRONLY, 0); err == nil {
		os.Stdout = f
		os.Stderr = f
	}
	if f, err := os.OpenFile("CONIN$", os.O_RDONLY, 0); err == nil {
		os.Stdin = f
	}
}

// stdoutIsUsable reports whether the process was handed a real stdout — a
// console, a pipe, or a redirected file.
func stdoutIsUsable() bool {
	h, _, _ := procGetStdHandle.Call(stdOutputHandle)
	return h != 0 && h != invalidHandleValue
}
