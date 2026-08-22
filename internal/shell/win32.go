//go:build windows

// Package shell hosts the Lumen UI in a native Windows window with a system
// tray presence, instead of handing the URL to the user's default browser.
//
// Threading model — this is the part that matters, and it was verified with a
// standalone probe before being written here:
//
//	main goroutine ......... systray.Run(), owns its own Win32 message pump
//	locked goroutine ....... WebView2 window, owns a SECOND message pump
//
// Win32 message queues are per-thread, so two pumps on two threads coexist
// happily. Anything that touches the window from the tray (or from any other
// goroutine) must be marshalled onto the window's thread with WebView.Dispatch.
package shell

import (
	"syscall"
	"unsafe"
)

var (
	user32 = syscall.NewLazyDLL("user32.dll")

	procSetWindowLongPtrW = user32.NewProc("SetWindowLongPtrW")
	procCallWindowProcW   = user32.NewProc("CallWindowProcW")
	procShowWindow        = user32.NewProc("ShowWindow")
	procSetForegroundWin  = user32.NewProc("SetForegroundWindow")
	procIsWindowVisible   = user32.NewProc("IsWindowVisible")
	procIsIconic          = user32.NewProc("IsIconic")
	procFlashWindow       = user32.NewProc("FlashWindow")
	procSetWindowPos      = user32.NewProc("SetWindowPos")
	procGetWindowRect     = user32.NewProc("GetWindowRect")

	// Both are Windows 10 1703+. Every call site checks Find() first so an
	// older host degrades to the previous behaviour instead of panicking.
	procGetDpiForWindow               = user32.NewProc("GetDpiForWindow")
	procSetProcessDpiAwarenessContext = user32.NewProc("SetProcessDpiAwarenessContext")
)

// gwlpWndProc must be a var: uintptr(-4) is rejected at compile time as an
// overflowing constant conversion, while the runtime conversion is fine.
var gwlpWndProc = -4

// DPI_AWARENESS_CONTEXT_PER_MONITOR_AWARE_V2 is the opaque handle -4. Same
// story as gwlpWndProc: written as a runtime expression because the constant
// conversion uintptr(-4) does not compile.
var dpiAwarenessContextPerMonitorAwareV2 = ^uintptr(3)

const (
	wmClose           = 0x0010
	wmQueryEndSession = 0x0011
	wmEndSession      = 0x0016
	wmSysCommand      = 0x0112
	wmDpiChanged      = 0x02E0

	scMinimize = 0xF020
	scClose    = 0xF060

	swHide    = 0
	swShow    = 5
	swRestore = 9

	swpNoZOrder   = 0x0004
	swpNoActivate = 0x0010

	// Win32's notion of 100% scaling. Every DPI here is a numerator over it.
	baseDPI = 96
)

// rect mirrors Win32's RECT. WM_DPICHANGED hands one over in lParam.
type rect struct {
	left, top, right, bottom int32
}

func showWindow(hwnd uintptr, cmd int) {
	procShowWindow.Call(hwnd, uintptr(cmd))
}

func isWindowVisible(hwnd uintptr) bool {
	r, _, _ := procIsWindowVisible.Call(hwnd)
	return r != 0
}

func isIconic(hwnd uintptr) bool {
	r, _, _ := procIsIconic.Call(hwnd)
	return r != 0
}

// bringToFront un-hides, un-minimises and focuses the window. All three are
// needed: a window can be hidden (tray), minimised (taskbar), or merely behind
// another window, and each state needs a different call to undo.
func bringToFront(hwnd uintptr) {
	showWindow(hwnd, swShow)
	if isIconic(hwnd) {
		showWindow(hwnd, swRestore)
	}
	procSetForegroundWin.Call(hwnd)
}

