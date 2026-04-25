package playback

import (
	"context"
	"fmt"
	"log"
	"net/url"
	"time"
)

const transcodeKeepAliveInterval = 10 * time.Second

// runTranscodeKeepAlive pings /video/:/transcode/universal/ping every
// transcodeKeepAliveInterval so Plex doesn't reap the transcode session.
// Only spawned when Context.Transcoding is true (gated by Manager.Start).
func (m *Manager) runTranscodeKeepAlive(ctx context.Context) {
	t := time.NewTicker(transcodeKeepAliveInterval)
	defer t.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-t.C:
		}

		m.mu.Lock()
		c := m.active
		m.mu.Unlock()
		if c == nil || !c.Transcoding || c.TranscodeSession == "" {
			return
		}

		// Plex auth is header-only; query carries only session id.
		q := url.Values{"session": []string{c.TranscodeSession}}
		u := fmt.Sprintf("%s/video/:/transcode/universal/ping?%s", c.Server.BaseURL, q.Encode())
		req, err := m.plex.NewRequest("POST", u, nil)
		if err != nil {
			log.Printf("playback: keepalive build: %v", err)
			continue
		}
		m.plex.SetToken(req, c.Server.AccessToken)
		resp, err := m.plex.Do(req)
		if err != nil {
			log.Printf("playback: keepalive request: %v", err)
			continue
		}
		// Explicit close — defer would accumulate inside the for-loop and leak fds.
		_ = resp.Body.Close()
	}
}
