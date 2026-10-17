package app

import (
	"bytes"
	"compress/gzip"
	"encoding/json"
	"fmt"
	"image"
	"image/png"
	"io"

	"c64dpainter/internal/c64"
)

// Project files (*.c64dp) keep everything that a KLA file cannot: the three
// paint layers with their visibility, the active layer and the template
// (layer 1) with its settings. The file is gzip-compressed JSON.

const projectFormat = "C64 DPainter project"
const projectVersion = 1

type projectFile struct {
	Format     string           `json:"format"`
	Version    int              `json:"version"`
	App        string           `json:"app"`
	Background uint8            `json:"background"`
	Border     uint8            `json:"border"`
	Active     int              `json:"activeLayer"`
	Palette    string           `json:"palette,omitempty"`
	Layers     []projectLayer   `json:"layers"`
	Template   *projectTemplate `json:"template,omitempty"`
}

// projectLayer: 160×200 bytes, 0 = transparent, 1..16 = C64 color + 1,
// 17 = background color.
type projectLayer struct {
	Name    string `json:"name"`
	Visible bool   `json:"visible"`
	Pixels  []byte `json:"pixels"`
}

type projectTemplate struct {
	Name    string `json:"name"`
	PNG     []byte `json:"png"`
	Visible bool   `json:"visible"`
	Opacity int    `json:"opacity"`
	Below   bool   `json:"below"`
	Fit     int    `json:"fit"`
}

// encodeProject writes the current page and the template as a project file.
func (a *App) encodeProject() ([]byte, error) {
	p := a.pic
	f := projectFile{
		Format: projectFormat, Version: projectVersion, App: AppName + " " + AppVersion,
		Background: p.BG, Border: p.Border, Active: p.Active, Palette: a.pal.Name,
	}
	for l := 0; l < c64.LayerCount; l++ {
		px := make([]byte, c64.W*c64.H)
		copy(px, p.L[l][:])
		f.Layers = append(f.Layers, projectLayer{Name: layerName(l), Visible: !p.Hide[l], Pixels: px})
	}
	if a.tpl.src != nil {
		var buf bytes.Buffer
		if err := png.Encode(&buf, a.tpl.src); err != nil {
			return nil, err
		}
		f.Template = &projectTemplate{Name: a.tpl.name, PNG: buf.Bytes(), Visible: a.tpl.visible,
			Opacity: a.tpl.opacity, Below: a.tpl.under, Fit: int(a.tpl.fit)}
	}
	js, err := json.Marshal(f)
	if err != nil {
		return nil, err
	}
	var out bytes.Buffer
	zw := gzip.NewWriter(&out)
	zw.Name = "project.json"
	if _, err := zw.Write(js); err != nil {
		return nil, err
	}
	if err := zw.Close(); err != nil {
		return nil, err
	}
	return out.Bytes(), nil
}

// isProject reports whether data looks like a project file (gzip).
func isProject(data []byte) bool {
	return len(data) > 2 && data[0] == 0x1f && data[1] == 0x8b
}

// decodeProject reads a project file into a picture and an optional template.
func decodeProject(data []byte) (*c64.Picture, *projectTemplate, image.Image, error) {
	zr, err := gzip.NewReader(bytes.NewReader(data))
	if err != nil {
		return nil, nil, nil, fmt.Errorf("not a C64 DPainter project: %v", err)
	}
	js, err := io.ReadAll(io.LimitReader(zr, 256<<20))
	if err != nil {
		return nil, nil, nil, fmt.Errorf("damaged project file: %v", err)
	}
	var f projectFile
	if err := json.Unmarshal(js, &f); err != nil || f.Format != projectFormat {
		return nil, nil, nil, fmt.Errorf("not a C64 DPainter project")
	}
	if f.Version > projectVersion {
		return nil, nil, nil, fmt.Errorf("this project was saved by a newer version of %s", AppName)
	}
	p := c64.NewPicture(f.Background, f.Border)
	p.ToLayers()
	for l := 0; l < c64.LayerCount && l < len(f.Layers); l++ {
		px := f.Layers[l].Pixels
		if len(px) != c64.W*c64.H {
			return nil, nil, nil, fmt.Errorf("damaged project file (layer %d)", l+2)
		}
		for i, v := range px {
			if v > c64.LayerBG {
				v = c64.LayerClear
			}
			p.L[l][i] = v
		}
		p.Hide[l] = !f.Layers[l].Visible
	}
	if f.Active >= 0 && f.Active < c64.LayerCount {
		p.Active = f.Active
	}
	p.Recomposite()
	var img image.Image
	if f.Template != nil && len(f.Template.PNG) > 0 {
		img, err = png.Decode(bytes.NewReader(f.Template.PNG))
		if err != nil {
			img = nil
		}
	}
	return p, f.Template, img, nil
}

// loadProject replaces the current page by a project file.
func (a *App) loadProject(data []byte, path string) bool {
	p, t, img, err := decodeProject(data)
	if err != nil {
		a.host.Info(AppName, err.Error())
		return false
	}
	if !a.confirmDiscard() {
		return false
	}
	a.pushUndo(a.pic.Clone())
	*a.pic = *p
	a.ditherRes = nil
	if img != nil {
		a.tpl.src = img
		a.tpl.name = t.Name
		a.tpl.visible = t.Visible
		a.tpl.opacity = clampI(t.Opacity, 0, 100)
		a.tpl.under = t.Below
		a.tpl.fit = c64.FitMode(clampI(t.Fit, 0, 2))
		a.tpl.rebuild()
	}
	return true
}
