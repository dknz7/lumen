# Next-Episode Autoplay Redesign Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Replace the 10-second countdown next-episode modal with EOF-driven auto-advance — scrobble stays at 95%, the next episode launches only when the current one *actually* ends (natural end-of-file or PotPlayer closed past the watched threshold).

**Architecture:** The Go poller (`internal/playback/poller.go`) already samples PotPlayer position/liveness every 5 s and fires `next-episode-prompt` at 95%. We add a new `episode-over` SSE event fired exactly once when position pins at the file's end (PotPlayer parks paused on the last frame) or when the player process dies with last-observed position past 95%. The SolidJS SPA turns the countdown modal into a passive "Up Next" card and runs the existing stop-and-play-next sequence immediately on `episode-over`, unless the user Dismissed (binge-cancel flag).

**Tech Stack:** Go (backend poller + SSE events), SolidJS + TypeScript (SPA), Vite (builds into `internal/server/web/dist`, embedded in the Go binary).

**Spec:** `docs/superpowers/specs/2026-07-19-next-episode-autoplay-design.md`

## Global Constraints

- 95% watched threshold (`watchedThresholdFrac = 0.95`) is unchanged and still drives scrobble + `next-episode-prompt`.
- Natural-EOF tolerance is 2 seconds: `position >= duration - 2s`.
- `episode-over` fires **at most once** per playback session and carries the same `NextEpisodeInfo` payload shape as `next-episode-prompt`.
- Manual close below 95% = stop, no event, no advance (existing behaviour untouched).
- Movies and last-episode-of-show keep the existing `ended` event path untouched.
- No countdown/timer anywhere in the new UI.
- Working directory: `C:\Users\dicke\Desktop\Dump Zone\STACK\04-DEV\lumen`. All commands below run from repo root unless stated.

---

### Task 1: EOF predicates in the poller (TDD)

Two pure functions decide "the episode is truly over". They are pure so they're unit-testable without faking the PotPlayer client (which is a concrete struct — there is no existing poller test infrastructure and we are not building a fake for this).

**Files:**
- Create: `internal/playback/poller_test.go`
- Modify: `internal/playback/poller.go` (const block + two new functions; no wiring yet)

**Interfaces:**
- Produces: `naturalEOF(pos, duration time.Duration) bool` and `advanceOnClose(lastPos, duration time.Duration) bool` — consumed by Task 2's poller wiring. Also `eofEpsilon = 2 * time.Second` const.

- [ ] **Step 1: Write the failing tests**

Create `internal/playback/poller_test.go`:

```go
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
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `go test ./internal/playback/ -run "TestNaturalEOF|TestAdvanceOnClose" -v`
Expected: FAIL to compile — `undefined: naturalEOF`, `undefined: advanceOnClose`, `undefined: eofEpsilon`

- [ ] **Step 3: Implement the predicates**

In `internal/playback/poller.go`, extend the const block at the top of the file:

```go
const (
	pollInterval         = 5 * time.Second
	directPlayTimeout    = 10 * time.Second
	watchedThresholdFrac = 0.95 // bumped from Plex's 90% default — leaves more room on shorter shows
	eofEpsilon           = 2 * time.Second // "position pinned at the end" tolerance for naturalEOF
)
```

Then add the two functions at the bottom of the file (after `fireEnded`):

```go
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
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `go test ./internal/playback/ -run "TestNaturalEOF|TestAdvanceOnClose" -v`
Expected: PASS (11 subtests)

- [ ] **Step 5: Commit**

```bash
git add internal/playback/poller.go internal/playback/poller_test.go
git commit -m "feat(playback): add EOF predicates for next-episode auto-advance"
```

---

### Task 2: `episode-over` event — poller wiring

Fire the new event exactly once when either predicate trips, and cache the next-episode info found at the 95% mark so the manual-close path has a payload.

**Files:**
- Modify: `internal/playback/context.go:73-79` (event const block)
- Modify: `internal/playback/poller.go` (`runPoller` + `fireEnded` signature)

**Interfaces:**
- Consumes: `naturalEOF`, `advanceOnClose` from Task 1.
- Produces: SSE event `Type: "episode-over"` (`EventEpisodeOver`) with `Payload: NextEpisodeInfo` — consumed by Task 3's frontend listener. `fireEnded` signature becomes `func (m *Manager) fireEnded(c *Context) *NextEpisodeInfo` (returns nil for movies / last episode / lookup error).

- [ ] **Step 1: Add the event constant**

In `internal/playback/context.go`, extend the event const block:

```go
// Event types — the SPA dispatches on Type.
const (
	EventStateUpdate     = "state"
	EventEnded           = "ended"
	EventNextEpisode     = "next-episode-prompt"
	EventEpisodeOver     = "episode-over"
	EventTranscodePrompt = "transcode-prompt"
	EventStopped         = "stopped"
)
```

- [ ] **Step 2: Make `fireEnded` return the next-episode info**

In `internal/playback/poller.go`, change `fireEnded` to return `*NextEpisodeInfo` (nil on every no-next-episode path). Full replacement:

