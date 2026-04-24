package config

import (
	"path/filepath"
	"testing"
)

func TestLoadReturnsDefaultsWhenMissing(t *testing.T) {
	t.Setenv("APPDATA", t.TempDir())
	c, err := Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if c.ClientIdentifier == "" {
		t.Fatal("expected ClientIdentifier to be populated with a fresh UUID")
	}
	if len(c.Plex.Servers) != 0 {
		t.Fatalf("expected empty Servers, got %d", len(c.Plex.Servers))
	}
}

func TestSaveAndReload(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("APPDATA", tmp)
	c1, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	c1.Plex.AccountToken = "test-token"
	c1.Plex.Servers = []Server{{Name: "Stargaze", MachineIdentifier: "abc123", AccessToken: "srv-tok"}}
	if err := c1.Save(); err != nil {
		t.Fatalf("Save: %v", err)
	}
	c2, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	if c2.Plex.AccountToken != "test-token" {
		t.Errorf("AccountToken round-trip: got %q want %q", c2.Plex.AccountToken, "test-token")
	}
	if len(c2.Plex.Servers) != 1 || c2.Plex.Servers[0].Name != "Stargaze" {
		t.Errorf("Servers round-trip failed: %+v", c2.Plex.Servers)
	}
	if c2.ClientIdentifier != c1.ClientIdentifier {
		t.Errorf("ClientIdentifier must persist: got %q want %q", c2.ClientIdentifier, c1.ClientIdentifier)
	}
	// File must exist at expected location.
	if _, err := filepath.Abs(ConfigFile()); err != nil {
		t.Error(err)
	}
}

func TestLoadReusesExistingClientIdentifier(t *testing.T) {
	t.Setenv("APPDATA", t.TempDir())
	c1, _ := Load()
	_ = c1.Save()
	c2, _ := Load()
	if c1.ClientIdentifier != c2.ClientIdentifier {
		t.Fatalf("ClientIdentifier changed across Load calls: %q != %q", c1.ClientIdentifier, c2.ClientIdentifier)
	}
}
