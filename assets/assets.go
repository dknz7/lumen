// Package assets embeds Lumen's brand assets into the binary.
//
// The .ico lives here rather than under internal/ because it is also consumed
// as a plain file by goversioninfo (which stamps it as the exe icon), by the
// Inno Setup script, and by the README. Making this directory a Go package
// keeps one canonical copy instead of three.
package assets

import _ "embed"

// Icon is lumen.ico — used for the system tray. The window and taskbar icon
// come from the Win32 resource that goversioninfo stamps into the exe, not
// from this.
//
//go:embed lumen.ico
var Icon []byte
