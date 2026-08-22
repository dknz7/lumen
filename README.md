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

## Themes

Lumen ships **Pure OLED** and **Tokyo Night**, and you can add your own without
touching the code or rebuilding anything.

A theme is 25 colour values in a JSON file. Drop it in

```
%APPDATA%\Lumen\themes
```

and hit **Reload** in *Settings > Appearance*. It appears in the picker under
**Custom**. There's a button there to open the folder, and another to export
whichever theme is active as a complete file to start from — so you never
begin from a blank document.

### Writing one

`extends` inherits from a built-in, so you only write the colours you actually
want to change:

```json
{
  "id": "gruvbox-dark",
  "name": "Gruvbox Dark",
  "extends": "pure-oled",
  "tokens": {
    "bg": "#282828",
    "bgMenu": "#1d2021",
    "text": "#ebdbb2",
    "accent": "#fabd2f",
    "accentContrast": "#282828",
    "danger": "#fb4934",
    "success": "#b8bb26"
  }
}
```

Without `extends` all 25 tokens are required. With it, anything you leave out
comes from the parent — which also means a token added in a later version of
Lumen inherits a sensible value instead of breaking your theme.

Any CSS colour works: hex, `rgb()`, `rgba()`, `color-mix()`, a named colour.

If a file is rejected, *Settings > Appearance* says which file and why — a
trailing comma, an unknown token name, a value the browser won't accept.
Nothing fails silently.

### The 25 tokens

| Token | CSS variable | Used for |
|---|---|---|
| `bg` | `--bg` | Page canvas |
| `bgMenu` | `--bg-menu` | Left rail |
| `bgElevated` | `--bg-elevated` | Top-bar pill, shelves, meta pills |
| `bgInverse` | `--bg-inverse` | Primary buttons, selected tabs |
| `text` | `--text` | Titles, headings, icons |
| `textMuted` | `--text-muted` | Body copy, dates, durations |
| `textInverse` | `--text-inverse` | Text on `bgInverse` |
| `menuIcon` | `--menu-icon` | Left-rail chevrons and idle nav links |
| `border` | `--border` | Hard dividers |
| `borderSoft` | `--border-soft` | Subtle separators, in-pill dividers |
| `stroke` | `--stroke` | Hover outlines, secondary button borders |
| `statusOnline` | `--status-online` | Reachable server dot |
| `statusOffline` | `--status-offline` | Unreachable server dot |
| `shadow` | `--shadow` | Full `box-shadow` value, not a colour |
| `accent` | `--accent`, `--led-teal` | LEDs, progress fills, Save button |
| `accentContrast` | `--accent-contrast` | Text drawn on `accent` |
| `danger` | `--danger` | Error text |
| `dangerStrong` | `--danger-strong` | Error borders, destructive fills |
| `warning` | `--warning` | Transcoding, degraded states |
| `success` | `--success` | Watched, direct play, healthy |
| `overlay` | `--overlay` | Scrim behind modals |
| `surfaceSubtle` | `--surface-subtle` | Hover fills, skeleton shimmer |
| `cardEmpty` | `--bg-card-empty` | Poster background with no artwork |
| `shelfOuter` | `--bg-shelf-outer` | Shelf container |
| `shelfInner` | `--bg-shelf-inner` | Shelf row background |

`--led-teal` is a legacy alias of `--accent`, kept so older rules keep working.

### Why JSON and not a module

A theme is data, so it's stored as data. Making it a TypeScript or JavaScript
file would mean either shipping a compiler or evaluating a file the user
downloaded from someone else — inside a page that can already reach the local
API. Themes are exactly the kind of thing people copy from a gist, and "here's
a nice theme, run this" is a working attack.

Because it's data, every value can be checked before it's used. Lumen asks the
browser whether each one is valid for the property it will become — `color`
for most, `box-shadow` for `shadow`, which is the token that would otherwise
accept a `url()` and fetch from a remote host on render. A file that fails is
reported, never partly applied.

### Deriving values

Alpha variants come from `color-mix` rather than being hardcoded, so they
follow the theme:

```css
background: color-mix(in srgb, var(--accent) 85%, transparent);
```

### What a theme deliberately cannot change

Two sets of colours stay fixed, on purpose:

- **Brand marks** — the IMDB yellow and the Rotten Tomatoes red and green.
  They identify someone else's product and must look the same everywhere.
- **Colours drawn over artwork** — the black gradients under poster titles,
  the white ring on the play button. These sit on top of arbitrary images,
  not on a theme surface, and a themed scrim stops doing its job.

If you're adding a token, that's the test: does it sit on a Lumen surface, or
on a poster?

### Built-in themes

The two that ship live in `web/src/themes/` as TypeScript, because they're
compiled in and get the benefit of the compiler checking them. They use the
same token set, so a JSON theme and a built-in are the same thing in different
clothes. Adding one is a file plus an entry in `BUILTIN_THEMES`.

### Checking your work

A theme is colour-only, so a green build proves nothing. Switch between themes
in *Settings > Appearance* and actually look at Home, a detail page, the
Watchlist and each Settings panel. The `danger` and `warning` tokens are the
easiest to miss — they only appear when something is wrong, so point Lumen at
an unreachable server and clear the OMDB key to see them.

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
