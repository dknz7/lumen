// Package potplayer drives Pot Player Mini 64-bit via Win32 IPC. All
// command IDs and message constants live in commands.go and were confirmed
// against Pot Player v260422 (1.7.22859) during the Session 0 spike.
package potplayer

import (
	"errors"
	"fmt"
	"os/exec"
	"sync"
	"syscall"
	"time"
	"unsafe"

	"golang.org/x/sys/windows"
)

// PlayState mirrors Pot Player's GetState return values plus a sentinel.
type PlayState int

const (
	PlayStateUnknown PlayState = iota // returned during cold-start (-1 from Pot Player)
	PlayStatePaused                   // 1
	PlayStatePlaying                  // 2
	PlayStateStopped                  // synthetic — produced when window is gone
)

func (s PlayState) String() string {
	switch s {
	case PlayStatePaused:
		return "paused"
	case PlayStatePlaying:
		return "playing"
	case PlayStateStopped:
		return "stopped"
	default:
		return "unknown"
	}
}

// Client wraps a single Pot Player subprocess. One Client per playback session.
// All methods are safe to call concurrently.
type Client struct {
	mu   sync.Mutex
	hwnd windows.Handle
	cmd  *exec.Cmd
	exe  string
}

// Launch spawns Pot Player against streamURL and waits up to 3 s for the
// HWND to appear before returning. Caller should keep the *Client until
// playback is torn down.
func Launch(exePath, streamURL string) (*Client, error) {
	if exePath == "" {
		return nil, errors.New("potplayer.Launch: empty exePath")
	}
	if streamURL == "" {
		return nil, errors.New("potplayer.Launch: empty streamURL")
	}
	cmd := exec.Command(exePath, streamURL)
	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("start PotPlayer: %w", err)
	}
	c := &Client{cmd: cmd, exe: exePath}

	// Poll for the window. ~10 attempts × 300 ms = 3 s budget.
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		hwnd, ok := findPotPlayerWindow()
		if ok {
			c.mu.Lock()
			c.hwnd = hwnd
			c.mu.Unlock()
			return c, nil
		}
		time.Sleep(300 * time.Millisecond)
	}
	// Window never appeared — kill the subprocess to avoid orphaning.
	_ = cmd.Process.Kill()
	return nil, errors.New("potplayer.Launch: window did not appear within 3s")
}

// IsAlive returns true while Pot Player's window is still valid. Cheap —
// safe to call from the status poller every 5 s.
func (c *Client) IsAlive() bool {
	c.mu.Lock()
	hwnd := c.hwnd
	c.mu.Unlock()
	if hwnd == 0 {
		return false
	}
	return isWindow(hwnd)
}

// findPotPlayerWindow looks up the top-level window with class PotPlayer64.
// Returns the HWND and true if found.
func findPotPlayerWindow() (windows.Handle, bool) {
	user32 := windows.NewLazySystemDLL("user32.dll")
	findWindowW := user32.NewProc("FindWindowW")
	className, _ := syscall.UTF16PtrFromString(potPlayerWindowClass)
	r1, _, _ := findWindowW.Call(
		uintptr(unsafe.Pointer(className)),
		0,
	)
	if r1 == 0 {
		return 0, false
	}
	return windows.Handle(r1), true
}

// isWindow wraps user32.IsWindow.
func isWindow(hwnd windows.Handle) bool {
	user32 := windows.NewLazySystemDLL("user32.dll")
	isWin := user32.NewProc("IsWindow")
	r1, _, _ := isWin.Call(uintptr(hwnd))
	return r1 != 0
}
