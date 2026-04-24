// Package potplayer drives Pot Player Mini 64-bit via Win32 IPC.
// Implementation lands in Session 4 based on docs/session-0-findings.md.
package potplayer

import (
	"errors"
	"time"
)

// ErrNotImplemented is returned by every method until Session 4.
var ErrNotImplemented = errors.New("potplayer: not implemented until Session 4")

// PlayState matches Pot Player's state codes confirmed in Session 0:
// 1 = Paused, 2 = Playing. Session 0 also found that Pot Player returns -1
// during the first ~2 s while media is loading — Session 4 maps that to Unknown.
type PlayState int

const (
	Unknown PlayState = 0
	Paused  PlayState = 1
	Playing PlayState = 2
	Stopped PlayState = 99 // synthetic; Pot Player itself doesn't emit this
)

// Client controls a single Pot Player instance via its HWND.
type Client struct {
	// Populated in Session 4.
}

// Launch spawns Pot Player against the given stream URL and returns a Client
// once the window handle is resolvable.
func Launch(streamURL string) (*Client, error) { return nil, ErrNotImplemented }

func (c *Client) GetPosition() (time.Duration, error) { return 0, ErrNotImplemented }
func (c *Client) GetDuration() (time.Duration, error) { return 0, ErrNotImplemented }
func (c *Client) GetState() (PlayState, error)        { return Unknown, ErrNotImplemented }
func (c *Client) Pause() error                        { return ErrNotImplemented }
func (c *Client) Resume() error                       { return ErrNotImplemented }
func (c *Client) Stop() error                         { return ErrNotImplemented }
func (c *Client) IsAlive() bool                       { return false }
