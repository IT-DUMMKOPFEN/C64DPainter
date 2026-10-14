//go:build linux

// C64 DPainter – Linux front end (X11 protocol in pure Go, also works on
// Wayland desktops through XWayland). No cgo, no libraries needed.
package main

import (
	"bytes"
	_ "embed"
	"fmt"
	"image/png"
	"io"
	"log"
	"math"
	"os"
	"os/exec"
	"path/filepath"
	"runtime/debug"
	"strconv"
	"strings"
	"time"

	"github.com/jezek/xgb"
	"github.com/jezek/xgb/xproto"

	"c64dpainter/internal/app"
	"c64dpainter/internal/gfx"
)

//go:embed appicon.png
var iconPNG []byte

var (
	conn       *xgb.Conn
	screen     *xproto.ScreenInfo
	win        xproto.Window
	gc         xproto.Gcontext
	a          *app.App
	frame      *gfx.Image
	scaled     []byte
	scale      = 1
	dpiScale   = 1
	winW, winH int
	maxReq     int
	startTime  = time.Now()
	events     = make(chan xgb.Event, 1024)
	inModal    int
	quit       bool
	buttons    [3]bool
	atoms      = map[string]xproto.Atom{}
	minKeycode xproto.Keycode
	keysPer    int
	keysyms    []xproto.Keysym
	cursorFont xproto.Font
	cursors    = map[int]xproto.Cursor{}
	curCursor  = -1
	lastClickT time.Time
	lastClickB int
	lastClickX int
	lastClickY int
	dirty      bool
	crashed    bool
)

func atom(name string) xproto.Atom {
	if at, ok := atoms[name]; ok {
		return at
	}
	r, err := xproto.InternAtom(conn, false, uint16(len(name)), name).Reply()
	if err != nil {
		return 0
	}
	atoms[name] = r.Atom
	return r.Atom
}

func setProp(prop, typ string, format byte, data []byte) {
	n := uint32(len(data)) / uint32(format/8)
	xproto.ChangeProperty(conn, xproto.PropModeReplace, win, atom(prop), atom(typ), format, n, data)
}

func u32s(v ...uint32) []byte {
	b := make([]byte, 4*len(v))
	for i, x := range v {
		xgb.Put32(b[i*4:], x)
	}
	return b
}

// --- host -------------------------------------------------------------------------

type linuxHost struct{}

func dialogTool() string {
	if os.Getenv("C64DPAINTER_DIALOGS") == "builtin" {
		return ""
	}
	for _, t := range []string{"zenity", "kdialog"} {
		if p, err := exec.LookPath(t); err == nil {
			return p
		}
	}
	return ""
}

// runExternal runs a native dialog program while keeping the window painted.
func runExternal(name string, args ...string) (string, bool) {
	cmd := exec.Command(name, args...)
	var out bytes.Buffer
	cmd.Stdout = &out
	if err := cmd.Start(); err != nil {
		return "", false
	}
	done := make(chan error, 1)
	go func() { done <- cmd.Wait() }()
	inModal++
	defer func() { inModal--; releaseAll() }()
	for {
		select {
		case err := <-done:
			p := strings.TrimSpace(out.String())
			return p, err == nil && p != ""
		case ev := <-events:
			handleEvent(ev) // repaints only while inModal
		}
	}
}

func (linuxHost) OpenFile(title string, fs []app.Filter, dir string) (string, bool) {
	return fileDialog(false, title, fs, dir, "", "")
}

func (linuxHost) SaveFile(title string, fs []app.Filter, dir, name, defExt string) (string, bool) {
	p, ok := fileDialog(true, title, fs, dir, name, defExt)
	if ok && filepath.Ext(p) == "" && defExt != "" {
		p += "." + defExt
	}
	return p, ok
}

