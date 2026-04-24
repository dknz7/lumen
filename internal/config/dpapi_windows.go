//go:build windows

package config

import (
	"fmt"
	"syscall"
	"unsafe"

	"golang.org/x/sys/windows"
)

// Windows DATA_BLOB shape for CryptProtectData / CryptUnprotectData.
type dataBlob struct {
	cbData uint32
	pbData *byte
}

var (
	crypt32            = windows.NewLazySystemDLL("crypt32.dll")
	procCryptProtect   = crypt32.NewProc("CryptProtectData")
	procCryptUnprotect = crypt32.NewProc("CryptUnprotectData")
)

// dpapiEncrypt wraps CryptProtectData with user-scoped keying.
// Output is opaque bytes safe to write to config.json (will be base64-encoded by callers in Task 5).
func dpapiEncrypt(plain []byte) ([]byte, error) {
	if len(plain) == 0 {
		return nil, nil
	}
	in := dataBlob{cbData: uint32(len(plain)), pbData: &plain[0]}
	var out dataBlob
	ret, _, err := procCryptProtect.Call(
		uintptr(unsafe.Pointer(&in)),
		0, 0, 0, 0,
		0, // flags (user scope default)
		uintptr(unsafe.Pointer(&out)),
	)
	if ret == 0 {
		return nil, fmt.Errorf("CryptProtectData: %w", err)
	}
	defer localFree(uintptr(unsafe.Pointer(out.pbData)))
	return copyBlob(out), nil
}

// dpapiDecrypt wraps CryptUnprotectData. Returns an error if the blob was
// produced by a different user (DPAPI is user-scoped) or if it's malformed.
func dpapiDecrypt(cipher []byte) ([]byte, error) {
	if len(cipher) == 0 {
		return nil, nil
	}
	in := dataBlob{cbData: uint32(len(cipher)), pbData: &cipher[0]}
	var out dataBlob
	ret, _, err := procCryptUnprotect.Call(
		uintptr(unsafe.Pointer(&in)),
		0, 0, 0, 0,
		0,
		uintptr(unsafe.Pointer(&out)),
	)
	if ret == 0 {
		return nil, fmt.Errorf("CryptUnprotectData: %w", err)
	}
	defer localFree(uintptr(unsafe.Pointer(out.pbData)))
	return copyBlob(out), nil
}

func copyBlob(b dataBlob) []byte {
	s := unsafe.Slice(b.pbData, b.cbData)
	c := make([]byte, b.cbData)
	copy(c, s)
	return c
}

// localFree releases memory allocated by the Crypt32 API.
func localFree(p uintptr) {
	_, _, _ = syscall.Syscall(procLocalFree.Addr(), 1, p, 0, 0)
}

var (
	kernel32      = windows.NewLazySystemDLL("kernel32.dll")
	procLocalFree = kernel32.NewProc("LocalFree")
)
