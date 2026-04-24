# Lumen — Session 1 Implementation Plan (Foundation & Plex API Client)

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Stand up the Lumen Go module, DPAPI-encrypted config store, and a Plex API client that can PIN-auth to plex.tv, discover both of Byron's servers (Stargaze + DKNZPLEX), pick working remote connections, and enumerate libraries/items/hubs. End state: `lumen auth` completes the PIN flow; `lumen list` prints both servers, their picked connection URLs, and every library on each.

**Architecture:** A single Go module rooted at the repo with three internal packages — `internal/config` (DPAPI-encrypted JSON at `%APPDATA%\Lumen\config.json`), `internal/plex` (HTTP client + auth + discovery + connection picker + library/item/hub calls), and `internal/potplayer` (Session 4 skeleton, method stubs only). The `cmd/lumen` binary exposes CLI subcommands (`auth`, `list`, `probe-hubs`) in Session 1; the HTTP server + SPA arrive in Session 2. TDD throughout for packages that don't require a live Plex account; end-to-end manual verification against Byron's real account in the final task.

**Tech Stack:**
- Go 1.22+ (1.26.2 confirmed installed).
- Stdlib `net/http` for Plex HTTP calls — no third-party HTTP client.
- `net/http/httptest` for unit-test fakes.
- `golang.org/x/sys/windows` for DPAPI (`CryptProtectData` / `CryptUnprotectData`).
- `github.com/google/uuid` for the stable `X-Plex-Client-Identifier`.
- `github.com/pkg/browser` for opening the default browser during PIN flow.

**Session 0 carry-in:** the probe's `probe/` module stays untouched — it's its own Go module (`module lumen/probe`) and is not imported by anything here. Session 0 findings (`docs/session-0-findings.md`) inform Session 4's `internal/potplayer` implementation, not this session.

**Pre-flight:**

- Working directory: `C:\Users\dicke\Desktop\Dump Zone\STACK\04-DEV\lumen`.
- Stay on `main` branch. Solo repo.
- Go module name: `lumen` (short; project is personal and may never be published, easy to rename later with `go mod edit -module` if needed).
- Byron needs a working Plex account (he has one — same credentials used by the official Plex Web client).
- Byron supplies a valid account when running Task 16's verification; no account needed until then.

---

## File Structure

```
lumen/
├── go.mod                              # module lumen
├── go.sum
├── cmd/
│   └── lumen/
│       ├── main.go                     # Subcommand dispatch
│       ├── auth.go                     # `lumen auth` — runs PIN flow
│       ├── list.go                     # `lumen list` — prints servers + libraries
│       └── probe_hubs.go               # `lumen probe-hubs` — Pick Up Again slug probe
├── internal/
│   ├── config/
│   │   ├── paths.go                    # %APPDATA%\Lumen\ resolver
│   │   ├── paths_test.go
│   │   ├── config.go                   # Config struct + Load/Save
│   │   ├── config_test.go
│   │   ├── dpapi_windows.go            # DPAPI encrypt/decrypt
│   │   └── dpapi_windows_test.go
│   ├── plex/
│   │   ├── types.go                    # Server, Library, Item, Connection, HubItem
│   │   ├── client.go                   # HTTP client + standard headers
│   │   ├── client_test.go
│   │   ├── auth.go                     # PIN create + poll
│   │   ├── auth_test.go
│   │   ├── resources.go                # DiscoverServers via /api/v2/resources
│   │   ├── resources_test.go
│   │   ├── connection.go               # PickConnection probe + fallback
│   │   ├── connection_test.go
│   │   ├── libraries.go                # GetLibraries / GetItems / GetItem / Search
│   │   ├── libraries_test.go
│   │   ├── hubs.go                     # GetHub(namespace, slug)
│   │   └── hubs_test.go
│   └── potplayer/
│       └── client.go                   # Skeleton only — methods return ErrNotImplemented
└── web/
    └── .gitkeep                        # Placeholder; populated Session 2
```

Probe module (`probe/`) is untouched.

---

## Task 1: Repo-root Go module + `cmd/lumen/main.go` skeleton

**Files:**
- Create: `go.mod`, `go.sum`
- Create: `cmd/lumen/main.go`
- Create: `web/.gitkeep`

- [ ] **Step 1: Initialise the module**

Run (from repo root):
```bash
go mod init lumen
```
Expected: `go: creating new go.mod: module lumen`.

- [ ] **Step 2: Create the main.go skeleton with subcommand dispatch**

Create `cmd/lumen/main.go`:
```go
// Lumen — personal Windows Plex companion. CLI entrypoint.
package main

import (
	"fmt"
	"os"
)

const version = "0.1.0-dev"

func main() {
	if len(os.Args) < 2 {
		usage()
		os.Exit(2)
	}
	switch os.Args[1] {
	case "auth":
		runAuth(os.Args[2:])
	case "list":
		runList(os.Args[2:])
	case "probe-hubs":
		runProbeHubs(os.Args[2:])
	case "version", "--version", "-v":
		fmt.Printf("lumen %s\n", version)
	default:
		fmt.Fprintf(os.Stderr, "unknown subcommand: %s\n\n", os.Args[1])
		usage()
		os.Exit(2)
	}
}

func usage() {
	fmt.Fprintln(os.Stderr, `usage: lumen <subcommand> [args]

subcommands:
  auth         Run Plex PIN flow and store account token
  list         List connected Plex servers and their libraries
  probe-hubs   Probe Plex Discover hub slugs (diagnostic)
  version      Print lumen version
`)
}
```

- [ ] **Step 3: Stub the subcommand handlers**

Create `cmd/lumen/auth.go`:
```go
package main

func runAuth(args []string) {
	panic("not implemented — Task 8")
}
```

Create `cmd/lumen/list.go`:
```go
package main

func runList(args []string) {
	panic("not implemented — Task 13")
}
```

Create `cmd/lumen/probe_hubs.go`:
```go
package main

func runProbeHubs(args []string) {
	panic("not implemented — Task 14")
}
```

- [ ] **Step 4: Placeholder for web embed**

Create `web/.gitkeep` (empty file).

- [ ] **Step 5: Verify it builds**

Run:
```bash
go build -o lumen.exe ./cmd/lumen
./lumen.exe version
./lumen.exe
```
Expected: build succeeds; `version` prints `lumen 0.1.0-dev`; no-args run prints usage and exits 2.

- [ ] **Step 6: Commit**

```bash
git add go.mod cmd/lumen/ web/.gitkeep
git commit -m "feat(lumen): scaffold Go module and CLI subcommand dispatch"
```

---

## Task 2: Config paths helper

**Files:**
- Create: `internal/config/paths.go`
- Create: `internal/config/paths_test.go`

**Context:** Spec §15 + §21 lock `%APPDATA%\Lumen\` as the config root, `%APPDATA%\Lumen\config.json` as the config file, and `%APPDATA%\Lumen\cache\` + `%APPDATA%\Lumen\logs\` + `%TEMP%\lumen\` as siblings. We need a single source of truth for these paths.

- [ ] **Step 1: Write the failing test**

Create `internal/config/paths_test.go`:
```go
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
```

- [ ] **Step 2: Run the test and confirm it fails**

```bash
go test ./internal/config/...
```
Expected: FAIL — package has no non-test source files or `Dir`/`ConfigFile`/`EnsureDirs` not defined.

- [ ] **Step 3: Implement paths.go**

Create `internal/config/paths.go`:
```go
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
```

- [ ] **Step 4: Verify the tests pass**

```bash
go test ./internal/config/...
```
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/config/paths.go internal/config/paths_test.go
git commit -m "feat(config): resolve %APPDATA%\\Lumen paths and ensure directory tree"
```

---

## Task 3: Config struct + JSON load/save (plaintext round-trip first)

**Files:**
- Create: `internal/config/config.go`
- Create: `internal/config/config_test.go`

**Context:** Spec §15 fields needed for Session 1 — `ClientIdentifier` (stable GUID), `Plex.AccountToken` (becomes encrypted in Task 5), `Plex.Servers[*].Name` / `MachineIdentifier` / `AccessToken` / `LastGoodConnection`, plus a top-level `OMDBKey` (used Session 2 onward, but the field lives here). We load/save as JSON in this task; Task 5 wires in DPAPI.

- [ ] **Step 1: Write the failing test**

Create `internal/config/config_test.go`:
```go
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
```