func fileDialog(save bool, title string, fs []app.Filter, dir, name, defExt string) (string, bool) {
	tool := dialogTool()
	start := dir
	if start == "" {
		start, _ = os.UserHomeDir()
	}
	switch filepath.Base(tool) {
	case "zenity":
		args := []string{"--file-selection", "--title=" + title, "--filename=" + filepath.Join(start, name)}
		if !save && name == "" {
			args[2] = "--filename=" + start + "/"
		}
		if save {
			args = append(args, "--save", "--confirm-overwrite")
		}
		for _, f := range fs {
			args = append(args, "--file-filter="+f.Name+" | "+strings.ReplaceAll(f.Pattern, ";", " "))
		}
		return runExternal(tool, args...)
	case "kdialog":
		var filt []string
		for _, f := range fs {
			filt = append(filt, strings.ReplaceAll(f.Pattern, ";", " ")+"|"+f.Name)
		}
		mode := "--getopenfilename"
		if save {
			mode = "--getsavefilename"
		}
		return runExternal(tool, mode, filepath.Join(start, name), strings.Join(filt, "\n"), "--title", title)
	}
	inModal++
	defer func() { inModal--; releaseAll() }()
	return a.SyncFileDialog(pump, save, title, fs, dir, name, defExt)
}

func (linuxHost) Ask(title, text string) int {
	inModal++
	defer func() { inModal--; releaseAll() }()
	return a.SyncMessage(pump, title, text, []string{"Save", "Don't save", "Cancel"})
}

func (linuxHost) Info(title, text string) {
	inModal++
	defer func() { inModal--; releaseAll() }()
	a.SyncMessage(pump, title, text, []string{"OK"})
}

func (linuxHost) Quit() { quit = true }

func (linuxHost) SetTitle(s string) {
	if win == 0 {
		return
	}
	setProp("_NET_WM_NAME", "UTF8_STRING", 8, []byte(s))
	setProp("WM_NAME", "STRING", 8, []byte(s))
}

func (linuxHost) ReadFile(p string) ([]byte, error)  { return os.ReadFile(p) }
func (linuxHost) WriteFile(p string, d []byte) error { return os.WriteFile(p, d, 0o644) }

// pump processes window events and draws one frame (for the built-in dialogs).
func pump() bool {
	select {
	case ev := <-events:
		dispatch(ev)
	case <-time.After(40 * time.Millisecond):
	}
	for drained := false; !drained; {
		select {
		case ev := <-events:
			dispatch(ev)
		default:
			drained = true
		}
	}
	computeScale()
	a.Resize((winW+scale-1)/scale, (winH+scale-1)/scale)
	a.Tick(time.Since(startTime).Milliseconds())
	frame = a.Frame()
	blit()
	return !quit
}

// --- drawing ------------------------------------------------------------------------

func blit() {
	if frame == nil {
		return
	}
	w, h := frame.W*scale, frame.H*scale
	if w > winW {
		w = winW
	}
	if h > winH {
		h = winH
	}
	if w <= 0 || h <= 0 {
		return
	}
	need := w * h * 4
	if cap(scaled) < need {
		scaled = make([]byte, need)
	}
	buf := scaled[:need]
	fw := frame.W
	for y := 0; y < h; y++ {
		src := frame.Pix[(y/scale)*fw:]
		row := buf[y*w*4 : (y+1)*w*4]
		for x := 0; x < w; x++ {
			c := src[x/scale]
			row[x*4] = byte(c)
			row[x*4+1] = byte(c >> 8)
			row[x*4+2] = byte(c >> 16)
			row[x*4+3] = 0
		}
	}
	rows := (maxReq - 64) / (w * 4)
	if rows < 1 {
		rows = 1
	}
	for y := 0; y < h; y += rows {
		n := rows
		if y+n > h {
			n = h - y
		}
		xproto.PutImage(conn, xproto.ImageFormatZPixmap, xproto.Drawable(win), gc,
			uint16(w), uint16(n), 0, int16(y), 0, screen.RootDepth, buf[y*w*4:(y+n)*w*4])
	}
	conn.Sync()
}

func computeScale() {
	scale = a.UIScale()
}

// usable screen area (minus room for panels and the title bar)
func screenArea() (int, int) {
	return int(float64(screen.WidthInPixels) * 0.97), int(float64(screen.HeightInPixels) * 0.92)
}

func (linuxHost) ScreenSize() (int, int) { return screenArea() }

