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
	} {
		if err := os.MkdirAll(d, 0o755); err != nil {
			return err
		}
	}
	return nil
}
