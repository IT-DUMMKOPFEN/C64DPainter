package app

import (
	"os"
	"path/filepath"
	"sort"
	"strings"

	"c64dpainter/internal/font"
	"c64dpainter/internal/gfx"
)

// Built-in dialogs for platforms without native ones (Linux without zenity).
// They run synchronously: the host passes a pump function that processes
// window events and draws a frame; the dialog loops until it is closed.

// SyncMessage shows a message box and returns the index of the pressed button
// (the last button is returned when the dialog is closed with Esc).
func (a *App) SyncMessage(pump func() bool, title, text string, buttons []string) int {
	d := &msgDialog{title: title, text: text, buttons: buttons, result: -1}
	a.runSync(pump, d, func() bool { return d.result >= 0 })
	if d.result < 0 {
		return len(buttons) - 1
	}
	return d.result
}

// SyncFileDialog shows the built-in file browser.
func (a *App) SyncFileDialog(pump func() bool, save bool, title string, filters []Filter, dir, name, defExt string) (string, bool) {
	d := newFileDialog(save, title, filters, dir, name, defExt)
	a.runSync(pump, d, func() bool { return d.done })
	return d.result, d.ok
}

func (a *App) runSync(pump func() bool, m Modal, done func() bool) {
	prev := a.modal
	prevMenu := a.menuOpen
	a.menuOpen = -1
	a.modal = m
	a.in.Down = [3]bool{}
	a.ui.active = ""
	for !done() {
		if a.modal != m { // closed with Esc or the close button
			break
		}
		if !pump() {
			break
		}
	}
	a.modal = prev
	a.menuOpen = prevMenu
	a.in.Down = [3]bool{}
	a.ui.active = ""
}

// --- message box -----------------------------------------------------------------

type msgDialog struct {
	title, text string
	buttons     []string
	result      int
}

func (d *msgDialog) Title() string { return d.title }
func (d *msgDialog) Size(a *App) (int, int) {
	lines := wrapText(font.Body, d.text, 420)
	return 480, 110 + len(lines)*(font.Body.Height+4)
}
func (d *msgDialog) Cancel(a *App) { d.result = len(d.buttons) - 1 }
func (d *msgDialog) Key(a *App, k KeyEvent) {
	if k.Key == KeyEnter {
		d.result = 0
	}
}
func (d *msgDialog) Draw(a *App, r gfx.Rect) {
	m := a.img
	y := r.Y
	for _, ln := range wrapText(font.Body, d.text, r.W) {
		m.Text(font.Body, r.X, y, ln, a.th.Text)
		y += font.Body.Height + 4
	}
	bw := 110
	x := r.Right() - len(d.buttons)*(bw+8) + 8
	for i, b := range d.buttons {
		if a.ui.ButtonEx("mb"+itoa(i), gfx.Rect{X: x + i*(bw+8), Y: r.Bottom() - 28, W: bw, H: 28}, b, i == 0, true) {
			d.result = i
		}
	}
}

func wrapText(f *font.Face, text string, w int) []string {
	var out []string
	for _, para := range strings.Split(text, "\n") {
		line := ""
		for _, word := range strings.Fields(para) {
			t := word
			if line != "" {
				t = line + " " + word
			}
			if f.Width(t) > w && line != "" {
				out = append(out, line)
				line = word
			} else {
				line = t
			}
		}
		out = append(out, line)
	}
	return out
}

// --- file browser ------------------------------------------------------------------

type fileEntry struct {
	name string
	dir  bool
}

type fileDialog struct {
	save      bool
	title     string
	filters   []Filter
	filter    int
	dir       string
	name      string
	defExt    string
	entries   []fileEntry
	sel       int
	scroll    int
	confirm   bool
	errText   string
	done, ok  bool
	result    string
	lastClick int
	fresh     bool
	lastTime  int64
}

func newFileDialog(save bool, title string, filters []Filter, dir, name, defExt string) *fileDialog {
	d := &fileDialog{save: save, title: title, filters: filters, name: name, defExt: defExt, sel: -1, lastClick: -1, fresh: name != ""}
	if dir == "" {
		dir, _ = os.UserHomeDir()
	}
	if st, err := os.Stat(dir); err != nil || !st.IsDir() {
		dir, _ = os.UserHomeDir()
	}
	d.load(dir)
	return d
}

func (d *fileDialog) patterns() []string {
	if d.filter < 0 || d.filter >= len(d.filters) {
		return nil
	}
	return strings.Split(d.filters[d.filter].Pattern, ";")
}

func (d *fileDialog) matches(name string) bool {
	pats := d.patterns()
	if len(pats) == 0 {
		return true
	}
	for _, p := range pats {
		p = strings.TrimSpace(p)
		if p == "*.*" || p == "*" {
			return true
		}
		if ok, _ := filepath.Match(strings.ToLower(p), strings.ToLower(name)); ok {
			return true
		}
	}
	return false
}

