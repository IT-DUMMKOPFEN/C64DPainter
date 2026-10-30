//go:build windows

// C64 DPainter – Windows-Hauptprogramm (reines Win32 über syscall, ohne cgo).
package main

import (
	"fmt"
	"math"
	"os"
	"path/filepath"
	"runtime"
	"runtime/debug"
	"strings"
	"syscall"
	"time"
	"unsafe"

	"c64dpainter/internal/app"
	"c64dpainter/internal/gfx"
)

var (
	user32   = syscall.NewLazyDLL("user32.dll")
	gdi32    = syscall.NewLazyDLL("gdi32.dll")
	kernel32 = syscall.NewLazyDLL("kernel32.dll")
	comdlg32 = syscall.NewLazyDLL("comdlg32.dll")
	shell32  = syscall.NewLazyDLL("shell32.dll")

	pRegisterClassExW              = user32.NewProc("RegisterClassExW")
	pCreateWindowExW               = user32.NewProc("CreateWindowExW")
	pDefWindowProcW                = user32.NewProc("DefWindowProcW")
	pShowWindow                    = user32.NewProc("ShowWindow")
	pUpdateWindow                  = user32.NewProc("UpdateWindow")
	pGetMessageW                   = user32.NewProc("GetMessageW")
	pTranslateMessage              = user32.NewProc("TranslateMessage")
	pDispatchMessageW              = user32.NewProc("DispatchMessageW")
	pPostQuitMessage               = user32.NewProc("PostQuitMessage")
	pLoadCursorW                   = user32.NewProc("LoadCursorW")
	pSetCursor                     = user32.NewProc("SetCursor")
	pBeginPaint                    = user32.NewProc("BeginPaint")
	pEndPaint                      = user32.NewProc("EndPaint")
	pInvalidateRect                = user32.NewProc("InvalidateRect")
	pGetClientRect                 = user32.NewProc("GetClientRect")
	pSetCapture                    = user32.NewProc("SetCapture")
	pReleaseCapture                = user32.NewProc("ReleaseCapture")
	pMessageBoxW                   = user32.NewProc("MessageBoxW")
	pSetWindowTextW                = user32.NewProc("SetWindowTextW")
	pSetTimer                      = user32.NewProc("SetTimer")
	pGetKeyState                   = user32.NewProc("GetKeyState")
	pDestroyWindow                 = user32.NewProc("DestroyWindow")
	pLoadImageW                    = user32.NewProc("LoadImageW")
	pScreenToClient                = user32.NewProc("ScreenToClient")
	pSetWindowPos                  = user32.NewProc("SetWindowPos")
	pGetDpiForWindow               = user32.NewProc("GetDpiForWindow")
	pSetProcessDpiAwarenessContext = user32.NewProc("SetProcessDpiAwarenessContext")
	pSetProcessDPIAware            = user32.NewProc("SetProcessDPIAware")
	pSystemParametersInfoW         = user32.NewProc("SystemParametersInfoW")
	pGetDC                         = user32.NewProc("GetDC")
	pReleaseDC                     = user32.NewProc("ReleaseDC")
	pAdjustWindowRectEx            = user32.NewProc("AdjustWindowRectEx")
	pMonitorFromWindow             = user32.NewProc("MonitorFromWindow")
	pGetMonitorInfoW               = user32.NewProc("GetMonitorInfoW")
	pIsZoomed                      = user32.NewProc("IsZoomed")
	pGetWindowPlacement            = user32.NewProc("GetWindowPlacement")
	pSetWindowPlacement            = user32.NewProc("SetWindowPlacement")
	pMonitorFromRect               = user32.NewProc("MonitorFromRect")

	pStretchDIBits     = gdi32.NewProc("StretchDIBits")
	pSetStretchBltMode = gdi32.NewProc("SetStretchBltMode")
	pGetDeviceCaps     = gdi32.NewProc("GetDeviceCaps")

	pGetModuleHandleW = kernel32.NewProc("GetModuleHandleW")

	pGetOpenFileNameW = comdlg32.NewProc("GetOpenFileNameW")
	pGetSaveFileNameW = comdlg32.NewProc("GetSaveFileNameW")

	pDragAcceptFiles = shell32.NewProc("DragAcceptFiles")
	pDragQueryFileW  = shell32.NewProc("DragQueryFileW")
	pDragFinish      = shell32.NewProc("DragFinish")
)

