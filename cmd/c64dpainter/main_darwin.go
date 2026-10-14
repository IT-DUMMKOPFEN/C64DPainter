//go:build darwin

// C64 DPainter – macOS front end (Cocoa via the Objective-C runtime, no cgo).
package main

import (
	"fmt"
	"math"
	"os"
	"path/filepath"
	"runtime"
	"runtime/debug"
	"strings"
	"time"
	"unsafe"

	"github.com/ebitengine/purego"
	"github.com/ebitengine/purego/objc"

	"c64dpainter/internal/app"
	"c64dpainter/internal/gfx"
)

func init() { runtime.LockOSThread() }

type NSPoint struct{ X, Y float64 }
type NSSize struct{ Width, Height float64 }
type NSRect struct {
	Origin NSPoint
	Size   NSSize
}

// --- Objective-C helpers ----------------------------------------------------------

var selCache = map[string]objc.SEL{}

func sel(name string) objc.SEL {
	s, ok := selCache[name]
	if !ok {
		s = objc.RegisterName(name)
		selCache[name] = s
	}
	return s
}

func cls(name string) objc.ID { return objc.ID(objc.GetClass(name)) }

func msg(id objc.ID, name string, args ...any) objc.ID { return id.Send(sel(name), args...) }

func nsstr(s string) objc.ID { return msg(cls("NSString"), "stringWithUTF8String:", s) }

func gostr(id objc.ID) string {
	if id == 0 {
		return ""
	}
	p := objc.Send[uintptr](id, sel("UTF8String"))
	if p == 0 {
		return ""
	}
	var b []byte
	for i := uintptr(0); ; i++ {
		c := *(*byte)(unsafe.Pointer(p + i))
		if c == 0 {
			break
		}
		b = append(b, c)
	}
	return string(b)
}

// --- CoreGraphics / CoreFoundation -------------------------------------------------

var (
	cgColorSpaceCreateDeviceRGB      func() uintptr
	cgBitmapContextCreate            func(data uintptr, w, h, bpc, bpr uintptr, cs uintptr, info uint32) uintptr
	cgBitmapContextGetData           func(ctx uintptr) uintptr
	cgBitmapContextCreateImage       func(ctx uintptr) uintptr
	cgContextDrawImage               func(ctx uintptr, r NSRect, img uintptr)
	cgContextSetInterpolationQuality func(ctx uintptr, q int32)
	cgImageRelease                   func(img uintptr)
	cgContextRelease                 func(ctx uintptr)
)

func loadFrameworks() {
	if _, err := purego.Dlopen("/System/Library/Frameworks/Cocoa.framework/Cocoa", purego.RTLD_GLOBAL|purego.RTLD_LAZY); err != nil {
		panic(err)
	}
	cg, err := purego.Dlopen("/System/Library/Frameworks/CoreGraphics.framework/CoreGraphics", purego.RTLD_GLOBAL|purego.RTLD_LAZY)
	if err != nil {
		panic(err)
	}
	purego.RegisterLibFunc(&cgColorSpaceCreateDeviceRGB, cg, "CGColorSpaceCreateDeviceRGB")
	purego.RegisterLibFunc(&cgBitmapContextCreate, cg, "CGBitmapContextCreate")
	purego.RegisterLibFunc(&cgBitmapContextGetData, cg, "CGBitmapContextGetData")
	purego.RegisterLibFunc(&cgBitmapContextCreateImage, cg, "CGBitmapContextCreateImage")
	purego.RegisterLibFunc(&cgContextDrawImage, cg, "CGContextDrawImage")
	purego.RegisterLibFunc(&cgContextSetInterpolationQuality, cg, "CGContextSetInterpolationQuality")
	purego.RegisterLibFunc(&cgImageRelease, cg, "CGImageRelease")
	purego.RegisterLibFunc(&cgContextRelease, cg, "CGContextRelease")
}

// --- state ------------------------------------------------------------------------

