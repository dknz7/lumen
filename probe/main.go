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
func step9DetectExit()   { log.Fatal("not implemented") }
