package potplayer

// Win32 message constants and Pot Player command IDs confirmed against Pot
// Player Mini v260422 (1.7.22859) during the Session 0 spike — see
// docs/session-0-findings.md.

const (
	wmUser       uintptr = 0x0400 // base for user-defined messages
	wmAppCommand uintptr = 0x0319 // multimedia-key-style commands
	wmClose      uintptr = 0x0010 // graceful close

	// Pot Player accepts read-only queries via WM_USER + offset. Each returns
	// its result as the SendMessage return value.
	ppGetPosition uintptr = 0x5004 // returns current position in milliseconds
	ppGetDuration uintptr = 0x5002 // returns total duration in milliseconds
	ppGetState    uintptr = 0x5006 // returns 1=PAUSED, 2=PLAYING, -1=NOT_READY

	// WM_APPCOMMAND high-word values for write-side control. Sent via
	// SendMessage(hwnd, wmAppCommand, 0, value<<16).
	appCmdMediaStop      uintptr = 13
	appCmdMediaPlayPause uintptr = 14 // toggle (kept for completeness; v1 UI doesn't use it)
)

// Window class for Pot Player's main window. Used by FindWindowW.
const potPlayerWindowClass = "PotPlayer64"

// Cold-start: position/duration/state can return 0 or -1 for ~2 s after
// launch while media loads. Wrap reads with retries up to coldStartRetry.
const (
	coldStartRetry  = 6           // number of read attempts during cold-start
	coldStartGap_ms = 500         // ms between attempts
	stateNotReady   = ^uintptr(0) // -1 cast to uintptr; Go sees uint64 max
)