- [ ] **Step 2: Run the test and confirm it fails**

```bash
go test ./internal/config/...
```
Expected: FAIL — `Load`, `Save`, `Server` not defined.

- [ ] **Step 3: Add the uuid dependency**

```bash
go get github.com/google/uuid
```

- [ ] **Step 4: Implement config.go**

Create `internal/config/config.go`:
```go
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
```

- [ ] **Step 5: Verify the tests pass**

```bash
go test ./internal/config/...
```
Expected: PASS.

- [ ] **Step 6: Commit**

```bash
git add go.mod go.sum internal/config/config.go internal/config/config_test.go
git commit -m "feat(config): Config struct with JSON round-trip and stable client identifier"
```

---

## Task 4: DPAPI encrypt/decrypt helper

**Files:**
- Create: `internal/config/dpapi_windows.go`
- Create: `internal/config/dpapi_windows_test.go`

**Context:** Spec §15 + §16 — user-scoped DPAPI for `plex.accountToken`, `plex.servers[*].accessToken`, `plex.servers[*].lastGoodConnection`. Windows only. We wrap `CryptProtectData` and `CryptUnprotectData` from `crypt32.dll`. File is `_windows.go` so the build won't try to compile on other platforms (not that Lumen supports them — but the syntax courtesy matters).

- [ ] **Step 1: Write the failing test**

Create `internal/config/dpapi_windows_test.go`:
```go
package config

import (
	"strings"
	"testing"
)

func TestDPAPIRoundTrip(t *testing.T) {
	plain := []byte("my-plex-token-12345")
	enc, err := dpapiEncrypt(plain)
	if err != nil {
		t.Fatalf("encrypt: %v", err)
	}
	if len(enc) == 0 {
		t.Fatal("ciphertext is empty")
	}
	if strings.Contains(string(enc), string(plain)) {
		t.Fatal("ciphertext contains plaintext — something is wrong")
	}
	dec, err := dpapiDecrypt(enc)
	if err != nil {
		t.Fatalf("decrypt: %v", err)
	}
	if string(dec) != string(plain) {
		t.Fatalf("round-trip mismatch: got %q want %q", dec, plain)
	}
}

func TestDPAPIRejectsGarbage(t *testing.T) {
	_, err := dpapiDecrypt([]byte("this is not a DPAPI blob"))
	if err == nil {
		t.Fatal("expected error on garbage input, got nil")
	}
}
```

- [ ] **Step 2: Run the test and confirm it fails**

```bash
go test ./internal/config/...
```
Expected: FAIL — `dpapiEncrypt` / `dpapiDecrypt` not defined.

- [ ] **Step 3: Implement dpapi_windows.go**

Create `internal/config/dpapi_windows.go`:
```go
//go:build windows

package config

import (
	"fmt"
	"syscall"
	"unsafe"

	"golang.org/x/sys/windows"
)

// Windows DATA_BLOB shape for CryptProtectData / CryptUnprotectData.
type dataBlob struct {
	cbData uint32
	pbData *byte
}

var (
	crypt32             = windows.NewLazySystemDLL("crypt32.dll")
	procCryptProtect    = crypt32.NewProc("CryptProtectData")
	procCryptUnprotect  = crypt32.NewProc("CryptUnprotectData")
)

// dpapiEncrypt wraps CryptProtectData with user-scoped keying.
// Output is opaque bytes safe to write to config.json (will be base64-encoded by callers in Task 5).
func dpapiEncrypt(plain []byte) ([]byte, error) {
	if len(plain) == 0 {
		return nil, nil
	}
	in := dataBlob{cbData: uint32(len(plain)), pbData: &plain[0]}
	var out dataBlob
	ret, _, err := procCryptProtect.Call(
		uintptr(unsafe.Pointer(&in)),
		0, 0, 0, 0,
		0, // flags (user scope default)
		uintptr(unsafe.Pointer(&out)),
	)
	if ret == 0 {
		return nil, fmt.Errorf("CryptProtectData: %w", err)
	}
	defer localFree(uintptr(unsafe.Pointer(out.pbData)))
	return copyBlob(out), nil
}

// dpapiDecrypt wraps CryptUnprotectData. Returns an error if the blob was
// produced by a different user (DPAPI is user-scoped) or if it's malformed.
func dpapiDecrypt(cipher []byte) ([]byte, error) {
	if len(cipher) == 0 {
		return nil, nil
	}
	in := dataBlob{cbData: uint32(len(cipher)), pbData: &cipher[0]}
	var out dataBlob
	ret, _, err := procCryptUnprotect.Call(
		uintptr(unsafe.Pointer(&in)),
		0, 0, 0, 0,
		0,
		uintptr(unsafe.Pointer(&out)),
	)
	if ret == 0 {
		return nil, fmt.Errorf("CryptUnprotectData: %w", err)
	}
	defer localFree(uintptr(unsafe.Pointer(out.pbData)))
	return copyBlob(out), nil
}

func copyBlob(b dataBlob) []byte {
	s := unsafe.Slice(b.pbData, b.cbData)
	c := make([]byte, b.cbData)
	copy(c, s)
	return c
}

// localFree releases memory allocated by the Crypt32 API.
func localFree(p uintptr) {
	_, _ = syscall.Syscall(procLocalFree.Addr(), 1, p, 0, 0)
}

var (
	kernel32      = windows.NewLazySystemDLL("kernel32.dll")
	procLocalFree = kernel32.NewProc("LocalFree")
)
```

- [ ] **Step 4: Add the dependency if missing**

```bash
go get golang.org/x/sys/windows
```
(May already be in `go.sum` via the probe module graph, but the root module needs its own record.)

- [ ] **Step 5: Verify the tests pass**

```bash
go test ./internal/config/...
```
Expected: PASS — both round-trip and garbage-input tests.

- [ ] **Step 6: Commit**

```bash
git add go.mod go.sum internal/config/dpapi_windows.go internal/config/dpapi_windows_test.go
git commit -m "feat(config): DPAPI encrypt/decrypt for user-scoped secret fields"
```

---

## Task 5: Wire DPAPI into Config load/save for encrypted fields

**Files:**
- Modify: `internal/config/config.go`
- Modify: `internal/config/config_test.go`

**Context:** On save, DPAPI-encrypt `AccountToken`, each `Server.AccessToken`, and each `Server.LastGoodConnection`; base64-encode the ciphertext so it round-trips through JSON. On load, base64-decode and DPAPI-decrypt. Unencrypted legacy values (empty strings stay empty) must not crash.

- [ ] **Step 1: Extend the existing test to cover encrypted storage**

Append to `internal/config/config_test.go`:
```go
import "encoding/json"

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
```

Add `"os"` and `"strings"` to the test-file imports if not already there.

- [ ] **Step 2: Run the test and confirm it fails**

```bash
go test ./internal/config/...
```
Expected: FAIL — plaintext secrets leak to disk (the test finds them).

- [ ] **Step 3: Rewrite Load/Save to encrypt secrets through a wire struct**

Replace `Load` and `Save` in `internal/config/config.go` with the DPAPI-aware implementation:
```go
import (
	"encoding/base64"
	// ... keep existing imports
)

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
	MachineIdentifier  string `json:"machineIdentifier"`
	AccessToken        string `json:"accessToken,omitempty"`
	LastGoodConnection string `json:"lastGoodConnection,omitempty"`
}

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
			MachineIdentifier:  ws.MachineIdentifier,
			AccessToken:        at,
			LastGoodConnection: lgc,
		})
	}
	return c, nil
}

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
```

Add `"fmt"` to imports if not already present.

- [ ] **Step 4: Verify all tests pass**

```bash
go test ./internal/config/...
```
Expected: PASS for all four tests (default, round-trip, identifier-reuse, encrypted-on-disk).

- [ ] **Step 5: Commit**

```bash
git add internal/config/config.go internal/config/config_test.go
git commit -m "feat(config): DPAPI-encrypt AccountToken, per-server tokens, last-good connections on disk"
```

---

## Task 6: Plex client foundation — standard headers + HTTP plumbing

**Files:**
- Create: `internal/plex/types.go`
- Create: `internal/plex/client.go`
- Create: `internal/plex/client_test.go`

