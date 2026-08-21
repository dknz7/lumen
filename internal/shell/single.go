//go:build windows

package shell

import (
	"syscall"

	"golang.org/x/sys/windows"
)

// SingleInstanceMutex is the name Setup checks via Inno Setup's AppMutex
// directive so an installer or upgrade can tell the user to quit Lumen first,
// instead of failing on a locked lumen.exe. Changing this string means also
// changing installer/lumen.iss.
const SingleInstanceMutex = "Lumen.SingleInstance.Mutex"

// AcquireSingleInstance takes the process-wide instance lock.
//
// It returns already=true when another Lumen is already running. The handle is
// deliberately never released: the OS drops it when the process dies, which is
// what we want even if we crash.
func AcquireSingleInstance() (already bool, err error) {
	name, err := syscall.UTF16PtrFromString(SingleInstanceMutex)
	if err != nil {
		return false, err
	}
	_, err = windows.CreateMutex(nil, false, name)
	if err == windows.ERROR_ALREADY_EXISTS {
		return true, nil
	}
	if err != nil {
		return false, err
	}
	return false, nil
}
