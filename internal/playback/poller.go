package playback

import (
	"context"
	"time"
)

const (
	pollInterval         = 5 * time.Second
	directPlayTimeout    = 10 * time.Second
	watchedThresholdFrac = 0.95 // bumped from Plex's 90% default — leaves more room on shorter shows
	eofEpsilon           = 2 * time.Second // "position pinned at the end" tolerance for naturalEOF
)

// runPoller reads Pot Player's position/state every pollInterval, broadcasts
// state updates, and triggers end-of-file / direct-play-failure logic.
func (m *Manager) runPoller(ctx context.Context, args StartArgs) {
	t := time.NewTicker(pollInterval)
	defer t.Stop()

	startedAt := time.Now()
	scrobbled := false
	// Note: args.Duration is sourced from Plex item metadata in the /api/play
	// handler (Task 15) BEFORE Start is called, so it's always non-zero in
	// practice — both for direct play and for transcoded sessions. The
	// durationConfirmed=false path below exists as a defensive backstop in
	// case Plex returns metadata without a duration (rare, but possible for
	// in-progress live recordings). The 10 s direct-play timeout is gated by
	// !c.Transcoding because transcode bootstrap can legitimately take longer
	// than that window — and again, the durationConfirmed=true entry case
	// covers the common transcode path.
	durationConfirmed := args.Duration > 0
	endedFired := false
	episodeOverFired := false
	var nextInfo *NextEpisodeInfo
	var lastPos time.Duration

	for {
		select {
		case <-ctx.Done():
			return
		case <-t.C:
		}

		m.mu.Lock()
		c := m.active
		m.mu.Unlock()
		if c == nil {
			return
		}

		// Liveness check first — fast and cheap.
		if !c.PotPlayer.IsAlive() {
			// Manual close past the watched threshold is the "next episode"
			// gesture (spec: scrobble at 95%, advance at actual end).
			if nextInfo != nil && !episodeOverFired && advanceOnClose(lastPos, c.Duration) {
				m.broadcast(Event{Type: EventEpisodeOver, Payload: *nextInfo})
				episodeOverFired = true
			}
			// Final position is whatever we last saw.
			m.broadcast(Event{Type: EventStateUpdate, State: m.snapshot()})
			m.Stop()
			return
		}

		pos, err := c.PotPlayer.GetPosition()
		if err != nil {
			m.logd.Logf("GetPosition", "playback: GetPosition: %v", err)
			continue
		}
		state, _ := c.PotPlayer.GetState()

		m.live.mu.Lock()
		m.live.position = pos
		m.live.state = state
		m.live.mu.Unlock()
		lastPos = pos

		// First non-zero duration confirms direct-play OR transcode bootstrapped.
		if !durationConfirmed {
			d, _ := c.PotPlayer.GetDuration()
			if d > 0 {
				m.mu.Lock()
				c.Duration = d
				m.mu.Unlock()
				durationConfirmed = true
			} else if time.Since(startedAt) > directPlayTimeout && !c.Transcoding {
				// No duration after 10 s — direct play failed. Emit prompt
				// and tear down so the SPA can decide whether to retry as
				// transcode.
				m.broadcast(Event{
					Type: EventTranscodePrompt,
					Payload: TranscodePromptInfo{
						RatingKey: c.RatingKey,
						ServerID:  c.Server.MachineIdentifier,
						Title:     c.Title,
						Reason:    "Pot Player did not report a duration within 10 s",
					},
				})
				m.Stop()
				return
			}
		}

		// 95% threshold: scrobble once, emit ended/next-episode once.
		if c.Duration > 0 && pos >= time.Duration(float64(c.Duration)*watchedThresholdFrac) {
			if !scrobbled {
				// Race guard: Stop may have run between the m.mu unlock above
				// and now. Without this, a stale Scrobble can land after Stop
				// nilled m.active and mark the wrong viewCount on Plex.
				select {
				case <-ctx.Done():
					return
				default:
				}
				if err := m.plex.Scrobble(c.Server, c.RatingKey); err != nil {
					m.logd.Logf("Scrobble", "playback: Scrobble: %v", err)
				} else {
					scrobbled = true
				}
			}
			if !endedFired {
				nextInfo = m.fireEnded(c)
				endedFired = true
			}
		}

		// True end-of-file: PotPlayer parks paused on the last frame, so
		// position pins at duration. Auto-advance — nothing left to protect.
		if nextInfo != nil && !episodeOverFired && naturalEOF(pos, c.Duration) {
			m.broadcast(Event{Type: EventEpisodeOver, Payload: *nextInfo})
			episodeOverFired = true
		}

		// Always rebroadcast latest state.
		m.broadcast(Event{Type: EventStateUpdate, State: m.snapshot()})
	}
}

// fireEnded emits the appropriate "we crossed the watched threshold" event.
// For episodes, looks up the next-up episode and emits next-episode-prompt;
// for movies, emits a generic ended event. Returns the next-episode info so
// the poller can replay it in the episode-over payload; nil when there is no
// next episode to advance to.
func (m *Manager) fireEnded(c *Context) *NextEpisodeInfo {
	if !c.IsEpisode || c.ShowRatingKey == "" {
		m.broadcast(Event{Type: EventEnded})
		return nil
	}
	next, err := m.plex.NextEpisode(c.Server, c.ShowRatingKey, c.RatingKey)
	if err != nil {
		m.logd.Logf("NextEpisode", "playback: NextEpisode: %v", err)
		m.broadcast(Event{Type: EventEnded})
		return nil
	}
	if next == nil {
		// Last episode in show.
		m.broadcast(Event{Type: EventEnded})
		return nil
	}
	info := NextEpisodeInfo{
		RatingKey: next.RatingKey,
		ServerID:  c.Server.MachineIdentifier,
		Title:     next.Title,
		Season:    next.ParentIndex,
		Episode:   next.Index,
	}
	if next.Thumb != "" {
		info.ThumbPath = next.Thumb
	} else if next.GrandparentThumb != "" {
		info.ThumbPath = next.GrandparentThumb
	}
	m.broadcast(Event{Type: EventNextEpisode, Payload: info})
	return &info
}

// naturalEOF reports whether playback reached the true end of the file.
// PotPlayer (per Byron's config) parks paused on the last frame, so position
// pins at/near duration; the 2 s epsilon absorbs the 5 s sample interval's
// coarseness. State is deliberately not consulted — position alone is enough.
func naturalEOF(pos, duration time.Duration) bool {
	return duration > 0 && pos >= duration-eofEpsilon
}

// advanceOnClose reports whether a manual PotPlayer close should count as the
// "next episode" gesture: only when the last-observed position was past the
// watched threshold. Closing earlier means "done watching" — no advance.
func advanceOnClose(lastPos, duration time.Duration) bool {
	return duration > 0 && lastPos >= time.Duration(float64(duration)*watchedThresholdFrac)
}
