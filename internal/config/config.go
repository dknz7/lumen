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

// Config is the full Lumen settings + credentials document stored at %APPDATA%\Lumen\config.json.
// Secret fields are DPAPI-encrypted on disk; Load/Save handle the round-trip transparently.
type Config struct {
	ClientIdentifier string     `json:"clientIdentifier"` // stable X-Plex-Client-Identifier
	OMDBKey          string     `json:"omdbKey,omitempty"`
	Plex             PlexConfig `json:"plex"`
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
	}
	if c.ClientIdentifier == "" {
		c.ClientIdentifier = uuid.NewString()
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
	return &Config{ClientIdentifier: uuid.NewString()}
}
