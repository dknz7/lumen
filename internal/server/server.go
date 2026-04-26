// Package server hosts Lumen's loopback HTTP server — it serves the embedded
// SolidJS SPA at /, and /api/* routes for Plex proxying.
package server

import (
	"context"
	"net"
	"net/http"
	"time"

	"lumen/internal/config"
	"lumen/internal/playback"
	"lumen/internal/plex"
	"lumen/internal/potplayer"
)

// Server bundles the http.Server, mux, Plex client, and loaded config into one
// lifecycle-managed unit.
type Server struct {
	cfg           *config.Config
	plex          *plex.Client
	mux           *http.ServeMux
	http          *http.Server
	ln            net.Listener
	hubs          *hubCache
	watchlist     *watchlistCache
	discoverItems *discoverItemCache
	images        *imageCache
	auth          *authState
	quit          chan struct{}
	playback      *playback.Manager
}

// New constructs the Server but does not bind yet. addr is in "host:port" form
// (e.g. "127.0.0.1:7832" in production; "127.0.0.1:0" in tests to pick any port).
func New(cfg *config.Config, c *plex.Client, addr string) *Server {
	mux := http.NewServeMux()
	s := &Server{
		cfg:  cfg,
		plex: c,
		mux:  mux,
		http: &http.Server{
			Addr:         addr,
			Handler:      mux,
			ReadTimeout:  15 * time.Second,
			WriteTimeout: 30 * time.Second,
			IdleTimeout:  60 * time.Second,
		},
		hubs:          newHubCache(),
		watchlist:     newWatchlistCache(),
		discoverItems: newDiscoverItemCache(),
		images:        newImageCache(),
		auth:          newAuthState(),
		// Buffered so handleQuit's send never blocks even if main isn't yet
		// selecting on the channel.
		quit: make(chan struct{}, 1),
	}
	s.playback = playback.NewManager(c, func() string {
		override := cfg.UI.Playback.PotPlayerPath
		p, err := potplayer.ResolveExePath(override)
		if err != nil {
			return ""
		}
		return p
	})
	s.registerRoutes()
	return s
}

// Quit returns a channel that fires when the SPA requests shutdown via
// /api/quit (e.g. the Close Lumen confirmation in the top bar).
func (s *Server) Quit() <-chan struct{} {
	return s.quit
}

// registerRoutes wires every /api/* endpoint. Later tasks register more.
func (s *Server) registerRoutes() {
	s.mux.HandleFunc("/api/health", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"status":"ok"}`))
	})
	s.mux.HandleFunc("/api/servers", s.handleServers)
	s.mux.HandleFunc("/api/servers/", s.handleServerScoped)
	s.mux.HandleFunc("/api/items/", s.handleItem)
	s.mux.HandleFunc("/api/hubs/", s.handleHub)
	s.mux.HandleFunc("/api/discover-item/", s.handleDiscoverItem)
	s.mux.HandleFunc("/api/watchlist", s.handleWatchlist)
	s.mux.HandleFunc("/api/watchlist/add", s.handleWatchlistAdd)
	s.mux.HandleFunc("/api/watchlist/remove", s.handleWatchlistRemove)
	s.mux.HandleFunc("/api/availability", s.handleAvailability)
	s.mux.HandleFunc("/api/image-proxy", s.handleImageProxy)
	s.mux.HandleFunc("/api/play", s.handlePlay)
	s.mux.HandleFunc("/api/play/transcode", s.handlePlayTranscode)
	s.mux.HandleFunc("/api/play/stop", s.handlePlayStop)
	s.mux.HandleFunc("/api/playback", s.handlePlaybackState)
	s.mux.HandleFunc("/api/playback/stream", s.handlePlaybackStream)
	s.mux.HandleFunc("/api/settings", s.handleSettings)
	s.mux.HandleFunc("/api/cache/size", s.handleCacheSize)
	s.mux.HandleFunc("/api/cache/clear", s.handleCacheClear)
	s.mux.HandleFunc("/api/user", s.handleUser)
	s.mux.HandleFunc("/api/servers/refresh", s.handleServersRefresh)
	s.mux.HandleFunc("/api/settings/omdb", s.handleSettingsOMDB)
	s.mux.HandleFunc("/api/settings/tmdb", s.handleSettingsTMDB)
	s.mux.HandleFunc("/api/auth/start", s.handleAuthStart)
	s.mux.HandleFunc("/api/auth/poll", s.handleAuthPoll)
	s.mux.HandleFunc("/api/imdb/", s.handleIMDB)
	s.mux.HandleFunc("/api/tmdb/trailer/", s.handleTMDBTrailer)
	s.mux.HandleFunc("/api/shortcut", s.handleShortcut)
	s.mux.HandleFunc("/api/quit", s.handleQuit)
	s.mux.HandleFunc("/", s.handleSPA)
}

// ListenAndServe binds and serves. Blocks until Shutdown is called.
func (s *Server) ListenAndServe() error {
	ln, err := net.Listen("tcp", s.http.Addr)
	if err != nil {
		return err
	}
	s.ln = ln
	return s.http.Serve(ln)
}

// Shutdown gracefully stops the server.
func (s *Server) Shutdown() error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	return s.http.Shutdown(ctx)
}

// Addr returns the actual bound address. Useful for tests using port 0.
func (s *Server) Addr() string {
	if s.ln == nil {
		return ""
	}
	return s.ln.Addr().String()
}
