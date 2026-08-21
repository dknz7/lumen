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

import "syscall"

var (
	user32 = syscall.NewLazyDLL("user32.dll")

	procSetWindowLongPtrW = user32.NewProc("SetWindowLongPtrW")
	procCallWindowProcW   = user32.NewProc("CallWindowProcW")
	procShowWindow        = user32.NewProc("ShowWindow")
	procSetForegroundWin  = user32.NewProc("SetForegroundWindow")
	procIsWindowVisible   = user32.NewProc("IsWindowVisible")
	procIsIconic          = user32.NewProc("IsIconic")
	procFlashWindow       = user32.NewProc("FlashWindow")
)

// gwlpWndProc must be a var: uintptr(-4) is rejected at compile time as an
// overflowing constant conversion, while the runtime conversion is fine.
var gwlpWndProc = -4

const (
	wmClose           = 0x0010
	wmQueryEndSession = 0x0011
	wmEndSession      = 0x0016
	wmSysCommand      = 0x0112

	scMinimize = 0xF020
	scClose    = 0xF060

	swHide    = 0
	swShow    = 5
	swRestore = 9
)

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