**Context:** Every plex.tv and per-server HTTP call carries the same base headers (§3 Session 1 deliverables: `X-Plex-Product`, `X-Plex-Version`, `X-Plex-Platform`, `X-Plex-Device`, `X-Plex-Client-Identifier`). We also need `Accept: application/json` since plex.tv defaults to XML. Building one `Client` struct that owns the HTTP client, the client identifier, and the version string keeps every other task simple.

- [ ] **Step 1: Define the core types (no methods yet)**

Create `internal/plex/types.go`:
```go
package plex

// Server is one Plex Media Server the account has access to.
// BaseURL is set by PickConnection.
type Server struct {
	Name              string
	MachineIdentifier string
	AccessToken       string
	BaseURL           string      // populated by connection picker
	Connections       []Connection
}

// Connection is a single advertised URI for a Plex server.
type Connection struct {
	URI      string // e.g. https://1-2-3-4.plex.direct:32400
	Address  string
	Port     int
	Local    bool
	Relay    bool
	Protocol string // "https" only in practice (we pass includeHttps=1)
	IPv6     bool
}

// Library is a top-level section on a server (Movies, TV Shows, Anime, etc.).
type Library struct {
	ID    string
	Key   string // numeric section key used in URLs
	Title string
	Type  string // "movie" | "show" | ...
}

// Item is a single piece of media — movie, show, season, or episode.
type Item struct {
	RatingKey string
	GUID      string // plex://movie/<guid>, plex://show/<guid>, etc.
	Title     string
	Type      string
	Year      int
	Summary   string
}

// HubItem is one card on a plex.tv Discover hub (home or watchlist namespace).
type HubItem struct {
	GUID      string
	RatingKey string
	Title     string
	Type      string
	Year      int
}

// ItemQuery carries optional filter/sort parameters for GetItems.
type ItemQuery struct {
	Sort    string // e.g. "addedAt:desc"
	Filters map[string]string
	Start   int // pagination offset
	Size    int // page size
}
```

- [ ] **Step 2: Write the failing client test**

Create `internal/plex/client_test.go`:
```go
package plex

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestStandardHeadersAppliedOnRequest(t *testing.T) {
	var got http.Header
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		got = r.Header.Clone()
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	c := NewClient("test-client-id", "1.2.3")
	req, err := c.NewRequest("GET", srv.URL+"/ping", nil)
	if err != nil {
		t.Fatal(err)
	}
	resp, err := c.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()

	checks := map[string]string{
		"X-Plex-Product":           "Lumen",
		"X-Plex-Version":           "1.2.3",
		"X-Plex-Platform":          "Windows",
		"X-Plex-Device":            "PC",
		"X-Plex-Client-Identifier": "test-client-id",
		"Accept":                   "application/json",
	}
	for k, want := range checks {
		if g := got.Get(k); g != want {
			t.Errorf("header %s: got %q want %q", k, g, want)
		}
	}
}

func TestClientPropagatesTokenHeader(t *testing.T) {
	var got http.Header
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		got = r.Header.Clone()
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	c := NewClient("id", "0.0.1")
	req, _ := c.NewRequest("GET", srv.URL, nil)
	c.SetToken(req, "token-abc")
	resp, _ := c.Do(req)
	resp.Body.Close()

	if got.Get("X-Plex-Token") != "token-abc" {
		t.Fatalf("X-Plex-Token: got %q want %q", got.Get("X-Plex-Token"), "token-abc")
	}
}
```

- [ ] **Step 3: Run the test and confirm it fails**

```bash
go test ./internal/plex/...
```
Expected: FAIL — `NewClient`, `Client.NewRequest`, `Client.Do`, `Client.SetToken` not defined.

- [ ] **Step 4: Implement client.go**

Create `internal/plex/client.go`:
```go
package plex

import (
	"io"
	"net/http"
	"time"
)

// Client is a thin wrapper around http.Client that stamps every outbound request
// with Lumen's stable identity headers.
type Client struct {
	http             *http.Client
	clientIdentifier string
	version          string
}

// NewClient builds a Plex-aware HTTP client. clientIdentifier should be the stable
// UUID loaded from config (Task 3); version is Lumen's semver.
func NewClient(clientIdentifier, version string) *Client {
	return &Client{
		http: &http.Client{
			Timeout: 15 * time.Second,
		},
		clientIdentifier: clientIdentifier,
		version:          version,
	}
}

// NewRequest constructs an http.Request with Lumen's identity headers applied.
// method is "GET", "POST", etc.; url is absolute; body may be nil.
func (c *Client) NewRequest(method, url string, body io.Reader) (*http.Request, error) {
	req, err := http.NewRequest(method, url, body)
	if err != nil {
		return nil, err
	}
	req.Header.Set("X-Plex-Product", "Lumen")
	req.Header.Set("X-Plex-Version", c.version)
	req.Header.Set("X-Plex-Platform", "Windows")
	req.Header.Set("X-Plex-Device", "PC")
	req.Header.Set("X-Plex-Client-Identifier", c.clientIdentifier)
	req.Header.Set("Accept", "application/json")
	return req, nil
}

// SetToken stamps the X-Plex-Token header on a request. Supply either an
// account-level token (plex.tv calls) or a per-server token (server calls).
func (c *Client) SetToken(req *http.Request, token string) {
	req.Header.Set("X-Plex-Token", token)
}

// Do executes the request via the underlying http.Client.
func (c *Client) Do(req *http.Request) (*http.Response, error) {
	return c.http.Do(req)
}
```

- [ ] **Step 5: Verify the tests pass**

```bash
go test ./internal/plex/...
```
Expected: PASS for both tests.

- [ ] **Step 6: Commit**

```bash
git add internal/plex/types.go internal/plex/client.go internal/plex/client_test.go
git commit -m "feat(plex): HTTP client with stable Lumen identity headers"
```

---

## Task 7: Plex PIN flow — create + poll

**Files:**
- Create: `internal/plex/auth.go`
- Create: `internal/plex/auth_test.go`

**Context:** Spec §3 + Plex docs. Two endpoints, both on `plex.tv`:

1. `POST https://plex.tv/api/v2/pins?strong=true` (with our identity headers, no token) — creates a PIN. Response JSON:
   ```json
   { "id": 1234567, "code": "ABCD", "authToken": null, "expiresAt": "..." }
   ```
2. `GET https://plex.tv/api/v2/pins/<id>` (with identity headers, no token) — poll. When the user visits `https://plex.tv/link` and enters the code, a subsequent poll returns the JSON with `authToken` populated.

The PIN expires after 30 minutes but in practice we poll for 5 minutes and stop. Poll every 2 s.

- [ ] **Step 1: Write the failing test with an httptest fake**

Create `internal/plex/auth_test.go`:
```go
package plex

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestCreatePINParsesResponse(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "POST" || !strings.HasSuffix(r.URL.Path, "/pins") {
			http.Error(w, "wrong path/method", 404)
			return
		}
		if r.Header.Get("X-Plex-Client-Identifier") == "" {
			http.Error(w, "missing identity header", 400)
			return
		}
		_ = json.NewEncoder(w).Encode(map[string]any{
			"id":   42,
			"code": "ABCD",
		})
	}))
	defer srv.Close()

	c := NewClient("id", "1.0.0")
	c.plexTVBase = srv.URL // overridable for tests
	pin, err := c.CreatePIN()
	if err != nil {
		t.Fatal(err)
	}
	if pin.ID != 42 || pin.Code != "ABCD" {
		t.Errorf("pin: got %+v", pin)
	}
}

func TestPollPINReturnsTokenWhenClaimed(t *testing.T) {
	calls := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		// First two polls: unclaimed. Third: claimed.
		resp := map[string]any{"id": 42, "code": "ABCD"}
		if calls >= 3 {
			resp["authToken"] = "live-token-xyz"
		}
		_ = json.NewEncoder(w).Encode(resp)
	}))
	defer srv.Close()

	c := NewClient("id", "1.0.0")
	c.plexTVBase = srv.URL
	c.pinPollInterval = 10 * time.Millisecond // speed the test up

	token, err := c.PollPIN(PIN{ID: 42}, 2*time.Second)
	if err != nil {
		t.Fatal(err)
	}
	if token != "live-token-xyz" {
		t.Errorf("token: got %q", token)
	}
	if calls < 3 {
		t.Errorf("expected at least 3 polls, got %d", calls)
	}
}

func TestPollPINTimesOut(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{"id": 42})
	}))
	defer srv.Close()

	c := NewClient("id", "1.0.0")
	c.plexTVBase = srv.URL
	c.pinPollInterval = 10 * time.Millisecond

	_, err := c.PollPIN(PIN{ID: 42}, 50*time.Millisecond)
	if err == nil {
		t.Fatal("expected timeout error, got nil")
	}
}
```

