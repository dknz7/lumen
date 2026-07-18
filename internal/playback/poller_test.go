package playback

import (
	"testing"
	"time"
)

func TestNaturalEOF(t *testing.T) {
	d := 40 * time.Minute
	cases := []struct {
		name string
		pos  time.Duration
		dur  time.Duration
		want bool
	}{
		{"position at duration", d, d, true},
		{"within epsilon of end", d - time.Second, d, true},
		{"exactly epsilon from end", d - eofEpsilon, d, true},
		{"just outside epsilon", d - eofEpsilon - time.Millisecond, d, false},
		{"paused mid-credits at 96%", time.Duration(float64(d) * 0.96), d, false},
		{"zero duration never ends", 5 * time.Minute, 0, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := naturalEOF(tc.pos, tc.dur); got != tc.want {
				t.Errorf("naturalEOF(%v, %v) = %v, want %v", tc.pos, tc.dur, got, tc.want)
			}
		})
	}
}

func TestAdvanceOnClose(t *testing.T) {
	d := 40 * time.Minute
	cases := []struct {
		name string
		frac float64
		dur  time.Duration
		want bool
	}{
		{"closed during credits at 97%", 0.97, d, true},
		{"closed exactly at threshold", 0.95, d, true},
		{"closed just below threshold", 0.94, d, false},
		{"rewound then closed at 50%", 0.50, d, false},
		{"zero duration never advances", 0.99, 0, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			lastPos := time.Duration(float64(d) * tc.frac)
			if got := advanceOnClose(lastPos, tc.dur); got != tc.want {
				t.Errorf("advanceOnClose(%v, %v) = %v, want %v", lastPos, tc.dur, got, tc.want)
			}
		})
	}
}