func windowSizeFor(s int) (w, h, minW, minH int) {
	sw, sh := screenArea()
	return min(app.PrefLogicalW*s, sw), min(app.PrefLogicalH*s, sh),
		min(app.MinLogicalW*s, sw), min(app.MinLogicalH*s, sh)
}

func setSizeHints(minW, minH int) {
	hints := make([]uint32, 18)
	hints[0] = 16 // PMinSize
	hints[5], hints[6] = uint32(minW), uint32(minH)
	setProp("WM_NORMAL_HINTS", "WM_SIZE_HINTS", 32, u32s(hints...))
}

func (linuxHost) ApplyScale(s int) {
	scale = s
	w, h, mw, mh := windowSizeFor(s)
	setSizeHints(mw, mh)
	x := (int(screen.WidthInPixels) - w) / 2
	y := (int(screen.HeightInPixels) - h) / 2
	xproto.ConfigureWindow(conn, win, xproto.ConfigWindowX|xproto.ConfigWindowY|xproto.ConfigWindowWidth|xproto.ConfigWindowHeight,
		[]uint32{uint32(x), uint32(y), uint32(w), uint32(h)})
	conn.Sync()
}

func render() {
	if a == nil || winW <= 0 || winH <= 0 {
		return
	}
	computeScale()
	a.Resize((winW+scale-1)/scale, (winH+scale-1)/scale)
	a.Tick(time.Since(startTime).Milliseconds())
	frame = a.Frame()
	if a.HasDeferred() {
		blit()
		inModal++
		a.RunDeferred()
		inModal--
		releaseAll()
		computeScale()
		a.Resize((winW+scale-1)/scale, (winH+scale-1)/scale)
		frame = a.Frame()
	}
	blit()
	updateCursor()
}

// --- input ---------------------------------------------------------------------------

func loadKeymap() {
	setup := xproto.Setup(conn)
	minKeycode = setup.MinKeycode
	count := byte(setup.MaxKeycode - setup.MinKeycode + 1)
	r, err := xproto.GetKeyboardMapping(conn, setup.MinKeycode, count).Reply()
	if err != nil {
		return
	}
	keysPer = int(r.KeysymsPerKeycode)
	keysyms = r.Keysyms
}

func keysym(code xproto.Keycode, idx int) xproto.Keysym {
	if keysPer == 0 || idx >= keysPer {
		return 0
	}
	i := int(code-minKeycode)*keysPer + idx
	if i < 0 || i >= len(keysyms) {
		return 0
	}
	return keysyms[i]
}

func symFor(code xproto.Keycode, state uint16) xproto.Keysym {
	shift := state&xproto.ModMaskShift != 0
	lock := state&xproto.ModMaskLock != 0
	base := 0
	if state&xproto.ModMask5 != 0 && keysym(code, 4) != 0 { // AltGr
		base = 4
	}
	s0 := keysym(code, base)
	s1 := keysym(code, base+1)
	isLetter := (s0 >= 'a' && s0 <= 'z') || (s0 >= 0xe0 && s0 <= 0xfe && s0 != 0xf7)
	if lock && isLetter {
		shift = !shift
	}
	if shift && s1 != 0 {
		return s1
	}
	return s0
}

func symRune(s xproto.Keysym) rune {
	switch {
	case s >= 0x20 && s <= 0x7e, s >= 0xa0 && s <= 0xff:
		return rune(s)
	case s&0xff000000 == 0x01000000:
		return rune(s & 0x00ffffff)
	case s >= 0xffb0 && s <= 0xffb9: // keypad digits
		return rune('0' + s - 0xffb0)
	case s == 0xffab:
		return '+'
	case s == 0xffad:
		return '-'
	}
	return 0
}

