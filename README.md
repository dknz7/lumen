<div align="center">

<img src="assets/lumen.ico" width="96" alt="Lumen">

# Lumen

**A fast Windows desktop client for Plex that plays everything through PotPlayer.**

No transcoding. No buffering wheel. No browser tab pretending to be an app.

[![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg)](LICENSE)
[![Go](https://img.shields.io/badge/Go-1.26-00ADD8.svg)](https://go.dev)
[![Platform](https://img.shields.io/badge/platform-Windows%2010%2F11-0078D6.svg)](#requirements)

</div>

---

## Why this exists

Plex's own clients love to transcode. You have a 4K HDR remux sitting on a server three metres away, on a gigabit LAN, and something in the chain decides today is a good day to re-encode it in software.

Lumen doesn't play video. It hands the stream to [PotPlayer](https://potplayer.daum.net/), which direct-plays basically anything you throw at it, and then gets out of the way. Lumen is the browsing experience — shelves, artwork, watchlist, search — and PotPlayer is the playback engine.

Watch progress still scrobbles back to Plex, so your library stays in sync exactly as if you'd used the official client.

## What it does

- **Browse your libraries** — shelves for Continue Watching, Recently Added, On Deck, and per-library grids with sorting. Home shelves are built from movie and TV libraries; music and photo libraries appear in the sidebar but have no artwork or playback support — Lumen is a video client.
- **Watchlist & Recommended** — your Plex watchlist, plus Plex's recommendations, with availability badges showing what you can actually play right now.
- **Discover** — browse the wider Plex catalogue, including things you don't own, with a detail page for each.
- **Search** — across your servers and Plex Discover at once.
- **Rich detail pages** — cast and crew, seasons and episodes, IMDB ratings, trailers.
- **Playback via PotPlayer** — direct play where possible, transcode only when the server insists, with proper keep-alive so Plex doesn't reap the session mid-film.
- **Progress sync** — resume points and watched state written back to Plex.
- **Next episode autoplay** — finish an episode, get offered the next one.
- **Multi-server** — everything your Plex account can see, with local display names.

## Requirements

| | |
|---|---|
| **OS** | Windows 10 version 2004 (build 19041) or newer, 64-bit — including Windows 11 |
| **Plex** | A Plex account with at least one server you can access |
| **Player** | [PotPlayer](https://potplayer.daum.net/) (64-bit) — required for playback |
| **Runtime** | Microsoft Edge WebView2 — preinstalled on Windows 11 and current Windows 10; the installer adds it if missing |

## Install

Grab the latest `LumenSetup.exe` from [Releases](https://github.com/dknz7/lumen/releases) and run it.

On first launch Lumen will walk you through linking your Plex account — it shows you a PIN, you approve it at [plex.tv/link](https://plex.tv/link), done. No password ever touches Lumen.

### Optional: metadata keys

Two features need free third-party API keys, set in **Settings → Accounts & Servers**:

| Key | Unlocks | Get one |
|---|---|---|
| **OMDB** | IMDB ratings on detail pages | [omdbapi.com](https://www.omdbapi.com/apikey.aspx) |
| **TMDB** | Trailer playback | [themoviedb.org](https://www.themoviedb.org/settings/api) |

Lumen works fine without either — you just lose those two features.

## How it works

```
┌──────────────────────────────────────────────────────────┐
│  lumen.exe  (single ~8.5 MB binary, no runtime deps)     │
│                                                          │
│   ┌────────────────┐         ┌───────────────────────┐   │
│   │  WebView2      │  HTTP   │  Go server            │   │
│   │  window        │◄───────►│  127.0.0.1:7832       │   │
│   │  (SolidJS UI)  │         │  · Plex API proxy     │   │
│   └────────────────┘         │  · image cache        │   │
│           ▲                  │  · playback manager   │   │
│           │                  └───────────┬───────────┘   │
│      system tray                         │               │
└──────────────────────────────────────────┼───────────────┘
                                           │
                        ┌──────────────────┼──────────────────┐
                        ▼                  ▼                  ▼
                  Plex servers        plex.tv API        PotPlayer
```

The UI is a SolidJS app compiled by Vite and **embedded into the binary** — there's nothing to serve from disk and no `node_modules` at runtime. The Go server listens on loopback only and proxies every Plex call, which means:

**Your Plex token never reaches the frontend.** Auth is header-only, server-side. Tokens are stored in `%APPDATA%\Lumen\config.json` encrypted at rest with Windows DPAPI, so they're bound to your Windows user account and unreadable by anyone else on the machine.

## Where things live

| What | Path |
|---|---|
| Config & tokens | `%APPDATA%\Lumen\config.json` |
| Image / metadata cache | `%APPDATA%\Lumen\cache\` |
| Logs | `%APPDATA%\Lumen\logs\` |

Clearing the cache is safe at any time — **Settings → Data & Cache**.

## Building from source

You'll need [Go 1.26+](https://go.dev/dl/) and [Node 20+](https://nodejs.org/).

```powershell
git clone https://github.com/dknz7/lumen.git
cd lumen

# 1. Build the SPA into the Go embed target
cd web
npm ci
npm run build
cd ..

# 2. Build the binary
go build -ldflags="-H windowsgui" -o lumen.exe ./cmd/lumen
```

Or just run the build script, which does both plus the version stamping:

```powershell
.\build.ps1
```

Add `-Installer` to also produce `dist\LumenSetup.exe` (requires [Inno Setup 6](https://jrsoftware.org/isdl.php)).

### Tests

```powershell
go test ./...          # Go unit tests — no extra tooling needed
```

The race detector needs cgo, which on Windows means a C compiler on `PATH`
(MSYS2 or TDM-GCC). It's optional for a PR, but worth running if you touch
anything concurrent:

```powershell
$env:CGO_ENABLED = "1"
go test -race ./...
```

Note the concurrency regression test in `internal/server` reproduces its bug
*without* `-race` — Go's map-corruption checks are unconditional — so a plain
`go test` still catches it.

## CLI

Lumen is a GUI app, but the binary still takes subcommands — handy for scripting and debugging:

```
lumen                   Launch the app (default)
lumen auth              Re-run the Plex PIN link flow
lumen list              List servers and libraries
lumen serve --browser   Run headless, open in your default browser instead
lumen version           Print version
```

## Contributing

Issues and PRs welcome. A few things worth knowing before you dive in:

- The frontend is **SolidJS, not React**. Signals, not hooks — and destructuring props breaks reactivity.
- Test fixtures are captured from **real Plex responses**, not hand-written. Plex's API is polymorphic in ways synthetic fixtures don't reproduce (fields that are a string on one server and an array on another). Capture from DevTools, then write the fixture.
- `go test ./...` must pass before a PR.

## Acknowledgements

Built on [PotPlayer](https://potplayer.daum.net/) for playback, [SolidJS](https://www.solidjs.com/) for the UI, and [go-webview2](https://github.com/jchv/go-webview2) for the window.

Not affiliated with, endorsed by, or supported by Plex Inc. or the PotPlayer team. "Plex" is a trademark of Plex Inc.

## License

[MIT](LICENSE) © Byron