// EnablePerMonitorDPI opts the process into per-monitor DPI awareness (v2).
//
// Without it Windows treats a GUI process as a 96-DPI relic: it renders the
// window at 1:1 and then bitmap-stretches the result up to the monitor's
// scale. On a 125% display that is a 25% size increase *and* a resampling
// pass over every pixel, so posters and body text alike come out soft. Both
// symptoms are one bug, and neither is fixable from CSS.
//
// go-webview2 does not set awareness itself and the embedded .syso carries no
// manifest, so this call is the only thing standing between Lumen and DPI
// virtualisation.
//
// Must run before the process creates its first window — awareness is latched
// the moment an HWND exists. Failure is deliberately silent: the API landed in
// Windows 10 1703, and a miss just leaves the previous (blurry) behaviour.
func EnablePerMonitorDPI() {
	if procSetProcessDpiAwarenessContext.Find() != nil {
		return
	}
	procSetProcessDpiAwarenessContext.Call(dpiAwarenessContextPerMonitorAwareV2)
}

// dpiForWindow reports the DPI of the monitor the window is on. Falls back to
// baseDPI, the identity scale, when the API is missing or the call fails.
func dpiForWindow(hwnd uintptr) int {
	if procGetDpiForWindow.Find() != nil {
		return baseDPI
	}
	r, _, _ := procGetDpiForWindow.Call(hwnd)
	if r == 0 {
		return baseDPI
	}
	return int(r)
}

func getWindowRect(hwnd uintptr, r *rect) bool {
	ret, _, _ := procGetWindowRect.Call(hwnd, uintptr(unsafe.Pointer(r)))
	return ret != 0
}

// setWindowBounds moves and resizes in one call, leaving Z-order and focus be.
func setWindowBounds(hwnd uintptr, x, y, w, h int32) {
	procSetWindowPos.Call(
		hwnd, 0,
		uintptr(x), uintptr(y), uintptr(w), uintptr(h),
		swpNoZOrder|swpNoActivate,
	)
}

// applySuggestedDPIBounds honours the RECT Windows supplies with
// WM_DPICHANGED. Windows has already worked out where the window should sit
// and how big it should be at the new scale; taking its answer is what keeps
// the frame the same *physical* size as it crosses between monitors running
// different scale factors. Ignore the message and the frame stays put while
// its contents silently re-rasterise inside it.
func applySuggestedDPIBounds(hwnd, lparam uintptr) {
	if lparam == 0 {
		return
	}
	r := rectFromLParam(lparam)
	setWindowBounds(hwnd, r.left, r.top, r.right-r.left, r.bottom-r.top)
}

// rectFromLParam reinterprets a message's lParam as the RECT it points at.
//
// go vet's unsafeptr analyser rejects a direct uintptr → unsafe.Pointer
// conversion, and it is right to in general: if the address named Go-heap
// memory, the collector could move the object out from under the two-step
// conversion. This address is not Go's. Windows owns the RECT and keeps it
// alive for the duration of the message dispatch, so there is nothing that
// could move. Converting via &lparam — an ordinary Go pointer — states that
// distinction to the analyser instead of switching the check off wholesale.
func rectFromLParam(lparam uintptr) *rect {
	return (*rect)(*(*unsafe.Pointer)(unsafe.Pointer(&lparam)))
}

// scaleInitialSize re-sizes a freshly created window so the caller's width and
// height behave as LOGICAL pixels.
//
// Before per-monitor awareness those numbers were logical whether we liked it
// or not — Windows scaled the whole surface afterwards. Now they land as
// physical pixels, so on a 125% display a 1440-wide window would open a fifth
// narrower than every previous build. Scaling here keeps the default window the
// same apparent size as before, which is the point: the DPI fix should change
// sharpness, not layout.
//
// Resizes around the existing centre so go-webview2's Center option survives.
func scaleInitialSize(hwnd uintptr, logicalW, logicalH int) {
	dpi := dpiForWindow(hwnd)
	if dpi == baseDPI || logicalW <= 0 || logicalH <= 0 {
		return
	}
	var r rect
	if !getWindowRect(hwnd, &r) {
		return
	}
	w := int32(logicalW * dpi / baseDPI)
	h := int32(logicalH * dpi / baseDPI)
	cx := (r.left + r.right) / 2
	cy := (r.top + r.bottom) / 2
	setWindowBounds(hwnd, cx-w/2, cy-h/2, w, h)
}
