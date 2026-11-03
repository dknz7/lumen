//go:build windows

package shell

import "testing"

// activate builds WM_ACTIVATE's wParam: LOWORD is the state, HIWORD is
// non-zero when the window is minimised.
func activate(state, minimised uintptr) uintptr { return state | minimised<<16 }

func TestNoteVisibility(t *testing.T) {
	const waActive = 1

	type step struct {
		name               string
		msg, wparam        uintptr
		wantRefreshRequest bool
		// hidden marks a message that arrives while the window is still off
		// screen (hidden to the tray, or minimised). Zero value = on screen.
		hidden bool
	}
	cases := []struct {
		name  string
		steps []step
	}{
		{"hide to tray then restore from tray", []step{
			{"hidden", wmShowWindow, 0, false, false},
			{"shown", wmShowWindow, 1, true, false},
			{"activated after show", wmActivate, activate(waActive, 0), false, false},
		}},
		{"minimise then restore from taskbar", []step{
			{"minimised", wmSize, sizeMinimized, false, false},
			{"activated while still minimised", wmActivate, activate(waActive, 1), false, false},
			{"restored", wmSize, sizeRestored, true, false},
		}},
		{"minimise then maximise", []step{
			{"minimised", wmSize, sizeMinimized, false, false},
			{"maximised", wmSize, sizeMaximized, true, false},
		}},
		{"hidden then only activated", []step{
			{"hidden", wmShowWindow, 0, false, false},
			{"activated", wmActivate, activate(waActive, 0), true, false},
			{"shown", wmShowWindow, 1, false, false},
		}},
		// A monitor wake, dock/undock or DPI change resizes the window while
		// it sits in the tray. That must not spend the refresh on a hidden
		// page, or the real restore afterwards gets nothing.
		{"resized and activated while hidden in the tray", []step{
			{"hidden", wmShowWindow, 0, false, true},
			{"resized while hidden", wmSize, sizeRestored, false, true},
			{"activated while hidden", wmActivate, activate(waActive, 0), false, true},
			{"shown", wmShowWindow, 1, true, true},
		}},
		{"alt-tab and resizing while visible never refresh", []step{
			{"deactivated", wmActivate, activate(waInactive, 0), false, false},
			{"activated", wmActivate, activate(waActive, 0), false, false},
			{"resized", wmSize, sizeRestored, false, false},
			{"shown", wmShowWindow, 1, false, false},
		}},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			away.Store(false)
			for _, s := range c.steps {
				onScreen := func() bool { return !s.hidden }
				if got := noteVisibility(s.msg, s.wparam, onScreen); got != s.wantRefreshRequest {
					t.Errorf("%s: refresh requested = %v, want %v", s.name, got, s.wantRefreshRequest)
				}
			}
		})
	}
}
