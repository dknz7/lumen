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

func step5ReadPosition() { log.Fatal("not implemented") }
func step6ReadDuration() { log.Fatal("not implemented") }
func step7ReadState()    { log.Fatal("not implemented") }
func step8SendCommand()  { log.Fatal("not implemented") }
func step9DetectExit()   { log.Fatal("not implemented") }
