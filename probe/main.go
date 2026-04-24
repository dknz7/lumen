// Lumen Session 0 spike — Pot Player Win32 IPC probe.
// Throwaway code. Not imported by production packages.
package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"os/exec"
	"syscall"
	"time"
	"unsafe"

	"golang.org/x/sys/windows"
	"golang.org/x/sys/windows/registry"
)

var (
	stepFlag      = flag.Int("step", 0, "which probe step to run (1..8)")
	videoFlag     = flag.String("video", "", "absolute path to test video file")
	potPlayerFlag = flag.String("potplayer", "", "absolute path to PotPlayerMini64.exe (overrides registry)")
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
		fmt.Fprintln(os.Stderr, "usage: probe.exe --step=<2..9> [--video=<path>] [--potplayer=<exe path>]")
		os.Exit(2)
	}
}

func step2DetectPath() {
	path, source := resolvePotPlayerPath()
	info, err := os.Stat(path)
	if err != nil {
		log.Fatalf("stat %q: %v", path, err)
	}
	log.Printf("Pot Player path: %s (size=%d, source=%s)", path, info.Size(), source)
}
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
	p, _ := resolvePotPlayerPath()
	return p
}

// resolvePotPlayerPath returns the exe path and a label for where it came from.
// Preference: --potplayer flag → HKCU\Software\DAUM\PotPlayerMini64\ProgramPath.
// Session 0 observation: not every install writes ProgramPath to the registry,
// so the flag is the reliable path and the registry is best-effort.
func resolvePotPlayerPath() (string, string) {
	if *potPlayerFlag != "" {
		return *potPlayerFlag, "flag"
	}
	k, err := registry.OpenKey(registry.CURRENT_USER, `Software\DAUM\PotPlayerMini64`, registry.QUERY_VALUE)
	if err != nil {
		log.Fatalf("Pot Player path not found. Pass --potplayer=<full path to PotPlayerMini64.exe>. (registry open failed: %v)", err)
	}
	defer k.Close()
	p, _, err := k.GetStringValue("ProgramPath")
	if err != nil {
		log.Fatalf("Pot Player path not found. Pass --potplayer=<full path to PotPlayerMini64.exe>. (ProgramPath value missing: %v)", err)
	}
	return p, "registry"
}
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
var (
	user32         = windows.NewLazySystemDLL("user32.dll")
	procFindWindow = user32.NewProc("FindWindowW")
	procIsWindow   = user32.NewProc("IsWindow")
	procSendMsgW   = user32.NewProc("SendMessageW")
	procPostMsgW   = user32.NewProc("PostMessageW")
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

const (
	WM_USER = 0x0400

	// Pot Player query codes (from rasvob/PotPlayerRemoteAPI).
	// Session 0 job: confirm these against v260422.
	PP_GET_POSITION = 0x5004 // expected: position in seconds (some sources say ms — probe both)
	PP_GET_DURATION = 0x5002
	PP_GET_STATE    = 0x5006 // expected: 0=stopped, 1=paused, 2=playing
)

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
const (
	WM_COMMAND = 0x0111

	// Pot Player menu command IDs (from rasvob repo).
	PP_CMD_PAUSE_TOGGLE = 0x4E5E // 20062
	PP_CMD_STOP         = 0x4E67 // 20071
)

const (
	WM_APPCOMMAND              = 0x0319
	APPCOMMAND_MEDIA_STOP      = 13
	APPCOMMAND_MEDIA_PLAY_PAUSE = 14
)

func step8SendCommand() {
	if *videoFlag == "" {
		log.Fatal("--video=<path> required")
	}
	cmd := exec.Command(potPlayerPath(), *videoFlag)
	if err := cmd.Start(); err != nil {
		log.Fatalf("launch: %v", err)
	}
	hwnd := findPotPlayerHWND()
	log.Printf("HWND=0x%x — trying 4 write-side approaches. WATCH THE VIDEO for visible pause/resume/stop.", hwnd)
	time.Sleep(3 * time.Second) // let playback start

	// Approach A: original — SendMessage + WM_COMMAND + PAUSE_TOGGLE (0x4E5E=20062)
	log.Printf("approach A: SendMessage + WM_COMMAND + 0x4E5E (pause toggle)")
	retA, _, _ := procSendMsgW.Call(hwnd, WM_COMMAND, uintptr(PP_CMD_PAUSE_TOGGLE), 0)
	log.Printf("  return=%d  observe for 3 s", retA)
	time.Sleep(3 * time.Second)

	// Approach B: PostMessage + WM_COMMAND + same ID
	log.Printf("approach B: PostMessage + WM_COMMAND + 0x4E5E (pause toggle)")
	retB, _, _ := procPostMsgW.Call(hwnd, WM_COMMAND, uintptr(PP_CMD_PAUSE_TOGGLE), 0)
	log.Printf("  return=%d  observe for 3 s", retB)
	time.Sleep(3 * time.Second)

	// Approach C: SendMessage + WM_APPCOMMAND + MEDIA_PLAY_PAUSE.
	// lParam encodes the app command in the high word.
	log.Printf("approach C: SendMessage + WM_APPCOMMAND + MEDIA_PLAY_PAUSE (14)")
	lparamPP := uintptr(APPCOMMAND_MEDIA_PLAY_PAUSE) << 16
	retC, _, _ := procSendMsgW.Call(hwnd, WM_APPCOMMAND, 0, lparamPP)
	log.Printf("  return=%d  observe for 3 s", retC)
	time.Sleep(3 * time.Second)

	// Approach D: SendMessage + WM_APPCOMMAND + MEDIA_STOP.
	log.Printf("approach D: SendMessage + WM_APPCOMMAND + MEDIA_STOP (13)")
	lparamStop := uintptr(APPCOMMAND_MEDIA_STOP) << 16
	retD, _, _ := procSendMsgW.Call(hwnd, WM_APPCOMMAND, 0, lparamStop)
	log.Printf("  return=%d", retD)

	log.Println("done — tell Archie which approaches (A/B/C/D) caused visible action")
	log.Println("close Pot Player manually")
}
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
