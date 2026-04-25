package server

import (
	"encoding/json"
	"fmt"
	"net/http"

	"lumen/internal/playback"
)

func (s *Server) handlePlaybackState(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "GET only")
		return
	}
	writeJSON(w, s.playback.SnapshotState())
}

func (s *Server) handlePlaybackStream(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "GET only")
		return
	}
	flusher, ok := w.(http.Flusher)
	if !ok {
		writeError(w, http.StatusInternalServerError, "streaming unsupported")
		return
	}

	// No keep-alive heartbeat. Lumen serves on 127.0.0.1 with no proxy
	// intermediaries, so idle SSE connections won't be culled. If we ever
	// expose this beyond loopback, add a periodic ":ping\n\n" comment line
	// every ~30s.
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("X-Accel-Buffering", "no")

	// Subscribe before snapshotting. There's a narrow race here: between
	// Subscribe and SnapshotState, a Manager broadcast can fire — the
	// subscriber would receive the new event in `ch` and the handler sends
	// an older snapshot as the "initial" payload. Out-of-order. Acceptable
	// for v1.0 because every State carries the full session shape (Task 10)
	// and the SPA's store (Task 19) idempotently merges by RatingKey —
	// older snapshots are self-correcting once newer events arrive.
	ch, cleanup := s.playback.Subscribe()
	defer cleanup()

	// Initial snapshot — guarantees the SPA has state even before any event
	// fires. Wrapped in an Event envelope so the SPA always parses the same
	// shape (rather than bare State for the first message and Event for
	// subsequent ones).
	initial := s.playback.SnapshotState()
	initialEvent := playback.Event{Type: playback.EventStateUpdate, State: &initial}
	writeSSE(w, "state", initialEvent)
	flusher.Flush()

	for {
		select {
		case <-r.Context().Done():
			return
		case ev := <-ch:
			writeSSE(w, ev.Type, ev)
			flusher.Flush()
		}
	}
}

// writeSSE writes one Server-Sent Event with a named event type and a
// JSON-encoded data payload. SPA dispatches on the event name via
// EventSource.addEventListener(<eventType>, ...).
func writeSSE(w http.ResponseWriter, eventType string, data any) {
	body, err := json.Marshal(data)
	if err != nil {
		// Silently skip malformed payloads — Event/State are well-typed
		// and shouldn't fail to marshal in practice. Logging here would
		// spam during edge cases (e.g. nil data) so we accept the drop.
		return
	}
	fmt.Fprintf(w, "event: %s\ndata: %s\n\n", eventType, body)
}