var (
	a          *app.App
	nsApp      objc.ID
	window     objc.ID
	view       objc.ID
	frame      *gfx.Image
	scale      = 1
	viewW      float64
	viewH      float64
	inModal    int
	startTime  = time.Now()
	buttons    [3]bool
	scrollAcc  float64
	magnifyAcc float64
	colorSpace uintptr
	bmpCtx     uintptr
	bmpW, bmpH int
	crashed    bool
)

// --- host interface -----------------------------------------------------------------

type macHost struct{}

func extensions(fs []app.Filter) objc.ID {
	arr := msg(cls("NSMutableArray"), "array")
	all := false
	for _, f := range fs {
		for _, p := range strings.Split(f.Pattern, ";") {
			p = strings.TrimSpace(p)
			if p == "*.*" || p == "*" {
				all = true
				continue
			}
			ext := strings.TrimPrefix(p, "*.")
			msg(arr, "addObject:", nsstr(ext))
		}
	}
	if all && len(fs) > 0 {
		// "All files" is offered together with specific types: allow everything
		return 0
	}
	return arr
}

func runPanel(panel objc.ID, title, dir string, fs []app.Filter) bool {
	msg(panel, "setMessage:", nsstr(title))
	if ext := extensions(fs); ext != 0 {
		msg(panel, "setAllowedFileTypes:", ext)
	}
	if dir != "" {
		msg(panel, "setDirectoryURL:", msg(cls("NSURL"), "fileURLWithPath:", nsstr(dir)))
	}
	inModal++
	r := objc.Send[int64](panel, sel("runModal"))
	inModal--
	releaseAll()
	return r == 1
}

func (macHost) OpenFile(title string, fs []app.Filter, dir string) (string, bool) {
	panel := msg(cls("NSOpenPanel"), "openPanel")
	msg(panel, "setCanChooseFiles:", true)
	msg(panel, "setCanChooseDirectories:", false)
	msg(panel, "setAllowsMultipleSelection:", false)
	if !runPanel(panel, title, dir, fs) {
		return "", false
	}
	url := msg(msg(panel, "URLs"), "firstObject")
	p := gostr(msg(url, "path"))
	return p, p != ""
}

func (macHost) SaveFile(title string, fs []app.Filter, dir, name, defExt string) (string, bool) {
	panel := msg(cls("NSSavePanel"), "savePanel")
	msg(panel, "setNameFieldStringValue:", nsstr(name))
	msg(panel, "setCanCreateDirectories:", true)
	if !runPanel(panel, title, dir, fs) {
		return "", false
	}
	p := gostr(msg(msg(panel, "URL"), "path"))
	if p != "" && filepath.Ext(p) == "" && defExt != "" {
		p += "." + defExt
	}
	return p, p != ""
}

func alert(title, text string, buttons ...string) int {
	al := msg(msg(cls("NSAlert"), "alloc"), "init")
	msg(al, "setMessageText:", nsstr(title))
	msg(al, "setInformativeText:", nsstr(text))
	for _, b := range buttons {
		msg(al, "addButtonWithTitle:", nsstr(b))
	}
	inModal++
	r := objc.Send[int64](al, sel("runModal"))
	inModal--
	releaseAll()
	return int(r - 1000)
}

func (macHost) Ask(title, text string) int {
	switch alert(title, text, "Save", "Don't Save", "Cancel") {
	case 0:
		return 0
	case 1:
		return 1
	}
	return 2
}

func (macHost) Info(title, text string) { alert(title, text, "OK") }
func (macHost) Quit()                   { msg(nsApp, "terminate:", objc.ID(0)) }
func (macHost) SetTitle(s string) {
	if window != 0 {
		msg(window, "setTitle:", nsstr(s))
	}
}