const (
	wmDestroy        = 0x0002
	wmSize           = 0x0005
	wmActivate       = 0x0006
	wmKillFocus      = 0x0008
	wmPaint          = 0x000F
	wmClose          = 0x0010
	wmEraseBkgnd     = 0x0014
	wmSetCursor      = 0x0020
	wmGetMinMaxInfo  = 0x0024
	wmKeyDown        = 0x0100
	wmKeyUp          = 0x0101
	wmChar           = 0x0102
	wmSysKeyDown     = 0x0104
	wmSysKeyUp       = 0x0105
	wmSysChar        = 0x0106
	wmTimer          = 0x0113
	wmMouseMove      = 0x0200
	wmLButtonDown    = 0x0201
	wmLButtonUp      = 0x0202
	wmLButtonDbl     = 0x0203
	wmRButtonDown    = 0x0204
	wmRButtonUp      = 0x0205
	wmRButtonDbl     = 0x0206
	wmMButtonDown    = 0x0207
	wmMButtonUp      = 0x0208
	wmMButtonDbl     = 0x0209
	wmMouseWheel     = 0x020A
	wmMouseHWheel    = 0x020E
	wmCaptureChanged = 0x0215
	wmDropFiles      = 0x0233
	wmDpiChanged     = 0x02E0

	wsOverlappedWindow = 0x00CF0000
	csVRedraw          = 0x0001
	csHRedraw          = 0x0002
	csDblClks          = 0x0008
	swShow             = 5
	swShowMaximized    = 3
	idcArrow           = 32512
	idcCross           = 32515
	idcSizeAll         = 32646
	idcHand            = 32649
	idcSizeNWSE        = 32642
	htClient           = 1
)

type wndClassEx struct {
	cbSize        uint32
	style         uint32
	lpfnWndProc   uintptr
	cbClsExtra    int32
	cbWndExtra    int32
	hInstance     uintptr
	hIcon         uintptr
	hCursor       uintptr
	hbrBackground uintptr
	lpszMenuName  *uint16
	lpszClassName *uint16
	hIconSm       uintptr
}

type point struct{ X, Y int32 }

type windowPlacement struct {
	length, flags, showCmd uint32
	minPos, maxPos         point
	normal                 rect
}

// rememberWindow stores the window position for the next start.
func rememberWindow() {
	var wp windowPlacement
	wp.length = uint32(unsafe.Sizeof(wp))
	if r, _, _ := pGetWindowPlacement.Call(hwnd, uintptr(unsafe.Pointer(&wp))); r == 0 {
		return
	}
	n := wp.normal
	a.RememberWindow(app.WindowState{X: int(n.Left), Y: int(n.Top), W: int(n.Right - n.Left), H: int(n.Bottom - n.Top), Max: wp.showCmd == 3})
}

// restoreWindow moves the window to where it was in the last session (when
// that place is still on a screen).
func restoreWindow(st app.WindowState) {
	rc := rect{int32(st.X), int32(st.Y), int32(st.X + st.W), int32(st.Y + st.H)}
	if m, _, _ := pMonitorFromRect.Call(uintptr(unsafe.Pointer(&rc)), 0); m == 0 {
		return // MONITOR_DEFAULTTONULL: the screen is gone
	}
	wp := windowPlacement{showCmd: 1, normal: rc}
	if st.Max {
		wp.showCmd = 3
	}
	wp.length = uint32(unsafe.Sizeof(wp))
	pSetWindowPlacement.Call(hwnd, uintptr(unsafe.Pointer(&wp)))
}

type msg struct {
	hwnd     uintptr
	message  uint32
	wParam   uintptr
	lParam   uintptr
	time     uint32
	pt       point
	lPrivate uint32
}

type rect struct{ Left, Top, Right, Bottom int32 }

type paintStruct struct {
	hdc         uintptr
	fErase      int32
	rcPaint     rect
	fRestore    int32
	fIncUpdate  int32
	rgbReserved [32]byte
}

type bitmapInfoHeader struct {
	biSize          uint32
	biWidth         int32
	biHeight        int32
	biPlanes        uint16
	biBitCount      uint16
	biCompression   uint32
	biSizeImage     uint32
	biXPelsPerMeter int32
	biYPelsPerMeter int32
	biClrUsed       uint32
	biClrImportant  uint32
}

