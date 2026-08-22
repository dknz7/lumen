package main

import (
	"fmt"
	"log"
	"net"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/pkg/browser"

	"lumen/assets"
	"lumen/internal/config"
	"lumen/internal/plex"
	"lumen/internal/server"
	"lumen/internal/shell"
)

const defaultAddr = "127.0.0.1:7832"

type appOptions struct {
	// StartHidden boots straight to the tray with no window.
	StartHidden bool
	// Browser skips the native window and opens the default browser instead.
	Browser bool
	// Addr overrides the listen address.
	Addr string
}

// runServe parses the `serve` subcommand's flags. It exists so shortcuts made
// by older versions (`lumen.exe serve`) keep working; it now opens the native
// window by default, with --browser for the previous behaviour.
func runServe(args []string) {
	opts := appOptions{}
	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--browser", "-browser":
			opts.Browser = true
		case "--tray", "-tray":
			opts.StartHidden = true
		case "--addr", "-addr":
			if i+1 >= len(args) {
				fatal("--addr needs a value, e.g. --addr 127.0.0.1:7832")
			}
			opts.Addr = args[i+1]
			i++
		default:
			if strings.HasPrefix(args[i], "--addr=") {
				opts.Addr = strings.TrimPrefix(args[i], "--addr=")
				continue
			}
			fatal(fmt.Sprintf("unknown flag for `serve`: %s", args[i]))
		}
	}
	runApp(opts)
}

func runApp(opts appOptions) {
	if opts.Addr == "" {
		opts.Addr = defaultAddr
	}

	// One Lumen at a time. A second launch surfaces the window that already
	// exists rather than failing to bind the port with no explanation.
	if already, err := shell.AcquireSingleInstance(); err == nil && already {
		signalExistingInstance(opts.Addr)
		return
	}

	cfg, err := config.Load()
	if err != nil {
		fatal(fmt.Sprintf("Could not read your Lumen settings.\n\n%v\n\n%s",
			err, config.ConfigFile()))
	}
	setupLogging()

	// Note: deliberately NOT gated on having a token or servers. A fresh
	// install has neither, and the SPA drives the Plex link flow itself via
	// /api/auth/start. Exiting here would leave a new user with no way in.
	server.SetBuildInfo(version, commit, buildDate)
	c := plex.NewClient(cfg.ClientIdentifier, version)
	srv := server.New(cfg, c, opts.Addr)

	ln, err := net.Listen("tcp", opts.Addr)
	if err != nil {
		fatal(fmt.Sprintf("Could not start Lumen on %s.\n\n%v\n\n"+
			"Another program may be using that port. Try:  lumen serve --addr 127.0.0.1:7833",
			opts.Addr, err))
	}

	errCh := make(chan error, 1)
	go func() { errCh <- srv.Serve(ln) }()

	url := "http://" + opts.Addr

	if opts.Browser {
		runBrowserMode(srv, url, errCh)
		return
	}

	// Let the SPA and any second launch drive the window.
	srv.SetWindowController(shell.Show, shell.Hide)

	// The SPA's "Close Lumen" button posts /api/quit, which fires this
	// channel. Only runBrowserMode ever consumed it, so in the desktop build
	// the endpoint answered "shutting down" and then nothing happened at all —
	// the button looked wired and wasn't.
	//
	// Guarded on shell.Running() because the WebView2 fallback below hands off
	// to runBrowserMode, which selects on this same channel. Without the
	// guard this goroutine would win the race, browser mode would wait on a
	// signal that had already been consumed, and closing from the SPA would
	// hang instead of exiting.
	go func() {
		<-srv.Quit()
		if shell.Running() {
			shell.Quit()
			return
		}
		_ = srv.Shutdown()
	}()

	// Native window + tray.
	shellErr := shell.Run(shell.Options{
		URL:         url,
		Title:       "Lumen",
		Icon:        assets.Icon,
		Width:       1440,
		Height:      900,
		IconID:      1, // resource ID stamped by goversioninfo
		StartHidden: opts.StartHidden || cfg.UI.Window.StartHidden,
		CloseToTray: func() bool {
			return cfg.UI.Window.CloseAction != "quit"
		},
		MinimizeToTray: func() bool {
			return cfg.UI.Window.MinimizeToTray
		},
		OnQuit: func() {
			_ = srv.Shutdown()
		},
	})
	if shellErr != nil {
		// WebView2 missing or broken. Don't just die — fall back to the browser
		// so the user still gets their app, and tell them why.
		log.Printf("shell: %v — falling back to browser mode", shellErr)
		fmt.Fprintln(os.Stderr, shellErr)
		runBrowserMode(srv, url, errCh)
	}
}

// runBrowserMode is the pre-1.0 behaviour: serve, open the default browser,
// and wait for a signal or the SPA's quit button.
func runBrowserMode(srv *server.Server, url string, errCh <-chan error) {
	time.Sleep(200 * time.Millisecond) // let the listener settle before we point a browser at it
	fmt.Printf("Lumen is serving at %s\n", url)
	if err := browser.OpenURL(url); err != nil {
		fmt.Fprintf(os.Stderr, "(couldn't open your browser automatically: %v)\n", err)
	}

	sig := make(chan os.Signal, 1)
	signal.Notify(sig, os.Interrupt, syscall.SIGTERM)
	select {
	case <-sig:
		_ = srv.Shutdown()
	case <-srv.Quit():
		_ = srv.Shutdown()
	case err := <-errCh:
		if err != nil && err != http.ErrServerClosed {
			fatal(fmt.Sprintf("Lumen's server stopped unexpectedly.\n\n%v", err))
		}
	}
}

// signalExistingInstance asks the Lumen that is already running to show itself,
// then exits. Uses the running instance's own HTTP server rather than a Win32
// window broadcast — it is already listening, and this needs no window title
// matching.
func signalExistingInstance(addr string) {
	client := &http.Client{Timeout: 3 * time.Second}
	resp, err := client.Post("http://"+addr+"/api/window/show", "application/json", nil)
	if err != nil {
		// Running but not answering — most likely mid-startup or wedged.
		fatal("Lumen is already running, but it isn't responding.\n\n" +
			"Look for the Lumen icon in your system tray, or end the lumen.exe " +
			"process in Task Manager and try again.")
	}
	defer resp.Body.Close()
}

// setupLogging sends log output to %APPDATA%\Lumen\logs\lumen.log. Under
// -H windowsgui there is no console to print to, so without this every
// log.Printf in the codebase would vanish.
func setupLogging() {
	path, err := config.LogFile()
	if err != nil {
		return
	}
	f, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o600)
	if err != nil {
		return
	}
	log.SetOutput(f)
	log.SetFlags(log.LstdFlags | log.Lmsgprefix)
	log.Printf("--- lumen %s starting ---", version)
}

// fatal reports an unrecoverable startup problem. Under -H windowsgui there is
// no console, so this puts up a message box; when launched from a terminal the
// attached console gets the text too.
func fatal(msg string) {
	fmt.Fprintln(os.Stderr, msg)
	errorBox("Lumen couldn't start", msg)
	os.Exit(1)
}
