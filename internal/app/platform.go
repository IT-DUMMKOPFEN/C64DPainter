package app

import (
	"regexp"
	"strings"
)

// MacOS is set by the macOS front end. It changes key names (Ctrl → Cmd,
// Alt → Option) and paths shown to the user.
var MacOS bool

// Linux is set by the Linux front end (only changes the settings path shown).
var Linux bool

var (
	reCtrl = regexp.MustCompile(`\bCtrl\b`)
	reAlt  = regexp.MustCompile(`\bAlt\b`)
)

// L adapts a user-visible text to the platform.
func L(s string) string {
	if Linux {
		s = strings.ReplaceAll(s, "{MACNOTE}", "")
		return strings.ReplaceAll(s, "{SETTINGS}", "~/.config/C64DPainter/settings.json")
	}
	if !MacOS {
		s = strings.ReplaceAll(s, "{MACNOTE}", "")
		return strings.ReplaceAll(s, "{SETTINGS}", `%APPDATA%\C64DPainter\settings.json`)
	}
	s = strings.ReplaceAll(s, "{MACNOTE}", "> **On the Mac:** shortcuts use the Command key (shown as Cmd) and the Option key. For color 2 use the right mouse button or a two-finger click on the trackpad. Holding Command or Control while painting draws a straight line. On a trackpad, two fingers move the view, pinching zooms, and Cmd + two fingers zooms too.")
	s = strings.ReplaceAll(s, "{SETTINGS}", "~/Library/Application Support/C64DPainter/settings.json")
	s = reCtrl.ReplaceAllString(s, "Cmd")
	s = reAlt.ReplaceAllString(s, "Option")
	return s
}

// OpenAbout shows the About window (used by the macOS menu).
func (a *App) OpenAbout() { a.openAbout() }

// OpenHelp opens a help topic (used by the macOS menu).
func (a *App) OpenHelp(topic string) { a.openHelp(topic) }

// IsCanvasAt reports whether (x,y) is on the canvas with nothing in front of it.
func (a *App) IsCanvasAt(x, y int) bool {
	return a.modal == nil && a.menuOpen < 0 && a.rCanvas.Contains(x, y)
}

// PanBy moves the view (used for trackpad scrolling).
func (a *App) PanBy(dx, dy int) {
	a.panX += dx
	a.panY += dy
	a.clampPan()
}

// ZoomAt zooms in (dir > 0) or out at a point (used for trackpad pinch).
func (a *App) ZoomAt(dir, x, y int) {
	a.zoomStep(dir, x, y)
}

// Quitting reports whether the user already confirmed quitting.
func (a *App) Quitting() bool { return a.quitting }

// RefreshTitle sets the window title again (after the window exists).
func (a *App) RefreshTitle() { a.updateTitle() }
