//go:build windows

package shell

import (
	"log"
	"runtime"
	"sync"
	"sync/atomic"
	"syscall"

	"github.com/energye/systray"
	webview2 "github.com/jchv/go-webview2"
)

// Options configures the desktop shell.
type Options struct {
	URL    string // what to load, e.g. http://127.0.0.1:7832
	Title  string // window title and tray tooltip
	Icon   []byte // .ico bytes for the tray icon
	Width  int
	Height int

	// IconID is the Windows resource ID of the window icon, embedded in the
	// binary by goversioninfo. 0 means "use the Win32 default", which looks
	// generic — the build stamps the Lumen icon at ID 1.
	IconID uint

	// StartHidden launches straight to the tray with no window shown. Used by
	// the "start with Windows" shortcut, which passes --tray.
	StartHidden bool

	// CloseToTray and MinimizeToTray are consulted at the moment the user
	// closes or minimises, not at startup, so changing the preference in
	// Settings takes effect immediately without a restart.
	CloseToTray    func() bool
	MinimizeToTray func() bool

	// OnQuit runs on the way out — before the process exits — so the caller
	// can shut the HTTP server down cleanly.
	OnQuit func()
}

// state is process-global because the subclassed WndProc is a C callback and
// has nowhere to carry a receiver. Exactly one shell per process, which is
// enforced upstream by the single-instance mutex.
var state struct {
	mu          sync.Mutex
	wv          webview2.WebView
	hwnd        uintptr
	origWndProc uintptr
	opts        Options
	quitting    atomic.Bool
	ready       atomic.Bool
}

// wndProc runs on the window's thread. It converts the two "the user made this
// window go away" messages into hide-to-tray, and lets everything else through
// to WebView2's own proc — which needs WM_SIZE, WM_PAINT and friends to keep
// the browser control glued to the frame.
func wndProc(hwnd, msg, wparam, lparam uintptr) uintptr {
	switch msg {
	case wmClose:
		// Quit() sets this first so the real teardown isn't swallowed.
		if !state.quitting.Load() && shouldCloseToTray() {
			showWindow(hwnd, swHide)
			return 0
		}

	case wmSysCommand:
		if wparam&0xFFF0 == scMinimize && shouldMinimizeToTray() {
			showWindow(hwnd, swHide)
			return 0
		}

	case wmQueryEndSession, wmEndSession:
		// Windows is logging off or shutting down. Never hide-to-tray here —
		// that blocks shutdown and Windows kills us anyway.
		state.quitting.Store(true)
	}

	r, _, _ := procCallWindowProcW.Call(state.origWndProc, hwnd, msg, wparam, lparam)
	return r
}

func shouldCloseToTray() bool {
	if state.opts.CloseToTray == nil {
		return true
	}
	return state.opts.CloseToTray()
}

func shouldMinimizeToTray() bool {
	if state.opts.MinimizeToTray == nil {
		return false
	}
	return state.opts.MinimizeToTray()
}

// Run creates the window and the tray icon, then blocks until the user quits.
// It must be called from the main goroutine: systray owns the main thread's
// message pump.
func Run(opts Options) error {
	state.opts = opts

	var wvReady sync.WaitGroup
	wvReady.Add(1)

	// --- Window thread. Locked, because a Win32 message pump is bound to the
	// thread that created the window. ---
	go func() {
		runtime.LockOSThread()
		defer runtime.UnlockOSThread()

		w := webview2.NewWithOptions(webview2.WebViewOptions{
			WindowOptions: webview2.WindowOptions{
				Title:  opts.Title,
				Width:  uint(opts.Width),
				Height: uint(opts.Height),
				IconId: opts.IconID,
				Center: true,
			},
		})
		if w == nil {
			log.Println("shell: WebView2 window could not be created")
			wvReady.Done()
			return
		}

		state.mu.Lock()
		state.wv = w
		state.hwnd = uintptr(w.Window())
		state.mu.Unlock()

		// Subclass so close/minimise can be intercepted. If this fails the app
		// still works — it just quits on close instead of hiding to tray.
		if prev, _, err := procSetWindowLongPtrW.Call(
			state.hwnd, uintptr(gwlpWndProc), syscall.NewCallback(wndProc),
		); prev != 0 {
			state.origWndProc = prev
		} else {
			log.Printf("shell: could not subclass window, close-to-tray disabled: %v", err)
		}

		w.Navigate(opts.URL)
		if opts.StartHidden {
			showWindow(state.hwnd, swHide)
		}
		state.ready.Store(true)
		wvReady.Done()

		w.Run() // blocks, pumping this thread's queue, until Terminate()

		w.Destroy()
		systray.Quit()
	}()

	wvReady.Wait()
	if state.wv == nil {
		return errNoWebView
	}

	// --- Main thread: tray icon and its own message pump. ---
	systray.Run(func() {
		if len(opts.Icon) > 0 {
			systray.SetIcon(opts.Icon)
		}
		systray.SetTitle(opts.Title)
		systray.SetTooltip(opts.Title)

		mOpen := systray.AddMenuItem("Open Lumen", "Show the Lumen window")
		systray.AddSeparator()
		mQuit := systray.AddMenuItem("Quit Lumen", "Stop Lumen and close it")

		mOpen.Click(Show)
		mQuit.Click(Quit)

		// Left-click and double-click the tray icon both open the window —
		// people expect one or the other and disagree about which.
		systray.SetOnClick(func(systray.IMenu) { Show() })
		systray.SetOnDClick(func(systray.IMenu) { Show() })
	}, func() {
		if opts.OnQuit != nil {
			opts.OnQuit()
		}
	})

	return nil
}

// Show brings the window back from the tray. Safe to call from any goroutine.
func Show() {
	state.mu.Lock()
	wv, hwnd := state.wv, state.hwnd
	state.mu.Unlock()
	if wv == nil || hwnd == 0 {
		return
	}
	wv.Dispatch(func() { bringToFront(hwnd) })
}

// Hide sends the window to the tray. Safe to call from any goroutine.
func Hide() {
	state.mu.Lock()
	wv, hwnd := state.wv, state.hwnd
	state.mu.Unlock()
	if wv == nil || hwnd == 0 {
		return
	}
	wv.Dispatch(func() { showWindow(hwnd, swHide) })
}

// Quit tears the whole shell down and lets Run return. Safe from any goroutine.
func Quit() {
	if !state.quitting.CompareAndSwap(false, true) {
		return // already quitting
	}
	state.mu.Lock()
	wv := state.wv
	state.mu.Unlock()
	if wv != nil {
		wv.Terminate()
	} else {
		systray.Quit()
	}
}

// Running reports whether the window exists yet. The HTTP server starts before
// the shell does, so an early /api/window/show needs to not panic.
func Running() bool { return state.ready.Load() }

// Flash bounces the taskbar button. Used when a second instance is launched
// while Lumen is already running and the window is already in front.
func Flash() {
	state.mu.Lock()
	hwnd := state.hwnd
	state.mu.Unlock()
	if hwnd != 0 {
		procFlashWindow.Call(hwnd, 1)
	}
}

type shellError string

func (e shellError) Error() string { return string(e) }

const errNoWebView = shellError(
	"could not create a WebView2 window — install the Microsoft Edge WebView2 runtime " +
		"from https://developer.microsoft.com/microsoft-edge/webview2/, or run `lumen serve --browser`")
