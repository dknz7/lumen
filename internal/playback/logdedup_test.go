package playback

import (
	"bytes"
	"log"
	"strings"
	"testing"
	"time"
)

// captureLog redirects the default log output to a buffer for the duration
// of the test, restoring the previous writer when done.
func captureLog(t *testing.T) *bytes.Buffer {
	t.Helper()
	buf := &bytes.Buffer{}
	prev := log.Writer()
	prevFlags := log.Flags()
	log.SetOutput(buf)
	log.SetFlags(0)
	t.Cleanup(func() {
		log.SetOutput(prev)
		log.SetFlags(prevFlags)
	})
	return buf
}

func TestLogDedup_FirstCallPrintsImmediately(t *testing.T) {
	buf := captureLog(t)
	d := newLogDedup(time.Hour)
	d.Logf("Scrobble", "playback: Scrobble: %v", "boom")
	if !strings.Contains(buf.String(), "playback: Scrobble: boom") {
		t.Fatalf("expected first call to print; got %q", buf.String())
	}
}

func TestLogDedup_WithinWindowSuppresses(t *testing.T) {
	buf := captureLog(t)
	d := newLogDedup(time.Hour)
	d.Logf("Scrobble", "playback: Scrobble: %v", "first")
	d.Logf("Scrobble", "playback: Scrobble: %v", "second")
	d.Logf("Scrobble", "playback: Scrobble: %v", "third")
	out := buf.String()
	if strings.Count(out, "Scrobble") != 1 {
		t.Fatalf("expected exactly one Scrobble line, got %d in %q", strings.Count(out, "Scrobble"), out)
	}
	if strings.Contains(out, "second") || strings.Contains(out, "third") {
		t.Fatalf("expected within-window calls to be suppressed; got %q", out)
	}
}

func TestLogDedup_PostWindowFlushesSummaryThenPrints(t *testing.T) {
	buf := captureLog(t)
	d := newLogDedup(10 * time.Millisecond)
	d.Logf("Scrobble", "playback: Scrobble: %v", "first")
	d.Logf("Scrobble", "playback: Scrobble: %v", "suppressed-1")
	d.Logf("Scrobble", "playback: Scrobble: %v", "suppressed-2")
	time.Sleep(20 * time.Millisecond)
	d.Logf("Scrobble", "playback: Scrobble: %v", "after-window")

	out := buf.String()
	if !strings.Contains(out, "first") {
		t.Fatalf("expected first line; got %q", out)
	}
	if !strings.Contains(out, "suppressed 2 times") {
		t.Fatalf("expected suppression summary with count 2; got %q", out)
	}
	if !strings.Contains(out, "after-window") {
		t.Fatalf("expected fresh log after window; got %q", out)
	}
	if strings.Contains(out, "suppressed-1") || strings.Contains(out, "suppressed-2") {
		t.Fatalf("expected within-window calls to be suppressed; got %q", out)
	}
}

func TestLogDedup_DistinctKeysDoNotInterfere(t *testing.T) {
	buf := captureLog(t)
	d := newLogDedup(time.Hour)
	d.Logf("Scrobble", "playback: Scrobble: %v", "x")
	d.Logf("ReportTimeline", "playback: ReportTimeline: %v", "y")
	out := buf.String()
	if !strings.Contains(out, "Scrobble: x") {
		t.Fatalf("missing Scrobble line: %q", out)
	}
	if !strings.Contains(out, "ReportTimeline: y") {
		t.Fatalf("missing ReportTimeline line: %q", out)
	}
}

func TestLogDedup_PostWindowWithNoSuppressedJustPrints(t *testing.T) {
	buf := captureLog(t)
	d := newLogDedup(10 * time.Millisecond)
	d.Logf("Scrobble", "playback: Scrobble: %v", "first")
	time.Sleep(20 * time.Millisecond)
	d.Logf("Scrobble", "playback: Scrobble: %v", "second")
	out := buf.String()
	if strings.Contains(out, "suppressed") {
		t.Fatalf("expected no suppression summary when nothing was suppressed; got %q", out)
	}
	if !strings.Contains(out, "first") || !strings.Contains(out, "second") {
		t.Fatalf("expected both lines printed; got %q", out)
	}
}
