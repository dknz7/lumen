package server

import (
	"crypto/rand"
	"encoding/hex"
	"io"
	"log"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"
)

// HLS trailer proxying.
//
// Plex's Discover hubs return clip items (New Trailers / Trending Trailers)
// whose playback URL is a native HLS manifest on the Discover host. Those
// requests need a Plex token.
//
// Stamping the account token into the URL and handing it to the SPA — which is
// what Lumen used to do — puts the *account* token (the broadest-scoped
// credential Lumen holds) into the DOM, the WebView's network log, and any
// screenshot or devtools capture a user might share in a bug report. It also
// falsified the project's stated security model, which is that the account
// token never leaves the Go process.
//
// Instead the SPA gets an opaque, expiring handle: /api/hls/<id>/... . The
// server keeps the real URL and token, and streams the bytes through.
//
// Relative URIs inside the manifest need no rewriting: the browser resolves
// them against the proxy path, which maps back onto the upstream base. Only
// absolute URLs have to be rewritten, and those are handled below.

const hlsHandleTTL = 30 * time.Minute

type hlsEntry struct {
	base    *url.URL // upstream manifest URL, token stripped
	token   string
	created time.Time
}

type hlsProxy struct {
	mu      sync.Mutex
	entries map[string]hlsEntry
}

func newHLSProxy() *hlsProxy { return &hlsProxy{entries: map[string]hlsEntry{}} }

// mint records an upstream manifest URL and returns the opaque path the SPA
// should use. Returns "" if the URL is unusable.
func (p *hlsProxy) mint(rawURL, token string) string {
	u, err := url.Parse(rawURL)
	if err != nil || u.Scheme == "" || u.Host == "" {
		return ""
	}
	// Never keep a token that arrived embedded in the URL.
	q := u.Query()
	q.Del("X-Plex-Token")
	u.RawQuery = q.Encode()

	var b [16]byte
	if _, err := rand.Read(b[:]); err != nil {
		return ""
	}
	id := hex.EncodeToString(b[:])

	p.mu.Lock()
	defer p.mu.Unlock()
	p.gcLocked()
	p.entries[id] = hlsEntry{base: u, token: token, created: time.Now()}
	return "/api/hls/" + id + "/" + lastSegment(u.Path)
}

func (p *hlsProxy) get(id string) (hlsEntry, bool) {
	p.mu.Lock()
	defer p.mu.Unlock()
	e, ok := p.entries[id]
	if !ok || time.Since(e.created) > hlsHandleTTL {
		return hlsEntry{}, false
	}
	return e, true
}

func (p *hlsProxy) gcLocked() {
	for id, e := range p.entries {
		if time.Since(e.created) > hlsHandleTTL {
			delete(p.entries, id)
		}
	}
}

func lastSegment(path string) string {
	if i := strings.LastIndex(path, "/"); i >= 0 && i < len(path)-1 {
		return path[i+1:]
	}
	return "index.m3u8"
}

