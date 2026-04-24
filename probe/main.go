// Lumen Session 0 spike — Pot Player Win32 IPC probe.
// Throwaway code. Not imported by production packages.
package main

import (
	"flag"
	"fmt"
	"log"
	"os"

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
func step3Launch()       { log.Fatal("not implemented") }
func step4FindHWND()     { log.Fatal("not implemented") }
func step5ReadPosition() { log.Fatal("not implemented") }
func step6ReadDuration() { log.Fatal("not implemented") }
func step7ReadState()    { log.Fatal("not implemented") }
func step8SendCommand()  { log.Fatal("not implemented") }
func step9DetectExit()   { log.Fatal("not implemented") }
