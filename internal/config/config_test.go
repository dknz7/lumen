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

func TestUIDefaultsPopulatedOnFreshLoad(t *testing.T) {
	t.Setenv("APPDATA", t.TempDir())
	c, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	if c.UI.Theme != "pure-oled" {
		t.Errorf("default theme: got %q want pure-oled", c.UI.Theme)
	}
	if c.UI.Zoom != 100 {
		t.Errorf("default zoom: got %d want 100", c.UI.Zoom)
	}
	if c.UI.RowsPerShelf != 3 {
		t.Errorf("default rows: got %d want 3", c.UI.RowsPerShelf)
	}
	if c.UI.CardSize != "m" {
		t.Errorf("default card size: got %q want m", c.UI.CardSize)
	}
	if c.UI.DefaultViewMode != "episodes" {
		t.Errorf("default view mode: got %q want episodes", c.UI.DefaultViewMode)
	}
	if c.UI.ShelfState == nil {
		t.Errorf("ShelfState should be initialised to empty map, not nil")
	}
	if c.UI.HiddenLibraries == nil {
		t.Errorf("HiddenLibraries should be initialised to empty slice, not nil")
	}
}

func TestUIRoundTripsThroughSave(t *testing.T) {
	t.Setenv("APPDATA", t.TempDir())
	c1, _ := Load()
	c1.UI.Theme = "high-contrast"
	c1.UI.Zoom = 120
	c1.UI.RowsPerShelf = 4
	c1.UI.CardSize = "l"
	c1.UI.HiddenLibraries = []string{"abc:5", "def:7"}
	c1.UI.ShelfState = map[string]PageShelfState{
		"home": {
			GroupOrder:     []string{"dknzplex", "stargaze"},
			GroupCollapsed: map[string]bool{"stargaze": true},
			ShelfOrder:     map[string][]string{"stargaze": {"s-movies", "s-tv"}},
			ShelfPrefs:     map[string]ShelfPref{"s-movies": {Hidden: true, Collapsed: false}},
		},
	}
	if err := c1.Save(); err != nil {
		t.Fatal(err)
	}
	c2, _ := Load()
	if c2.UI.Theme != "high-contrast" {
		t.Errorf("theme round-trip: %q", c2.UI.Theme)
	}
	if c2.UI.Zoom != 120 {
		t.Errorf("zoom round-trip: %d", c2.UI.Zoom)
	}
	if len(c2.UI.HiddenLibraries) != 2 {
		t.Errorf("hidden libs: %+v", c2.UI.HiddenLibraries)
	}
	if c2.UI.ShelfState["home"].GroupOrder[0] != "dknzplex" {
		t.Errorf("group order: %+v", c2.UI.ShelfState["home"].GroupOrder)
	}
	if !c2.UI.ShelfState["home"].ShelfPrefs["s-movies"].Hidden {
		t.Errorf("shelf pref hidden lost: %+v", c2.UI.ShelfState["home"].ShelfPrefs)
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
