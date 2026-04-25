package playback

import (
	"context"
	"log"
	"time"
)

const (
	pollInterval         = 5 * time.Second
	directPlayTimeout    = 10 * time.Second
	watchedThresholdFrac = 0.90 // mirrors Plex's server-side default
)

// runPoller reads Pot Player's position/state every pollInterval, broadcasts
// state updates, and triggers end-of-file / direct-play-failure logic.
func (m *Manager) runPoller(ctx context.Context, args StartArgs) {
	t := time.NewTicker(pollInterval)
	defer t.Stop()

	startedAt := time.Now()
	scrobbled := false
	durationConfirmed := args.Duration > 0
	endedFired := false

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
			// Final position is whatever we last saw.
			m.broadcast(Event{Type: EventStateUpdate, State: m.snapshot()})
			m.Stop()
			return
		}

		pos, err := c.PotPlayer.GetPosition()
		if err != nil {
			log.Printf("playback: GetPosition: %v", err)
			continue
		}
		state, _ := c.PotPlayer.GetState()

		m.live.mu.Lock()
		m.live.position = pos
		m.live.state = state
		m.live.mu.Unlock()

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

		// 90% threshold: scrobble once, emit ended/next-episode once.
		if c.Duration > 0 && pos >= time.Duration(float64(c.Duration)*watchedThresholdFrac) {
			if !scrobbled {
				if err := m.plex.Scrobble(c.Server, c.RatingKey); err != nil {
					log.Printf("playback: Scrobble: %v", err)
				}
				scrobbled = true
			}
			if !endedFired {
				m.fireEnded(c)
				endedFired = true
			}
		}

		// Always rebroadcast latest state.
		m.broadcast(Event{Type: EventStateUpdate, State: m.snapshot()})
	}
}

// fireEnded emits the appropriate "we crossed the watched threshold" event.
// For episodes, looks up the next-up episode and emits next-episode-prompt;
// for movies, emits a generic ended event.
func (m *Manager) fireEnded(c *Context) {
	if !c.IsEpisode || c.ShowRatingKey == "" {
		m.broadcast(Event{Type: EventEnded})
		return
	}
	next, err := m.plex.NextEpisode(c.Server, c.ShowRatingKey, c.RatingKey)
	if err != nil {
		log.Printf("playback: NextEpisode: %v", err)
		m.broadcast(Event{Type: EventEnded})
		return
	}
	if next == nil {
		// Last episode in show.
		m.broadcast(Event{Type: EventEnded})
		return
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
}
