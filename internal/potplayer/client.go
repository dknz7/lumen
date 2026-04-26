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

// Lazily-resolved Win32 procs used across the package.
var (
	user32           = windows.NewLazySystemDLL("user32.dll")
	procFindWindowW  = user32.NewProc("FindWindowW")
	procIsWindow     = user32.NewProc("IsWindow")
	procSendMessageW = user32.NewProc("SendMessageW")
	procPostMessageW = user32.NewProc("PostMessageW")
)

// ErrWindowNotFound is returned by Client methods when no Pot Player window
// is currently resolvable (either the cached HWND died and the helper
// couldn't find a replacement, or the subprocess never produced one).
// Callers can errors.Is this to discriminate transient-window failures from
// e.g. SendMessageW hangs.
var ErrWindowNotFound = errors.New("pot player window not found")

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
// playback is torn down. When resumeOffsetMs > 0, Pot Player's
// /seek=hh:mm:ss CLI flag is appended so playback opens at the resume
// position instead of position 0.
func Launch(exePath, streamURL string, resumeOffsetMs int64) (*Client, error) {
	if exePath == "" {
		return nil, errors.New("potplayer.Launch: empty exePath")
	}
	if streamURL == "" {
		return nil, errors.New("potplayer.Launch: empty streamURL")
	}
	args := []string{streamURL}
	if resumeOffsetMs > 0 {
		args = append(args, formatSeekArg(resumeOffsetMs))
	}
	cmd := exec.Command(exePath, args...)
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

// GetPosition returns the current playback position. May block up to
// coldStartRetry × coldStartGap (~3 s) immediately after launch while
// Pot Player loads the media; final value is whatever the last query
// returned (typically zero if the file never loaded).
func (c *Client) GetPosition() (time.Duration, error) {
	return c.getMillisQuery(ppGetPosition)
}

// GetDuration returns the media's total duration. Same cold-start envelope
// as GetPosition.
func (c *Client) GetDuration() (time.Duration, error) {
	return c.getMillisQuery(ppGetDuration)
}

// getMillisQuery is the shared retry envelope for ppGetPosition + ppGetDuration.
// Both query types return milliseconds in SendMessageW's r1; both treat 0 as
// "not yet ready, retry" and any positive value as the answer.
func (c *Client) getMillisQuery(wParam uintptr) (time.Duration, error) {
	var ms uintptr
	for i := 0; i < coldStartRetry; i++ {
		v, err := c.sendUserQuery(wParam)
		if err != nil {
			return 0, err
		}
		ms = v
		if ms > 0 {
			return time.Duration(ms) * time.Millisecond, nil
		}
		if i < coldStartRetry-1 {
			time.Sleep(coldStartGap)
		}
	}
	// All retries returned zero — surface the last value (likely zero) without
	// erroring; callers decide what to do (status poller treats it as "still
	// loading", direct-play timeout in api_play.go decides at 10 s).
	return time.Duration(ms) * time.Millisecond, nil
}

// GetState returns the current play state. Maps Pot Player's -1 sentinel to
// PlayStateUnknown so callers can distinguish "still loading" from "stopped".
// State should never legitimately be Unknown after the cold-start budget —
// if it still is, the system is wedged and callers should treat that as
// an error condition.
func (c *Client) GetState() (PlayState, error) {
	for i := 0; i < coldStartRetry; i++ {
		raw, err := c.sendUserQuery(ppGetState)
		if err != nil {
			return PlayStateUnknown, err
		}
		if raw == stateNotReady {
			if i < coldStartRetry-1 {
				time.Sleep(coldStartGap)
			}
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

// Stop halts playback and closes the Pot Player window. Returns when the
// window is gone (or after a 2 s force-kill fallback).
func (c *Client) Stop() error {
	c.mu.Lock()
	hwnd := c.hwnd
	cmd := c.cmd
	c.mu.Unlock()

	if hwnd == 0 {
		return nil // already gone
	}

	// Halt playback first.
	_ = sendAppCommand(hwnd, appCmdMediaStop)

	// Ask the window to close.
	_, _, _ = procPostMessageW.Call(uintptr(hwnd), wmClose, 0, 0)

	// Wait up to 2 s for IsWindow to flip false.
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if !isWindow(hwnd) {
			return nil
		}
		time.Sleep(100 * time.Millisecond)
	}

	// Force-kill the subprocess. Last resort.
	if cmd != nil && cmd.Process != nil {
		_ = cmd.Process.Kill()
		go func() { _ = cmd.Wait() }()
	}
	return nil
}

// formatSeekArg converts ms to Pot Player's /seek=hh:mm:ss CLI argument.
// Pot Player Mini accepts this as a launch-time seek directive.
func formatSeekArg(ms int64) string {
	s := ms / 1000
	h := s / 3600
	m := (s % 3600) / 60
	sec := s % 60
	return fmt.Sprintf("/seek=%02d:%02d:%02d", h, m, sec)
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

// sendUserQuery wraps SendMessageW with the WM_USER + wParam pattern Pot
// Player uses for read-only queries. Refreshes the HWND if the cached one
// has gone stale (Pot Player crash + auto-restart, etc.).
//
// Lock is held across the SendMessageW call. Pot Player responds in
// microseconds; if it ever hangs, every caller will block — that's
// acceptable for this single-user desktop app.
func (c *Client) sendUserQuery(wParam uintptr) (uintptr, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	// Re-find HWND if the cached one died.
	if c.hwnd == 0 || !isWindow(c.hwnd) {
		hwnd, ok := findPotPlayerWindow()
		if !ok {
			return 0, ErrWindowNotFound
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

// sendAppCommand fires WM_APPCOMMAND with the given command in the high
// word of lParam. Per Session 0 findings: SendMessage(hwnd, 0x0319, 0, cmd<<16).
func sendAppCommand(hwnd windows.Handle, cmd uintptr) error {
	_, _, _ = procSendMessageW.Call(
		uintptr(hwnd),
		wmAppCommand,
		0,
		cmd<<16,
	)
	return nil
}