- [ ] **Step 2: Run the test and confirm it fails**

```bash
go test ./internal/plex/...
```
Expected: FAIL — `CreatePIN`, `PollPIN`, `PIN`, `plexTVBase`, `pinPollInterval` not defined.

- [ ] **Step 3: Extend Client with plex.tv base + poll interval**

Modify `internal/plex/client.go` — inside the `Client` struct add:
```go
	plexTVBase      string        // override for tests; default set in NewClient
	pinPollInterval time.Duration // override for tests; default 2 s
```

And in `NewClient`, after initializing the struct but before returning:
```go
	c := &Client{
		http: &http.Client{
			Timeout: 15 * time.Second,
		},
		clientIdentifier: clientIdentifier,
		version:          version,
		plexTVBase:       "https://plex.tv",
		pinPollInterval:  2 * time.Second,
	}
	return c
```

(Adjust the existing `return &Client{...}` accordingly; the shape is: assign to variable `c`, return `c`.)

- [ ] **Step 4: Implement auth.go**

Create `internal/plex/auth.go`:
```go
package plex

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"time"
)

// PIN is the result of Client.CreatePIN. Code is what the user types at plex.tv/link.
type PIN struct {
	ID   int    `json:"id"`
	Code string `json:"code"`
}

// pinResponse matches the plex.tv /api/v2/pins JSON shape, with the authToken
// field populated once the user claims the PIN.
type pinResponse struct {
	ID        int    `json:"id"`
	Code      string `json:"code"`
	AuthToken string `json:"authToken"`
}

// CreatePIN asks plex.tv for a new PIN with a 4-char Code and numeric ID.
// Byron will enter the Code at https://plex.tv/link.
func (c *Client) CreatePIN() (PIN, error) {
	u := c.plexTVBase + "/api/v2/pins?strong=true"
	req, err := c.NewRequest("POST", u, nil)
	if err != nil {
		return PIN{}, err
	}
	resp, err := c.Do(req)
	if err != nil {
		return PIN{}, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusCreated && resp.StatusCode != http.StatusOK {
		return PIN{}, fmt.Errorf("create pin: status %d", resp.StatusCode)
	}
	var p pinResponse
	if err := json.NewDecoder(resp.Body).Decode(&p); err != nil {
		return PIN{}, err
	}
	return PIN{ID: p.ID, Code: p.Code}, nil
}

// PollPIN polls /api/v2/pins/<id> until authToken is populated or timeout elapses.
// Returns the account token on success.
func (c *Client) PollPIN(pin PIN, timeout time.Duration) (string, error) {
	deadline := time.Now().Add(timeout)
	u := fmt.Sprintf("%s/api/v2/pins/%d", c.plexTVBase, pin.ID)
	for {
		req, err := c.NewRequest("GET", u, nil)
		if err != nil {
			return "", err
		}
		resp, err := c.Do(req)
		if err != nil {
			return "", err
		}
		var p pinResponse
		_ = json.NewDecoder(resp.Body).Decode(&p)
		resp.Body.Close()
		if p.AuthToken != "" {
			return p.AuthToken, nil
		}
		if time.Now().After(deadline) {
			return "", fmt.Errorf("PIN poll timed out after %s", timeout)
		}
		time.Sleep(c.pinPollInterval)
	}
}

// LinkURL returns the user-visible URL Byron opens in a browser.
func LinkURL() string {
	return "https://plex.tv/link"
}

// ForceBrowserURL returns the URL with the code pre-filled as a query parameter.
// (Plex's /link page reads ?code=XXXX if present.)
func ForceBrowserURL(code string) string {
	return LinkURL() + "?" + url.Values{"code": []string{code}}.Encode()
}
```

- [ ] **Step 5: Verify the tests pass**

```bash
go test ./internal/plex/...
```
Expected: PASS — create, poll, timeout.

- [ ] **Step 6: Commit**

```bash
git add internal/plex/auth.go internal/plex/auth_test.go internal/plex/client.go
git commit -m "feat(plex): PIN create and poll flow against plex.tv/api/v2/pins"
```

---

## Task 8: `lumen auth` subcommand

**Files:**
- Modify: `cmd/lumen/auth.go`

**Context:** Wire Task 7's PIN flow into the CLI. The subcommand should:
1. Load config (creates stable client identifier on first run).
2. Create a PIN.
3. Print `"Open this URL and enter code ABCD: https://plex.tv/link?code=ABCD"` and open the default browser.
4. Poll for up to 5 minutes.
5. On success, save the account token to config and print `"Authentication successful."`. On timeout/failure, print the error and exit 1.

- [ ] **Step 1: Add the browser-opening dependency**

```bash
go get github.com/pkg/browser
```

- [ ] **Step 2: Implement runAuth**

Replace `cmd/lumen/auth.go`:
```go
package main

import (
	"fmt"
	"os"
	"time"

	"github.com/pkg/browser"

	"lumen/internal/config"
	"lumen/internal/plex"
)

func runAuth(args []string) {
	cfg, err := config.Load()
	if err != nil {
		fmt.Fprintf(os.Stderr, "load config: %v\n", err)
		os.Exit(1)
	}
	c := plex.NewClient(cfg.ClientIdentifier, version)

	pin, err := c.CreatePIN()
	if err != nil {
		fmt.Fprintf(os.Stderr, "create PIN: %v\n", err)
		os.Exit(1)
	}

	link := plex.ForceBrowserURL(pin.Code)
	fmt.Printf("Enter this code at %s\n  Code: %s\n", plex.LinkURL(), pin.Code)
	if err := browser.OpenURL(link); err != nil {
		fmt.Fprintf(os.Stderr, "(couldn't open browser automatically: %v)\n", err)
	}
	fmt.Println("Waiting for you to link the PIN...")

	token, err := c.PollPIN(pin, 5*time.Minute)
	if err != nil {
		fmt.Fprintf(os.Stderr, "poll PIN: %v\n", err)
		os.Exit(1)
	}

	cfg.Plex.AccountToken = token
	if err := cfg.Save(); err != nil {
		fmt.Fprintf(os.Stderr, "save config: %v\n", err)
		os.Exit(1)
	}
	fmt.Println("Authentication successful.")
}
```

- [ ] **Step 3: Build and run**

```bash
go build -o lumen.exe ./cmd/lumen
```
Expected: build succeeds.

Do NOT run `./lumen.exe auth` yet — that hits live plex.tv and should only happen during Task 16's verification.

- [ ] **Step 4: Commit**

```bash
git add go.mod go.sum cmd/lumen/auth.go
git commit -m "feat(cli): lumen auth runs PIN flow and persists account token"
```

---

## Task 9: Connection discovery — `DiscoverServers`

**Files:**
- Create: `internal/plex/resources.go`
- Create: `internal/plex/resources_test.go`

**Context:** Spec §5.1. `GET https://plex.tv/api/v2/resources?includeHttps=1&includeRelay=1` with `X-Plex-Token: <accountToken>`. Response is a JSON array; each element has `name`, `clientIdentifier`, `accessToken`, `product`, and a `connections[]` array. Filter to `product == "Plex Media Server"` and keep both owned servers and shared (Stargaze is shared, DKNZPLEX is owned).

- [ ] **Step 1: Write the failing test**

