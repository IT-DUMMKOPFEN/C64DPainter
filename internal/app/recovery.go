package app

import (
	"encoding/json"
	"os"
	"path/filepath"
)

// --- Crash recovery --------------------------------------------------------------------
//
// Independent of Options → Auto backup, changed pictures are saved quietly
// every minute into the "Recovery" folder next to the settings. A normal
// quit removes the folder. When it is still there at the next start, the
// program did not end normally and offers to restore the pictures.

const recoveryInterval = 60 * 1000 // milliseconds

type recoveryInfo struct {
	Paths [2]string `json:"paths"` // file name of each page ("" = untitled)
	Page  int       `json:"page"`
}

func recoveryDir() string {
	p := SettingsPath()
	if p == "" {
		return ""
	}
	return filepath.Join(filepath.Dir(p), "Recovery")
}

func recoveryFile(pg int) string {
	if pg == 1 {
		return "page2.c64dp"
	}
	return "page1.c64dp"
}

// checkRecovery runs from Tick.
func (a *App) checkRecovery() {
	if a.now-a.lastRecovery < recoveryInterval || a.drag.active {
		return
	}
	a.lastRecovery = a.now
	a.writeRecovery()
}

func (a *App) writeRecovery() {
	dir := recoveryDir()
	if dir == "" {
		return
	}
	changed := false
	for pg := 0; pg < 2; pg++ {
		p := a.pages[pg]
		f := filepath.Join(dir, recoveryFile(pg))
		if p == nil || !a.dirty[pg] {
			if a.recoveryHash[pg] != 0 {
				_ = os.Remove(f)
				a.recoveryHash[pg] = 0
				changed = true
			}
			continue
		}
		h := pictureHash(p) ^ uint64(p.Mode+1)
		if h == a.recoveryHash[pg] {
			continue
		}
		data, err := a.encodeProjectOf(p)
		if err != nil {
			continue
		}
		if os.MkdirAll(dir, 0o755) != nil || os.WriteFile(f, data, 0o644) != nil {
			continue
		}
		a.recoveryHash[pg] = h
		changed = true
	}
	if !changed {
		return
	}
	if a.recoveryHash[0] == 0 && a.recoveryHash[1] == 0 {
		a.clearRecovery()
		return
	}
	info := recoveryInfo{Paths: a.filePath, Page: a.page}
	if b, err := json.Marshal(info); err == nil {
		_ = os.WriteFile(filepath.Join(dir, "recovery.json"), b, 0o644)
	}
}

// clearRecovery removes the recovery folder (normal quit).
func (a *App) clearRecovery() {
	a.recoveryHash = [2]uint64{}
	if dir := recoveryDir(); dir != "" {
		_ = os.RemoveAll(dir)
	}
}

// hasRecovery reports whether pictures of an earlier session can be restored.
func hasRecovery() bool {
	dir := recoveryDir()
	if dir == "" {
		return false
	}
	for pg := 0; pg < 2; pg++ {
		if _, err := os.Stat(filepath.Join(dir, recoveryFile(pg))); err == nil {
			return true
		}
	}
	return false
}

// offerRecovery asks whether to restore the pictures of the last session.
// It returns true when they were restored.
func (a *App) offerRecovery() bool {
	dir := recoveryDir()
	if a.host.Ask(AppName, AppName+" was not closed normally last time.\n\n"+
		"Restore the unsaved picture(s)?\n\n"+
		"Yes: restore  ·  No: discard them") != 0 {
		a.clearRecovery()
		return false
	}
	var info recoveryInfo
	if b, err := os.ReadFile(filepath.Join(dir, "recovery.json")); err == nil {
		_ = json.Unmarshal(b, &info)
	}
	restored := 0
	for pg := 0; pg < 2; pg++ {
		data, err := os.ReadFile(filepath.Join(dir, recoveryFile(pg)))
		if err != nil {
			continue
		}
		p, _, _, err := decodeProject(data)
		if err != nil {
			continue
		}
		*a.pages[pg] = *p
		a.filePath[pg] = info.Paths[pg]
		a.dirty[pg] = true
		restored++
	}
	a.clearRecovery()
	if restored == 0 {
		a.host.Info(AppName, "The pictures could not be restored.")
		return false
	}
	a.page = clampI(info.Page, 0, 1)
	if !a.dirty[a.page] {
		a.page = 1 - a.page
	}
	a.pic = a.pages[a.page]
	a.extraLayers = (a.pic.LayerPixels(1) > 0 && !a.pic.Hide[1]) || (a.pic.LayerPixels(2) > 0 && !a.pic.Hide[2]) || a.pic.Active > 0
	a.syncExtraLayers()
	a.syncMode()
	a.fitPending = true
	a.updateTitle()
	a.setStatus("Restored the pictures of the last session – save them now")
	return true
}