// visibleFrame returns the usable screen area (in points) of the window's screen.
func visibleFrame() NSRect {
	scr := objc.ID(0)
	if window != 0 {
		scr = msg(window, "screen")
	}
	if scr == 0 {
		scr = msg(cls("NSScreen"), "mainScreen")
	}
	if scr == 0 {
		return NSRect{Size: NSSize{1440, 900}}
	}
	return objc.Send[NSRect](scr, sel("visibleFrame"))
}

func (macHost) ScreenSize() (int, int) {
	vf := visibleFrame()
	return int(vf.Size.Width), int(vf.Size.Height) - 28 // minus title bar
}

// contentSizeFor returns the preferred and minimum content size for a scale.
func contentSizeFor(s int) (pref, minSz NSSize) {
	vf := visibleFrame()
	maxW, maxH := vf.Size.Width, vf.Size.Height-28
	pref = NSSize{math.Min(float64(app.PrefLogicalW*s), maxW*0.96), math.Min(float64(app.PrefLogicalH*s), maxH*0.96)}
	minSz = NSSize{math.Min(float64(app.MinLogicalW*s), maxW), math.Min(float64(app.MinLogicalH*s), maxH)}
	return
}

func (macHost) ApplyScale(s int) {
	scale = s
	pref, minSz := contentSizeFor(s)
	msg(window, "setContentMinSize:", minSz)
	if objc.Send[uint64](window, sel("styleMask"))&(1<<14) != 0 {
		return // full screen
	}
	msg(window, "setContentSize:", pref)
	msg(window, "center")
	updateViewSize()
}

func (macHost) ReadFile(p string) ([]byte, error)  { return os.ReadFile(p) }
func (macHost) WriteFile(p string, d []byte) error { return os.WriteFile(p, d, 0o644) }

// --- rendering -----------------------------------------------------------------------

func updateViewSize() {
	r := objc.Send[NSRect](view, sel("bounds"))
	viewW, viewH = r.Size.Width, r.Size.Height
}

func render() {
	if a == nil || view == 0 || viewW < 1 || viewH < 1 {
		return
	}
	scale = a.UIScale()
	a.Resize((int(viewW)+scale-1)/scale, (int(viewH)+scale-1)/scale)
	a.Tick(time.Since(startTime).Milliseconds())
	frame = a.Frame()
	if a.HasDeferred() {
		msg(view, "setNeedsDisplay:", true)
		msg(view, "displayIfNeeded")
		inModal++
		a.RunDeferred()
		inModal--
		releaseAll()
		if a == nil {
			return
		}
		a.Resize((int(viewW)+scale-1)/scale, (int(viewH)+scale-1)/scale)
		frame = a.Frame()
	}
	msg(view, "setNeedsDisplay:", true)
}

func drawRect(self objc.ID, cmd objc.SEL) {
	defer guard()
	if frame == nil {
		return
	}
	gctx := msg(cls("NSGraphicsContext"), "currentContext")
	if gctx == 0 {
		return
	}
	ctx := uintptr(msg(gctx, "CGContext"))
	if ctx == 0 {
		return
	}
	if bmpCtx == 0 || bmpW != frame.W || bmpH != frame.H {
		if bmpCtx != 0 {
			cgContextRelease(bmpCtx)
		}
		bmpW, bmpH = frame.W, frame.H
		// kCGImageAlphaNoneSkipFirst | kCGBitmapByteOrder32Little = BGRX in memory = 0x00RRGGBB
		bmpCtx = cgBitmapContextCreate(0, uintptr(bmpW), uintptr(bmpH), 8, uintptr(bmpW*4), colorSpace, 0x2006)
		if bmpCtx == 0 {
			return
		}
	}
	data := cgBitmapContextGetData(bmpCtx)
	if data == 0 {
		return
	}
	dst := unsafe.Slice((*uint32)(unsafe.Pointer(data)), bmpW*bmpH)
	copy(dst, frame.Pix)
	img := cgBitmapContextCreateImage(bmpCtx)
	if img == 0 {
		return
	}
	cgContextSetInterpolationQuality(ctx, 1) // none: crisp pixels
	w := float64(frame.W * scale)
	h := float64(frame.H * scale)
	cgContextDrawImage(ctx, NSRect{NSPoint{0, viewH - h}, NSSize{w, h}}, img)
	cgImageRelease(img)
}

