package playback

import (
	"context"
	"errors"
	"sync"
	"time"

	"lumen/internal/plex"
	"lumen/internal/potplayer"
)

// Manager owns the lifecycle of the single active playback Context. Methods
// are safe to call concurrently.
type Manager struct {
	plex    *plex.Client
	potPath func() string // closure so the path picks up Settings updates

	mu     sync.Mutex
	active *Context
	live   liveState
	cancel context.CancelFunc
	subs   map[chan Event]struct{}
}

// NewManager wires a Manager to a Plex client and a Pot Player path resolver.
// The resolver is a closure so updates to Settings → Playback → Pot Player
// path are visible without restarting the manager.
func NewManager(c *plex.Client, potPath func() string) *Manager {
	return &Manager{
		plex:    c,
		potPath: potPath,
		subs:    make(map[chan Event]struct{}),
	}
}

// StartArgs is the input to Start.
type StartArgs struct {
	Server           *plex.Server
	RatingKey        string
	ShowRatingKey    string // empty for movies
	IsEpisode        bool
	PartID           string
	Container        string
	StreamURL        string // built by caller (DirectPlayURL or TranscodeURL)
	Transcoding      bool
	TranscodeSession string
	Duration         time.Duration // initial duration from Plex metadata; refined after launch
	Title            string
	ShowTitle        string
	ThumbPath        string
	Quality          string // e.g. "1080p H.264"
}

// Start launches Pot Player, builds the Context, kicks the three goroutines.
// Returns ErrAlreadyActive if another session is live.
func (m *Manager) Start(args StartArgs) (err error) {
	m.mu.Lock()
	if m.active != nil {
		m.mu.Unlock()
		return ErrAlreadyActive
	}

	exe := m.potPath()
	if exe == "" {
		m.mu.Unlock()
		return ErrPotPlayerPathUnresolved
	}

	pp, err := potplayer.Launch(exe, args.StreamURL)
	if err != nil {
		m.mu.Unlock()
		return err
	}
	defer func() {
		if err != nil {
			_ = pp.Stop()
		}
	}()

	ctx, cancel := context.WithCancel(context.Background())
	c := &Context{
		RatingKey:        args.RatingKey,
		Server:           args.Server,
		ShowRatingKey:    args.ShowRatingKey,
		IsEpisode:        args.IsEpisode,
		PartID:           args.PartID,
		Container:        args.Container,
		StartedAt:        time.Now(),
		Duration:         args.Duration,
		Transcoding:      args.Transcoding,
		TranscodeSession: args.TranscodeSession,
		Title:            args.Title,
		ShowTitle:        args.ShowTitle,
		ThumbPath:        args.ThumbPath,
		Quality:          args.Quality,
		PotPlayer:        pp,
	}
	m.active = c
	m.cancel = cancel
	m.live.position = 0
	m.live.state = potplayer.PlayStateUnknown
	m.mu.Unlock()

	// Initial state broadcast.
	m.broadcast(Event{Type: EventStateUpdate, State: m.snapshot()})

	// Kick goroutines (each is defined in its own file).
	go m.runPoller(ctx, args)
	go m.runReporter(ctx)
	if args.Transcoding {
		go m.runTranscodeKeepAlive(ctx)
	}

	return nil
}

// Stop tears down the active session. Idempotent — capture-and-clear under
// one lock so concurrent Stop() calls don't double-fire teardown side effects
// (notably the final ReportTimeline POST). Also captures c.Duration in the
// same critical section so the poller's duration-refinement write doesn't
// race the final timeline report.
func (m *Manager) Stop() {
	m.mu.Lock()
	c := m.active
	cancel := m.cancel
	m.active = nil
	m.cancel = nil
	var duration time.Duration
	if c != nil {
		duration = c.Duration
	}
	m.mu.Unlock()
	if c == nil {
		return
	}
	if cancel != nil {
		cancel()
	}
	if c.PotPlayer != nil {
		_ = c.PotPlayer.Stop()
	}
	// Final timeline report — best-effort, swallow error.
	pos := m.currentPosition()
	_ = m.plex.ReportTimeline(c.Server, plex.TimelineReport{
		RatingKey: c.RatingKey,
		State:     "stopped",
		Position:  pos,
		Duration:  duration,
	})
	m.broadcast(Event{Type: EventStopped})
}

// Subscribe returns a channel of events. Caller MUST call the returned cleanup
// func — even on error or early return — or the subscription leaks for the
// process lifetime. The channel is buffered (16 slots) and broadcast drops
// events on full channels rather than blocking.
func (m *Manager) Subscribe() (<-chan Event, func()) {
	ch := make(chan Event, 16)
	m.mu.Lock()
	m.subs[ch] = struct{}{}
	m.mu.Unlock()
	cleanup := func() {
		m.mu.Lock()
		delete(m.subs, ch)
		m.mu.Unlock()
	}
	return ch, cleanup
}

// SnapshotState returns the current playback State (whether or not a session
// is active).
func (m *Manager) SnapshotState() State {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.active == nil {
		return State{Active: false}
	}
	return m.snapshotLocked()
}

// broadcast fans an event out to all subscribers. Drops events on full
// channels rather than blocking.
func (m *Manager) broadcast(e Event) {
	m.mu.Lock()
	subs := make([]chan Event, 0, len(m.subs))
	for ch := range m.subs {
		subs = append(subs, ch)
	}
	m.mu.Unlock()
	for _, ch := range subs {
		select {
		case ch <- e:
		default:
		}
	}
}

// currentPosition reads the latest position the poller cached.
func (m *Manager) currentPosition() time.Duration {
	m.live.mu.Lock()
	defer m.live.mu.Unlock()
	return m.live.position
}

// snapshot builds a fresh State; call without holding m.mu (it locks itself).
func (m *Manager) snapshot() *State {
	m.mu.Lock()
	defer m.mu.Unlock()
	st := m.snapshotLocked()
	return &st
}

func (m *Manager) snapshotLocked() State {
	if m.active == nil {
		return State{Active: false}
	}
	m.live.mu.Lock()
	pos := m.live.position
	stateStr := m.live.state.String()
	m.live.mu.Unlock()
	return State{
		Active:      true,
		RatingKey:   m.active.RatingKey,
		ServerID:    m.active.Server.MachineIdentifier,
		Title:       m.active.Title,
		ShowTitle:   m.active.ShowTitle,
		Position:    pos,
		Duration:    m.active.Duration,
		State:       stateStr,
		Transcoding: m.active.Transcoding,
		ThumbPath:   m.active.ThumbPath,
		Quality:     m.active.Quality,
	}
}

// ErrAlreadyActive is returned by Start when a session is already running.
var ErrAlreadyActive = errors.New("playback session already active")

// ErrPotPlayerPathUnresolved is returned by Start when the Pot Player path
// resolver returns an empty string — typically a Settings configuration gap.
var ErrPotPlayerPathUnresolved = errors.New("pot player path not resolved")

// --- goroutine stubs — replaced by Tasks 11-13 ---

func (m *Manager) runReporter(ctx context.Context)           {}
func (m *Manager) runTranscodeKeepAlive(ctx context.Context) {}
