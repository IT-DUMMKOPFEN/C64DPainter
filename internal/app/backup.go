package app

import (
	"hash/fnv"
	"os"
	"path/filepath"

	"c64dpainter/internal/c64"
)

// --- Auto backup ---------------------------------------------------------------------
//
// Options → Auto backup saves the pictures every few minutes as project
// files into the working folder: "Backup.c64dp" (and "Backup (page 2).c64dp"
// when the spare page is used). It is off at every start.

const backupInterval = 3 * 60 * 1000 // milliseconds

func (a *App) toggleAutoBackup() {
	a.autoBackup = !a.autoBackup
	a.lastBackup = a.now
	if a.autoBackup {
		a.setStatus("Auto backup on: every 3 minutes into " + filepath.Join(a.workDir(), "Backup.c64dp"))
	} else {
		a.setStatus("Auto backup off")
	}
}

func pictureHash(p *c64.Picture) uint64 {
	h := fnv.New64a()
	for l := range p.L {
		h.Write(p.L[l][:])
	}
	h.Write([]byte{p.BG, p.Border})
	return h.Sum64()
}

func pageUsed(p *c64.Picture) bool {
	for l := range p.L {
		for _, v := range p.L[l] {
			if v != c64.LayerClear {
				return true
			}
		}
	}
	return false
}

// checkBackup runs from Tick: saves when the interval is over and a page
// changed since the last backup.
func (a *App) checkBackup() {
	if !a.autoBackup || a.now-a.lastBackup < backupInterval || a.drag.active {
		return
	}
	a.lastBackup = a.now
	a.writeBackup()
}

func (a *App) writeBackup() {
	saved := 0
	for pg := 0; pg < 2; pg++ {
		p := a.pages[pg]
		if p == nil || (pg == 1 && !pageUsed(p)) {
			continue
		}
		h := pictureHash(p)
		if h == a.backupHash[pg] {
			continue
		}
		data, err := a.encodeProjectOf(p)
		name := "Backup.c64dp"
		if pg == 1 {
			name = "Backup (page 2).c64dp"
		}
		if err == nil {
			err = os.WriteFile(filepath.Join(a.workDir(), name), data, 0o644)
		}
		if err != nil {
			a.setStatus("Auto backup failed: " + err.Error())
			return
		}
		a.backupHash[pg] = h
		saved++
	}
	if saved > 0 {
		a.setStatus("Backup saved in " + a.workDir())
	}
}

// --- Recently opened files -----------------------------------------------------------

const maxRecent = 8

func (a *App) addRecent(p string) {
	if abs, err := filepath.Abs(p); err == nil {
		p = abs
	}
	list := []string{p}
	for _, q := range a.set.Recent {
		if q != p && len(list) < maxRecent {
			list = append(list, q)
		}
	}
	a.set.Recent = list
	a.saveSettings()
	a.buildMenus()
}

func (a *App) openRecent(p string) {
	if _, err := os.Stat(p); err != nil {
		a.host.Info(AppName, "The file no longer exists:\n"+p)
		var list []string
		for _, q := range a.set.Recent {
			if q != p {
				list = append(list, q)
			}
		}
		a.set.Recent = list
		a.saveSettings()
		a.buildMenus()
		return
	}
	a.openPath(p)
}

func (a *App) clearRecent() {
	a.set.Recent = nil
	a.saveSettings()
	a.buildMenus()
}

// recentItems returns the menu items for the recent files.
func (a *App) recentItems() []MenuItem {
	if len(a.set.Recent) == 0 {
		return []MenuItem{{Label: "(no recent files)", Enabled: func() bool { return false }}}
	}
	var out []MenuItem
	for _, p := range a.set.Recent {
		p := p
		dir := filepath.Base(filepath.Dir(p))
		label := filepath.Base(p)
		if dir != "" && dir != "." && dir != string(filepath.Separator) {
			label += "   (" + dir + ")"
		}
		if len([]rune(label)) > 48 {
			label = string([]rune(label)[:47]) + "…"
		}
		out = append(out, MenuItem{Label: label, Action: func() { a.later(func() { a.openRecent(p) }) }})
	}
	return append(out, MenuItem{Label: "Clear list", Action: a.clearRecent})
}
