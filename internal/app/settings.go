package app

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"c64dpainter/internal/c64"
)

// Settings werden in %APPDATA%\C64DPainter\settings.json gespeichert.
type Settings struct {
	Theme       string `json:"theme"`
	Palette     string `json:"palette"`
	GridOn      bool   `json:"gridOn"`
	GridW       int    `json:"gridW"`
	GridH       int    `json:"gridH"`
	GridOX      int    `json:"gridOffsetX"`
	GridOY      int    `json:"gridOffsetY"`
	GridAlpha   int    `json:"gridAlpha"`
	GridColor   int    `json:"gridColor"` // 0 = automatisch, sonst C64-Farbe+1
	PixelGrid   bool   `json:"pixelGrid"`
	ShowBorder  bool   `json:"showBorder"`
	Conflict    int    `json:"conflict"`
	AspectBrush bool   `json:"aspectBrush"`
	ShowPanel   bool   `json:"showPanel"`
	UIScale     int    `json:"uiScale"` // 1, 2 or 3 (0 = not chosen yet)
	LastDir     string `json:"lastDir"`
	HelpSeen    bool   `json:"helpSeen"`
	CellHint    bool   `json:"cellHint"`

	DitherBrush   c64.AutoDitherOptions `json:"ditherBrush"`
	SmearStrength int                   `json:"smearStrength"`
	DitherAuto    c64.AutoDitherOptions `json:"autoDither"`
	Gradient      GradientSettings      `json:"gradient"`
	Sym           SymSettings           `json:"symmetry"`
	ShadeRamp     int                   `json:"shadeRamp"` // -1 = all colors by brightness
	CRT           CRTSettings           `json:"crt"`
	WorkDir       string                `json:"workDir"`
	StampTab      int                   `json:"stampTab"`
	StampSize     int                   `json:"stampSize"`
}

func defaultSettings() Settings {
	return Settings{
		Theme:         "light",
		Palette:       "PIXCEN_Colodore",
		GridOn:        true,
		GridW:         8,
		GridH:         8,
		GridAlpha:     35,
		ShowBorder:    true,
		Conflict:      0,
		AspectBrush:   true,
		ShowPanel:     true,
		CellHint:      true,
		DitherBrush:   defaultDitherBrush(),
		SmearStrength: 60,
		DitherAuto:    c64.DefaultAutoDither(),
		ShadeRamp:     -1,
		CRT:           CRTSettings{Scale: 2, On: true},
	}
}

// defaultDitherBrush: the same as Auto dither (new colors only in free cell
// slots, so the brush never runs into color conflicts).
func defaultDitherBrush() c64.AutoDitherOptions {
	return c64.DefaultAutoDither()
}

func validDither(o *c64.AutoDitherOptions, def c64.AutoDitherOptions) {
	if !o.Valid() {
		*o = def
	}
}

// SettingsPath kann vom Testprogramm überschrieben werden.
var SettingsPath = func() string {
	d, err := os.UserConfigDir()
	if err != nil {
		return ""
	}
	return filepath.Join(d, "C64DPainter", "settings.json")
}

func loadSettings() Settings {
	s := defaultSettings()
	p := SettingsPath()
	if p == "" {
		return s
	}
	if b, err := os.ReadFile(p); err == nil {
		_ = json.Unmarshal(b, &s)
	}
	if s.GridW < 2 {
		s.GridW = 8
	}
	if s.GridH < 1 {
		s.GridH = 8
	}
	if s.Conflict < 0 || s.Conflict > 2 {
		s.Conflict = 0
	}
	validDither(&s.DitherBrush, defaultDitherBrush())
	validDither(&s.DitherAuto, c64.DefaultAutoDither())
	return s
}

func (a *App) saveSettings() {
	p := SettingsPath()
	if p == "" {
		return
	}
	_ = os.MkdirAll(filepath.Dir(p), 0o755)
	b, _ := json.MarshalIndent(a.set, "", "  ")
	_ = os.WriteFile(p, b, 0o644)
}

// UIScale returns the interface size factor (1, 2 or 3).
func (a *App) UIScale() int { return clampI(a.set.UIScale, 1, 3) }

// InitScale is called by the front end at start. On the very first start
// (or for old settings with "automatic") the suggested factor is stored.
func (a *App) InitScale(suggested int) {
	if a.set.UIScale < 1 || a.set.UIScale > 3 {
		a.set.UIScale = clampI(suggested, 1, 3)
		for a.set.UIScale > 1 && !a.scaleFits(a.set.UIScale) {
			a.set.UIScale--
		}
		a.saveSettings()
	}
}

// ScaleHost is implemented by front ends that can resize their window when
// the interface size changes.
type ScaleHost interface {
	// ScreenSize is the usable screen area in the units the scale applies to.
	ScreenSize() (w, h int)
	// ApplyScale resizes the window for the new interface size.
	ApplyScale(scale int)
}

// Logical size the interface needs at least, and the preferred size.
const (
	MinLogicalW  = 960
	MinLogicalH  = 600
	PrefLogicalW = 1360
	PrefLogicalH = 860
)

// scaleFits reports whether the screen is large enough for an interface size.
func (a *App) scaleFits(s int) bool {
	if s <= 1 {
		return true
	}
	h, ok := a.host.(ScaleHost)
	if !ok {
		return true
	}
	w, ht := h.ScreenSize()
	if w <= 0 || ht <= 0 {
		return true
	}
	return w >= MinLogicalW*s && ht >= MinLogicalH*s
}

func (a *App) setUIScale(s int) {
	if s == a.UIScale() || !a.scaleFits(s) {
		return
	}
	a.set.UIScale = s
	a.saveSettings()
	if h, ok := a.host.(ScaleHost); ok {
		// resize the window outside of the frame being drawn
		a.later(func() { h.ApplyScale(s) })
	}
	a.setStatus(fmt.Sprintf("Interface size %d %%", s*100))
}
