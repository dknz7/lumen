package server

import (
	"net/http"
	"runtime"

	"lumen/internal/config"
	"lumen/internal/potplayer"
)

// buildInfo is stamped from cmd/lumen at startup so the server layer doesn't
// need its own copy of the version constant.
var buildInfo = struct {
	Version string
	Commit  string
	Date    string
}{Version: "dev"}

// SetBuildInfo records the version details shown in Settings → About.
func SetBuildInfo(version, commit, date string) {
	buildInfo.Version = version
	buildInfo.Commit = commit
	buildInfo.Date = date
}

type aboutResponse struct {
	Version   string `json:"version"`
	Commit    string `json:"commit,omitempty"`
	BuildDate string `json:"buildDate,omitempty"`

	GoVersion string `json:"goVersion"`
	Platform  string `json:"platform"`

	Repository string `json:"repository"`
	License    string `json:"license"`
	Issues     string `json:"issues"`

	Paths struct {
		Config string `json:"config"`
		Cache  string `json:"cache"`
		Logs   string `json:"logs"`
	} `json:"paths"`

	PotPlayer struct {
		Detected bool   `json:"detected"`
		Path     string `json:"path,omitempty"`
		Override string `json:"override,omitempty"`
	} `json:"potPlayer"`

	Dependencies []dependency `json:"dependencies"`
}

type dependency struct {
	Name    string `json:"name"`
	License string `json:"license"`
	URL     string `json:"url"`
}

// handleAbout backs Settings → About. Everything here is safe to show and safe
// to paste into a bug report — no tokens, no keys, no library contents.
func (s *Server) handleAbout(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "GET only")
		return
	}

	var resp aboutResponse
	resp.Version = buildInfo.Version
	resp.Commit = buildInfo.Commit
	resp.BuildDate = buildInfo.Date
	resp.GoVersion = runtime.Version()
	resp.Platform = runtime.GOOS + "/" + runtime.GOARCH
	resp.Repository = "https://github.com/dknz7/lumen"
	resp.Issues = "https://github.com/dknz7/lumen/issues"
	resp.License = "MIT"

	resp.Paths.Config = config.ConfigFile()
	resp.Paths.Cache = config.CacheDir()
	resp.Paths.Logs = config.LogsDir()

	override := s.potPlayerPath()
	resp.PotPlayer.Override = override
	if p, err := potplayer.ResolveExePath(override); err == nil {
		resp.PotPlayer.Detected = true
		resp.PotPlayer.Path = p
	}

	// Hand-maintained: Go's module graph doesn't carry licence metadata, and
	// an MIT project should credit what it ships.
	resp.Dependencies = []dependency{
		{Name: "SolidJS", License: "MIT", URL: "https://github.com/solidjs/solid"},
		{Name: "go-webview2", License: "MIT", URL: "https://github.com/jchv/go-webview2"},
		{Name: "energye/systray", License: "Apache-2.0", URL: "https://github.com/energye/systray"},
		{Name: "google/uuid", License: "BSD-3-Clause", URL: "https://github.com/google/uuid"},
		{Name: "pkg/browser", License: "BSD-2-Clause", URL: "https://github.com/pkg/browser"},
		{Name: "hls.js", License: "Apache-2.0", URL: "https://github.com/video-dev/hls.js"},
		{Name: "Lucide", License: "ISC", URL: "https://github.com/lucide-icons/lucide"},
		{Name: "Rajdhani + Saira", License: "OFL-1.1", URL: "https://fontsource.org"},
	}

	writeJSON(w, resp)
}

type statusResponse struct {
	Linked     bool   `json:"linked"`     // a Plex account token is stored
	HasServers bool   `json:"hasServers"` // at least one server is known
	Username   string `json:"username,omitempty"`
	Version    string `json:"version"`
}

// handleStatus tells the SPA whether onboarding is needed. A fresh install has
// no token and no servers; the SPA uses this to show the Plex link flow instead
// of an empty home page full of failed requests.
func (s *Server) handleStatus(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "GET only")
		return
	}
	writeJSON(w, statusResponse{
		Linked:     s.accountToken() != "",
		HasServers: len(s.serverList()) > 0,
		Version:    buildInfo.Version,
	})
}