```go
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
```

- [ ] **Step 3: Wire the poller loop**

In `runPoller`, make four changes:

**(a)** Extend the local state before the `for` loop (currently `durationConfirmed` / `endedFired`):

```go
	durationConfirmed := args.Duration > 0
	endedFired := false
	episodeOverFired := false
	var nextInfo *NextEpisodeInfo
	var lastPos time.Duration
```

**(b)** In the liveness branch (`if !c.PotPlayer.IsAlive() {`), fire episode-over for a manual close past threshold, before the existing teardown:

```go
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
```

**(c)** Record `lastPos` right after the successful position read (after the `m.live.mu` update block):

```go
		m.live.mu.Lock()
		m.live.position = pos
		m.live.state = state
		m.live.mu.Unlock()
		lastPos = pos
```

**(d)** Capture `fireEnded`'s return in the threshold block, then check natural EOF after it (insert between the threshold block and the final "Always rebroadcast" line):

```go
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
```

Note: `c.Duration` is only ever mutated inside this same goroutine (the `durationConfirmed` block), so the unlocked reads match the file's existing pattern (see the threshold check).

- [ ] **Step 4: Build and run the full backend test suite**

Run: `go build ./... && go test ./...`
Expected: build OK, all tests PASS (including Task 1's predicates and the existing `internal/plex` / `logdedup` tests)

- [ ] **Step 5: Commit**

```bash
git add internal/playback/context.go internal/playback/poller.go
git commit -m "feat(playback): fire episode-over on true EOF or manual close past threshold"
```

---

### Task 3: Frontend — event type + store handling with binge-cancel flag

**Files:**
- Modify: `web/src/api/types.ts:143-148` (`PlaybackEvent` union)
- Modify: `web/src/state/playback.ts`

**Interfaces:**
- Consumes: SSE event `episode-over` with `NextEpisodeInfo` payload from Task 2.
- Produces (consumed by Task 4's modal): store members `episodeOver: () => NextEpisodeInfo | null`, `clearEpisodeOver(): void`, `cancelBinge(): void` — alongside existing `nextEpisode`, `dismissNextEpisode`.

- [ ] **Step 1: Add the event variant**

In `web/src/api/types.ts`, extend the `PlaybackEvent` union:

```ts
export type PlaybackEvent =
  | { type: "state"; state: PlaybackState }
  | { type: "ended" }
  | { type: "next-episode-prompt"; payload: NextEpisodeInfo }
  | { type: "episode-over"; payload: NextEpisodeInfo }
  | { type: "transcode-prompt"; payload: TranscodePromptInfo }
  | { type: "stopped" };
```

- [ ] **Step 2: Store — episode-over signal + binge-cancel flag**

In `web/src/state/playback.ts`:

**(a)** Add a signal and a plain flag next to the existing modal-trigger signals (after the `endedAt` line):

```ts
  const [nextEpisode, setNextEpisode] = createSignal<NextEpisodeInfo | null>(null);
  const [transcodePrompt, setTranscodePrompt] = createSignal<TranscodePromptInfo | null>(null);
  const [endedAt, setEndedAt] = createSignal<number>(0);
  // Set when the backend says the episode truly ended (natural EOF or player
  // closed past the watched threshold) — the SPA advances immediately.
  const [episodeOver, setEpisodeOver] = createSignal<NextEpisodeInfo | null>(null);
  // Dismiss = opt out of auto-advance for this transition only. Reset when
  // the next prompt arrives (i.e. the next episode's own 95% mark).
  let bingeCancelled = false;
```

**(b)** Replace the existing `next-episode-prompt` listener so each new prompt resets the flag, and add the `episode-over` listener after it:

```ts
    es.addEventListener("next-episode-prompt", (ev) => {
      const evt = parse(ev);
      if (evt && evt.type === "next-episode-prompt") {
        bingeCancelled = false;
        setNextEpisode(evt.payload);
      }
    });

    es.addEventListener("episode-over", (ev) => {
      const evt = parse(ev);
      if (evt && evt.type === "episode-over" && !bingeCancelled) {
        setEpisodeOver(evt.payload);
      }
    });
```

**(c)** Add the two store functions next to `dismissNextEpisode` and export the new members:

```ts
  function dismissNextEpisode() { setNextEpisode(null); }
  function dismissTranscodePrompt() { setTranscodePrompt(null); }
  // Dismiss button: hide the card AND opt out of auto-advance this episode.
  function cancelBinge() {
    bingeCancelled = true;
    setNextEpisode(null);
  }
  function clearEpisodeOver() { setEpisodeOver(null); }

  return {
    state,
    nextEpisode,
    transcodePrompt,
    endedAt,
    episodeOver,
    connect,
    dismissNextEpisode,
    dismissTranscodePrompt,
    cancelBinge,
    clearEpisodeOver,
  };
```

- [ ] **Step 3: Typecheck**

Run: `cd web && npx tsc --noEmit`
Expected: no errors

- [ ] **Step 4: Commit**

```bash
git add web/src/api/types.ts web/src/state/playback.ts
git commit -m "feat(web): handle episode-over event with binge-cancel flag"
```

---

### Task 4: Frontend — countdown modal becomes passive "Up Next" card

**Files:**
- Modify: `web/src/components/Modal/NextEpisodeModal.tsx` (full rewrite)
- Modify: `web/src/components/Modal/NextEpisodeModal.css` (remove orphaned progress-bar rules)

**Interfaces:**
- Consumes: `playbackStore.episodeOver` / `clearEpisodeOver` / `cancelBinge` from Task 3; existing `api.playStop()` / `api.play(serverID, ratingKey)`.

- [ ] **Step 1: Rewrite the modal**

Replace the full contents of `web/src/components/Modal/NextEpisodeModal.tsx`:

```tsx
import { createEffect, Show } from "solid-js";
import ModalShell from "./ModalShell";
import { playbackStore } from "../../state/playback";
import { api } from "../../api/client";
import "./NextEpisodeModal.css";

export default function NextEpisodeModal() {
  const info = playbackStore.nextEpisode;

  async function play(target: { serverID: string; ratingKey: string }) {
    try {
      await api.playStop();
      await api.play(target.serverID, target.ratingKey);
    } catch (e) {
      console.error("play next failed:", e);
      alert(`Failed to play next episode: ${(e as Error).message}`);
    } finally {
      playbackStore.dismissNextEpisode();
    }
  }

  function playNow() {
    const i = info();
    if (i) play(i);
  }

  // Backend says the episode truly ended (natural EOF or PotPlayer closed
  // past the watched threshold): advance immediately. The store already
  // swallowed the event if the user Dismissed.
  createEffect(() => {
    const over = playbackStore.episodeOver();
    if (!over) return;
    playbackStore.clearEpisodeOver();
    play(over);
  });

  return (
    <ModalShell open={info() !== null} onCancel={playbackStore.cancelBinge} ariaLabel="Next episode">
      <h2 class="nem-title">Up Next</h2>
      <Show when={info()}>
        {(i) => (
          <div class="nem-card">
            <Show when={i().thumbPath}>
              <img class="nem-thumb" src={api.image(i().serverID, i().thumbPath!)} alt="" />
            </Show>
            <div class="nem-meta">
              <div class="nem-ep">S{i().season} · E{i().episode}</div>
              <div class="nem-name">{i().title}</div>
            </div>
          </div>
        )}
      </Show>
      <div class="nem-actions">
        <button class="nem-cancel" onClick={playbackStore.cancelBinge}>Dismiss</button>
        <button class="nem-now" onClick={playNow} autofocus>Play Now</button>
      </div>
    </ModalShell>
  );
}
```

What's gone versus the old file: `COUNTDOWN_MS`/`TICK_MS`, the `remaining` signal, the interval timer, the countdown title, the progress bar, `onCleanup`/`createSignal` imports. Escape/backdrop-close now routes to `cancelBinge` (closing the card = opting out, same as Dismiss).

- [ ] **Step 2: Remove the orphaned progress-bar CSS**

In `web/src/components/Modal/NextEpisodeModal.css`, delete these two rules (nothing else):

```css
.nem-progress { height: 3px; background: rgba(255,255,255,0.1); border-radius: 2px; overflow: hidden; }
.nem-progress-fill { height: 100%; background: var(--led-teal); transition: width 0.1s linear; }
```

- [ ] **Step 3: Typecheck**

Run: `cd web && npx tsc --noEmit`
Expected: no errors

- [ ] **Step 4: Commit**

```bash
git add web/src/components/Modal/NextEpisodeModal.tsx web/src/components/Modal/NextEpisodeModal.css
git commit -m "feat(web): replace next-episode countdown with passive Up Next card"
```

---

### Task 5: Full build for smoke test

Vite builds the SPA into `internal/server/web/dist` (embedded in the Go binary), then rebuild `lumen.exe`.

**Files:**
- Modify (generated): `internal/server/web/dist/*`
- Output: `lumen.exe` (repo root; note `lumen.exe~` backup file exists — leave it alone)

- [ ] **Step 1: Build the SPA**

Run: `cd web && npm run build`
Expected: vite build succeeds, output written to `../internal/server/web/dist`

- [ ] **Step 2: Build the binary**

Run (repo root): `go build -o lumen.exe .`
Expected: builds cleanly

- [ ] **Step 3: Run backend tests one final time**

Run: `go test ./...`
Expected: all PASS

- [ ] **Step 4: Commit the embedded dist**

```bash
git add internal/server/web/dist
git commit -m "build(web): embed next-episode autoplay redesign"
```

- [ ] **Step 5: Hand to Byron for smoke test**

Per the spec's testing section, the smoke checklist is:
1. Watch an episode to the very end → next episode auto-launches within ~5 s.
2. Close PotPlayer during credits (past 95%) → next episode auto-launches.
3. Dismiss the Up Next card, let the episode end → nothing launches.
4. Close PotPlayer mid-episode (< 95%) → nothing launches (existing stop behaviour).
5. Movie / last episode of a show → no card, no advance (existing `ended` path).