Create `internal/plex/resources_test.go`:
```go
package plex

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestDiscoverServersFiltersToPMSAndParsesConnections(t *testing.T) {
	payload := []map[string]any{
		{
			"name":              "Stargaze",
			"clientIdentifier":  "m-stargaze",
			"accessToken":       "tok-stargaze",
			"product":           "Plex Media Server",
			"connections": []map[string]any{
				{"uri": "https://1-2-3-4.plex.direct:32400", "address": "1.2.3.4", "port": 32400, "local": false, "relay": false, "protocol": "https", "IPv6": false},
				{"uri": "https://relay.plex.tv/abc", "address": "relay", "port": 443, "local": false, "relay": true, "protocol": "https", "IPv6": false},
			},
		},
		{
			"name":             "DKNZPLEX",
			"clientIdentifier": "m-dknzplex",
			"accessToken":      "tok-dknz",
			"product":          "Plex Media Server",
			"connections":      []map[string]any{{"uri": "https://5-6-7-8.plex.direct:32400", "address": "5.6.7.8", "port": 32400, "local": false, "relay": false, "protocol": "https", "IPv6": false}},
		},
		{ // non-PMS product — must be filtered out
			"name":    "Plex Web",
			"product": "Plex Web",
		},
	}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Get("includeHttps") != "1" || r.URL.Query().Get("includeRelay") != "1" {
			http.Error(w, "missing query params", 400)
			return
		}
		if r.Header.Get("X-Plex-Token") != "acct-tok" {
			http.Error(w, "missing token", 401)
			return
		}
		_ = json.NewEncoder(w).Encode(payload)
	}))
	defer srv.Close()

	c := NewClient("id", "1.0.0")
	c.plexTVBase = srv.URL
	servers, err := c.DiscoverServers("acct-tok")
	if err != nil {
		t.Fatal(err)
	}
	if len(servers) != 2 {
		t.Fatalf("got %d servers, want 2 (PMS-only)", len(servers))
	}
	if servers[0].Name != "Stargaze" || servers[0].MachineIdentifier != "m-stargaze" {
		t.Errorf("server 0: %+v", servers[0])
	}
	if servers[0].AccessToken != "tok-stargaze" {
		t.Errorf("AccessToken mismatch")
	}
	if len(servers[0].Connections) != 2 {
		t.Errorf("want 2 connections, got %d", len(servers[0].Connections))
	}
	if !servers[0].Connections[1].Relay {
		t.Errorf("second connection should be relay=true")
	}
}
```

- [ ] **Step 2: Run the test and confirm it fails**

```bash
go test ./internal/plex/...
```
Expected: FAIL — `DiscoverServers` not defined.

- [ ] **Step 3: Implement resources.go**

Create `internal/plex/resources.go`:
```go
package plex

import (
	"encoding/json"
	"fmt"
	"net/http"
)

type resourceWire struct {
	Name              string           `json:"name"`
	ClientIdentifier  string           `json:"clientIdentifier"`
	AccessToken       string           `json:"accessToken"`
	Product           string           `json:"product"`
	Connections       []connectionWire `json:"connections"`
}

type connectionWire struct {
	URI      string `json:"uri"`
	Address  string `json:"address"`
	Port     int    `json:"port"`
	Local    bool   `json:"local"`
	Relay    bool   `json:"relay"`
	Protocol string `json:"protocol"`
	IPv6     bool   `json:"IPv6"`
}

// DiscoverServers calls plex.tv/api/v2/resources and returns every Plex Media Server
// the account has access to — both owned and shared.
func (c *Client) DiscoverServers(accountToken string) ([]*Server, error) {
	u := c.plexTVBase + "/api/v2/resources?includeHttps=1&includeRelay=1"
	req, err := c.NewRequest("GET", u, nil)
	if err != nil {
		return nil, err
	}
	c.SetToken(req, accountToken)
	resp, err := c.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("resources: status %d", resp.StatusCode)
	}

	var wires []resourceWire
	if err := json.NewDecoder(resp.Body).Decode(&wires); err != nil {
		return nil, err
	}

	var out []*Server
	for _, w := range wires {
		if w.Product != "Plex Media Server" {
			continue
		}
		s := &Server{
			Name:              w.Name,
			MachineIdentifier: w.ClientIdentifier,
			AccessToken:       w.AccessToken,
		}
		for _, cw := range w.Connections {
			s.Connections = append(s.Connections, Connection{
				URI:      cw.URI,
				Address:  cw.Address,
				Port:     cw.Port,
				Local:    cw.Local,
				Relay:    cw.Relay,
				Protocol: cw.Protocol,
				IPv6:     cw.IPv6,
			})
		}
		out = append(out, s)
	}
	return out, nil
}
```

- [ ] **Step 4: Verify the tests pass**

```bash
go test ./internal/plex/...
```
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/plex/resources.go internal/plex/resources_test.go
git commit -m "feat(plex): DiscoverServers against plex.tv/api/v2/resources"
```

---

## Task 10: Connection picker — `PickConnection` with probe + cache

**Files:**
- Create: `internal/plex/connection.go`
- Create: `internal/plex/connection_test.go`

**Context:** Spec §5.1 preference order: non-relay HTTPS → relay HTTPS; IPv6 variants are a lower-priority alternate within each tier. Probe each candidate with a 2 s `HEAD /identity`. Cache the winner on the `Server` struct (`BaseURL`). Production will also persist `LastGoodConnection` back to config — that happens in Task 13's `lumen list` plumbing.

- [ ] **Step 1: Write the failing test**

Create `internal/plex/connection_test.go`:
```go
package plex

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

// helper spins up a test server that either answers /identity OK or sleeps forever
// to simulate unreachable.
func identityServer(t *testing.T, ok bool) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/identity" {
			http.Error(w, "wrong path", 404)
			return
		}
		if !ok {
			http.Error(w, "boom", 500)
			return
		}
		w.WriteHeader(http.StatusOK)
	}))
}

func TestPickConnectionPrefersNonRelay(t *testing.T) {
	good := identityServer(t, true)
	defer good.Close()
	relay := identityServer(t, true)
	defer relay.Close()

	s := &Server{Connections: []Connection{
		{URI: relay.URL, Relay: true},
		{URI: good.URL, Relay: false},
	}}

	c := NewClient("id", "1.0.0")
	conn, err := c.PickConnection(s)
	if err != nil {
		t.Fatal(err)
	}
	if conn.URI != good.URL {
		t.Errorf("picked %q, want non-relay %q", conn.URI, good.URL)
	}
	if s.BaseURL != good.URL {
		t.Errorf("BaseURL not cached: %q", s.BaseURL)
	}
}

func TestPickConnectionFallsBackToRelayWhenDirectFails(t *testing.T) {
	bad := identityServer(t, false)
	defer bad.Close()
	relay := identityServer(t, true)
	defer relay.Close()

	s := &Server{Connections: []Connection{
		{URI: bad.URL, Relay: false},
		{URI: relay.URL, Relay: true},
	}}
	c := NewClient("id", "1.0.0")
	conn, err := c.PickConnection(s)
	if err != nil {
		t.Fatal(err)
	}
	if conn.URI != relay.URL {
		t.Errorf("fallback failed: picked %q", conn.URI)
	}
}

func TestPickConnectionReturnsErrorWhenAllFail(t *testing.T) {
	a := identityServer(t, false)
	defer a.Close()
	b := identityServer(t, false)
	defer b.Close()

	s := &Server{Connections: []Connection{{URI: a.URL}, {URI: b.URL}}}
	c := NewClient("id", "1.0.0")
	_, err := c.PickConnection(s)
	if err == nil {
		t.Fatal("expected error when all candidates fail")
	}
}
```

- [ ] **Step 2: Run the test and confirm it fails**

```bash
go test ./internal/plex/...
```
Expected: FAIL — `PickConnection` not defined.

- [ ] **Step 3: Implement connection.go**

Create `internal/plex/connection.go`:
```go
package plex

import (
	"context"
	"fmt"
	"net/http"
	"sort"
	"time"
)

// PickConnection probes the server's candidate URIs in preference order and returns
// the first one that responds to HEAD /identity within 2 s. Writes the winner to
// Server.BaseURL as a side-effect so subsequent calls can use it.
func (c *Client) PickConnection(s *Server) (Connection, error) {
	if len(s.Connections) == 0 {
		return Connection{}, fmt.Errorf("server %q has no connections", s.Name)
	}
	ordered := sortConnections(s.Connections)
	for _, conn := range ordered {
		if c.probe(conn.URI, 2*time.Second) {
			s.BaseURL = conn.URI
			return conn, nil
		}
	}
	return Connection{}, fmt.Errorf("no reachable connection for server %q", s.Name)
}

func (c *Client) probe(baseURL string, timeout time.Duration) bool {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, "HEAD", baseURL+"/identity", nil)
	if err != nil {
		return false
	}
	resp, err := c.http.Do(req)
	if err != nil {
		return false
	}
	resp.Body.Close()
	return resp.StatusCode < 400
}

