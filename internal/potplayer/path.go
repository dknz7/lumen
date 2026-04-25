package potplayer

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"golang.org/x/sys/windows/registry"
)

// ResolveExePath returns the absolute path to PotPlayerMini64.exe, in order:
//
//  1. override (Settings → Playback → Pot Player path) if non-empty AND the
//     file exists.
//  2. HKCU\Software\DAUM\PotPlayerMini64\ProgramPath registry value.
//  3. Default install locations: C:\Program Files\DAUM\PotPlayer\,
//     C:\Program Files\DAUM\PotPlayerMini64\.
//
// Returns an error only if every stage fails. Stage-1 misses (override given
// but file absent) fall through silently — they're not user-facing errors.
func ResolveExePath(override string) (string, error) {
	if override != "" {
		if _, err := os.Stat(override); err == nil {
			return override, nil
		}
		// Override given but missing — fall through, do not error here.
	}

	if p, ok := readRegistryProgramPath(); ok {
		if _, err := os.Stat(p); err == nil {
			return p, nil
		}
	}

	defaults := []string{
		`C:\Program Files\DAUM\PotPlayer\PotPlayerMini64.exe`,
		`C:\Program Files\DAUM\PotPlayerMini64\PotPlayerMini64.exe`,
		`C:\Program Files (x86)\DAUM\PotPlayer\PotPlayerMini64.exe`,
	}
	for _, p := range defaults {
		if _, err := os.Stat(p); err == nil {
			return p, nil
		}
	}

	return "", fmt.Errorf("Pot Player executable not found — set Settings → Playback → Pot Player path")
}

func readRegistryProgramPath() (string, bool) {
	k, err := registry.OpenKey(registry.CURRENT_USER, `Software\DAUM\PotPlayerMini64`, registry.QUERY_VALUE)
	if err != nil {
		return "", false
	}
	defer k.Close()
	v, _, err := k.GetStringValue("ProgramPath")
	if err != nil || v == "" {
		return "", false
	}
	// Some installers store the directory; append exe name if so.
	if filepath.Ext(v) == "" {
		v = filepath.Join(v, "PotPlayerMini64.exe")
	}
	return v, true
}

// ErrExeNotFound is exported for callers that want to discriminate.
var ErrExeNotFound = errors.New("pot player executable not found")
