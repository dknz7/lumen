// Package server hosts Lumen's loopback HTTP server — it serves the embedded
// SolidJS SPA at /, and /api/* routes for Plex proxying.
package server

import (
	"context"
	"net"
	"net/http"
	"time"

	"lumen/internal/config"
	"lumen/internal/plex"
)

// Server bundles the http.Server, mux, Plex client, and loaded config into one
// lifecycle-managed unit.
type Server struct {
	cfg    *config.Config
	plex   *plex.Client
	mux    *http.ServeMux
	http   *http.Server
	ln     net.Listener
	hubs   *hubCache
	images *imageCache
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
		hubs:   newHubCache(),
		images: newImageCache(),
	}
	s.registerRoutes()
	return s
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
	s.mux.HandleFunc("/api/availability", s.handleAvailability)
	s.mux.HandleFunc("/api/image-proxy", s.handleImageProxy)
	s.mux.HandleFunc("/api/play", s.handlePlay)
	s.mux.HandleFunc("/api/settings", s.handleSettings)
	s.mux.HandleFunc("/api/cache/size", s.handleCacheSize)
	s.mux.HandleFunc("/api/cache/clear", s.handleCacheClear)
	s.mux.HandleFunc("/api/user", s.handleUser)
	s.mux.HandleFunc("/api/servers/refresh", s.handleServersRefresh)
	s.mux.HandleFunc("/api/settings/omdb", s.handleSettingsOMDB)
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