type bitmapInfo struct {
	header bitmapInfoHeader
	colors [4]byte
}

type minMaxInfo struct {
	ptReserved     point
	ptMaxSize      point
	ptMaxPosition  point
	ptMinTrackSize point
	ptMaxTrackSize point
}

type openFileName struct {
	lStructSize       uint32
	hwndOwner         uintptr
	hInstance         uintptr
	lpstrFilter       *uint16
	lpstrCustomFilter *uint16
	nMaxCustFilter    uint32
	nFilterIndex      uint32
	lpstrFile         *uint16
	nMaxFile          uint32
	lpstrFileTitle    *uint16
	nMaxFileTitle     uint32
	lpstrInitialDir   *uint16
	lpstrTitle        *uint16
	Flags             uint32
	nFileOffset       uint16
	nFileExtension    uint16
	lpstrDefExt       *uint16
	lCustData         uintptr
	lpfnHook          uintptr
	lpTemplateName    *uint16
	pvReserved        uintptr
	dwReserved        uint32
	FlagsEx           uint32
}

// --- Zustand -------------------------------------------------------------------------

var (
	hwnd      uintptr
	hInst     uintptr
	a         *app.App
	frame     *gfx.Image
	scale     = 1
	dpi       = 96
	clientW   int
	clientH   int
	inModal   int
	startTime = time.Now()
	cursors   = map[int]uintptr{}
	buttons   [3]bool
)

func u16(s string) *uint16 {
	p, _ := syscall.UTF16PtrFromString(s)
	return p
}

func loword(v uintptr) int { return int(int16(v & 0xFFFF)) }
func hiword(v uintptr) int { return int(int16((v >> 16) & 0xFFFF)) }

// --- Host-Schnittstelle --------------------------------------------------------------

type winHost struct{}

func filterString(fs []app.Filter) []uint16 {
	var out []uint16
	for _, f := range fs {
		n, _ := syscall.UTF16FromString(f.Name)
		p, _ := syscall.UTF16FromString(f.Pattern)
		out = append(out, n...) // enthält bereits die 0
		out = append(out, p...)
	}
	return append(out, 0)
}

func (winHost) fileDialog(save bool, title string, fs []app.Filter, dir, name, defExt string) (string, bool) {
	buf := make([]uint16, 4096)
	if name != "" {
		n, _ := syscall.UTF16FromString(name)
		copy(buf, n)
	}
	filter := filterString(fs)
	var ofn openFileName
	ofn.lStructSize = uint32(unsafe.Sizeof(ofn))
	ofn.hwndOwner = hwnd
	ofn.lpstrFilter = &filter[0]
	ofn.nFilterIndex = 1
	ofn.lpstrFile = &buf[0]
	ofn.nMaxFile = uint32(len(buf))
	ofn.lpstrTitle = u16(title)
	if dir != "" {
		ofn.lpstrInitialDir = u16(dir)
	}
	if defExt != "" {
		ofn.lpstrDefExt = u16(defExt)
	}
	const (
		ofnOverwritePrompt = 0x2
		ofnNoChangeDir     = 0x8
		ofnPathMustExist   = 0x800
		ofnFileMustExist   = 0x1000
		ofnExplorer        = 0x80000
	)
	ofn.Flags = ofnExplorer | ofnNoChangeDir | ofnPathMustExist
	proc := pGetOpenFileNameW
	if save {
		ofn.Flags |= ofnOverwritePrompt
		proc = pGetSaveFileNameW
	} else {
		ofn.Flags |= ofnFileMustExist
	}
	inModal++
	r, _, _ := proc.Call(uintptr(unsafe.Pointer(&ofn)))
	inModal--
	runtime.KeepAlive(filter)
	runtime.KeepAlive(buf)
	if r == 0 {
		return "", false
	}
	return syscall.UTF16ToString(buf), true
}

func (h winHost) OpenFile(title string, fs []app.Filter, dir string) (string, bool) {
	return h.fileDialog(false, title, fs, dir, "", "")
}

func (h winHost) SaveFile(title string, fs []app.Filter, dir, name, defExt string) (string, bool) {
	p, ok := h.fileDialog(true, title, fs, dir, name, defExt)
	if ok && filepath.Ext(p) == "" && defExt != "" {
		p += "." + defExt
	}
	return p, ok
}