// guard catches Go panics inside Cocoa callbacks so a bug does not lose the picture.
func guard() {
	if r := recover(); r != nil {
		inModal = 0
		m := fmt.Sprintf("%v\n\n%s", r, debug.Stack())
		if d, err := os.UserConfigDir(); err == nil {
			p := filepath.Join(d, "C64DPainter")
			os.MkdirAll(p, 0o755)
			os.WriteFile(filepath.Join(p, "error.log"), []byte(time.Now().Format(time.RFC3339)+"\n"+m), 0o644)
		}
		if !crashed {
			crashed = true
			alert(app.AppName, "An internal error occurred:\n\n"+fmt.Sprint(r)+"\n\nThe program keeps running. Please save your picture to be safe.\nDetails: ~/Library/Application Support/C64DPainter/error.log", "OK")
			crashed = false
		}
	}
}

// --- input ---------------------------------------------------------------------------

const (
	flagShift   = 1 << 17
	flagControl = 1 << 18
	flagOption  = 1 << 19
	flagCommand = 1 << 20
)

func modsOf(ev objc.ID) (app.Mods, uint64) {
	f := objc.Send[uint64](ev, sel("modifierFlags"))
	// Both Command and Control count as "Ctrl" (straight lines, Ctrl+click …)
	return app.Mods{Shift: f&flagShift != 0, Ctrl: f&(flagCommand|flagControl) != 0, Alt: f&flagOption != 0}, f
}

func mousePos(ev objc.ID) (int, int) {
	p := objc.Send[NSPoint](ev, sel("locationInWindow"))
	return int(p.X) / scale, int(viewH-p.Y) / scale
}

func releaseAll() {
	if a == nil {
		return
	}
	for b := 0; b < 3; b++ {
		if buttons[b] {
			buttons[b] = false
			a.MouseUp(b, -1, -1)
		}
	}
	a.KeyUp(app.KeySpace, app.Mods{})
}

var cursorNames = map[int]string{
	app.CursorArrow: "arrowCursor",
	app.CursorCross: "crosshairCursor",
	app.CursorHand:  "openHandCursor",
	app.CursorMove:  "closedHandCursor",
	app.CursorSize:  "pointingHandCursor",
}

func updateCursor() {
	if inModal > 0 {
		return
	}
	msg(msg(cls("NSCursor"), cursorNames[a.Cursor()]), "set")
}

func mouseDown(btn int) func(self objc.ID, cmd objc.SEL, ev objc.ID) {
	return func(self objc.ID, cmd objc.SEL, ev objc.ID) {
		defer guard()
		if inModal > 0 {
			return
		}
		m, _ := modsOf(ev)
		b := btn
		buttons[b] = true
		x, y := mousePos(ev)
		a.SetMods(m)
		a.MouseDown(b, x, y, objc.Send[int64](ev, sel("clickCount")) >= 2)
		render()
		updateCursor()
	}
}

func mouseUp(btn int) func(self objc.ID, cmd objc.SEL, ev objc.ID) {
	return func(self objc.ID, cmd objc.SEL, ev objc.ID) {
		defer guard()
		if inModal > 0 {
			return
		}
		b := btn
		if !buttons[b] {
			return
		}
		buttons[b] = false
		m, _ := modsOf(ev)
		x, y := mousePos(ev)
		a.SetMods(m)
		a.MouseUp(b, x, y)
		render()
		updateCursor()
	}
}

func mouseMoved(self objc.ID, cmd objc.SEL, ev objc.ID) {
	defer guard()
	if inModal > 0 {
		return
	}
	m, _ := modsOf(ev)
	x, y := mousePos(ev)
	a.SetMods(m)
	a.MouseMove(x, y)
	render()
	updateCursor()
}