// sortConnections returns a copy of the input sorted by preference:
//   1. non-relay IPv4
//   2. non-relay IPv6
//   3. relay IPv4
//   4. relay IPv6
func sortConnections(in []Connection) []Connection {
	out := make([]Connection, len(in))
	copy(out, in)
	sort.SliceStable(out, func(i, j int) bool {
		return score(out[i]) < score(out[j])
	})
	return out
}

func score(c Connection) int {
	s := 0
	if c.Relay {
		s += 10
	}
	if c.IPv6 {
		s += 1
	}
	return s
}
```

- [ ] **Step 4: Verify the tests pass**

```bash
go test ./internal/plex/...
```
Expected: PASS for all three scenarios.

- [ ] **Step 5: Commit**

```bash
git add internal/plex/connection.go internal/plex/connection_test.go
git commit -m "feat(plex): PickConnection probes candidates and prefers direct HTTPS"
```

---

## Task 11: Libraries, items, search

**Files:**
- Create: `internal/plex/libraries.go`
- Create: `internal/plex/libraries_test.go`

**Context:** Per-server endpoints (use `Server.BaseURL` + `Server.AccessToken`):
- `GET /library/sections` — returns all libraries.
- `GET /library/sections/<key>/all` — items in a library; supports `X-Plex-Container-Start` / `X-Plex-Container-Size` headers for pagination, and query args for sort/filters.
- `GET /library/metadata/<ratingKey>` — single item.
- `GET /search?query=<q>` — cross-library search on a single server.

Plex responses are wrapped in `{ "MediaContainer": { ... } }`. Keep decoding minimal — only parse fields we use.

- [ ] **Step 1: Write the failing test with a mock server**

Create `internal/plex/libraries_test.go`:
```go
package plex

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// fakePlexServer returns a test server that responds to the four library
// endpoints with canned JSON.
func fakePlexServer(t *testing.T) *httptest.Server {
	t.Helper()
	mux := http.NewServeMux()
	mux.HandleFunc("/library/sections", func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("X-Plex-Token") != "srv-tok" {
			http.Error(w, "no token", 401)
			return
		}
		w.Write([]byte(`{"MediaContainer":{"Directory":[
			{"key":"1","title":"Movies","type":"movie"},
			{"key":"2","title":"TV Shows","type":"show"}
		]}}`))
	})
	mux.HandleFunc("/library/sections/1/all", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`{"MediaContainer":{"Metadata":[
			{"ratingKey":"100","guid":"plex://movie/abc","title":"Dune","type":"movie","year":2021}
		]}}`))
	})
	mux.HandleFunc("/library/metadata/100", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`{"MediaContainer":{"Metadata":[
			{"ratingKey":"100","guid":"plex://movie/abc","title":"Dune","type":"movie","year":2021,"summary":"Sand worm."}
		]}}`))
	})
	mux.HandleFunc("/search", func(w http.ResponseWriter, r *http.Request) {
		if !strings.Contains(r.URL.RawQuery, "query=dune") {
			http.Error(w, "missing query", 400)
			return
		}
		w.Write([]byte(`{"MediaContainer":{"Metadata":[{"ratingKey":"100","title":"Dune","type":"movie"}]}}`))
	})
	return httptest.NewServer(mux)
}

func TestGetLibraries(t *testing.T) {
	fake := fakePlexServer(t)
	defer fake.Close()

	c := NewClient("id", "1.0.0")
	s := &Server{BaseURL: fake.URL, AccessToken: "srv-tok"}
	libs, err := c.GetLibraries(s)
	if err != nil {
		t.Fatal(err)
	}
	if len(libs) != 2 || libs[0].Title != "Movies" {
		t.Fatalf("got %+v", libs)
	}
}

func TestGetItemsReturnsMetadata(t *testing.T) {
	fake := fakePlexServer(t)
	defer fake.Close()
	c := NewClient("id", "1.0.0")
	s := &Server{BaseURL: fake.URL, AccessToken: "srv-tok"}
	items, err := c.GetItems(s, "1", ItemQuery{})
	if err != nil {
		t.Fatal(err)
	}
	if len(items) != 1 || items[0].Title != "Dune" || items[0].Year != 2021 {
		t.Fatalf("got %+v", items)
	}
}

func TestGetItem(t *testing.T) {
	fake := fakePlexServer(t)
	defer fake.Close()
	c := NewClient("id", "1.0.0")
	s := &Server{BaseURL: fake.URL, AccessToken: "srv-tok"}
	it, err := c.GetItem(s, "100")
	if err != nil {
		t.Fatal(err)
	}
	if it.Summary != "Sand worm." {
		t.Fatalf("got %+v", it)
	}
}

func TestSearch(t *testing.T) {
	fake := fakePlexServer(t)
	defer fake.Close()
	c := NewClient("id", "1.0.0")
	s := &Server{BaseURL: fake.URL, AccessToken: "srv-tok"}
	items, err := c.Search(s, "dune")
	if err != nil {
		t.Fatal(err)
	}
	if len(items) != 1 || items[0].Title != "Dune" {
		t.Fatalf("got %+v", items)
	}
}
```

- [ ] **Step 2: Run the test and confirm it fails**

```bash
go test ./internal/plex/...
```
Expected: FAIL — `GetLibraries`, `GetItems`, `GetItem`, `Search` not defined.

- [ ] **Step 3: Implement libraries.go**

Create `internal/plex/libraries.go`:
```go
package plex

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
)

// mediaContainer is the envelope Plex wraps all library responses in.
type mediaContainer struct {
	MediaContainer struct {
		Directory []directoryWire `json:"Directory"`
		Metadata  []metadataWire  `json:"Metadata"`
	} `json:"MediaContainer"`
}

type directoryWire struct {
	Key   string `json:"key"`
	Title string `json:"title"`
	Type  string `json:"type"`
}

type metadataWire struct {
	RatingKey string `json:"ratingKey"`
	GUID      string `json:"guid"`
	Title     string `json:"title"`
	Type      string `json:"type"`
	Year      int    `json:"year"`
	Summary   string `json:"summary"`
}

// GetLibraries returns all top-level library sections on the server.
func (c *Client) GetLibraries(s *Server) ([]Library, error) {
	mc, err := c.serverGet(s, "/library/sections", nil)
	if err != nil {
		return nil, err
	}
	out := make([]Library, 0, len(mc.MediaContainer.Directory))
	for _, d := range mc.MediaContainer.Directory {
		out = append(out, Library{
			ID:    d.Key,
			Key:   d.Key,
			Title: d.Title,
			Type:  d.Type,
		})
	}
	return out, nil
}

// GetItems lists items in a library section. libraryID is the section's numeric key.
func (c *Client) GetItems(s *Server, libraryID string, q ItemQuery) ([]Item, error) {
	qs := url.Values{}
	if q.Sort != "" {
		qs.Set("sort", q.Sort)
	}
	for k, v := range q.Filters {
		qs.Set(k, v)
	}
	if q.Size > 0 {
		qs.Set("X-Plex-Container-Start", strconv.Itoa(q.Start))
		qs.Set("X-Plex-Container-Size", strconv.Itoa(q.Size))
	}
	path := fmt.Sprintf("/library/sections/%s/all", libraryID)
	if len(qs) > 0 {
		path += "?" + qs.Encode()
	}
	mc, err := c.serverGet(s, path, nil)
	if err != nil {
		return nil, err
	}
	return metadataSliceToItems(mc.MediaContainer.Metadata), nil
}

// GetItem fetches a single item by ratingKey.
func (c *Client) GetItem(s *Server, ratingKey string) (Item, error) {
	mc, err := c.serverGet(s, "/library/metadata/"+ratingKey, nil)
	if err != nil {
		return Item{}, err
	}
	items := metadataSliceToItems(mc.MediaContainer.Metadata)
	if len(items) == 0 {
		return Item{}, fmt.Errorf("item %s not found", ratingKey)
	}
	return items[0], nil
}

// Search performs a cross-library search on a single server.
func (c *Client) Search(s *Server, query string) ([]Item, error) {
	path := "/search?" + url.Values{"query": []string{query}}.Encode()
	mc, err := c.serverGet(s, path, nil)
	if err != nil {
		return nil, err
	}
	return metadataSliceToItems(mc.MediaContainer.Metadata), nil
}