func msgBox(text, title string, flags uintptr) int {
	inModal++
	r, _, _ := pMessageBoxW.Call(hwnd, uintptr(unsafe.Pointer(u16(text))), uintptr(unsafe.Pointer(u16(title))), flags)
	inModal--
	releaseAll()
	return int(r)
}

func (winHost) Ask(title, text string) int {
	switch msgBox(text, title, 0x3|0x20) { // MB_YESNOCANCEL | MB_ICONQUESTION
	case 6:
		return 0
	case 7:
		return 1
	}
	return 2
}

func (winHost) Info(title, text string)            { msgBox(text, title, 0x40) }
func (winHost) Quit()                              { rememberWindow(); pDestroyWindow.Call(hwnd) }
func (winHost) SetTitle(s string)                  { pSetWindowTextW.Call(hwnd, uintptr(unsafe.Pointer(u16(s)))) }
func (winHost) ReadFile(p string) ([]byte, error)  { return os.ReadFile(p) }
func (winHost) WriteFile(p string, d []byte) error { return os.WriteFile(p, d, 0o644) }

// --- Darstellung ---------------------------------------------------------------------

func computeScale() {
	scale = a.UIScale()
}

type monitorInfo struct {
	cbSize    uint32
	rcMonitor rect
	rcWork    rect
	dwFlags   uint32
}

// workArea returns the usable area of the monitor that shows the window.
func workArea() rect {
	var mi monitorInfo
	mi.cbSize = uint32(unsafe.Sizeof(mi))
	if hwnd != 0 {
		if mon, _, _ := pMonitorFromWindow.Call(hwnd, 2); mon != 0 { // MONITOR_DEFAULTTONEAREST
			if r, _, _ := pGetMonitorInfoW.Call(mon, uintptr(unsafe.Pointer(&mi))); r != 0 {
				return mi.rcWork
			}
		}
	}
	var work rect
	pSystemParametersInfoW.Call(0x0030, 0, uintptr(unsafe.Pointer(&work)), 0)
	return work
}

// frameSize returns the extra width/height of the window border and title bar.
func frameSize() (int, int) {
	r := rect{0, 0, 1000, 1000}
	pAdjustWindowRectEx.Call(uintptr(unsafe.Pointer(&r)), wsOverlappedWindow, 0, 0)
	return int(r.Right-r.Left) - 1000, int(r.Bottom-r.Top) - 1000
}

// windowRectFor returns the outer window rectangle for an interface size,
// centered in the work area and never larger than it.
func windowRectFor(s int) (x, y, w, h int) {
	work := workArea()
	ww, wh := int(work.Right-work.Left), int(work.Bottom-work.Top)
	fw, fh := frameSize()
	if ww <= 0 || wh <= 0 {
		return 100, 60, app.PrefLogicalW*s + fw, app.PrefLogicalH*s + fh
	}
	w = min(app.PrefLogicalW*s+fw, ww)
	h = min(app.PrefLogicalH*s+fh, wh)
	return int(work.Left) + (ww-w)/2, int(work.Top) + (wh-h)/2, w, h
}

func (winHost) ScreenSize() (int, int) {
	work := workArea()
	fw, fh := frameSize()
	return int(work.Right-work.Left) - fw, int(work.Bottom-work.Top) - fh
}

func (winHost) ApplyScale(s int) {
	scale = s
	if z, _, _ := pIsZoomed.Call(hwnd); z != 0 {
		return // maximized: the window already fills the screen
	}
	x, y, w, h := windowRectFor(s)
	pSetWindowPos.Call(hwnd, 0, uintptr(x), uintptr(y), uintptr(w), uintptr(h), 0x0004|0x0010)
	updateClient()
}

func render() {
	if a == nil || clientW <= 0 || clientH <= 0 {
		return
	}
	computeScale()
	a.Resize((clientW+scale-1)/scale, (clientH+scale-1)/scale)
	a.Tick(time.Since(startTime).Milliseconds())
	frame = a.Frame()
	if a.HasDeferred() {
		pInvalidateRect.Call(hwnd, 0, 0)
		pUpdateWindow.Call(hwnd)
		inModal++
		a.RunDeferred()
		inModal--
		releaseAll()
		if a == nil || hwnd == 0 {
			return
		}
		computeScale()
		a.Resize((clientW+scale-1)/scale, (clientH+scale-1)/scale)
		frame = a.Frame()
	}
	pInvalidateRect.Call(hwnd, 0, 0)
}

