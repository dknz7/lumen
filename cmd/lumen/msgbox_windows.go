package main

import (
	"syscall"
	"unsafe"
)

var procMessageBoxW = syscall.NewLazyDLL("user32.dll").NewProc("MessageBoxW")

const (
	mbOK            = 0x00000000
	mbIconError     = 0x00000010
	mbSetForeground = 0x00010000
	mbTopMost       = 0x00040000
)

// errorBox shows a native error dialog.
//
// lumen.exe is a GUI-subsystem binary, so a startup failure printed to stderr
// goes nowhere the user will ever look. Anything fatal enough to stop the app
// has to be shown on screen.
func errorBox(title, msg string) {
	t, err := syscall.UTF16PtrFromString(title)
	if err != nil {
		return
	}
	m, err := syscall.UTF16PtrFromString(msg)
	if err != nil {
		return
	}
	procMessageBoxW.Call(
		0,
		uintptr(unsafe.Pointer(m)),
		uintptr(unsafe.Pointer(t)),
		uintptr(mbOK|mbIconError|mbSetForeground|mbTopMost),
	)
}
