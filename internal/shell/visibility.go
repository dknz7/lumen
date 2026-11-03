//go:build windows

package shell

import "sync/atomic"

// Telling the page it has come back.
//
// Hiding to the tray is a bare ShowWindow(SW_HIDE), and minimising doesn't
// reach WebView2 either, so the page is never told it went away: to it,
// document.visibilityState is "visible" for as long as the process runs, and
// visibilitychange never fires. The SPA's refetch-on-return listeners
// (web/src/util/focusRefetch.ts) therefore never ran on a restore. Lumen lives in the
// tray for days, so every restore showed the view exactly as it was left.
//
// So the shell says it explicitly: the first time the window comes back after
// being hidden or minimised, it fires the SPA's own lumen:data-invalidated
// event, which every view already listens for. Alt-tabbing between visible
// windows doesn't count as coming back, so it doesn't trigger a refetch storm.

const (
	wmSize       = 0x0005
	wmActivate   = 0x0006
	wmShowWindow = 0x0018

	sizeRestored  = 0
	sizeMinimized = 1
	sizeMaximized = 2

	waInactive = 0
)

const refreshScript = `window.dispatchEvent(new CustomEvent("lumen:data-invalidated"))`

// away is true from the moment the window is hidden or minimised until the
// first message that shows it again.
var away atomic.Bool

// noteVisibility tracks hide/minimise/restore from the window's messages and
// reports true exactly once per return. Several messages arrive on a return
// (shown, restored, activated); whichever lands first claims it.
//
// onScreen reports whether the window is visible and not minimised right now.
// WM_SIZE and WM_ACTIVATE also reach a window that is still hidden: a monitor
// waking, a dock or undock, or a DPI change resizes it in the tray. Taking
// those as a return spent the refresh on a page nobody could see, and the real
// restore afterwards got nothing. WM_SHOWWINDOW doesn't consult it, because
// it is sent before the window becomes visible, so onScreen is still false.
func noteVisibility(msg, wparam uintptr, onScreen func() bool) bool {
	switch msg {
	case wmShowWindow:
		if wparam == 0 {
			away.Store(true)
			return false
		}
		return away.CompareAndSwap(true, false)

	case wmSize:
		switch wparam {
		case sizeMinimized:
			away.Store(true)
		case sizeRestored, sizeMaximized:
			return away.Load() && onScreen() && away.CompareAndSwap(true, false)
		}

	case wmActivate:
		// LOWORD is the activation state. HIWORD is non-zero when the window
		// is activated while still minimised (a taskbar click activates
		// before it restores); that isn't back on screen yet.
		if wparam&0xFFFF != waInactive && (wparam>>16)&0xFFFF == 0 {
			return away.Load() && onScreen() && away.CompareAndSwap(true, false)
		}
	}
	return false
}

// requestRefresh asks the page to refetch. Queued with Dispatch rather than run
// inline: it is called from inside the window procedure, possibly mid-ShowWindow,
// and the dispatch queue runs it once that message has been handled.
func requestRefresh() {
	state.mu.Lock()
	wv := state.wv
	state.mu.Unlock()
	if wv == nil {
		return
	}
	wv.Dispatch(func() { wv.Eval(refreshScript) })
}
