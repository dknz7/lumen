package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestDirResolvesUnderAppData(t *testing.T) {
	t.Setenv("APPDATA", `C:\fake\AppData\Roaming`)
	got := Dir()
	want := filepath.Join(`C:\fake\AppData\Roaming`, "Lumen")
	if got != want {
		t.Fatalf("Dir() = %q, want %q", got, want)
	}
}

func TestConfigFilePath(t *testing.T) {
	t.Setenv("APPDATA", `C:\fake\AppData\Roaming`)
	got := ConfigFile()
	if !strings.HasSuffix(got, filepath.Join("Lumen", "config.json")) {
		t.Fatalf("ConfigFile() = %q, missing Lumen\\config.json suffix", got)
	}
}

func TestEnsureDirsCreatesExpectedTree(t *testing.T) {
	root := t.TempDir()
	t.Setenv("APPDATA", root)
	if err := EnsureDirs(); err != nil {
		t.Fatalf("EnsureDirs: %v", err)
	}
	for _, sub := range []string{"Lumen", "Lumen/cache", "Lumen/cache/images", "Lumen/cache/omdb", "Lumen/logs"} {
		p := filepath.Join(root, sub)
		if info, err := os.Stat(p); err != nil || !info.IsDir() {
			t.Errorf("expected directory at %q; err=%v", p, err)
		}
	}
}