func (d *fileDialog) load(dir string) {
	ents, err := os.ReadDir(dir)
	if err != nil {
		d.errText = err.Error()
		return
	}
	d.errText = ""
	d.dir = dir
	d.entries = d.entries[:0]
	for _, e := range ents {
		n := e.Name()
		if strings.HasPrefix(n, ".") {
			continue
		}
		isDir := e.IsDir()
		if e.Type()&os.ModeSymlink != 0 {
			if st, err := os.Stat(filepath.Join(dir, n)); err == nil {
				isDir = st.IsDir()
			}
		}
		if isDir || d.matches(n) {
			d.entries = append(d.entries, fileEntry{n, isDir})
		}
	}
	sort.Slice(d.entries, func(i, j int) bool {
		a, b := d.entries[i], d.entries[j]
		if a.dir != b.dir {
			return a.dir
		}
		return strings.ToLower(a.name) < strings.ToLower(b.name)
	})
	d.sel = -1
	d.scroll = 0
	d.confirm = false
}

func (d *fileDialog) Title() string          { return d.title }
func (d *fileDialog) Size(a *App) (int, int) { return 780, 560 }
func (d *fileDialog) Cancel(a *App)          { d.done, d.ok = true, false }

func (d *fileDialog) Char(a *App, r rune) {
	if r >= 32 && r != '/' {
		if d.fresh {
			d.name = ""
			d.fresh = false
		}
		d.name += string(r)
		d.confirm = false
	}
}

func (d *fileDialog) Key(a *App, k KeyEvent) {
	switch k.Key {
	case KeyBackspace:
		if d.fresh {
			d.name, d.fresh = "", false
		} else if d.name != "" {
			rs := []rune(d.name)
			d.name = string(rs[:len(rs)-1])
			d.confirm = false
		} else {
			d.load(filepath.Dir(d.dir))
		}
	case KeyEnter:
		d.accept()
	case KeyUp:
		if d.sel > 0 {
			d.pick(d.sel - 1)
		}
	case KeyDown:
		if d.sel < len(d.entries)-1 {
			d.pick(d.sel + 1)
		}
	}
}

func (d *fileDialog) pick(i int) {
	d.sel = i
	if i >= 0 && i < len(d.entries) && !d.entries[i].dir {
		d.name = d.entries[i].name
		d.confirm = false
		d.fresh = true
	}
}

func (d *fileDialog) accept() {
	if d.sel >= 0 && d.sel < len(d.entries) && d.entries[d.sel].dir && (d.name == "" || !d.save) {
		d.load(filepath.Join(d.dir, d.entries[d.sel].name))
		return
	}
	if d.name == "" {
		return
	}
	p := d.name
	if !filepath.IsAbs(p) {
		if strings.HasPrefix(p, "~/") {
			home, _ := os.UserHomeDir()
			p = filepath.Join(home, p[2:])
		} else {
			p = filepath.Join(d.dir, p)
		}
	}
	if st, err := os.Stat(p); err == nil && st.IsDir() {
		d.name = ""
		d.load(p)
		return
	}
	if d.save {
		if filepath.Ext(p) == "" && d.defExt != "" {
			p += "." + d.defExt
		}
		if _, err := os.Stat(p); err == nil && !d.confirm {
			d.confirm = true
			return
		}
	} else if _, err := os.Stat(p); err != nil {
		d.errText = "File not found: " + filepath.Base(p)
		return
	}
	d.result, d.ok, d.done = p, true, true
}

