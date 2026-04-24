package config

import (
	"encoding/json"
	"errors"
	"io/fs"
	"os"

	"github.com/google/uuid"
)

// Config is the full Lumen settings + credentials document stored at %APPDATA%\Lumen\config.json.
// Encrypted fields are handled in Task 5; for now every field is plaintext JSON.
type Config struct {
	ClientIdentifier string     `json:"clientIdentifier"` // stable X-Plex-Client-Identifier
	OMDBKey          string     `json:"omdbKey,omitempty"`
	Plex             PlexConfig `json:"plex"`
}

type PlexConfig struct {
	AccountToken string   `json:"accountToken,omitempty"` // DPAPI-encrypted in Task 5
	Servers      []Server `json:"servers,omitempty"`
}

type Server struct {
	Name               string `json:"name"`
	MachineIdentifier  string `json:"machineIdentifier"`
	AccessToken        string `json:"accessToken,omitempty"` // DPAPI-encrypted in Task 5
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
	var c Config
	if err := json.Unmarshal(b, &c); err != nil {
		return nil, err
	}
	if c.ClientIdentifier == "" {
		c.ClientIdentifier = uuid.NewString()
	}
	return &c, nil
}

// Save writes the config back to disk atomically (write to temp, rename).
func (c *Config) Save() error {
	if err := EnsureDirs(); err != nil {
		return err
	}
	b, err := json.MarshalIndent(c, "", "  ")
	if err != nil {
		return err
	}
	tmp := ConfigFile() + ".tmp"
	if err := os.WriteFile(tmp, b, 0o600); err != nil {
		return err
	}
	return os.Rename(tmp, ConfigFile())
}

func newDefault() *Config {
	return &Config{ClientIdentifier: uuid.NewString()}
}