func paint() {
	var ps paintStruct
	hdc, _, _ := pBeginPaint.Call(hwnd, uintptr(unsafe.Pointer(&ps)))
	if frame != nil {
		var bi bitmapInfo
		bi.header.biSize = uint32(unsafe.Sizeof(bi.header))
		bi.header.biWidth = int32(frame.W)
		bi.header.biHeight = -int32(frame.H) // von oben nach unten
		bi.header.biPlanes = 1
		bi.header.biBitCount = 32
		pSetStretchBltMode.Call(hdc, 3) // COLORONCOLOR
		pStretchDIBits.Call(hdc,
			0, 0, uintptr(frame.W*scale), uintptr(frame.H*scale),
			0, 0, uintptr(frame.W), uintptr(frame.H),
			uintptr(unsafe.Pointer(&frame.Pix[0])), uintptr(unsafe.Pointer(&bi)),
			0, 0x00CC0020)
		runtime.KeepAlive(frame)
	}
	pEndPaint.Call(hwnd, uintptr(unsafe.Pointer(&ps)))
}

func updateClient() {
	var r rect
	pGetClientRect.Call(hwnd, uintptr(unsafe.Pointer(&r)))
	clientW, clientH = int(r.Right-r.Left), int(r.Bottom-r.Top)
}

func mods() app.Mods {
	ks := func(vk uintptr) bool {
		r, _, _ := pGetKeyState.Call(vk)
		return r&0x8000 != 0
	}
	return app.Mods{Shift: ks(0x10), Ctrl: ks(0x11), Alt: ks(0x12)}
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
	pReleaseCapture.Call()
}

func mapKey(vk uintptr) int {
	switch {
	case vk >= 'A' && vk <= 'Z', vk >= '0' && vk <= '9':
		return int(vk)
	case vk >= 0x60 && vk <= 0x69:
		return int('0' + vk - 0x60)
	case vk >= 0x70 && vk <= 0x7B:
		return app.KeyF1 + int(vk-0x70)
	}
	switch vk {
	case 0x08:
		return app.KeyBackspace
	case 0x09:
		return app.KeyTab
	case 0x0D:
		return app.KeyEnter
	case 0x1B:
		return app.KeyEsc
	case 0x20:
		return app.KeySpace
	case 0x21:
		return app.KeyPgUp
	case 0x22:
		return app.KeyPgDn
	case 0x23:
		return app.KeyEnd
	case 0x24:
		return app.KeyHome
	case 0x25:
		return app.KeyLeft
	case 0x26:
		return app.KeyUp
	case 0x27:
		return app.KeyRight
	case 0x28:
		return app.KeyDown
	case 0x2D:
		return app.KeyInsert
	case 0x2E:
		return app.KeyDelete
	case 0x6B, 0xBB:
		return app.KeyPlus
	case 0x6D, 0xBD:
		return app.KeyMinus
	}
	return 0
}

func setCursor() {
	kind := a.Cursor()
	id := map[int]uintptr{app.CursorArrow: idcArrow, app.CursorCross: idcCross, app.CursorHand: idcHand, app.CursorMove: idcSizeAll, app.CursorSize: idcSizeNWSE}[kind]
	c, ok := cursors[int(id)]
	if !ok {
		c, _, _ = pLoadCursorW.Call(0, id)
		cursors[int(id)] = c
	}
	pSetCursor.Call(c)
}

// --- Fensterprozedur -----------------------------------------------------------------

var crashed bool

