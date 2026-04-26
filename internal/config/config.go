package config

import (
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"

	"github.com/google/uuid"
)

// UIConfig holds every user-tunable UI preference. Persisted in config.json.
type UIConfig struct {
	Theme           string                    `json:"theme"`           // "pure-oled" | future themes
	Zoom            int                       `json:"zoom"`            // viewport zoom percentage, 80-150
	CardSize        string                    `json:"cardSize"`        // "s" | "m" | "l" | "xl"
	CardDensity     int                       `json:"cardDensity"`     // 0-100, grid gap slider
	RowsPerShelf    int                       `json:"rowsPerShelf"`    // 1-4
	FontSize        int                       `json:"fontSize"`        // base rem in px, default 14
	CardLayout      string                    `json:"cardLayout"`      // "poster" | "landscape"
	DefaultSort     string                    `json:"defaultSort"`     // library default sort
	DefaultViewMode string                    `json:"defaultViewMode"` // "shows" | "episodes" for TV libraries
	Kiosk           KioskConfig               `json:"kiosk"`
	Playback        PlaybackUIConfig          `json:"playback"`
	HiddenLibraries []string                  `json:"hiddenLibraries"` // "serverID:libraryKey" entries
	ShelfState      map[string]PageShelfState `json:"shelfState"`      // keyed by page: "home", "recommended", "discover"
}

// KioskConfig — launched for real in Session 5; Session 3 just persists the toggles.
type KioskConfig struct {
	EnableOnStartup bool   `json:"enableOnStartup"`
	Browser         string `json:"browser"` // "edge" | "chrome" | "system"
}

// PlaybackUIConfig — persists the Pot Player override path (Session 4 reads it).
type PlaybackUIConfig struct {
	PotPlayerPath string `json:"potPlayerPath"`
}

// PageShelfState stores per-page shelf/group order + visibility + collapse.
type PageShelfState struct {
	GroupOrder     []string             `json:"groupOrder,omitempty"`     // order of groups on pages that have groups (Home)
	GroupCollapsed map[string]bool      `json:"groupCollapsed,omitempty"` // group ID → collapsed?
	ShelfOrder     map[string][]string  `json:"shelfOrder,omitempty"`     // group ID (or "" for ungrouped) → ordered shelf IDs
	ShelfPrefs     map[string]ShelfPref `json:"shelfPrefs,omitempty"`     // shelf ID → pref
}

// ShelfPref is per-shelf visibility/collapse state.
type ShelfPref struct {
	Hidden    bool `json:"hidden,omitempty"`
	Collapsed bool `json:"collapsed,omitempty"`
}

// Config is the full Lumen settings + credentials document stored at %APPDATA%\Lumen\config.json.
// Secret fields are DPAPI-encrypted on disk; Load/Save handle the round-trip transparently.
type Config struct {
	ClientIdentifier string     `json:"clientIdentifier"` // stable X-Plex-Client-Identifier
	OMDBKey          string     `json:"omdbKey,omitempty"`
	Plex             PlexConfig `json:"plex"`
	UI               UIConfig   `json:"ui"`
}

type PlexConfig struct {
	AccountToken string   `json:"accountToken,omitempty"` // plaintext in memory, DPAPI on disk
	Servers      []Server `json:"servers,omitempty"`
}

type Server struct {
	Name               string `json:"name"`
	DisplayName        string `json:"displayName,omitempty"` // local override — wins over Name in UI
	MachineIdentifier  string `json:"machineIdentifier"`
	AccessToken        string `json:"accessToken,omitempty"` // plaintext in memory, DPAPI on disk
	LastGoodConnection string `json:"lastGoodConnection,omitempty"`
}

// Wire shapes — what actually lives in config.json. Secret fields hold base64(DPAPI ciphertext).
type wireConfig struct {
	ClientIdentifier string         `json:"clientIdentifier"`
	OMDBKey          string         `json:"omdbKey,omitempty"`
	Plex             wirePlexConfig `json:"plex"`
	UI               UIConfig       `json:"ui"`
}

type wirePlexConfig struct {
	AccountToken string       `json:"accountToken,omitempty"`
	Servers      []wireServer `json:"servers,omitempty"`
}

type wireServer struct {
	Name               string `json:"name"`
	DisplayName        string `json:"displayName,omitempty"`
	MachineIdentifier  string `json:"machineIdentifier"`
	AccessToken        string `json:"accessToken,omitempty"`
	LastGoodConnection string `json:"lastGoodConnection,omitempty"`
}

