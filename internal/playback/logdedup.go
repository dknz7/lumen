package playback

import (
	"log"
	"sync"
	"time"
)

// logDedupWindow is the per-key suppression window. Plex outages drive the
// poller (5s), reporter (10s), and transcode keep-alive (10s) tickers all
// at once — up to ~25 identical error lines/min. With a 60s window each
// key prints at most once per minute plus a summary line when fresh
// errors arrive after the window expires.
const logDedupWindow = 60 * time.Second

// logDedup is a per-key rate limiter for log.Printf-style calls. First
// occurrence of a key prints immediately. Subsequent occurrences within
// logDedupWindow are silently tallied. The next call after the window
// elapses prints a one-line suppression summary then the fresh error.
//
// Trade-off: if errors stop entirely after suppression, the user never
// sees the final summary. Acceptable — the absence of further error
// lines is itself the recovery signal.
type logDedup struct {
	mu      sync.Mutex
	entries map[string]*logDedupEntry
	window  time.Duration
}

type logDedupEntry struct {
	lastPrintedAt     time.Time
	firstSuppressedAt time.Time
	suppressedCount   int
}

func newLogDedup(window time.Duration) *logDedup {
	return &logDedup{
		entries: make(map[string]*logDedupEntry),
		window:  window,
	}
}

// Logf is the rate-limited equivalent of log.Printf. Key identifies the
// error class (e.g. "Scrobble", "ReportTimeline") for dedup purposes;
// format/args are passed through to log.Printf when a print occurs.
func (d *logDedup) Logf(key, format string, args ...any) {
	now := time.Now()
	d.mu.Lock()
	defer d.mu.Unlock()
	e, ok := d.entries[key]
	if !ok {
		d.entries[key] = &logDedupEntry{lastPrintedAt: now}
		log.Printf(format, args...)
		return
	}
	if now.Sub(e.lastPrintedAt) < d.window {
		if e.suppressedCount == 0 {
			e.firstSuppressedAt = now
		}
		e.suppressedCount++
		return
	}
	if e.suppressedCount > 0 {
		elapsed := now.Sub(e.firstSuppressedAt).Round(time.Second)
		log.Printf("playback: %s suppressed %d times in %s", key, e.suppressedCount, elapsed)
		e.suppressedCount = 0
	}
	log.Printf(format, args...)
	e.lastPrintedAt = now
}