func scrollWheel(self objc.ID, cmd objc.SEL, ev objc.ID) {
	defer guard()
	if inModal > 0 {
		return
	}
	m, _ := modsOf(ev)
	x, y := mousePos(ev)
	a.SetMods(m)
	dy := objc.Send[float64](ev, sel("scrollingDeltaY"))
	dx := objc.Send[float64](ev, sel("scrollingDeltaX"))
	precise := objc.Send[bool](ev, sel("hasPreciseScrollingDeltas"))
	if precise && !m.Ctrl && a.IsCanvasAt(x, y) {
		// trackpad: two fingers move the view, pinch zooms
		a.MouseMove(x, y)
		a.PanBy(int(math.Round(dx)), int(math.Round(dy)))
		render()
		return
	}
	if precise {
		scrollAcc += dy
		steps := int(scrollAcc / 24)
		if steps == 0 {
			return
		}
		scrollAcc -= float64(steps) * 24
		a.Wheel(steps, x, y)
	} else {
		steps := 0
		if dy > 0 {
			steps = 1
		} else if dy < 0 {
			steps = -1
		}
		if steps == 0 && dx != 0 {
			a.SetMods(app.Mods{Shift: true})
			if dx > 0 {
				steps = 1
			} else {
				steps = -1
			}
		}
		if steps == 0 {
			return
		}
		a.Wheel(steps, x, y)
	}
	render()
}

func magnify(self objc.ID, cmd objc.SEL, ev objc.ID) {
	defer guard()
	if inModal > 0 {
		return
	}
	x, y := mousePos(ev)
	magnifyAcc += objc.Send[float64](ev, sel("magnification"))
	for magnifyAcc > 0.25 {
		magnifyAcc -= 0.25
		a.ZoomAt(1, x, y)
	}
	for magnifyAcc < -0.25 {
		magnifyAcc += 0.25
		a.ZoomAt(-1, x, y)
	}
	render()
}

func macKey(code uint16, ign string) int {
	switch code {
	case 53:
		return app.KeyEsc
	case 36, 76:
		return app.KeyEnter
	case 51:
		return app.KeyBackspace
	case 117:
		return app.KeyDelete
	case 48:
		return app.KeyTab
	case 49:
		return app.KeySpace
	case 123:
		return app.KeyLeft
	case 124:
		return app.KeyRight
	case 125:
		return app.KeyDown
	case 126:
		return app.KeyUp
	case 115:
		return app.KeyHome
	case 119:
		return app.KeyEnd
	case 116:
		return app.KeyPgUp
	case 121:
		return app.KeyPgDn
	case 122:
		return app.KeyF1
	case 120:
		return app.KeyF2
	}
	if ign != "" {
		r := []rune(strings.ToUpper(ign))[0]
		switch {
		case r >= 'A' && r <= 'Z', r >= '0' && r <= '9':
			return int(r)
		case r == '+' || r == '=':
			return app.KeyPlus
		case r == '-':
			return app.KeyMinus
		}
	}
	return 0
}

func keyDown(self objc.ID, cmd objc.SEL, ev objc.ID) {
	defer guard()
	if inModal > 0 {
		return
	}
	m, f := modsOf(ev)
	code := objc.Send[uint16](ev, sel("keyCode"))
	ign := gostr(msg(ev, "charactersIgnoringModifiers"))
	text := gostr(msg(ev, "characters"))
	if m.Ctrl && (text == "?" || ign == "?" || ign == "/") {
		a.KeyDown(app.KeyF1, app.Mods{})
		render()
		return
	}
	if k := macKey(code, ign); k != 0 {
		a.KeyDown(k, m)
	} else {
		a.SetMods(m)
	}
	if f&(flagCommand|flagControl) == 0 {
		for _, r := range text {
			if r >= 32 && r != 127 && (r < 0xF700 || r > 0xF8FF) {
				a.Char(r)
			}
		}
	}
	render()
}

