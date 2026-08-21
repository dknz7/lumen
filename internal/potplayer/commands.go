package potplayer

import "time"

// Win32 message constants and Pot Player command IDs confirmed against Pot
// Player Mini v260422 (1.7.22859) during the an earlier session spike — see
// docs/the project notes.

const (
	wmUser       uintptr = 0x0400 // base for user-defined messages
	wmAppCommand uintptr = 0x0319 // multimedia-key-style commands
	wmClose      uintptr = 0x0010 // graceful close

	// Pot Player accepts read-only queries via WM_USER + offset. Each returns
	// its result as the SendMessage return value.
	ppGetPosition uintptr = 0x5004 // returns current position in milliseconds
	ppGetDuration uintptr = 0x5002 // returns total duration in milliseconds
	ppGetState    uintptr = 0x5006 // returns 0=STOPPED, 1=PAUSED, 2=PLAYING, -1=NOT_READY

	// WM_APPCOMMAND high-word values for write-side control. Sent via
	// SendMessage(hwnd, wmAppCommand, 0, value<<16).
	appCmdMediaStop      uintptr = 13
	appCmdMediaPlayPause uintptr = 14 // toggle (kept for completeness; v1 UI doesn't use it)
)

// Sentinel returned by ppGetState while media is still loading. Pot Player
// returns -1; Go's SendMessage wrapper sees this as ^uintptr(0).
const stateNotReady = ^uintptr(0)

// Window class for Pot Player's main window. Used by FindWindowW.
const potPlayerWindowClass = "PotPlayer64"

// Cold-start: position/duration/state can return 0 or -1 for ~2 s after
// launch while media loads. Wrap reads with retries up to coldStartRetry,
// sleeping coldStartGap between attempts.
const (
	coldStartRetry = 7
	coldStartGap   = 500 * time.Millisecond
)
