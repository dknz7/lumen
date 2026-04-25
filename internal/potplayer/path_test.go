package potplayer

import (
	"os"
	"path/filepath"
	"testing"
)

func TestResolveExePath_OverrideTakesPrecedence(t *testing.T) {
	dir := t.TempDir()
	fake := filepath.Join(dir, "PotPlayerMini64.exe")
	if err := os.WriteFile(fake, []byte{0x4D, 0x5A}, 0o644); err != nil {
		t.Fatal(err)
	}
	got, err := ResolveExePath(fake)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != fake {
		t.Errorf("override ignored; got %q, want %q", got, fake)
	}
}

func TestResolveExePath_OverrideMissing_FallsThrough(t *testing.T) {
	// Override points at a non-existent path; resolver should NOT return it.
	missing := filepath.Join(t.TempDir(), "nope.exe")
	got, err := ResolveExePath(missing)
	// On a CI box with no Pot Player installed, this errors. On Byron's box it
	// returns the real install path.
	if err == nil && got == missing {
		t.Errorf("resolver returned non-existent override path %q", got)
	}
}