// Load reads config.json or returns a fresh default populated with a newly-generated
// ClientIdentifier. The returned Config is never nil; its Save method writes back to
// the same location.
func Load() (*Config, error) {
	if err := EnsureDirs(); err != nil {
		return nil, err
	}
	b, err := os.ReadFile(ConfigFile())
	if errors.Is(err, fs.ErrNotExist) {
		return newDefault(), nil
	}
	if err != nil {
		return nil, err
	}
	var w wireConfig
	if err := json.Unmarshal(b, &w); err != nil {
		return nil, err
	}

	c := &Config{
		ClientIdentifier: w.ClientIdentifier,
		OMDBKey:          w.OMDBKey,
		UI:               w.UI,
	}
	if c.ClientIdentifier == "" {
		c.ClientIdentifier = uuid.NewString()
	}

	// Apply UI defaults for missing fields. Preserves any persisted values.
	if c.UI.Theme == "" {
		c.UI.Theme = "pure-oled"
	}
	if c.UI.Zoom == 0 {
		c.UI.Zoom = 100
	}
	if c.UI.CardSize == "" {
		c.UI.CardSize = "m"
	}
	if c.UI.CardDensity == 0 {
		c.UI.CardDensity = 50
	}
	if c.UI.RowsPerShelf == 0 {
		c.UI.RowsPerShelf = 3
	}
	if c.UI.FontSize == 0 {
		c.UI.FontSize = 14
	}
	if c.UI.CardLayout == "" {
		c.UI.CardLayout = "poster"
	}
	if c.UI.DefaultSort == "" {
		c.UI.DefaultSort = "addedAt:desc"
	}
	if c.UI.DefaultViewMode == "" {
		c.UI.DefaultViewMode = "episodes"
	}
	if c.UI.Kiosk.Browser == "" {
		c.UI.Kiosk.Browser = "edge"
	}
	if c.UI.ShelfState == nil {
		c.UI.ShelfState = map[string]PageShelfState{}
	}
	if c.UI.HiddenLibraries == nil {
		c.UI.HiddenLibraries = []string{}
	}

	tok, err := decryptField(w.Plex.AccountToken)
	if err != nil {
		return nil, fmt.Errorf("decrypt AccountToken: %w", err)
	}
	c.Plex.AccountToken = tok

	for _, ws := range w.Plex.Servers {
		at, err := decryptField(ws.AccessToken)
		if err != nil {
			return nil, fmt.Errorf("decrypt server %q AccessToken: %w", ws.Name, err)
		}
		lgc, err := decryptField(ws.LastGoodConnection)
		if err != nil {
			return nil, fmt.Errorf("decrypt server %q LastGoodConnection: %w", ws.Name, err)
		}
		c.Plex.Servers = append(c.Plex.Servers, Server{
			Name:               ws.Name,
			DisplayName:        ws.DisplayName,
			MachineIdentifier:  ws.MachineIdentifier,
			AccessToken:        at,
			LastGoodConnection: lgc,
		})
	}
	return c, nil
}

// Save writes the config back to disk atomically (write to temp, rename).
func (c *Config) Save() error {
	if err := EnsureDirs(); err != nil {
		return err
	}

	w := wireConfig{
		ClientIdentifier: c.ClientIdentifier,
		OMDBKey:          c.OMDBKey,
		UI:               c.UI,
	}
	at, err := encryptField(c.Plex.AccountToken)
	if err != nil {
		return fmt.Errorf("encrypt AccountToken: %w", err)
	}
	w.Plex.AccountToken = at
	for _, s := range c.Plex.Servers {
		eat, err := encryptField(s.AccessToken)
		if err != nil {
			return fmt.Errorf("encrypt server %q AccessToken: %w", s.Name, err)
		}
		elgc, err := encryptField(s.LastGoodConnection)
		if err != nil {
			return fmt.Errorf("encrypt server %q LastGoodConnection: %w", s.Name, err)
		}
		w.Plex.Servers = append(w.Plex.Servers, wireServer{
			Name:               s.Name,
			DisplayName:        s.DisplayName,
			MachineIdentifier:  s.MachineIdentifier,
			AccessToken:        eat,
			LastGoodConnection: elgc,
		})
	}

	b, err := json.MarshalIndent(w, "", "  ")
	if err != nil {
		return err
	}
	tmp := ConfigFile() + ".tmp"
	if err := os.WriteFile(tmp, b, 0o600); err != nil {
		return err
	}
	return os.Rename(tmp, ConfigFile())
}

func encryptField(plain string) (string, error) {
	if plain == "" {
		return "", nil
	}
	enc, err := dpapiEncrypt([]byte(plain))
	if err != nil {
		return "", err
	}
	return base64.StdEncoding.EncodeToString(enc), nil
}

func decryptField(encoded string) (string, error) {
	if encoded == "" {
		return "", nil
	}
	cipher, err := base64.StdEncoding.DecodeString(encoded)
	if err != nil {
		return "", fmt.Errorf("base64: %w", err)
	}
	dec, err := dpapiDecrypt(cipher)
	if err != nil {
		return "", err
	}
	return string(dec), nil
}

func newDefault() *Config {
	c := &Config{ClientIdentifier: uuid.NewString()}
	c.UI = UIConfig{
		Theme:           "pure-oled",
		Zoom:            100,
		CardSize:        "m",
		CardDensity:     50,
		RowsPerShelf:    3,
		FontSize:        14,
		CardLayout:      "poster",
		DefaultSort:     "addedAt:desc",
		DefaultViewMode: "episodes",
		Kiosk:           KioskConfig{Browser: "edge"},
		ShelfState:      map[string]PageShelfState{},
		HiddenLibraries: []string{},
	}
	return c
}
