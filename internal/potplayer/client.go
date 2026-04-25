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

// Lazily-resolved Win32 procs used across the package. Each call to
// LazyProc.Call resolves the proc address on first use and caches it; we
// hold these at package level so the cache is shared across goroutines and
// helpers don't re-allocate the wrappers per invocation.
var (
	user32           = windows.NewLazySystemDLL("user32.dll")
	procFindWindowW  = user32.NewProc("FindWindowW")
	procIsWindow     = user32.NewProc("IsWindow")
	procSendMessageW = user32.NewProc("SendMessageW")
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
	// Window never appeared — kill the subprocess to avoid orphaning. Reap in
	// the background so the OS process handle is released without slowing the
	// error return.
	_ = cmd.Process.Kill()
	go func() { _ = cmd.Wait() }()
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
	className, _ := syscall.UTF16PtrFromString(potPlayerWindowClass)
	r1, _, _ := procFindWindowW.Call(
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
	r1, _, _ := procIsWindow.Call(uintptr(hwnd))
	return r1 != 0
}

// GetPosition returns the current playback position. May block up to
// coldStartRetry × coldStartGap (~3 s) immediately after launch while
// Pot Player loads the media.
func (c *Client) GetPosition() (time.Duration, error) {
	for i := 0; i < coldStartRetry; i++ {
		ms, err := c.sendUserQuery(ppGetPosition)
		if err != nil {
			return 0, err
		}
		if ms > 0 {
			return time.Duration(ms) * time.Millisecond, nil
		}
		time.Sleep(coldStartGap)
	}
	// Final attempt — return whatever we got, even zero.
	ms, err := c.sendUserQuery(ppGetPosition)
	if err != nil {
		return 0, err
	}
	return time.Duration(ms) * time.Millisecond, nil
}

// GetDuration returns the media's total duration. Same cold-start envelope.
func (c *Client) GetDuration() (time.Duration, error) {
	for i := 0; i < coldStartRetry; i++ {
		ms, err := c.sendUserQuery(ppGetDuration)
		if err != nil {
			return 0, err
		}
		if ms > 0 {
			return time.Duration(ms) * time.Millisecond, nil
		}
		time.Sleep(coldStartGap)
	}
	ms, err := c.sendUserQuery(ppGetDuration)
	if err != nil {
		return 0, err
	}
	return time.Duration(ms) * time.Millisecond, nil
}

// GetState returns the current play state. Maps Pot Player's -1 sentinel to
// PlayStateUnknown so callers can distinguish "still loading" from "stopped".
func (c *Client) GetState() (PlayState, error) {
	for i := 0; i < coldStartRetry; i++ {
		raw, err := c.sendUserQuery(ppGetState)
		if err != nil {
			return PlayStateUnknown, err
		}
		if raw == stateNotReady {
			time.Sleep(coldStartGap)
			continue
		}
		switch raw {
		case 1:
			return PlayStatePaused, nil
		case 2:
			return PlayStatePlaying, nil
		default:
			// Unrecognized — surface as Unknown rather than guessing.
			return PlayStateUnknown, nil
		}
	}
	return PlayStateUnknown, nil
}

// sendUserQuery wraps SendMessageW with the WM_USER + wParam pattern Pot
// Player uses for read-only queries. Refreshes the HWND if the cached one
// has gone stale (Pot Player crash + auto-restart, etc.).
func (c *Client) sendUserQuery(wParam uintptr) (uintptr, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	// Re-find HWND if the cached one died.
	if c.hwnd == 0 || !isWindow(c.hwnd) {
		hwnd, ok := findPotPlayerWindow()
		if !ok {
			return 0, errors.New("pot player window not found")
		}
		c.hwnd = hwnd
	}

	r1, _, _ := procSendMessageW.Call(
		uintptr(c.hwnd),
		wmUser,
		wParam,
		0,
	)
	return r1, nil
}