// handleHLS streams a trailer manifest or segment, attaching the Plex token
// server-side. Path shape: /api/hls/<id>/<upstream-relative-path>
func (s *Server) handleHLS(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet && r.Method != http.MethodHead {
		writeError(w, http.StatusMethodNotAllowed, "GET only")
		return
	}
	rest := strings.TrimPrefix(r.URL.Path, "/api/hls/")
	id, sub, _ := strings.Cut(rest, "/")
	if id == "" {
		writeError(w, http.StatusBadRequest, "missing handle")
		return
	}
	entry, ok := s.hls.get(id)
	if !ok {
		writeError(w, http.StatusGone, "this trailer link has expired — reopen the trailer")
		return
	}

	// Resolve the requested sub-path against the upstream manifest URL. Using
	// ResolveReference keeps ".." contained: the result can't escape the
	// upstream host, and we re-check the host below regardless.
	ref, err := url.Parse(sub)
	if err != nil {
		writeError(w, http.StatusBadRequest, "bad path")
		return
	}
	target := entry.base.ResolveReference(ref)
	if target.Host != entry.base.Host || target.Scheme != entry.base.Scheme {
		writeError(w, http.StatusForbidden, "cross-host redirect refused")
		return
	}
	// Carry through range/bitrate query params the player added.
	if r.URL.RawQuery != "" {
		q := target.Query()
		for k, vs := range r.URL.Query() {
			if strings.EqualFold(k, "X-Plex-Token") {
				continue // never let the client choose the token
			}
			for _, v := range vs {
				q.Set(k, v)
			}
		}
		target.RawQuery = q.Encode()
	}

	req, err := http.NewRequestWithContext(r.Context(), r.Method, target.String(), nil)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "could not build request")
		return
	}
	// Header-only auth, as everywhere else in Lumen.
	req.Header.Set("X-Plex-Token", entry.token)
	req.Header.Set("Accept", "*/*")
	if rng := r.Header.Get("Range"); rng != "" {
		req.Header.Set("Range", rng)
	}

	resp, err := s.hlsClient.Do(req)
	if err != nil {
		log.Printf("hls proxy: %s: %v", target.Path, err)
		writeError(w, http.StatusBadGateway, "could not reach Plex for this trailer")
		return
	}
	defer resp.Body.Close()

	ct := resp.Header.Get("Content-Type")
	isManifest := strings.Contains(ct, "mpegurl") ||
		strings.HasSuffix(target.Path, ".m3u8") ||
		strings.HasSuffix(target.Path, ".m3u")

	if isManifest {
		body, err := io.ReadAll(io.LimitReader(resp.Body, 4<<20))
		if err != nil {
			writeError(w, http.StatusBadGateway, "could not read trailer manifest")
			return
		}
		rewritten := rewriteManifest(string(body), entry.base, "/api/hls/"+id+"/")
		w.Header().Set("Content-Type", "application/vnd.apple.mpegurl")
		w.Header().Set("Cache-Control", "no-store")
		w.WriteHeader(resp.StatusCode)
		_, _ = io.WriteString(w, rewritten)
		return
	}

	for _, h := range []string{"Content-Type", "Content-Length", "Content-Range", "Accept-Ranges"} {
		if v := resp.Header.Get(h); v != "" {
			w.Header().Set(h, v)
		}
	}
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(resp.StatusCode)
	_, _ = io.Copy(w, resp.Body)
}

// rewriteManifest points any absolute upstream URL in an HLS manifest back
// through the proxy. Relative URIs are left alone — the browser resolves them
// against the proxy path, which already maps onto the upstream base — and any
// token found in a URL is dropped rather than forwarded to the client.
func rewriteManifest(body string, base *url.URL, prefix string) string {
	upstreamPrefix := base.Scheme + "://" + base.Host

	rewrite := func(raw string) string {
		if !strings.HasPrefix(raw, upstreamPrefix) {
			return raw
		}
		u, err := url.Parse(raw)
		if err != nil {
			return raw
		}
		q := u.Query()
		q.Del("X-Plex-Token")
		rel := strings.TrimPrefix(u.Path, "/")
		if enc := q.Encode(); enc != "" {
			return prefix + rel + "?" + enc
		}
		return prefix + rel
	}

	lines := strings.Split(body, "\n")
	for i, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" {
			continue
		}
		if strings.HasPrefix(trimmed, "#") {
			// Tag lines can carry URI="..." (keys, media renditions, maps).
			if idx := strings.Index(line, `URI="`); idx >= 0 {
				start := idx + len(`URI="`)
				if end := strings.Index(line[start:], `"`); end >= 0 {
					uri := line[start : start+end]
					lines[i] = line[:start] + rewrite(uri) + line[start+end:]
				}
			}
			continue
		}
		lines[i] = rewrite(trimmed)
	}
	return strings.Join(lines, "\n")
}