// serverGet issues a GET to the server with the per-server token applied, parsing
// the MediaContainer envelope.
func (c *Client) serverGet(s *Server, path string, extraHeaders http.Header) (*mediaContainer, error) {
	if s.BaseURL == "" {
		return nil, fmt.Errorf("server %q has no BaseURL — call PickConnection first", s.Name)
	}
	req, err := c.NewRequest("GET", s.BaseURL+path, nil)
	if err != nil {
		return nil, err
	}
	c.SetToken(req, s.AccessToken)
	for k, vals := range extraHeaders {
		for _, v := range vals {
			req.Header.Add(k, v)
		}
	}
	resp, err := c.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("%s %s: status %d", req.Method, path, resp.StatusCode)
	}
	var mc mediaContainer
	if err := json.NewDecoder(resp.Body).Decode(&mc); err != nil {
		return nil, err
	}
	return &mc, nil
}

func metadataSliceToItems(mw []metadataWire) []Item {
	out := make([]Item, 0, len(mw))
	for _, m := range mw {
		out = append(out, Item{
			RatingKey: m.RatingKey,
			GUID:      m.GUID,
			Title:     m.Title,
			Type:      m.Type,
			Year:      m.Year,
			Summary:   m.Summary,
		})
	}
	return out
}
```

- [ ] **Step 4: Verify the tests pass**

```bash
go test ./internal/plex/...
```
Expected: PASS for all four library tests.

- [ ] **Step 5: Commit**

```bash
git add internal/plex/libraries.go internal/plex/libraries_test.go
git commit -m "feat(plex): GetLibraries, GetItems, GetItem, Search"
```

---

## Task 12: Discover hubs — `GetHub`

**Files:**
- Create: `internal/plex/hubs.go`
- Create: `internal/plex/hubs_test.go`

**Context:** Spec §5.2. `GET https://discover.provider.plex.tv/hubs/sections/<namespace>/<slug>?contentDirectoryID=<namespace>` with `X-Plex-Token: <accountToken>`. Namespaces: `home` or `watchlist`. Response is the same `MediaContainer` envelope as libraries, with `Metadata` entries. We parse into `[]HubItem`. Caching (5-minute in-memory) is deferred to Session 2 where the image proxy and HTTP server live.

- [ ] **Step 1: Write the failing test**

Create `internal/plex/hubs_test.go`:
```go
package plex

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestGetHubBuildsCorrectURLAndParses(t *testing.T) {
	var gotPath, gotQuery, gotToken string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		gotQuery = r.URL.RawQuery
		gotToken = r.Header.Get("X-Plex-Token")
		w.Write([]byte(`{"MediaContainer":{"Metadata":[
			{"ratingKey":"9","guid":"plex://show/xyz","title":"Show One","type":"show","year":2024}
		]}}`))
	}))
	defer srv.Close()

	c := NewClient("id", "1.0.0")
	c.discoverBase = srv.URL

	items, err := c.GetHub("home", "trending-plex", "acct-tok")
	if err != nil {
		t.Fatal(err)
	}
	if gotPath != "/hubs/sections/home/trending-plex" {
		t.Errorf("path: %q", gotPath)
	}
	if !strings.Contains(gotQuery, "contentDirectoryID=home") {
		t.Errorf("query missing contentDirectoryID: %q", gotQuery)
	}
	if gotToken != "acct-tok" {
		t.Errorf("token header: %q", gotToken)
	}
	if len(items) != 1 || items[0].Title != "Show One" {
		t.Fatalf("items: %+v", items)
	}
}
```

- [ ] **Step 2: Run the test and confirm it fails**

```bash
go test ./internal/plex/...
```
Expected: FAIL — `GetHub` and `discoverBase` not defined.

- [ ] **Step 3: Add the discover-provider base to Client**

Modify `NewClient` in `internal/plex/client.go` — add to the struct-literal initializer:
```go
		discoverBase:     "https://discover.provider.plex.tv",
```

And declare it in the struct:
```go
	discoverBase string // overridable for tests
```

- [ ] **Step 4: Implement hubs.go**

Create `internal/plex/hubs.go`:
```go
package plex

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
)

// GetHub calls the plex.tv Discover hubs endpoint for a given namespace + slug.
// Namespace is "home" or "watchlist".
func (c *Client) GetHub(namespace, slug, accountToken string) ([]HubItem, error) {
	qs := url.Values{"contentDirectoryID": []string{namespace}}
	u := fmt.Sprintf("%s/hubs/sections/%s/%s?%s", c.discoverBase, namespace, slug, qs.Encode())
	req, err := c.NewRequest("GET", u, nil)
	if err != nil {
		return nil, err
	}
	c.SetToken(req, accountToken)
	resp, err := c.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("hub %s/%s: status %d", namespace, slug, resp.StatusCode)
	}
	var mc mediaContainer
	if err := json.NewDecoder(resp.Body).Decode(&mc); err != nil {
		return nil, err
	}
	out := make([]HubItem, 0, len(mc.MediaContainer.Metadata))
	for _, m := range mc.MediaContainer.Metadata {
		out = append(out, HubItem{
			GUID:      m.GUID,
			RatingKey: m.RatingKey,
			Title:     m.Title,
			Type:      m.Type,
			Year:      m.Year,
		})
	}
	return out, nil
}
```

- [ ] **Step 5: Verify the tests pass**

```bash
go test ./internal/plex/...
```
Expected: PASS.

- [ ] **Step 6: Commit**

```bash
git add internal/plex/hubs.go internal/plex/hubs_test.go internal/plex/client.go
git commit -m "feat(plex): GetHub covers every plex.tv Discover namespace+slug"
```

---

## Task 13: `lumen list` subcommand

**Files:**
- Modify: `cmd/lumen/list.go`

**Context:** Ties Tasks 9–11 together via the CLI. Loads config, verifies account token exists, discovers servers, picks a connection per server (parallel), enumerates libraries on each, and prints a two-level report. Also persists `MachineIdentifier`, `AccessToken`, `LastGoodConnection` back to config so subsequent runs remember what we found.

- [ ] **Step 1: Implement runList**

Replace `cmd/lumen/list.go`:
```go
package main

import (
	"fmt"
	"os"
	"sync"

	"lumen/internal/config"
	"lumen/internal/plex"
)

func runList(args []string) {
	cfg, err := config.Load()
	if err != nil {
		fmt.Fprintf(os.Stderr, "load config: %v\n", err)
		os.Exit(1)
	}
	if cfg.Plex.AccountToken == "" {
		fmt.Fprintln(os.Stderr, "no Plex account token — run `lumen auth` first")
		os.Exit(1)
	}

	c := plex.NewClient(cfg.ClientIdentifier, version)
	servers, err := c.DiscoverServers(cfg.Plex.AccountToken)
	if err != nil {
		fmt.Fprintf(os.Stderr, "discover servers: %v\n", err)
		os.Exit(1)
	}
	if len(servers) == 0 {
		fmt.Fprintln(os.Stderr, "no Plex Media Servers accessible on this account")
		os.Exit(1)
	}

	// Pick connections in parallel — one slow server shouldn't block the other.
	var wg sync.WaitGroup
	errs := make([]error, len(servers))
	for i, s := range servers {
		wg.Add(1)
		go func(i int, s *plex.Server) {
			defer wg.Done()
			if _, err := c.PickConnection(s); err != nil {
				errs[i] = err
			}
		}(i, s)
	}
	wg.Wait()

	// Print report + collect persisted server state.
	var persisted []config.Server
	for i, s := range servers {
		fmt.Printf("=== %s ===\n", s.Name)
		if errs[i] != nil {
			fmt.Printf("  connection: OFFLINE (%v)\n", errs[i])
			continue
		}
		fmt.Printf("  connection: %s\n", s.BaseURL)
		libs, err := c.GetLibraries(s)
		if err != nil {
			fmt.Printf("  libraries: ERROR — %v\n", err)
			continue
		}
		fmt.Printf("  libraries:\n")
		for _, l := range libs {
			fmt.Printf("    [%s] %s (%s)\n", l.Key, l.Title, l.Type)
		}
		persisted = append(persisted, config.Server{
			Name:               s.Name,
			MachineIdentifier:  s.MachineIdentifier,
			AccessToken:        s.AccessToken,
			LastGoodConnection: s.BaseURL,
		})
	}

	cfg.Plex.Servers = persisted
	if err := cfg.Save(); err != nil {
		fmt.Fprintf(os.Stderr, "save config: %v\n", err)
		os.Exit(1)
	}

	// Fail the overall command only if EVERY server was offline.
	allDown := true
	for _, e := range errs {
		if e == nil {
			allDown = false
			break
		}
	}
	if allDown {
		os.Exit(1)
	}
}
```