func appKey(code xproto.Keycode, state uint16) int {
	s := keysym(code, 0)
	switch s {
	case 0xff1b:
		return app.KeyEsc
	case 0xff0d, 0xff8d:
		return app.KeyEnter
	case 0xff08:
		return app.KeyBackspace
	case 0xffff:
		return app.KeyDelete
	case 0xff09, 0xfe20:
		return app.KeyTab
	case 0x20:
		return app.KeySpace
	case 0xff51:
		return app.KeyLeft
	case 0xff52:
		return app.KeyUp
	case 0xff53:
		return app.KeyRight
	case 0xff54:
		return app.KeyDown
	case 0xff50:
		return app.KeyHome
	case 0xff57:
		return app.KeyEnd
	case 0xff55:
		return app.KeyPgUp
	case 0xff56:
		return app.KeyPgDn
	case 0xff63:
		return app.KeyInsert
	case 0xffab, 0x2b:
		return app.KeyPlus
	case 0xffad, 0x2d:
		return app.KeyMinus
	}
	if s >= 0xffbe && s <= 0xffc9 {
		return app.KeyF1 + int(s-0xffbe)
	}
	if s >= 'a' && s <= 'z' {
		return int(s - 32)
	}
	if (s >= 'A' && s <= 'Z') || (s >= '0' && s <= '9') {
		return int(s)
	}
	// keys whose unshifted symbol is not a letter, e.g. "+" on German keyboards
	if r := symRune(symFor(code, state&^xproto.ModMaskControl)); r == '+' || r == '=' {
		return app.KeyPlus
	}
	return 0
}