// wndProc fängt Programmfehler ab, damit ein Fehler nicht das Bild kostet.
func wndProc(h uintptr, m uint32, wp, lp uintptr) (ret uintptr) {
	defer func() {
		if r := recover(); r != nil {
			inModal = 0
			msg := fmt.Sprintf("%v\n\n%s", r, debug.Stack())
			if d, err := os.UserConfigDir(); err == nil {
				p := filepath.Join(d, "C64DPainter")
				os.MkdirAll(p, 0o755)
				os.WriteFile(filepath.Join(p, "error.log"), []byte(time.Now().Format(time.RFC3339)+"\n"+msg), 0o644)
			}
			if !crashed {
				crashed = true
				inModal++
				pMessageBoxW.Call(hwnd, uintptr(unsafe.Pointer(u16("An internal error occurred:\n\n"+fmt.Sprint(r)+"\n\nThe program keeps running. Please save your picture to be safe.\nDetails: %APPDATA%\\C64DPainter\\error.log"))), uintptr(unsafe.Pointer(u16(app.AppName))), 0x30)
				inModal--
				crashed = false
			}
			ret = 0
		}
	}()
	return wndProcInner(h, m, wp, lp)
}

func wndProcInner(h uintptr, m uint32, wp, lp uintptr) uintptr {
	if a == nil {
		r, _, _ := pDefWindowProcW.Call(h, uintptr(m), wp, lp)
		return r
	}
	busy := inModal > 0
	switch m {
	case wmPaint:
		paint()
		return 0
	case wmEraseBkgnd:
		return 1
	case wmSize:
		updateClient()
		if !busy && clientW > 0 && clientH > 0 {
			render()
		}
		return 0
	case wmGetMinMaxInfo:
		mmi := (*minMaxInfo)(unsafe.Pointer(lp))
		fw, fh := frameSize()
		work := workArea()
		mw := min(app.MinLogicalW*scale+fw, int(work.Right-work.Left))
		mh := min(app.MinLogicalH*scale+fh, int(work.Bottom-work.Top))
		mmi.ptMinTrackSize = point{int32(mw), int32(mh)}
		return 0
	case wmDpiChanged:
		dpi = loword(wp)
		r := (*rect)(unsafe.Pointer(lp))
		pSetWindowPos.Call(h, 0, uintptr(r.Left), uintptr(r.Top), uintptr(r.Right-r.Left), uintptr(r.Bottom-r.Top), 0x0014)
		updateClient()
		render()
		return 0
	case wmSetCursor:
		if lp&0xFFFF == htClient && !busy {
			setCursor()
			return 1
		}
	case wmTimer:
		if !busy && a.Tick(time.Since(startTime).Milliseconds()) {
			render()
		}
		return 0
	case wmMouseMove:
		if busy {
			return 0
		}
		a.MouseMove(loword(lp)/scale, hiword(lp)/scale)
		a.SetMods(mods())
		render()
		return 0
	case wmLButtonDown, wmLButtonDbl, wmRButtonDown, wmRButtonDbl, wmMButtonDown, wmMButtonDbl:
		if busy {
			return 0
		}
		b := 0
		switch m {
		case wmRButtonDown, wmRButtonDbl:
			b = 1
		case wmMButtonDown, wmMButtonDbl:
			b = 2
		}
		buttons[b] = true
		pSetCapture.Call(h)
		a.SetMods(mods())
		a.MouseDown(b, loword(lp)/scale, hiword(lp)/scale, m == wmLButtonDbl || m == wmRButtonDbl || m == wmMButtonDbl)
		render()
		return 0
	case wmLButtonUp, wmRButtonUp, wmMButtonUp:
		if busy {
			return 0
		}
		b := 0
		if m == wmRButtonUp {
			b = 1
		} else if m == wmMButtonUp {
			b = 2
		}
		buttons[b] = false
		if !buttons[0] && !buttons[1] && !buttons[2] {
			pReleaseCapture.Call()
		}
		a.SetMods(mods())
		a.MouseUp(b, loword(lp)/scale, hiword(lp)/scale)
		render()
		return 0
	case wmCaptureChanged:
		if lp != h && !busy {
			releaseAll()
			render()
		}
		return 0
	case wmMouseWheel, wmMouseHWheel:
		if busy {
			return 0
		}
		pt := point{int32(loword(lp)), int32(hiword(lp))}
		pScreenToClient.Call(h, uintptr(unsafe.Pointer(&pt)))
		d := hiword(wp)
		steps := d / 120
		if steps == 0 {
			if d > 0 {
				steps = 1
			} else {
				steps = -1
			}
		}
		md := mods()
		if m == wmMouseHWheel {
			md.Shift = true
			steps = -steps
		}
		a.SetMods(md)
		a.Wheel(steps, int(pt.X)/scale, int(pt.Y)/scale)
		render()
		return 0
	case wmKeyDown, wmSysKeyDown:
		if busy {
			return 0
		}
		if m == wmSysKeyDown && wp == 0x73 { // Alt+F4
			break
		}
		if k := mapKey(wp); k != 0 {
			a.KeyDown(k, mods())
			render()
		} else {
			a.SetMods(mods())
			if wp == 0x12 { // Alt: Mauszeiger/Zustand aktualisieren
				render()
			}
		}
		if m == wmSysKeyDown {
			return 0
		}
		return 0
	case wmKeyUp, wmSysKeyUp:
		if k := mapKey(wp); k != 0 {
			a.KeyUp(k, mods())
		} else {
			a.SetMods(mods())
		}
		if m == wmSysKeyUp {
			return 0
		}
		return 0
	case wmChar:
		if busy {
			return 0
		}
		if wp >= 32 {
			a.Char(rune(wp))
			render()
		}
		return 0
	case wmSysChar:
		return 0
	case wmActivate:
		if wp&0xFFFF == 0 {
			releaseAll()
		}
	case wmKillFocus:
		releaseAll()
	case wmDropFiles:
		hd := wp
		n, _, _ := pDragQueryFileW.Call(hd, 0xFFFFFFFF, 0, 0)
		var files []string
		for i := uintptr(0); i < n; i++ {
			buf := make([]uint16, 1024)
			pDragQueryFileW.Call(hd, i, uintptr(unsafe.Pointer(&buf[0])), uintptr(len(buf)))
			files = append(files, syscall.UTF16ToString(buf))
		}
		pDragFinish.Call(hd)
		if !busy && len(files) > 0 {
			a.DropFiles(files)
			render()
		}
		return 0
	case wmClose:
		if busy {
			return 0
		}
		rememberWindow()
		inModal++
		ok := a.CanClose()
		inModal--
		if ok {
			pDestroyWindow.Call(h)
		}
		return 0
	case wmDestroy:
		pPostQuitMessage.Call(0)
		return 0
	}
	r, _, _ := pDefWindowProcW.Call(h, uintptr(m), wp, lp)
	return r
}