func keyUp(self objc.ID, cmd objc.SEL, ev objc.ID) {
	defer guard()
	m, _ := modsOf(ev)
	code := objc.Send[uint16](ev, sel("keyCode"))
	if k := macKey(code, gostr(msg(ev, "charactersIgnoringModifiers"))); k != 0 {
		a.KeyUp(k, m)
	} else {
		a.SetMods(m)
	}
}

func flagsChanged(self objc.ID, cmd objc.SEL, ev objc.ID) {
	defer guard()
	if inModal > 0 {
		return
	}
	m, _ := modsOf(ev)
	a.SetMods(m)
	render()
	updateCursor()
}

func tick(self objc.ID, cmd objc.SEL, timer objc.ID) {
	defer guard()
	if inModal > 0 || a == nil {
		return
	}
	if a.Tick(time.Since(startTime).Milliseconds()) {
		render()
	}
}

// --- drag & drop ----------------------------------------------------------------------

func draggingEntered(self objc.ID, cmd objc.SEL, info objc.ID) uint64 { return 1 } // copy

func performDrag(self objc.ID, cmd objc.SEL, info objc.ID) bool {
	defer guard()
	pb := msg(info, "draggingPasteboard")
	classes := msg(cls("NSArray"), "arrayWithObject:", cls("NSURL"))
	urls := msg(pb, "readObjectsForClasses:options:", classes, objc.ID(0))
	if urls == 0 || objc.Send[uint64](urls, sel("count")) == 0 {
		return false
	}
	p := gostr(msg(msg(urls, "firstObject"), "path"))
	if p == "" {
		return false
	}
	a.DropFiles([]string{p})
	render()
	return true
}

// --- window and application delegate --------------------------------------------------

func windowShouldClose(self objc.ID, cmd objc.SEL, sender objc.ID) bool {
	defer guard()
	if a.Quitting() {
		return true
	}
	inModal++
	ok := a.CanClose()
	inModal--
	if ok {
		quitConfirmed = true
	}
	return ok
}

var quitConfirmed bool

func windowWillClose(self objc.ID, cmd objc.SEL, n objc.ID) {
	quitConfirmed = true
	msg(nsApp, "terminate:", objc.ID(0))
}

func windowDidResize(self objc.ID, cmd objc.SEL, n objc.ID) {
	defer guard()
	updateViewSize()
	if inModal == 0 {
		render()
	}
}

func shouldTerminate(self objc.ID, cmd objc.SEL, sender objc.ID) uint64 {
	defer guard()
	if quitConfirmed || a.Quitting() {
		return 1
	}
	inModal++
	ok := a.CanClose()
	inModal--
	if ok {
		return 1 // NSTerminateNow
	}
	return 0 // NSTerminateCancel
}

func terminateAfterLastWindow(self objc.ID, cmd objc.SEL, sender objc.ID) bool { return true }

func openFile(self objc.ID, cmd objc.SEL, sender objc.ID, name objc.ID) bool {
	defer guard()
	p := gostr(name)
	if p == "" || a == nil {
		pendingOpen = p
		return true
	}
	a.DropFiles([]string{p})
	render()
	return true
}

var pendingOpen string

func showAbout(self objc.ID, cmd objc.SEL, sender objc.ID) {
	defer guard()
	a.OpenAbout()
	render()
}

func showHelp(self objc.ID, cmd objc.SEL, sender objc.ID) {
	defer guard()
	a.OpenHelp("")
	render()
}

// --- setup ------------------------------------------------------------------------------

func boolRet(v bool) func(objc.ID, objc.SEL) bool {
	return func(objc.ID, objc.SEL) bool { return v }
}

