# Next-Episode Autoplay Redesign — "Scrobble at 95%, advance at actual end"

**Date:** 2026-07-19
**Status:** Approved by Byron

## Problem

The current next-episode flow fires a 10-second countdown modal the moment the
poller sees 95% watched. On a 45-minute episode that's ~2+ minutes of content
still to go. PotPlayer is fullscreen/foregrounded, so the browser modal is
invisible — the countdown expires silently, kills the current episode
mid-cliffhanger, and launches the next one. Recovering means closing the new
instance, reopening the old episode, seeking back, and racing the modal's
10-second window with a Cancel click.

Root cause: one signal (95% watched) is doing two jobs — "mark as watched"
(correct at 95%) and "start the next episode" (wrong at 95%; correct only when
the episode actually ends).

## Design

Split the two concerns. 95% keeps scrobbling; auto-advance keys off the
episode *actually* ending.

### Backend — `internal/playback/poller.go`

1. **95% threshold — unchanged for scrobble.** Scrobble fires once as today.
   `fireEnded` still runs at 95% and still emits `next-episode-prompt`
   (`EventNextEpisode`) with the next-up info, but its meaning changes from
   "countdown started" to "next episode is queued".
2. **New: EOF detection, active only after the 95% threshold has been crossed
   on an episode with a known next episode.** Each 5-second poll checks for
   the episode truly ending, in either of two ways:
   - **Natural end:** `position >= duration - 2s`. PotPlayer (per Byron's
     config) parks paused on the last frame, so position pins at/near
     duration. State check is not required — position alone is sufficient and
     robust to the 5 s sample interval.
   - **Manual close:** `IsAlive()` returns false AND the last-observed
     position was past the 95% threshold. Detected in the existing liveness
     branch, before the `Stop()` teardown.
   Either path broadcasts a new event, `EventEpisodeOver`
   (`"episode-over"`), exactly once, then existing behaviour continues
   (poller keeps running on natural end until the player closes or the SPA
   stops/starts playback; manual-close path proceeds to `Stop()` as today).
3. **Unchanged:** movies and last-episode-of-show keep the existing
   `EventEnded` path. Manual close below 95% = "done watching", no event, no
   advance — existing behaviour.
4. `EventEpisodeOver` carries the same `NextEpisodeInfo` payload as
   `next-episode-prompt` so the SPA can act on it even if it missed the
   earlier prompt (e.g. page refreshed mid-episode).

### Frontend — `web/src/components/Modal/NextEpisodeModal.tsx`, `web/src/state/playback.ts`

5. **Countdown deleted entirely.** No timer, no progress bar, no auto-fire
   from the modal. On `next-episode-prompt` the modal renders as a passive
   "Up Next" card — same thumbnail/season/episode/title layout — with two
   buttons:
   - **Play Now** — immediate stop-and-play-next (existing `playNow` logic).
   - **Dismiss** — closes the card AND sets a per-episode "binge cancelled"
     flag in the playback store.
6. **New `episode-over` handler in the playback store.** When received:
   - If the dismiss flag is set → ignore (user opted out for this
     transition). Flag resets when the next playback session starts.
   - Otherwise → run the stop-and-play-next sequence immediately. No modal,
     no countdown — the episode is over or the player was closed, so there is
     nothing left to interrupt.

### Resulting UX

- Credits roll → watch to the last frame **or** close PotPlayer → next
  episode launches within ~5 s (one poll interval). Zero content lost, zero
  clicks in binge mode.
- Closing PotPlayer past 95% *is* the "next episode" gesture.
- Dismiss is a relaxed choice available any time in the final minutes, not a
  10-second quick-time event.
- Closing PotPlayer before 95% still just stops, as today.

## Edge Cases

- **Rewind after crossing 95%** (rewatching a scene): scrobble has already
  fired (unchanged). Natural-end detection only triggers when position
  reaches the actual end again. Manual-close advance gates on the
  *last-observed* position, so closing at 50% after a rewind does not
  advance. (`endedFired`/prompt having fired earlier is fine — the card just
  sits there.)
- **Pause during credits** (e.g. paused at 96%): position is not within 2 s
  of the end, so no false trigger.
- **SPA tab closed/asleep when `episode-over` fires:** same limitation as the
  current design (SPA orchestrates playback); event is lost and no advance
  happens. Accepted — Lumen's model already assumes the browser tab exists.
- **`playStop` on the manual-close path:** the session is already stopped by
  the time the SPA reacts; `playStop` is a no-op there. Harmless.

## Trade-offs

- Up to ~5 s of latency between the episode ending and the next one starting
  (poll interval). Accepted.
- Auto-advance without any visible countdown means a user who closes
  PotPlayer at 96% intending to stop for the night gets the next episode
  launched. Mitigation: Dismiss beforehand, or just close the new instance —
  it's one close, at episode start, with nothing lost (contrast with the old
  failure mode which destroyed an in-progress episode).

## Addendum (2026-07-19, post-smoke-test): EOF-during-playback stops PotPlayer

Live probe finding (250ms sampling of a real session): PotPlayer only parks
paused on the last frame when EOF is reached **while paused** (e.g. a
seek-to-end from a paused state). When EOF is reached **during active
playback** — a seek to 100% while playing, or natural play-out — PotPlayer
*stops*: raw state `0` (previously unmapped by our client) and position
resets to `0` within ~300ms. The original design's naturalEOF check misses
this reliably; natural play-out only ever worked when a 5s poll tick
happened to sample inside the final 2s window.

Fix: raw state `0` now maps to `PlayStateStopped`, and the poller fires
`episode-over` on `stoppedAdvance(state, prevPos, duration)` — Stopped state
with the *pre-reset* position past the 95% threshold. Stop below the
threshold (including rewind-then-stop) still means "done watching".

Known residual limitation: seeking from below 95% directly to 100% while
playing does not advance — after PotPlayer's stop-and-reset it is
indistinguishable from a mid-episode Stop, and the transient
position-at-duration sample (~265ms) cannot be reliably caught at a 5s poll
interval. Accepted: the realistic gestures (credits skip past 95%, natural
play-out, manual close past 95%, paused seek-to-end) are all covered.

## Testing

- **Backend:** unit tests around the poller's EOF branch — natural-end
  trigger at `duration - 2s`, manual-close trigger past threshold,
  no-trigger below threshold, single-fire semantics, movie/last-episode
  unaffected. Follow the existing poller test patterns.
- **Frontend:** modal renders without timer; Dismiss sets the flag;
  `episode-over` honours/ignores the flag correctly.
- **Smoke test (Byron):** watch an episode to the end (natural advance);
  close PotPlayer during credits (gesture advance); Dismiss then let it end
  (no advance); close mid-episode (no advance).
