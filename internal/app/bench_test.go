package app

import (
	"os"
	"path/filepath"
	"testing"
)

type nullHost struct{}

func (nullHost) OpenFile(string, []Filter, string) (string, bool)                 { return "", false }
func (nullHost) SaveFile(string, []Filter, string, string, string) (string, bool) { return "", false }
func (nullHost) Ask(string, string) int                                           { return 1 }
func (nullHost) Info(string, string)                                              {}
func (nullHost) Quit()                                                            {}
func (nullHost) SetTitle(string)                                                  {}
func (nullHost) ReadFile(p string) ([]byte, error)                                { return os.ReadFile(p) }
func (nullHost) WriteFile(p string, d []byte) error                               { return nil }

func newTestApp(t testing.TB) *App {
	dir := t.TempDir()
	SettingsPath = func() string { return filepath.Join(dir, "s.json") }
	a := New(nullHost{})
	a.modal = nil
	a.Resize(1360, 860)
	a.Frame()
	return a
}

func BenchmarkFrame(b *testing.B) {
	a := newTestApp(b)
	for i := 0; i < b.N; i++ {
		a.MouseMove(600+i%50, 400)
		a.Frame()
	}
}

func BenchmarkFrameZoom8Template(b *testing.B) {
	a := newTestApp(b)
	a.zoom = 8
	img := a.pic.ToImage(a.pal, 0)
	a.setTemplate(img, "x")
	for i := 0; i < b.N; i++ {
		a.MouseMove(600+i%50, 400)
		a.Frame()
	}
}
