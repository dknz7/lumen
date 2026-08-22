package config

import (
	"os"
	"path/filepath"
)

// Dir returns the Lumen config root (%APPDATA%\Lumen).
func Dir() string {
	return filepath.Join(os.Getenv("APPDATA"), "Lumen")
}

// ConfigFile returns the absolute path of config.json.
func ConfigFile() string {
	return filepath.Join(Dir(), "config.json")
}

// CacheDir returns %APPDATA%\Lumen\cache.
func CacheDir() string {
	return filepath.Join(Dir(), "cache")
}

// LogsDir returns %APPDATA%\Lumen\logs.
func LogsDir() string {
	return filepath.Join(Dir(), "logs")
}

// ThemesDir returns %APPDATA%\Lumen\themes, where user-written themes live.
//
// Themes are JSON, never code. A theme is twenty-five strings; making it a
// module would mean either shipping a compiler or evaluating a file the user
// downloaded from someone else, inside a page that can reach the local API.
// Data can be validated before it is applied, which is the whole point.
func ThemesDir() string {
	return filepath.Join(Dir(), "themes")
}

// ScratchDir returns %TEMP%\lumen.
func ScratchDir() string {
	return filepath.Join(os.TempDir(), "lumen")
}

// LogFile returns the absolute path of the main log file, creating the log
// directory if needed. Lumen is built with -H windowsgui and so has no console
// to print to; this file is where log output actually goes.
func LogFile() (string, error) {
	if err := EnsureDirs(); err != nil {
		return "", err
	}
	return filepath.Join(LogsDir(), "lumen.log"), nil
}

// EnsureDirs creates every directory Lumen expects under %APPDATA%\Lumen.
// Idempotent — safe to call on every startup.
func EnsureDirs() error {
	for _, d := range []string{
		Dir(),
		CacheDir(),
		filepath.Join(CacheDir(), "images"),
		filepath.Join(CacheDir(), "omdb"),
		LogsDir(),
		ThemesDir(),
	} {
		if err := os.MkdirAll(d, 0o755); err != nil {
			return err
		}
	}
	return nil
}
