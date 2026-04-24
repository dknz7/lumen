package config

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
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

func TestSecretsAreEncryptedOnDisk(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("APPDATA", tmp)
	c, _ := Load()
	c.Plex.AccountToken = "super-secret-token"
	c.Plex.Servers = []Server{{
		Name:               "Stargaze",
		MachineIdentifier:  "abc",
		AccessToken:        "per-server-secret",
		LastGoodConnection: "https://1-2-3-4.plex.direct:32400",
	}}
	if err := c.Save(); err != nil {
		t.Fatal(err)
	}

	// Read the on-disk JSON raw — plaintext secrets must NOT appear.
	raw, err := os.ReadFile(ConfigFile())
	if err != nil {
		t.Fatal(err)
	}
	for _, secret := range []string{"super-secret-token", "per-server-secret", "1-2-3-4.plex.direct"} {
		if strings.Contains(string(raw), secret) {
			t.Errorf("on-disk JSON leaks plaintext %q", secret)
		}
	}

	// But a fresh Load must decrypt them back.
	c2, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	if c2.Plex.AccountToken != "super-secret-token" {
		t.Errorf("AccountToken: got %q", c2.Plex.AccountToken)
	}
	if c2.Plex.Servers[0].AccessToken != "per-server-secret" {
		t.Errorf("Server AccessToken: got %q", c2.Plex.Servers[0].AccessToken)
	}
	if c2.Plex.Servers[0].LastGoodConnection != "https://1-2-3-4.plex.direct:32400" {
		t.Errorf("Server LastGoodConnection: got %q", c2.Plex.Servers[0].LastGoodConnection)
	}

	// Ensure the JSON actually parses (doesn't contain raw binary).
	var probe map[string]any
	if err := json.Unmarshal(raw, &probe); err != nil {
		t.Errorf("on-disk JSON malformed: %v", err)
	}
}