func registerClasses() (viewClass, delegateClass objc.Class) {
	ev := func(name string, fn func(objc.ID, objc.SEL, objc.ID)) objc.MethodDef {
		return objc.MethodDef{Cmd: sel(name), Fn: fn}
	}
	var err error
	viewClass, err = objc.RegisterClass("C64DPainterView", objc.GetClass("NSView"), nil, nil, []objc.MethodDef{
		{Cmd: sel("drawRect:"), Fn: drawRect},
		{Cmd: sel("isOpaque"), Fn: boolRet(true)},
		{Cmd: sel("acceptsFirstResponder"), Fn: boolRet(true)},
		{Cmd: sel("acceptsFirstMouse:"), Fn: func(objc.ID, objc.SEL, objc.ID) bool { return true }},
		ev("mouseDown:", mouseDown(0)),
		ev("mouseUp:", mouseUp(0)),
		ev("mouseDragged:", mouseMoved),
		ev("rightMouseDown:", mouseDown(1)),
		ev("rightMouseUp:", mouseUp(1)),
		ev("rightMouseDragged:", mouseMoved),
		ev("otherMouseDown:", mouseDown(2)),
		ev("otherMouseUp:", mouseUp(2)),
		ev("otherMouseDragged:", mouseMoved),
		ev("mouseMoved:", mouseMoved),
		ev("scrollWheel:", scrollWheel),
		ev("magnifyWithEvent:", magnify),
		ev("keyDown:", keyDown),
		ev("keyUp:", keyUp),
		ev("flagsChanged:", flagsChanged),
		ev("tick:", tick),
		{Cmd: sel("draggingEntered:"), Fn: draggingEntered},
		{Cmd: sel("performDragOperation:"), Fn: performDrag},
	})
	if err != nil {
		panic(err)
	}
	delegateClass, err = objc.RegisterClass("C64DPainterDelegate", objc.GetClass("NSObject"), nil, nil, []objc.MethodDef{
		{Cmd: sel("windowShouldClose:"), Fn: windowShouldClose},
		ev("windowWillClose:", windowWillClose),
		ev("windowDidResize:", windowDidResize),
		{Cmd: sel("applicationShouldTerminate:"), Fn: shouldTerminate},
		{Cmd: sel("applicationShouldTerminateAfterLastWindowClosed:"), Fn: terminateAfterLastWindow},
		{Cmd: sel("application:openFile:"), Fn: openFile},
		ev("showAbout:", showAbout),
		ev("showHelp:", showHelp),
	})
	if err != nil {
		panic(err)
	}
	return
}

func menuItem(title, action, key string, target objc.ID) objc.ID {
	it := msg(msg(cls("NSMenuItem"), "alloc"), "initWithTitle:action:keyEquivalent:", nsstr(title), sel(action), nsstr(key))
	if target != 0 {
		msg(it, "setTarget:", target)
	}
	return it
}

func buildMainMenu(delegate objc.ID) {
	bar := msg(msg(cls("NSMenu"), "alloc"), "init")
	// application menu
	appItem := msg(msg(cls("NSMenuItem"), "alloc"), "init")
	msg(bar, "addItem:", appItem)
	appMenu := msg(msg(cls("NSMenu"), "alloc"), "initWithTitle:", nsstr(app.AppName))
	msg(appMenu, "addItem:", menuItem("About "+app.AppName, "showAbout:", "", delegate))
	msg(appMenu, "addItem:", msg(cls("NSMenuItem"), "separatorItem"))
	msg(appMenu, "addItem:", menuItem("Hide "+app.AppName, "hide:", "h", 0))
	others := menuItem("Hide Others", "hideOtherApplications:", "h", 0)
	msg(others, "setKeyEquivalentModifierMask:", uint64(flagCommand|flagOption))
	msg(appMenu, "addItem:", others)
	msg(appMenu, "addItem:", menuItem("Show All", "unhideAllApplications:", "", 0))
	msg(appMenu, "addItem:", msg(cls("NSMenuItem"), "separatorItem"))
	msg(appMenu, "addItem:", menuItem("Quit "+app.AppName, "terminate:", "q", 0))
	msg(appItem, "setSubmenu:", appMenu)
	// window menu
	winItem := msg(msg(cls("NSMenuItem"), "alloc"), "init")
	msg(bar, "addItem:", winItem)
	winMenu := msg(msg(cls("NSMenu"), "alloc"), "initWithTitle:", nsstr("Window"))
	msg(winMenu, "addItem:", menuItem("Minimize", "performMiniaturize:", "m", 0))
	msg(winMenu, "addItem:", menuItem("Zoom", "performZoom:", "", 0))
	fs := menuItem("Enter Full Screen", "toggleFullScreen:", "f", 0)
	msg(fs, "setKeyEquivalentModifierMask:", uint64(flagCommand|flagControl))
	msg(winMenu, "addItem:", fs)
	msg(winItem, "setSubmenu:", winMenu)
	msg(nsApp, "setWindowsMenu:", winMenu)
	// help menu
	helpItem := msg(msg(cls("NSMenuItem"), "alloc"), "init")
	msg(bar, "addItem:", helpItem)
	helpMenu := msg(msg(cls("NSMenu"), "alloc"), "initWithTitle:", nsstr("Help"))
	msg(helpMenu, "addItem:", menuItem(app.AppName+" Help", "showHelp:", "?", delegate))
	msg(helpItem, "setSubmenu:", helpMenu)
	msg(nsApp, "setMainMenu:", bar)
}

