package app

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"

	"c64dpainter/internal/c64"
	"c64dpainter/internal/font"
	"c64dpainter/internal/gfx"
)

// --- Working folder and quick save (F2) ----------------------------------------------
//
// The working folder is where pictures and data are saved. It is asked for
// at the start and with File → New; the save and open dialogs start there,
// and F2 saves a numbered PNG into it without asking.

// FolderChooser is implemented by front ends that can show a folder dialog.
type FolderChooser interface {
	ChooseFolder(title, dir string) (string, bool)
}

// workDir returns the working folder (or a sensible fallback).
func (a *App) workDir() string {
	for _, d := range []string{a.set.WorkDir, a.set.LastDir} {
		if d == "" {
			continue
		}
		if st, err := os.Stat(d); err == nil && st.IsDir() {
			return d
		}
	}
	home, _ := os.UserHomeDir()
	return home
}

func (a *App) setWorkDir(d string) {
	a.set.WorkDir = d
	a.set.LastDir = d
	a.saveSettings()
	a.setStatus("Working folder: " + d)
}

// askWorkDir shows the working folder window (only when the front end can
// choose folders). then runs after it was closed.
func (a *App) askWorkDir(then func()) {
	if _, ok := a.host.(FolderChooser); !ok {
		if then != nil {
			then()
		}
		return
	}
	a.menuOpen = -1
	a.modal = &workDirDialog{dir: a.workDir(), then: then}
}

func (a *App) chooseWorkDir() {
	fc, ok := a.host.(FolderChooser)
	if !ok {
		return
	}
	if d, ok := fc.ChooseFolder("Choose the working folder", a.workDir()); ok && d != "" {
		a.setWorkDir(d)
	}
}

type workDirDialog struct {
	dir  string
	then func()
}

func (d *workDirDialog) Title() string          { return "Working folder" }
func (d *workDirDialog) Size(a *App) (int, int) { return 560, 196 }
func (d *workDirDialog) Key(a *App, k KeyEvent) {
	if k.Key == KeyEnter {
		d.ok(a)
	}
}
func (d *workDirDialog) Cancel(a *App) { d.finish(a) }

func (d *workDirDialog) ok(a *App) {
	a.setWorkDir(d.dir)
	a.closeModal(false)
	d.finish(a)
}

func (d *workDirDialog) finish(a *App) {
	if d.then != nil {
		f := d.then
		d.then = nil
		a.later(f)
	}
}

func (d *workDirDialog) Draw(a *App, r gfx.Rect) {
	u := &a.ui
	m := a.img
	th := a.th
	y := r.Y
	for _, ln := range wrapText(font.UI, "Pictures and data are saved in this folder. The save and open dialogs start here, and F2 saves a numbered PNG picture into it.", r.W) {
		m.Text(font.UI, r.X, y, ln, th.TextDim)
		y += font.UI.Height + 2
	}
	y += 10
	pr := gfx.Rect{X: r.X, Y: y, W: r.W - 130, H: 28}
	m.Fill(pr, th.Popup)
	m.Frame(pr, th.Border)
	m.TextL(font.UI, gfx.Rect{X: pr.X + 8, Y: pr.Y, W: pr.W - 16, H: pr.H}, gfx.Elide(font.UI, d.dir, pr.W-16), th.Text)
	if u.Button("wd_choose", gfx.Rect{X: r.Right() - 120, Y: y, W: 120, H: 28}, "Choose …") {
		a.later(func() {
			if fc, ok := a.host.(FolderChooser); ok {
				if p, ok := fc.ChooseFolder("Choose the working folder", d.dir); ok && p != "" {
					d.dir = p
				}
			}
		})
	}
	by := r.Bottom() - 28
	if u.ButtonEx("wd_ok", gfx.Rect{X: r.Right() - 120, Y: by, W: 120, H: 28}, "OK", true, true) {
		d.ok(a)
	}
}

var quickRe = regexp.MustCompile(`^(.*)_(\d{4,})\.png$`)

// quickBase returns the file name stem for quick saves.
func (a *App) quickBase() string {
	if p := a.filePath[a.page]; p != "" {
		b := strings.TrimSuffix(filepath.Base(p), filepath.Ext(p))
		if m := quickRe.FindStringSubmatch(filepath.Base(p)); m != nil {
			b = m[1]
		}
		if b != "" {
			return b
		}
	}
	return "picture"
}

// nextQuickPath finds the next free numbered file name in the folder.
func nextQuickPath(dir, base string) string {
	n := 0
	if ents, err := os.ReadDir(dir); err == nil {
		for _, e := range ents {
			m := quickRe.FindStringSubmatch(e.Name())
			if m == nil || !strings.EqualFold(m[1], base) {
				continue
			}
			if v, err := strconv.Atoi(m[2]); err == nil && v > n {
				n = v
			}
		}
	}
	for {
		n++
		p := filepath.Join(dir, fmt.Sprintf("%s_%04d.png", base, n))
		if _, err := os.Stat(p); err != nil {
			return p
		}
	}
}

// quickSave (F2) saves the picture as a numbered PNG (320 × 200) into the
// working folder.
func (a *App) quickSave() {
	a.commitOrCancelForHistory()
	dir := a.workDir()
	p := nextQuickPath(dir, a.quickBase())
	data, err := a.pic.SavePNG(a.pal, c64.PNG320)
	if err == nil {
		err = a.host.WriteFile(p, data)
	}
	if err != nil {
		a.host.Info(AppName, "Quick save failed:\n"+err.Error())
		return
	}
	a.setStatus("Saved " + filepath.Base(p) + " in " + dir)
}