func (d *fileDialog) Draw(a *App, r gfx.Rect) {
	u := &a.ui
	m := a.img
	th := a.th
	// places
	home, _ := os.UserHomeDir()
	places := []struct{ n, p string }{{"Home", home}}
	for _, sub := range []string{"Desktop", "Pictures", "Documents", "Downloads", "Bilder", "Dokumente", "Schreibtisch"} {
		p := filepath.Join(home, sub)
		if st, err := os.Stat(p); err == nil && st.IsDir() {
			places = append(places, struct{ n, p string }{sub, p})
		}
	}
	places = append(places, struct{ n, p string }{"Computer /", "/"})
	pw := 150
	py := r.Y
	m.Text(font.UIB, r.X, py, "Places", th.TextDim)
	py += 20
	for i, pl := range places {
		if u.ButtonEx("fd_pl"+itoa(i), gfx.Rect{X: r.X, Y: py, W: pw, H: 24}, pl.n, d.dir == pl.p, true) {
			d.load(pl.p)
		}
		py += 28
	}
	// path + up
	lx := r.X + pw + 12
	lw := r.Right() - lx
	if u.ButtonEx("fd_up", gfx.Rect{X: lx, Y: r.Y, W: 70, H: 24}, "↑ Up", false, d.dir != "/") {
		d.load(filepath.Dir(d.dir))
	}
	m.TextL(font.UI, gfx.Rect{X: lx + 80, Y: r.Y, W: lw - 80, H: 24}, gfx.Elide(font.UI, d.dir, lw-80), th.Text)
	// list
	lr := gfx.Rect{X: lx, Y: r.Y + 32, W: lw, H: r.H - 32 - 110}
	m.Fill(lr, th.PanelAlt)
	m.Frame(lr, th.Border)
	const rh = 22
	total := len(d.entries) * rh
	if u.hover(lr) && a.in.Wheel != 0 {
		d.scroll -= a.in.Wheel * rh * 3
	}
	if d.sel >= 0 {
		// keep the selection visible
		sy := d.sel * rh
		if sy < d.scroll {
			d.scroll = sy
		}
		if sy+rh > d.scroll+lr.H-2 {
			d.scroll = sy + rh - lr.H + 2
		}
	}
	d.scroll = clampI(d.scroll, 0, max(0, total-lr.H+2))
	old := m.SetClip(lr.Inset(1))
	for i, e := range d.entries {
		y := lr.Y + 1 + i*rh - d.scroll
		if y+rh < lr.Y || y > lr.Bottom() {
			continue
		}
		row := gfx.Rect{X: lr.X + 1, Y: y, W: lr.W - 2, H: rh}
		tc := th.Text
		if i == d.sel {
			m.Fill(row, th.Accent)
			tc = th.AccentText
		} else if u.hover(row) {
			m.Fill(row, th.BtnHover)
		}
		label := e.name
		if e.dir {
			label = "▸ " + e.name + "/"
		} else {
			label = "   " + e.name
		}
		f := font.UI
		if e.dir {
			f = font.UIB
		}
		m.TextL(f, gfx.Rect{X: row.X + 8, Y: row.Y, W: row.W - 16, H: rh}, gfx.Elide(f, label, row.W-16), tc)
		if c, _ := u.press("fd_e"+itoa(i), row, 0); c {
			if d.lastClick == i && a.now-d.lastTime < 450 {
				d.pick(i)
				if e.dir {
					d.load(filepath.Join(d.dir, e.name))
				} else {
					d.accept()
				}
				d.lastClick = -1
			} else {
				d.pick(i)
				d.lastClick, d.lastTime = i, a.now
			}
		}
	}
	m.SetClip(old)
	if len(d.entries) == 0 {
		m.TextC(font.UI, lr, "(no matching files)", th.TextDim)
	}
	// name + filter
	y := lr.Bottom() + 10
	m.TextL(font.UI, gfx.Rect{X: lx, Y: y, W: 80, H: 26}, "File name", th.Text)
	nr := gfx.Rect{X: lx + 80, Y: y, W: lw - 80, H: 26}
	m.Fill(nr, th.Popup)
	m.Frame(nr, th.Accent)
	shown := gfx.Elide(font.UI, d.name, nr.W-14)
	tc := th.Text
	if d.fresh && d.name != "" {
		m.Fill(gfx.Rect{X: nr.X + 4, Y: nr.Y + 4, W: font.UI.Width(shown) + 4, H: nr.H - 8}, th.Accent)
		tc = th.AccentText
	}
	tw := m.Text(font.UI, nr.X+6, nr.Y+(nr.H-font.UI.Height)/2, shown, tc)
	if (a.now/500)%2 == 0 {
		m.VLine(nr.X+7+tw, nr.Y+5, nr.Bottom()-6, th.Text)
	}
	a.needAnim = true
	y += 34
	if len(d.filters) > 0 {
		m.TextL(font.UI, gfx.Rect{X: lx, Y: y, W: 80, H: 24}, "Show", th.Text)
		fx := lx + 80
		for i, f := range d.filters {
			label := f.Name
			w := min(font.UI.Width(label)+20, lw-80)
			if fx+w > r.Right() {
				break
			}
			if u.ButtonEx("fd_f"+itoa(i), gfx.Rect{X: fx, Y: y, W: w, H: 24}, label, d.filter == i, true) {
				d.filter = i
				d.load(d.dir)
				// in a save dialog the file type follows the chosen filter
				if d.save && d.name != "" {
					if p := strings.TrimPrefix(strings.Split(f.Pattern, ";")[0], "*"); strings.HasPrefix(p, ".") && p != ".*" {
						d.name = strings.TrimSuffix(d.name, filepath.Ext(d.name)) + p
						d.defExt = p[1:]
					}
				}
			}
			fx += w + 4
		}
	}
	// status + buttons
	by := r.Bottom() - 28
	msgText := d.errText
	if d.confirm {
		msgText = "\"" + filepath.Base(d.name) + "\" already exists. Click Save again to replace it."
	}
	if msgText != "" {
		c := th.Danger
		m.TextL(font.UI, gfx.Rect{X: r.X, Y: by, W: r.W - 260, H: 28}, gfx.Elide(font.UI, msgText, r.W-260), c)
	}
	if u.Button("fd_cancel", gfx.Rect{X: r.Right() - 240, Y: by, W: 115, H: 28}, "Cancel") {
		d.done, d.ok = true, false
	}
	okLabel := "Open"
	if d.save {
		okLabel = "Save"
	}
	if u.ButtonEx("fd_ok", gfx.Rect{X: r.Right() - 115, Y: by, W: 115, H: 28}, okLabel, true, true) {
		d.accept()
	}
}