func main() {
	loadFrameworks()
	app.MacOS = true
	colorSpace = cgColorSpaceCreateDeviceRGB()

	nsApp = msg(cls("NSApplication"), "sharedApplication")
	msg(nsApp, "setActivationPolicy:", int64(0)) // regular app with Dock icon

	viewClass, delegateClass := registerClasses()
	delegate := msg(objc.ID(delegateClass), "new")
	msg(nsApp, "setDelegate:", delegate)
	buildMainMenu(delegate)

	// the interface size comes from the settings (100 % on the first start;
	// Retina screens are already sharp at 100 %)
	first := app.New(macHost{})
	first.InitScale(1)
	scale = first.UIScale()
	pref, minSz := contentSizeFor(scale)
	rect := NSRect{NSPoint{0, 0}, pref}
	const style = 1 | 2 | 4 | 8 // titled, closable, miniaturizable, resizable
	window = msg(msg(cls("NSWindow"), "alloc"), "initWithContentRect:styleMask:backing:defer:", rect, uint64(style), uint64(2), false)
	msg(window, "setReleasedWhenClosed:", false)
	msg(window, "setContentMinSize:", minSz)
	msg(window, "setAcceptsMouseMovedEvents:", true)
	msg(window, "setCollectionBehavior:", uint64(1<<7)) // full screen primary
	msg(window, "setTitle:", nsstr(app.AppName))

	view = msg(msg(objc.ID(viewClass), "alloc"), "initWithFrame:", rect)
	msg(view, "registerForDraggedTypes:", msg(cls("NSArray"), "arrayWithObject:", nsstr("public.file-url")))
	msg(window, "setContentView:", view)
	msg(window, "setDelegate:", delegate)
	msg(window, "center")
	msg(window, "makeFirstResponder:", view)

	updateViewSize()
	a = first
	a.RefreshTitle()
	msg(window, "makeKeyAndOrderFront:", objc.ID(0))
	msg(nsApp, "activateIgnoringOtherApps:", true)
	updateViewSize()
	render()

	timer := msg(cls("NSTimer"), "scheduledTimerWithTimeInterval:target:selector:userInfo:repeats:",
		0.03, view, sel("tick:"), objc.ID(0), true)
	_ = timer

	if pendingOpen != "" {
		a.DropFiles([]string{pendingOpen})
		render()
	} else if len(os.Args) > 1 && !strings.HasPrefix(os.Args[1], "-psn") {
		if _, err := os.Stat(os.Args[1]); err == nil {
			a.DropFiles([]string{os.Args[1]})
			render()
		}
	}
	msg(nsApp, "run")
}
