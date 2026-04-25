// Package playback orchestrates a single active media playback session —
// driving Pot Player, syncing progress to Plex, and streaming state to the
// SPA. Spec §9.
package playback

import (
	"sync"
	"time"

	"lumen/internal/plex"
	"lumen/internal/potplayer"
)

// Context carries everything one active playback session needs. There is at
// most one Context alive in the process at a time (UI enforces single-stream).
type Context struct {
	RatingKey        string
	Server           *plex.Server
	ShowRatingKey    string // empty for movies
	IsEpisode        bool
	PartID           string
	Container        string
	StartedAt        time.Time
	Duration         time.Duration
	Transcoding      bool
	TranscodeSession string

	PotPlayer *potplayer.Client
}

// State is the snapshot the SPA consumes via /api/playback (and SSE).
type State struct {
	Active      bool          `json:"active"`
	RatingKey   string        `json:"ratingKey,omitempty"`
	ServerID    string        `json:"serverID,omitempty"`
	Title       string        `json:"title,omitempty"`
	ShowTitle   string        `json:"showTitle,omitempty"`
	Position    time.Duration `json:"position"`
	Duration    time.Duration `json:"duration"`
	State       string        `json:"state"` // "playing" | "paused" | "stopped" | "unknown"
	Quality     string        `json:"quality,omitempty"`
	Transcoding bool          `json:"transcoding"`
	ThumbPath   string        `json:"thumbPath,omitempty"`
}

// Event is the discriminated message the SPA receives over SSE.
type Event struct {
	Type    string `json:"type"`
	State   *State `json:"state,omitempty"`
	Payload any    `json:"payload,omitempty"`
}

// Event types — the SPA dispatches on Type.
const (
	EventStateUpdate     = "state"
	EventEnded           = "ended"
	EventNextEpisode     = "next-episode-prompt"
	EventTranscodePrompt = "transcode-prompt"
	EventStopped         = "stopped"
)

// NextEpisodeInfo accompanies an EventNextEpisode payload.
type NextEpisodeInfo struct {
	RatingKey string `json:"ratingKey"`
	ServerID  string `json:"serverID"`
	Title     string `json:"title"`
	Season    int    `json:"season"`
	Episode   int    `json:"episode"`
	ThumbPath string `json:"thumbPath,omitempty"`
}

// TranscodePromptInfo accompanies EventTranscodePrompt.
type TranscodePromptInfo struct {
	RatingKey string `json:"ratingKey"`
	ServerID  string `json:"serverID"`
	Title     string `json:"title"`
	Reason    string `json:"reason"`
}

// stateMu protects mutable fields on the live Context (position, etc.) so
// the poller and SSE encoder don't race.
type liveState struct {
	mu       sync.Mutex
	position time.Duration
	state    potplayer.PlayState
}
