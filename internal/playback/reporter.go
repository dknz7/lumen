package playback

import (
	"context"
	"time"

	"lumen/internal/plex"
	"lumen/internal/potplayer"
)

const reporterInterval = 10 * time.Second

// runReporter POSTs /:/timeline every reporterInterval so Plex updates
// viewOffset, triggers server-side watched-state machinery, and keeps
// the session anchored as "in progress" for cross-device continuation.
func (m *Manager) runReporter(ctx context.Context) {
	t := time.NewTicker(reporterInterval)
	defer t.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-t.C:
		}

		// Capture c and c.Duration in the same critical section. The poller
		// refines c.Duration under m.mu the first time GetDuration returns
		// non-zero, so reading c.Duration outside the lock would race that
		// write. Mirrors the fix already applied in Stop().
		m.mu.Lock()
		c := m.active
		var duration time.Duration
		if c != nil {
			duration = c.Duration
		}
		m.mu.Unlock()
		if c == nil {
			return
		}

		m.live.mu.Lock()
		pos := m.live.position
		state := stateToPlexString(m.live.state)
		m.live.mu.Unlock()

		// Race guard: Stop may have run between capturing c above and now.
		// Without this, a stale ReportTimeline can land after Stop's final
		// stopped-state report and overwrite Plex's viewOffset.
		select {
		case <-ctx.Done():
			return
		default:
		}

		err := m.plex.ReportTimeline(c.Server, plex.TimelineReport{
			RatingKey: c.RatingKey,
			State:     state,
			Position:  pos,
			Duration:  duration,
		})
		if err != nil {
			m.logd.Logf("ReportTimeline", "playback: ReportTimeline: %v", err)
		}
	}
}

// stateToPlexString maps potplayer.PlayState to the lowercase strings
// /:/timeline expects. Defaults to "playing" on unknown states (cold-start)
// so Plex doesn't pause the session needlessly.
func stateToPlexString(s potplayer.PlayState) string {
	switch s {
	case potplayer.PlayStatePaused:
		return "paused"
	case potplayer.PlayStatePlaying:
		return "playing"
	default:
		return "playing"
	}
}
