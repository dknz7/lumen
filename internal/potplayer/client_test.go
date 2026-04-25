package potplayer

import "testing"

func TestPlayStateString(t *testing.T) {
	cases := map[PlayState]string{
		PlayStateUnknown: "unknown",
		PlayStatePaused:  "paused",
		PlayStatePlaying: "playing",
		PlayStateStopped: "stopped",
	}
	for s, want := range cases {
		if got := s.String(); got != want {
			t.Errorf("%d.String() = %q, want %q", s, got, want)
		}
	}
}