func modsOf(state uint16) app.Mods {
	return app.Mods{
		Shift: state&xproto.ModMaskShift != 0,
		Ctrl:  state&xproto.ModMaskControl != 0,
		Alt:   state&xproto.ModMask1 != 0,
	}
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

// cursor font glyphs
var cursorGlyphs = map[int]uint16{
	app.CursorArrow: 68,  // left_ptr
	app.CursorCross: 34,  // crosshair
	app.CursorHand:  58,  // hand1
	app.CursorMove:  52,  // fleur
	app.CursorSize:  120, // sizing
}

func updateCursor() {
	if a == nil || inModal > 0 {
		return
	}
	k := a.Cursor()
	if k == curCursor {
		return
	}
	c, ok := cursors[k]
	if !ok {
		id, err := xproto.NewCursorId(conn)
		if err != nil {
			return
		}
		g := cursorGlyphs[k]
		xproto.CreateGlyphCursor(conn, id, cursorFont, cursorFont, g, g+1, 0, 0, 0, 0xffff, 0xffff, 0xffff)
		cursors[k] = id
		c = id
	}
	xproto.ChangeWindowAttributes(conn, win, xproto.CwCursor, []uint32{uint32(c)})
	curCursor = k
}

func mouseButton(detail xproto.Button) int {
	switch detail {
	case 1:
		return 0
	case 2:
		return 2
	case 3:
		return 1
	}
	return -1
}

// handleEvent: while a native dialog is open only repaint.
func handleEvent(ev xgb.Event) {
	switch e := ev.(type) {
	case xproto.ExposeEvent:
		if e.Count == 0 {
			blit()
		}
	case xproto.ConfigureNotifyEvent:
		if int(e.Width) != winW || int(e.Height) != winH {
			winW, winH = int(e.Width), int(e.Height)
			dirty = true
		}
	}
}

// dispatch feeds one X event into the app. Returns true if a frame is needed now.
func dispatch(ev xgb.Event) bool {
	if ev == nil {
		return false
	}
	switch e := ev.(type) {
	case xproto.ExposeEvent:
		if e.Count == 0 {
			blit()
		}
	case xproto.ConfigureNotifyEvent:
		if int(e.Width) != winW || int(e.Height) != winH {
			winW, winH = int(e.Width), int(e.Height)
			dirty = true
		}
	case xproto.MotionNotifyEvent:
		a.SetMods(modsOf(e.State))
		a.MouseMove(int(e.EventX)/scale, int(e.EventY)/scale)
		dirty = true
	case xproto.ButtonPressEvent:
		x, y := int(e.EventX)/scale, int(e.EventY)/scale
		a.SetMods(modsOf(e.State))
		switch e.Detail {
		case 4, 5, 6, 7:
			d := 1
			if e.Detail == 5 || e.Detail == 7 {
				d = -1
			}
			if e.Detail >= 6 {
				m := modsOf(e.State)
				m.Shift = true
				a.SetMods(m)
			}
			a.Wheel(d, x, y)
			return true
		}
		b := mouseButton(e.Detail)
		if b < 0 {
			return false
		}
		now := time.Now()
		double := b == lastClickB && now.Sub(lastClickT) < 400*time.Millisecond &&
			abs(x-lastClickX) < 5 && abs(y-lastClickY) < 5
		lastClickT, lastClickB, lastClickX, lastClickY = now, b, x, y
		buttons[b] = true
		a.MouseDown(b, x, y, double)
		return true
	case xproto.ButtonReleaseEvent:
		b := mouseButton(e.Detail)
		if b < 0 || !buttons[b] {
			return false
		}
		buttons[b] = false
		a.SetMods(modsOf(e.State))
		a.MouseUp(b, int(e.EventX)/scale, int(e.EventY)/scale)
		return true
	case xproto.KeyPressEvent:
		m := modsOf(e.State)
		if k := appKey(e.Detail, e.State); k != 0 {
			a.KeyDown(k, m)
		} else {
			a.SetMods(m)
		}
		if !m.Ctrl && !m.Alt {
			if r := symRune(symFor(e.Detail, e.State)); r >= 32 {
				a.Char(r)
			}
		}
		return true
	case xproto.KeyReleaseEvent:
		m := modsOf(e.State)
		if k := appKey(e.Detail, e.State); k != 0 {
			a.KeyUp(k, m)
		}
	case xproto.FocusOutEvent:
		releaseAll()
		dirty = true
	case xproto.MappingNotifyEvent:
		loadKeymap()
	case xproto.ClientMessageEvent:
		if e.Type == atom("WM_PROTOCOLS") && len(e.Data.Data32) > 0 && xproto.Atom(e.Data.Data32[0]) == atom("WM_DELETE_WINDOW") {
			if inModal == 0 && a.CanClose() {
				quit = true
			}
		}
	}
	return false
}

func abs(v int) int {
	if v < 0 {
		return -v
	}
	return v
}

// --- setup ----------------------------------------------------------------------------

func readDPI() {
	r, err := xproto.GetProperty(conn, false, screen.Root, xproto.AtomResourceManager, xproto.AtomString, 0, 1<<16).Reply()
	if err != nil || r == nil {
		return
	}
	for _, line := range strings.Split(string(r.Value), "\n") {
		if strings.HasPrefix(line, "Xft.dpi:") {
			v, err := strconv.ParseFloat(strings.TrimSpace(line[8:]), 64)
			if err == nil && v > 0 {
				dpiScale = int(math.Floor(v/96 + 0.25))
				if dpiScale < 1 {
					dpiScale = 1
				}
			}
		}
	}
}

func setIcon() {
	img, err := png.Decode(bytes.NewReader(iconPNG))
	if err != nil {
		return
	}
	var data []uint32
	for _, s := range []int{16, 32, 48, 64, 128} {
		data = append(data, uint32(s), uint32(s))
		b := img.Bounds()
		for y := 0; y < s; y++ {
			for x := 0; x < s; x++ {
				c := img.At(b.Min.X+x*b.Dx()/s, b.Min.Y+y*b.Dy()/s)
				r, g, bl, al := c.RGBA()
				data = append(data, (al>>8)<<24|(r>>8)<<16|(g>>8)<<8|bl>>8)
			}
		}
	}
	setProp("_NET_WM_ICON", "CARDINAL", 32, u32s(data...))
}

func fail(msg string) {
	fmt.Fprintln(os.Stderr, "C64 DPainter: "+msg)
	if tool := dialogTool(); filepath.Base(tool) == "zenity" {
		exec.Command(tool, "--error", "--text="+msg).Run()
	}
	os.Exit(1)
}

func main() {
	app.Linux = true
	xgb.Logger = log.New(io.Discard, "", 0)
	var err error
	conn, err = xgb.NewConn()
	if err != nil {
		fail("cannot connect to the X server (" + err.Error() + ").\nC64 DPainter needs an X11 desktop or XWayland.")
	}
	defer conn.Close()
	setup := xproto.Setup(conn)
	screen = setup.DefaultScreen(conn)
	if screen.RootDepth != 24 && screen.RootDepth != 32 {
		fail(fmt.Sprintf("unsupported screen depth %d (24-bit color needed)", screen.RootDepth))
	}
	maxReq = int(setup.MaximumRequestLength) * 4
	if maxReq < 16384 {
		maxReq = 16384
	}
	readDPI()
	loadKeymap()
	a = app.New(linuxHost{})
	// interface size from the settings; on the first start from Xft.dpi
	a.InitScale(dpiScale)
	scale = a.UIScale()
	var minW, minH int
	winW, winH, minW, minH = windowSizeFor(scale)
	win, err = xproto.NewWindowId(conn)
	if err != nil {
		fail(err.Error())
	}
	mask := uint32(xproto.EventMaskExposure | xproto.EventMaskKeyPress | xproto.EventMaskKeyRelease |
		xproto.EventMaskButtonPress | xproto.EventMaskButtonRelease | xproto.EventMaskPointerMotion |
		xproto.EventMaskStructureNotify | xproto.EventMaskFocusChange)
	xproto.CreateWindow(conn, screen.RootDepth, win, screen.Root,
		int16((int(screen.WidthInPixels)-winW)/2), int16((int(screen.HeightInPixels)-winH)/2),
		uint16(winW), uint16(winH), 0, xproto.WindowClassInputOutput, screen.RootVisual,
		xproto.CwBackPixel|xproto.CwEventMask, []uint32{0x202020, mask})
	setProp("WM_PROTOCOLS", "ATOM", 32, u32s(uint32(atom("WM_DELETE_WINDOW"))))
	setProp("WM_CLASS", "STRING", 8, []byte("c64dpainter\x00C64 DPainter\x00"))
	setSizeHints(minW, minH)
	setProp("_NET_WM_PID", "CARDINAL", 32, u32s(uint32(os.Getpid())))
	setIcon()
	gc, _ = xproto.NewGcontextId(conn)
	xproto.CreateGC(conn, gc, xproto.Drawable(win), 0, nil)
	cursorFont, _ = xproto.NewFontId(conn)
	xproto.OpenFont(conn, cursorFont, uint16(len("cursor")), "cursor")

	a.RefreshTitle()
	xproto.MapWindow(conn, win)

	go func() {
		for {
			ev, xerr := conn.WaitForEvent()
			if ev == nil && xerr == nil {
				close(events)
				return
			}
			if ev != nil {
				events <- ev
			}
		}
	}()

	if len(os.Args) > 1 {
		if _, err := os.Stat(os.Args[1]); err == nil {
			a.DropFiles([]string{os.Args[1]})
		}
	}
	render()
	ticker := time.NewTicker(30 * time.Millisecond)
	defer ticker.Stop()
	for !quit {
		select {
		case ev, ok := <-events:
			if !ok {
				return
			}
			safe(func() {
				if dispatch(ev) {
					render()
					dirty = false
				}
			})
		case <-ticker.C:
			safe(func() {
				if a.Tick(time.Since(startTime).Milliseconds()) {
					dirty = true
				}
			})
		}
		// draw after the queue is empty (coalesces mouse movement)
		if dirty && len(events) == 0 && !quit {
			safe(render)
			dirty = false
		}
	}
	xproto.DestroyWindow(conn, win)
	conn.Sync()
}

// safe catches internal errors so a bug does not lose the picture.
func safe(f func()) {
	defer func() {
		if r := recover(); r != nil {
			inModal = 0
			m := fmt.Sprintf("%v\n\n%s", r, debug.Stack())
			if d, err := os.UserConfigDir(); err == nil {
				p := filepath.Join(d, "C64DPainter")
				os.MkdirAll(p, 0o755)
				os.WriteFile(filepath.Join(p, "error.log"), []byte(time.Now().Format(time.RFC3339)+"\n"+m), 0o644)
			}
			fmt.Fprintln(os.Stderr, m)
			if !crashed {
				crashed = true
				a.SyncMessage(pump, app.AppName, "An internal error occurred:\n\n"+fmt.Sprint(r)+"\n\nThe program keeps running. Please save your picture to be safe.\nDetails: ~/.config/C64DPainter/error.log", []string{"OK"})
				crashed = false
			}
		}
	}()
	f()
}
