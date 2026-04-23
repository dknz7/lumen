# Lumen — Session 0 Implementation Plan (Pot Player Control Spike)

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Prove that Go can reliably drive Pot Player Mini 64-bit v260422 via Win32 IPC (`SendMessageW` + `WM_COMMAND` + `WM_USER` message offsets) before any Lumen production code is written. Decide go/no-go for the rest of the project.

**Architecture:** A throwaway Go program under `probe/` with a `--step=<n>` flag that isolates each capability (launch → find HWND → read position → read duration → read state → send command → detect exit). Each step runs independently so Byron can observe Pot Player's behaviour in real time and log the result. No production code, no unit tests — the spike's "test" is manual observation against a real running Pot Player window. Output is a single findings document.

**Tech Stack:** Go 1.26.2 (already installed), `golang.org/x/sys/windows` for Win32 calls, `golang.org/x/sys/windows/registry` for Pot Player path lookup, Pot Player Mini 64-bit v260422 (Byron's installed version).

**Reference:** Command IDs from [`rasvob/PotPlayerRemoteAPI`](https://github.com/rasvob/PotPlayerRemoteAPI). Starting values used below are the widely-documented ones — Session 0's job is to confirm them on v260422 or adjust.

**Session 0 exit criteria:** `docs/session-0-findings.md` exists and answers yes/no for each of: position-read, duration-read, state-read, clean-exit-detect, dirty-exit-detect. **Go decision** requires position-read AND exit-detect both working. Fail on either → halt and discuss pivoting the player choice.

---

## Pre-flight

**Byron provides:**

- A small local test video file (MP4, 30 s – 2 min). Path noted below as `<TEST_VIDEO_PATH>` — the executing agent substitutes Byron's actual path at run time.
- Pot Player Mini 64-bit v260422 installed (default path: `C:\Program Files\DAUM\PotPlayerMini64\PotPlayerMini64.exe`).

**Working directory:** `C:\Users\dicke\Desktop\Dump Zone\STACK\04-DEV\lumen`

---

## File Structure

```
lumen/
├── probe/
│   ├── go.mod
│   ├── go.sum
│   └── main.go              # Single-file spike, subcommand via --step flag
└── docs/
    └── session-0-findings.md  # Final deliverable
```

`probe/` is throwaway. After Session 0 signs off, it stays in the repo as historical reference but is not imported by any production package.

---

## Task 1: Scaffold probe module

**Files:**
- Create: `probe/go.mod`
- Create: `probe/main.go`

- [ ] **Step 1: Initialise the module**

Run (from repo root):
```bash
mkdir -p probe && cd probe && go mod init lumen/probe
```
Expected: `go: creating new go.mod: module lumen/probe`.

- [ ] **Step 2: Add the Windows syscall dependency**

Run (from `probe/`):
```bash
go get golang.org/x/sys/windows
go get golang.org/x/sys/windows/registry
```
Expected: both commands succeed, `go.sum` populated.

- [ ] **Step 3: Write the main.go skeleton**

Create `probe/main.go`:
```go
// Lumen Session 0 spike — Pot Player Win32 IPC probe.
// Throwaway code. Not imported by production packages.
package main

import (
	"flag"
	"fmt"
	"log"
	"os"
)

var (
	stepFlag  = flag.Int("step", 0, "which probe step to run (1..8)")
	videoFlag = flag.String("video", "", "absolute path to test video file")
)

func main() {
	flag.Parse()
	log.SetFlags(log.Ltime | log.Lmicroseconds)

	switch *stepFlag {
	case 2:
		step2DetectPath()
	case 3:
		step3Launch()
	case 4:
		step4FindHWND()
	case 5:
		step5ReadPosition()
	case 6:
		step6ReadDuration()
	case 7:
		step7ReadState()
	case 8:
		step8SendCommand()
	case 9:
		step9DetectExit()
	default:
		fmt.Fprintln(os.Stderr, "usage: probe.exe --step=<2..9> [--video=<path>]")
		os.Exit(2)
	}
}

// Placeholder functions — populated task-by-task.
func step2DetectPath()   { log.Fatal("not implemented") }
func step3Launch()       { log.Fatal("not implemented") }
func step4FindHWND()     { log.Fatal("not implemented") }
func step5ReadPosition() { log.Fatal("not implemented") }
func step6ReadDuration() { log.Fatal("not implemented") }
func step7ReadState()    { log.Fatal("not implemented") }
func step8SendCommand()  { log.Fatal("not implemented") }
func step9DetectExit()   { log.Fatal("not implemented") }
```

- [ ] **Step 4: Verify it builds**

Run (from `probe/`):
```bash
go build -o probe.exe .
./probe.exe --step=1
```
Expected: build succeeds; running with `--step=1` prints the usage line and exits 2 (since only 2..9 are wired).

- [ ] **Step 5: Commit**

```bash
cd ..
git add probe/go.mod probe/go.sum probe/main.go
git commit -m "probe(session-0): scaffold Pot Player IPC spike module"
```

---

## Task 2: Detect Pot Player install path from registry

**Files:**
- Modify: `probe/main.go` (replace `step2DetectPath`)

**Context:** Spec §7.2 says install path comes from `HKCU\Software\DAUM\PotPlayerMini64\ProgramPath`. Confirm the registry key exists and returns a valid path.

- [ ] **Step 1: Implement the registry read**

Replace the placeholder `step2DetectPath` in `probe/main.go`:
```go
import (
	// ... existing imports ...
	"golang.org/x/sys/windows/registry"
)

func step2DetectPath() {
	k, err := registry.OpenKey(registry.CURRENT_USER, `Software\DAUM\PotPlayerMini64`, registry.QUERY_VALUE)
	if err != nil {
		log.Fatalf("open key: %v", err)
	}
	defer k.Close()

	path, _, err := k.GetStringValue("ProgramPath")
	if err != nil {
		log.Fatalf("get ProgramPath: %v", err)
	}

	info, err := os.Stat(path)
	if err != nil {
		log.Fatalf("stat %q: %v", path, err)
	}
	log.Printf("Pot Player path: %s (size=%d)", path, info.Size())
}
```

- [ ] **Step 2: Build and run**

```bash
cd probe && go build -o probe.exe . && ./probe.exe --step=2
```
Expected output like:
```
12:34:56.123456 Pot Player path: C:\Program Files\DAUM\PotPlayerMini64\PotPlayerMini64.exe (size=4123456)
```

If the key is missing, the probe dies with `open key: The system cannot find the file specified.` — resolve by opening Pot Player once to force the key to be created, or note it in findings.

- [ ] **Step 3: Commit**

```bash
cd ..
git add probe/main.go
git commit -m "probe(session-0): detect Pot Player install path from HKCU"
```

---

## Task 3: Launch Pot Player with a test video

**Files:**
- Modify: `probe/main.go` (replace `step3Launch`)

- [ ] **Step 1: Implement the launcher**

Add to imports if missing: `"os/exec"`. Replace `step3Launch`:
```go
func step3Launch() {
	if *videoFlag == "" {
		log.Fatal("--video=<path> required")
	}
	path := potPlayerPath()
	cmd := exec.Command(path, *videoFlag)
	if err := cmd.Start(); err != nil {
		log.Fatalf("start Pot Player: %v", err)
	}
	log.Printf("launched Pot Player pid=%d with %s", cmd.Process.Pid, *videoFlag)
}

// Shared helper — also used by later steps.
func potPlayerPath() string {
	k, err := registry.OpenKey(registry.CURRENT_USER, `Software\DAUM\PotPlayerMini64`, registry.QUERY_VALUE)
	if err != nil {
		log.Fatalf("open key: %v", err)
	}
	defer k.Close()
	p, _, err := k.GetStringValue("ProgramPath")
	if err != nil {
		log.Fatalf("get ProgramPath: %v", err)
	}
	return p
}
```

- [ ] **Step 2: Build and run**

```bash
cd probe && go build -o probe.exe . && ./probe.exe --step=3 --video="<TEST_VIDEO_PATH>"
```
Expected: Pot Player window opens playing the test video; console prints `launched Pot Player pid=<N> with <path>`.

Close Pot Player manually before moving on.

- [ ] **Step 3: Commit**

```bash
cd ..
git add probe/main.go
git commit -m "probe(session-0): launch Pot Player subprocess with test video"
```

---

## Task 4: Find HWND via FindWindowW on class `PotPlayer64`

**Files:**
- Modify: `probe/main.go` (replace `step4FindHWND`, add win32 helpers)

**Context:** Spec §7.2 pins the window class name as `PotPlayer64`. We need a handle before any `SendMessage` calls.

- [ ] **Step 1: Add Win32 plumbing**

Add to imports: `"syscall"`, `"time"`, `"unsafe"`, `"golang.org/x/sys/windows"`.

Add after the `main` func:
```go
var (
	user32         = windows.NewLazySystemDLL("user32.dll")
	procFindWindow = user32.NewProc("FindWindowW")
	procIsWindow   = user32.NewProc("IsWindow")
	procSendMsgW   = user32.NewProc("SendMessageW")
)

// findPotPlayerHWND polls for up to 3 s waiting for the window to appear.
func findPotPlayerHWND() uintptr {
	class, _ := syscall.UTF16PtrFromString("PotPlayer64")
	deadline := time.Now().Add(3 * time.Second)
	for {
		hwnd, _, _ := procFindWindow.Call(uintptr(unsafe.Pointer(class)), 0)
		if hwnd != 0 {
			return hwnd
		}
		if time.Now().After(deadline) {
			log.Fatal("timed out waiting for PotPlayer64 window")
		}
		time.Sleep(100 * time.Millisecond)
	}
}
```

- [ ] **Step 2: Implement step4FindHWND**

Replace the placeholder:
```go
func step4FindHWND() {
	if *videoFlag == "" {
		log.Fatal("--video=<path> required")
	}
	cmd := exec.Command(potPlayerPath(), *videoFlag)
	if err := cmd.Start(); err != nil {
		log.Fatalf("launch: %v", err)
	}
	hwnd := findPotPlayerHWND()
	log.Printf("HWND=0x%x (pid=%d)", hwnd, cmd.Process.Pid)
}
```

- [ ] **Step 3: Build and run**

```bash
cd probe && go build -o probe.exe . && ./probe.exe --step=4 --video="<TEST_VIDEO_PATH>"
```
Expected: Pot Player window opens, console prints `HWND=0x<hex> (pid=<N>)` within 3 s. Close Pot Player manually.

- [ ] **Step 4: Commit**

```bash
cd ..
git add probe/main.go
git commit -m "probe(session-0): locate Pot Player HWND via FindWindowW"
```

---

## Task 5: Read playback position via SendMessage

**Files:**
- Modify: `probe/main.go` (replace `step5ReadPosition`)

**Context:** Pot Player exposes queries via `SendMessageW(hwnd, WM_USER=0x400, wParam, lParam)` where `wParam` is the query code. The widely documented query code for current position (ms) is `0x5004`. **This is the first capability that must work — position-read is a go/no-go gate per §3 Session 0 exit criteria.**

- [ ] **Step 1: Add Win32 message constants**

Append to the constants section at the top of `main.go`:
```go
const (
	WM_USER = 0x0400

	// Pot Player query codes (from rasvob/PotPlayerRemoteAPI).
	// Session 0 job: confirm these against v260422.
	PP_GET_POSITION = 0x5004 // expected: position in seconds (some sources say ms — probe both)
	PP_GET_DURATION = 0x5002
	PP_GET_STATE    = 0x5006 // expected: 0=stopped, 1=paused, 2=playing
)
```

- [ ] **Step 2: Implement step5ReadPosition**

Replace the placeholder:
```go
func step5ReadPosition() {
	if *videoFlag == "" {
		log.Fatal("--video=<path> required")
	}
	cmd := exec.Command(potPlayerPath(), *videoFlag)
	if err := cmd.Start(); err != nil {
		log.Fatalf("launch: %v", err)
	}
	hwnd := findPotPlayerHWND()
	log.Printf("HWND=0x%x — polling position every 1 s for 30 s", hwnd)

	start := time.Now()
	for i := 0; i < 30; i++ {
		ret, _, _ := procSendMsgW.Call(hwnd, WM_USER, uintptr(PP_GET_POSITION), 0)
		log.Printf("t+%4.1fs  raw=%d  (if seconds: %ds / if ms: %dms)",
			time.Since(start).Seconds(), ret, ret, ret)
		time.Sleep(1 * time.Second)
	}
}
```

- [ ] **Step 3: Build and run**

```bash
cd probe && go build -o probe.exe . && ./probe.exe --step=5 --video="<TEST_VIDEO_PATH>"
```

**Manual observation — this is the go/no-go gate:**

- Let the video play freely (don't pause).
- Watch the printed `raw=` values across 30 s.
- **Pass criteria:** the raw value increases monotonically and matches wall-clock playback. If values look like seconds (e.g. raw=0, 1, 2, 3, …), note that. If they look like ms (e.g. raw=0, 1000, 2000, …), note that.
- **Fail criteria:** raw is 0 every time, or stays constant, or returns garbage. If this fails, **halt the spike** and record the failure in findings before proceeding.

Close Pot Player after the probe exits.

- [ ] **Step 4: Commit**

```bash
cd ..
git add probe/main.go
git commit -m "probe(session-0): read playback position via WM_USER 0x5004"
```

---

## Task 6: Read media duration via SendMessage

**Files:**
- Modify: `probe/main.go` (replace `step6ReadDuration`)

- [ ] **Step 1: Implement step6ReadDuration**

Replace the placeholder:
```go
func step6ReadDuration() {
	if *videoFlag == "" {
		log.Fatal("--video=<path> required")
	}
	cmd := exec.Command(potPlayerPath(), *videoFlag)
	if err := cmd.Start(); err != nil {
		log.Fatalf("launch: %v", err)
	}
	hwnd := findPotPlayerHWND()

	// Duration may not be available immediately — poll up to 10 s.
	deadline := time.Now().Add(10 * time.Second)
	for {
		ret, _, _ := procSendMsgW.Call(hwnd, WM_USER, uintptr(PP_GET_DURATION), 0)
		log.Printf("duration raw=%d", ret)
		if ret > 0 {
			log.Printf("DURATION OK (if seconds: %ds / if ms: %dms)", ret, ret)
			return
		}
		if time.Now().After(deadline) {
			log.Fatal("duration never became non-zero within 10 s — FAIL")
		}
		time.Sleep(500 * time.Millisecond)
	}
}
```

- [ ] **Step 2: Build and run**

```bash
cd probe && go build -o probe.exe . && ./probe.exe --step=6 --video="<TEST_VIDEO_PATH>"
```

**Manual observation:**
- Value should stabilise to a constant within 10 s (video's total length).
- Compare against the file's actual duration (e.g. right-click the test file → Properties → Details).
- Record: did it report in seconds or milliseconds? Did it match?

Close Pot Player after the probe exits.

- [ ] **Step 3: Commit**

```bash
cd ..
git add probe/main.go
git commit -m "probe(session-0): read media duration via WM_USER 0x5002"
```

---

## Task 7: Read play state via SendMessage

**Files:**
- Modify: `probe/main.go` (replace `step7ReadState`)

- [ ] **Step 1: Implement step7ReadState**

Replace the placeholder:
```go
func step7ReadState() {
	if *videoFlag == "" {
		log.Fatal("--video=<path> required")
	}
	cmd := exec.Command(potPlayerPath(), *videoFlag)
	if err := cmd.Start(); err != nil {
		log.Fatalf("launch: %v", err)
	}
	hwnd := findPotPlayerHWND()
	log.Printf("HWND=0x%x — polling state every 1 s for 20 s. Pause/resume manually.", hwnd)

	for i := 0; i < 20; i++ {
		ret, _, _ := procSendMsgW.Call(hwnd, WM_USER, uintptr(PP_GET_STATE), 0)
		label := "UNKNOWN"
		switch ret {
		case 0:
			label = "STOPPED"
		case 1:
			label = "PAUSED"
		case 2:
			label = "PLAYING"
		}
		log.Printf("state raw=%d (%s)", ret, label)
		time.Sleep(1 * time.Second)
	}
}
```

- [ ] **Step 2: Build and run**

```bash
cd probe && go build -o probe.exe . && ./probe.exe --step=7 --video="<TEST_VIDEO_PATH>"
```

**Manual observation:**
- While the probe polls, click Pot Player's pause/play button a few times (or press Space).
- **Pass criteria:** raw values visibly change between 1 and 2 in step with your clicks. State 2 while playing, state 1 while paused.
- If raw returns random values or never changes, record the mismatch in findings. State-read is **not** a hard gate — it's a nice-to-have for the Now Playing strip.

Close Pot Player after the probe exits.

- [ ] **Step 3: Commit**

```bash
cd ..
git add probe/main.go
git commit -m "probe(session-0): read play state via WM_USER 0x5006"
```

---

## Task 8: Send a WM_COMMAND control (pause toggle) and observe

**Files:**
- Modify: `probe/main.go` (replace `step8SendCommand`)

**Context:** Write-side commands (pause, resume, stop) go via `WM_COMMAND` (0x0111) with a menu command ID in wParam. Pot Player's pause/play toggle is widely documented as `0x4E5E` (20062). Stop is `0x4E67` (20071). We only need to prove ONE write works — pause toggle is the simplest observable.

- [ ] **Step 1: Add WM_COMMAND constants**

Append to constants:
```go
const (
	WM_COMMAND = 0x0111

	// Pot Player menu command IDs (from rasvob repo).
	PP_CMD_PAUSE_TOGGLE = 0x4E5E // 20062
	PP_CMD_STOP         = 0x4E67 // 20071
)
```

- [ ] **Step 2: Implement step8SendCommand**

Replace the placeholder:
```go
func step8SendCommand() {
	if *videoFlag == "" {
		log.Fatal("--video=<path> required")
	}
	cmd := exec.Command(potPlayerPath(), *videoFlag)
	if err := cmd.Start(); err != nil {
		log.Fatalf("launch: %v", err)
	}
	hwnd := findPotPlayerHWND()
	log.Printf("HWND=0x%x — will send pause toggle 4 times every 3 s.", hwnd)

	time.Sleep(2 * time.Second) // let playback start
	for i := 0; i < 4; i++ {
		time.Sleep(3 * time.Second)
		log.Printf("sending PAUSE_TOGGLE (%d/4)", i+1)
		procSendMsgW.Call(hwnd, WM_COMMAND, uintptr(PP_CMD_PAUSE_TOGGLE), 0)
	}
	log.Println("done — closing in 3 s via STOP command")
	time.Sleep(3 * time.Second)
	procSendMsgW.Call(hwnd, WM_COMMAND, uintptr(PP_CMD_STOP), 0)
}
```

- [ ] **Step 3: Build and run**

```bash
cd probe && go build -o probe.exe . && ./probe.exe --step=8 --video="<TEST_VIDEO_PATH>"
```

**Manual observation:**
- You should see the video visibly pause → resume → pause → resume (4 toggles, 3 s apart).
- Then it should stop.
- If toggles don't visibly take effect, the command ID for v260422 differs from the rasvob reference. Record and cross-check with the repo's version tag.

- [ ] **Step 4: Commit**

```bash
cd ..
git add probe/main.go
git commit -m "probe(session-0): send pause/stop commands via WM_COMMAND"
```

---

## Task 9: Detect Pot Player exit (clean and dirty)

**Files:**
- Modify: `probe/main.go` (replace `step9DetectExit`)

**Context:** Spec §9.3 — when Pot Player exits (user closes window OR Lumen sends stop), the session manager needs to notice within the 5 s poll cycle. `IsWindow(hwnd)` returns 0 once the window is destroyed. This is the second hard gate.

- [ ] **Step 1: Implement step9DetectExit**

Replace the placeholder:
```go
func step9DetectExit() {
	if *videoFlag == "" {
		log.Fatal("--video=<path> required")
	}
	cmd := exec.Command(potPlayerPath(), *videoFlag)
	if err := cmd.Start(); err != nil {
		log.Fatalf("launch: %v", err)
	}
	hwnd := findPotPlayerHWND()
	log.Printf("HWND=0x%x — polling IsWindow every 1 s. Close Pot Player manually (X button OR Task Manager kill).", hwnd)

	start := time.Now()
	for {
		alive, _, _ := procIsWindow.Call(hwnd)
		log.Printf("t+%4.1fs  IsWindow=%d", time.Since(start).Seconds(), alive)
		if alive == 0 {
			log.Println("EXIT DETECTED")
			return
		}
		if time.Since(start) > 60*time.Second {
			log.Fatal("still alive after 60 s — please close Pot Player manually")
		}
		time.Sleep(1 * time.Second)
	}
}
```

- [ ] **Step 2: Build and run — clean exit test**

```bash
cd probe && go build -o probe.exe . && ./probe.exe --step=9 --video="<TEST_VIDEO_PATH>"
```

After a few polls, click Pot Player's X button normally.
**Pass criteria:** within ~1 s, IsWindow returns 0 and probe prints `EXIT DETECTED`.

- [ ] **Step 3: Run again — dirty exit test**

Re-run the same command. This time, kill Pot Player via Task Manager (End Task on `PotPlayerMini64.exe`).
**Pass criteria:** within ~1 s, probe prints `EXIT DETECTED`.

Record both results for findings.

- [ ] **Step 4: Commit**

```bash
cd ..
git add probe/main.go
git commit -m "probe(session-0): detect Pot Player exit via IsWindow"
```

---

## Task 10: Write the findings document

**Files:**
- Create: `docs/session-0-findings.md`

**Context:** This document is the actual Session 0 deliverable. Sessions 1+ do not begin until the go/no-go verdict is written here.

- [ ] **Step 1: Draft the findings document**

Create `docs/session-0-findings.md` using this exact template — fill every `<…>` placeholder from Tasks 2–9 observations:

```markdown
# Session 0 — Pot Player IPC Findings

**Date:** <YYYY-MM-DD of the probe run>
**Pot Player version:** Mini 64-bit v260422 (1.7.22859)
**Test video:** <path + duration>
**Probe source:** `probe/main.go` at commit <short-sha>

## Results

| Capability | Probe step | Result | Notes |
|---|---|---|---|
| Registry path lookup | 2 | <PASS / FAIL> | <path returned, or error> |
| Launch subprocess | 3 | <PASS / FAIL> | <pid, latency until window> |
| FindWindowW on `PotPlayer64` | 4 | <PASS / FAIL> | <HWND, time to appear> |
| Position read (WM_USER+0x5004) | 5 | <PASS / FAIL> | Units: <seconds / milliseconds / other>. Matched wall clock: <yes / no> |
| Duration read (WM_USER+0x5002) | 6 | <PASS / FAIL> | Units: <seconds / milliseconds>. Matched file: <yes / no, off by X> |
| State read (WM_USER+0x5006) | 7 | <PASS / FAIL> | Values observed: <0 / 1 / 2 / other>. Responded to pause: <yes / no> |
| WM_COMMAND pause toggle (0x4E5E) | 8 | <PASS / FAIL> | <visible effect yes/no> |
| WM_COMMAND stop (0x4E67) | 8 | <PASS / FAIL> | <visible effect yes/no> |
| Clean exit detect (IsWindow) | 9 | <PASS / FAIL> | Detection latency: <seconds> |
| Dirty exit detect (Task Manager kill) | 9 | <PASS / FAIL> | Detection latency: <seconds> |

## Hard-gate capabilities (per spec §3)

- [ ] Position read works reliably
- [ ] Exit detect works reliably (clean AND dirty)

## Go / No-Go decision

**<GO / NO-GO>** — <one-line justification>

## Unit / encoding quirks to pin into `internal/potplayer/commands.go`

<Bullet list of anything surprising: units, off-by-one, sentinel values for "not ready", retries needed, etc.>

## Command IDs confirmed for v260422

| Name | Value | Confirmed? |
|---|---|---|
| WM_USER base | 0x0400 | <y / n> |
| PP_GET_POSITION | 0x5004 | <y / n — or corrected value> |
| PP_GET_DURATION | 0x5002 | <y / n — or corrected value> |
| PP_GET_STATE | 0x5006 | <y / n — or corrected value> |
| PP_CMD_PAUSE_TOGGLE | 0x4E5E | <y / n — or corrected value> |
| PP_CMD_STOP | 0x4E67 | <y / n — or corrected value> |

## Follow-ups for Session 4

<Bullet list of anything that needs to be handled when the production `internal/potplayer` package is built.>
```

- [ ] **Step 2: Fill in the template from observed results**

Transcribe the actual observations from each step's run log. Be specific — "position value is in seconds, not milliseconds" is useful; "worked" is not.

- [ ] **Step 3: Commit the findings**

```bash
git add docs/session-0-findings.md
git commit -m "docs(session-0): record Pot Player IPC probe findings"
```

- [ ] **Step 4: Announce the verdict**

Post the "Go / No-Go decision" line back to Byron in the session summary. If NO-GO, halt — Session 1 is blocked pending a player-pivot discussion.

---

## Self-review checklist (for the executing agent)

Before calling Session 0 done, confirm:

- [ ] All 9 probe steps ran against a live Pot Player v260422 window (not simulated, not mocked).
- [ ] Every row of the findings table has a PASS or FAIL — no blanks, no "TBD".
- [ ] Units for position and duration are explicitly recorded (seconds vs milliseconds matters for Session 4).
- [ ] Both hard-gate checkboxes are ticked before declaring GO.
- [ ] The `probe/` directory is left in the repo (not deleted) — it's historical reference for Session 4.