- [ ] **Step 2: Build**

```bash
go build -o lumen.exe ./cmd/lumen
```
Expected: build succeeds. Do NOT run `./lumen.exe list` yet — it hits live Plex servers and belongs to Task 16.

- [ ] **Step 3: Commit**

```bash
git add cmd/lumen/list.go
git commit -m "feat(cli): lumen list discovers servers, picks connections, prints libraries"
```

---

## Task 14: `lumen probe-hubs` — resolve Pick Up Again slug

**Files:**
- Modify: `cmd/lumen/probe_hubs.go`

**Context:** Spec §20 open item. Byron needs to pin the exact `watchlist/<slug>` string that returns the "Pick Up Again" hub. Candidates: `continue-watching`, `on-deck`, `pick-up-again`, `in-progress`. The subcommand tries each against Byron's account and reports which return non-empty results + a sample item title.

- [ ] **Step 1: Implement runProbeHubs**

Replace `cmd/lumen/probe_hubs.go`:
```go
package main

import (
	"fmt"
	"os"

	"lumen/internal/config"
	"lumen/internal/plex"
)

// Ordered list of candidate Pick Up Again slugs (spec §20).
var pickUpAgainCandidates = []string{
	"continue-watching",
	"on-deck",
	"pick-up-again",
	"in-progress",
}

func runProbeHubs(args []string) {
	cfg, err := config.Load()
	if err != nil {
		fmt.Fprintf(os.Stderr, "load config: %v\n", err)
		os.Exit(1)
	}
	if cfg.Plex.AccountToken == "" {
		fmt.Fprintln(os.Stderr, "no Plex account token — run `lumen auth` first")
		os.Exit(1)
	}
	c := plex.NewClient(cfg.ClientIdentifier, version)

	for _, slug := range pickUpAgainCandidates {
		items, err := c.GetHub("watchlist", slug, cfg.Plex.AccountToken)
		if err != nil {
			fmt.Printf("  watchlist/%-20s  ERROR: %v\n", slug, err)
			continue
		}
		sample := "<none>"
		if len(items) > 0 {
			sample = items[0].Title
		}
		fmt.Printf("  watchlist/%-20s  items=%d  first=%q\n", slug, len(items), sample)
	}
}
```

- [ ] **Step 2: Build**

```bash
go build -o lumen.exe ./cmd/lumen
```
Expected: build succeeds.

- [ ] **Step 3: Commit**

```bash
git add cmd/lumen/probe_hubs.go
git commit -m "feat(cli): lumen probe-hubs enumerates Pick Up Again slug candidates"
```

---

## Task 15: `internal/potplayer` skeleton

**Files:**
- Create: `internal/potplayer/client.go`

**Context:** Spec §3 Session 1 deliverables: the directory exists with method signatures that match §7.1, all stubbed to return `ErrNotImplemented`. Session 4 fills in the bodies using findings from `docs/session-0-findings.md` (notably: writes go via `WM_APPCOMMAND`, reads go via `WM_USER`, path resolution has a three-stage fallback).

- [ ] **Step 1: Create the skeleton**

Create `internal/potplayer/client.go`:
```go
// Package potplayer drives Pot Player Mini 64-bit via Win32 IPC.
// Implementation lands in Session 4 based on docs/session-0-findings.md.
package potplayer

import (
	"errors"
	"time"
)

// ErrNotImplemented is returned by every method until Session 4.
var ErrNotImplemented = errors.New("potplayer: not implemented until Session 4")

// PlayState matches Pot Player's state codes confirmed in Session 0:
// 1 = Paused, 2 = Playing. Session 0 also found that Pot Player returns -1
// during the first ~2 s while media is loading — Session 4 maps that to Unknown.
type PlayState int

const (
	Unknown PlayState = 0
	Paused  PlayState = 1
	Playing PlayState = 2
	Stopped PlayState = 99 // synthetic; Pot Player itself doesn't emit this
)

// Client controls a single Pot Player instance via its HWND.
type Client struct {
	// Populated in Session 4.
}

// Launch spawns Pot Player against the given stream URL and returns a Client
// once the window handle is resolvable.
func Launch(streamURL string) (*Client, error) { return nil, ErrNotImplemented }

func (c *Client) GetPosition() (time.Duration, error) { return 0, ErrNotImplemented }
func (c *Client) GetDuration() (time.Duration, error) { return 0, ErrNotImplemented }
func (c *Client) GetState() (PlayState, error)        { return Unknown, ErrNotImplemented }
func (c *Client) Pause() error                        { return ErrNotImplemented }
func (c *Client) Resume() error                       { return ErrNotImplemented }
func (c *Client) Stop() error                         { return ErrNotImplemented }
func (c *Client) IsAlive() bool                       { return false }
```

- [ ] **Step 2: Build**

```bash
go build ./...
```
Expected: build succeeds (this package has no tests yet — Session 4 adds them).

- [ ] **Step 3: Commit**

```bash
git add internal/potplayer/client.go
git commit -m "feat(potplayer): skeleton with method signatures from spec §7.1 (Session 4 populates)"
```

---

## Task 16: End-to-end verification against Byron's live Plex account

**Files:**
- No code changes.

**Context:** Session 1's exit criteria per spec §3: `lumen auth` completes the PIN flow end to end; `lumen list` prints Stargaze and DKNZPLEX with every library on each. This task is driven by Byron on his own machine — the subagent cannot authenticate.

This task is **not** commit-producing. Its deliverable is confirmation (pasted output) that both subcommands work against real Plex.

- [ ] **Step 1: Build the binary**

```bash
go build -o lumen.exe ./cmd/lumen
```

- [ ] **Step 2: Run `lumen auth`**

```bash
./lumen.exe auth
```

Byron follows the printed prompt: opens the browser (or opens `https://plex.tv/link?code=<CODE>` manually) and enters the code. Expected terminal output:

```
Enter this code at https://plex.tv/link
  Code: ABCD
Waiting for you to link the PIN...
Authentication successful.
```

Verify the token landed on disk:

```powershell
Get-Content "$env:APPDATA\Lumen\config.json"
```

`accountToken` should be a long base64 string (DPAPI ciphertext). **Plaintext token must NOT appear.**

- [ ] **Step 3: Run `lumen list`**

```bash
./lumen.exe list
```

Expected: both `=== Stargaze ===` and `=== DKNZPLEX ===` blocks, each showing a `connection:` URL (plex.direct HTTPS preferred) and a `libraries:` list that matches spec §12.1's shelf definitions (Movies, Movies - 4K, TV Shows, etc. for Stargaze; Movies, Movies - 4K UHD, etc. for DKNZPLEX).

- [ ] **Step 4: Run `lumen probe-hubs`**

```bash
./lumen.exe probe-hubs
```

Expected: four lines, one per candidate slug. Record which slug returned a non-empty `items=N` with a sensible first-title. Paste output back to Archie — the winner gets pinned into Session 2's Recommended page code.

- [ ] **Step 5: Report back**

Paste back:
- `lumen auth` output + `accountToken` is encrypted on disk (yes/no).
- `lumen list` full output.
- `lumen probe-hubs` full output.
- Any errors or surprises.

Session 1 is DONE once all three subcommands succeed and the probe-hubs winner is recorded. Archie drops a note in `docs/session-0-findings.md` (or a new `docs/session-1-findings.md`) pinning the slug for Session 2.

---

## Self-review checklist (for the executing agent)

Before marking Session 1 done, confirm:

- [ ] Every Task 1–15 step checkbox is ticked.
- [ ] `go test ./...` passes from the repo root.
- [ ] `go build ./cmd/lumen` produces `lumen.exe` without warnings.
- [ ] No secret literal appears in `config.json` after Task 16's auth run — only base64(DPAPI) ciphertext.
- [ ] `internal/potplayer/client.go` compiles and returns `ErrNotImplemented` from every method (Session 4 will populate).
- [ ] `probe/` is untouched (its `go.mod` is still `module lumen/probe`; no imports cross the boundary).
- [ ] Session 0 findings remain the source of truth for Session 4's Pot Player implementation — nothing in this session contradicts them.
- [ ] Git log shows a coherent sequence of 15 commits with conventional-commit-style messages (one per task, Task 16 adds no commit).