func loadIcon(size int) uintptr {
	for _, id := range []uintptr{2, 1, 3, 4, 5} {
		h, _, _ := pLoadImageW.Call(hInst, id, 1, uintptr(size), uintptr(size), 0)
		if h != 0 {
			return h
		}
	}
	return 0
}

func setDPIAware() {
	if pSetProcessDpiAwarenessContext.Find() == nil {
		// DPI_AWARENESS_CONTEXT_PER_MONITOR_AWARE_V2 = -4
		r, _, _ := pSetProcessDpiAwarenessContext.Call(^uintptr(3))
		if r != 0 {
			return
		}
	}
	if pSetProcessDPIAware.Find() == nil {
		pSetProcessDPIAware.Call()
	}
}

func main() {
	runtime.LockOSThread()
	setDPIAware()
	hInst, _, _ = pGetModuleHandleW.Call(0)

	// System-DPI für die Startgröße
	if hdc, _, _ := pGetDC.Call(0); hdc != 0 {
		d, _, _ := pGetDeviceCaps.Call(hdc, 88) // LOGPIXELSX
		if d > 0 {
			dpi = int(d)
		}
		pReleaseDC.Call(0, hdc)
	}

	className := u16("C64DPainterWindow")
	arrow, _, _ := pLoadCursorW.Call(0, idcArrow)
	wc := wndClassEx{
		style:         csHRedraw | csVRedraw | csDblClks,
		lpfnWndProc:   syscall.NewCallback(wndProc),
		hInstance:     hInst,
		hIcon:         loadIcon(32),
		hCursor:       arrow,
		lpszClassName: className,
		hIconSm:       loadIcon(16),
	}
	wc.cbSize = uint32(unsafe.Sizeof(wc))
	pRegisterClassExW.Call(uintptr(unsafe.Pointer(&wc)))

	// Settings decide the interface size; on the very first start it follows
	// the Windows display scaling (100 % → 1, 200 % → 2, …).
	a = nil
	first := app.New(winHost{})
	first.InitScale(int(math.Floor(float64(dpi)/96 + 0.25)))
	scale = first.UIScale()
	x, y, w, h := windowRectFor(scale)
	hwnd, _, _ = pCreateWindowExW.Call(0,
		uintptr(unsafe.Pointer(className)), uintptr(unsafe.Pointer(u16(app.AppName))),
		wsOverlappedWindow, uintptr(x), uintptr(y), uintptr(w), uintptr(h), 0, 0, hInst, 0)
	if hwnd == 0 {
		pMessageBoxW.Call(0, uintptr(unsafe.Pointer(u16("Could not create the window."))), uintptr(unsafe.Pointer(u16(app.AppName))), 0x10)
		return
	}
	if pGetDpiForWindow.Find() == nil {
		if d, _, _ := pGetDpiForWindow.Call(hwnd); d > 0 {
			dpi = int(d)
		}
	}
	a = first
	if st, ok := a.SavedWindow(); ok {
		restoreWindow(st)
	}
	a.RefreshTitle()
	updateClient()
	pDragAcceptFiles.Call(hwnd, 1)
	pSetTimer.Call(hwnd, 1, 30, 0)
	pShowWindow.Call(hwnd, swShow)
	updateClient()
	render()
	pUpdateWindow.Call(hwnd)

	// Datei aus der Kommandozeile (z. B. „Öffnen mit“)
	if len(os.Args) > 1 {
		arg := strings.Join(os.Args[1:], " ")
		if _, err := os.Stat(arg); err == nil {
			a.DropFiles([]string{arg})
			render()
		}
	}

	var mg msg
	for {
		r, _, _ := pGetMessageW.Call(uintptr(unsafe.Pointer(&mg)), 0, 0, 0)
		if int32(r) <= 0 {
			break
		}
		pTranslateMessage.Call(uintptr(unsafe.Pointer(&mg)))
		pDispatchMessageW.Call(uintptr(unsafe.Pointer(&mg)))
	}
}

// --- Folder dialog -------------------------------------------------------------------

var (
	ole32                 = syscall.NewLazyDLL("ole32.dll")
	pOleInitialize        = ole32.NewProc("OleInitialize")
	pCoTaskMemFree        = ole32.NewProc("CoTaskMemFree")
	pSHBrowseForFolderW   = shell32.NewProc("SHBrowseForFolderW")
	pSHGetPathFromIDListW = shell32.NewProc("SHGetPathFromIDListW")
	pSendMessageW         = user32.NewProc("SendMessageW")
	browseStart           []uint16
	browseCallback        = syscall.NewCallback(browseProc)
)

type browseInfo struct {
	hwndOwner      uintptr
	pidlRoot       uintptr
	pszDisplayName *uint16
	lpszTitle      *uint16
	ulFlags        uint32
	lpfn           uintptr
	lParam         uintptr
	iImage         int32
}

// browseProc selects the start folder when the dialog opens.
func browseProc(h, msg, lp, data uintptr) uintptr {
	const bffmInitialized, bffmSetSelectionW = 1, 0x467
	if msg == bffmInitialized && len(browseStart) > 0 {
		pSendMessageW.Call(h, bffmSetSelectionW, 1, uintptr(unsafe.Pointer(&browseStart[0])))
	}
	return 0
}

func (winHost) ChooseFolder(title, dir string) (string, bool) {
	pOleInitialize.Call(0)
	display := make([]uint16, 260)
	browseStart = nil
	if dir != "" {
		browseStart, _ = syscall.UTF16FromString(dir)
	}
	bi := browseInfo{
		hwndOwner:      hwnd,
		pszDisplayName: &display[0],
		lpszTitle:      u16(title),
		ulFlags:        0x1 | 0x10 | 0x40, // only file system folders, edit box, new style
		lpfn:           browseCallback,
	}
	inModal++
	pidl, _, _ := pSHBrowseForFolderW.Call(uintptr(unsafe.Pointer(&bi)))
	inModal--
	releaseAll()
	runtime.KeepAlive(display)
	if pidl == 0 {
		return "", false
	}
	defer pCoTaskMemFree.Call(pidl)
	buf := make([]uint16, 1024)
	if r, _, _ := pSHGetPathFromIDListW.Call(pidl, uintptr(unsafe.Pointer(&buf[0]))); r == 0 {
		return "", false
	}
	p := syscall.UTF16ToString(buf)
	return p, p != ""
}
