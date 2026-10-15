# C64 DPainter

[![Build](https://github.com/IT-DUMMKOPFEN/C64DPainter/actions/workflows/build.yml/badge.svg)](https://github.com/IT-DUMMKOPFEN/C64DPainter/actions/workflows/build.yml)

A paint program for Windows, macOS and Linux for pictures in the **multicolor bitmap mode of the Commodore 64**. Its look and controls follow Deluxe Paint and PyDPainter.

Version 0.9 beta · Created by IT-Dummköpfen · Free for private use.

## Download

- **Releases**: finished programs for every tagged version are on the [Releases](https://github.com/IT-DUMMKOPFEN/C64DPainter/releases) page.
- **Latest build**: every push to `main` is built automatically. Open the newest run under [Actions](https://github.com/IT-DUMMKOPFEN/C64DPainter/actions/workflows/build.yml) and download the programs under "Artifacts" (you must be signed in to GitHub).

| File | System |
|---|---|
| `C64DPainter.exe` | Windows 10/11 (64 bit), just start it |
| `C64DPainter-macOS.zip` | macOS 11 or newer, Apple silicon and Intel – read "How to open.txt" inside |
| `c64dpainter-linux-x86_64.tar.gz` | Linux on PCs (64 bit) |
| `c64dpainter-linux-arm64.tar.gz` | Linux on ARM (e.g. Raspberry Pi 4/5 with a 64-bit system) |

## Features

- Strict multicolor rules (160×200 pixels, 4×8 cells, bit pairs %00/%01/%10/%11). When a cell is full, the program can block the pixel, replace the rarest cell color or use the nearest cell color.
- Tools: dots, freehand, line, curve, fill, airbrush, rectangle, circle and ellipse (each outlined or filled), select and color picker.
- Round, square or diamond brushes from 1 to 16 pixels, plus custom brushes made from a selection.
- 42 patterns (density steps, dots, lines, diagonals, grids and textures).
- **Auto dither** (Edit menu) and a **Dither brush**: blend the borders between flat color areas in seven styles (Artist, Scanlines, Hatching, Checker, Columns, Stipple, Organic) that were reconstructed from hand-pixelled C64 pictures, with live preview, softness, strength, edge protection and color ramps.
- Swap colors in the whole picture or with a cell brush (the background color is never touched).
- Selections can be moved, copied, flipped, rotated by 90° or freely, scaled and sheared in both directions.
- Zoom from 1× to 32×, configurable grid (8×8 by default), pixel grid and C64 border.
- Only the 16 C64 colors, with 116 palette variants from c64gfx.com.
- Layers: a template (PNG/BMP/JPG/GIF, above or below the picture, with opacity) and the picture, plus a spare page.
- **Import & convert** for 320×200 images: live multicolor preview (multicolor / original / split view), brightness, contrast, saturation, hue and gamma, 13 dithering methods with adjustable strength, pixel-pair modes and background color choice.
- Loading: KLA/KOA, PNG, BMP.
- Saving: KLA, PNG (320×200, 160×200, 384×272 with border) and PRG (a runnable C64 program with a viewer).
- Detailed built-in help (F1), light and dark theme, interface size 100/200/300 %, movable dialog windows.

## Building

All programs at once (Windows, macOS, Linux x86-64 and arm64, packaged into `dist/`):

```
tools/build-all.sh
```

This needs Go, Python 3 with Pillow (`pip install pillow`, for the macOS icon), `tar` and `zip`. The same script runs on GitHub (`.github/workflows/build.yml`).

### Making a release

Either on GitHub: **Actions → Build → Run workflow**, enter a tag such as `v0.9b` and start it. Or push a tag that starts with `v`:

```
git tag v0.9b
git push origin v0.9b
```

GitHub then builds everything and publishes a release with all four files.

### Single platforms

Requires Go 1.22 or newer and no cgo. The two dependencies are included in `third_party`: [purego](https://github.com/ebitengine/purego) (Apache 2.0, macOS front end) and [xgb](https://github.com/jezek/xgb) (BSD, Linux front end).

Windows:

```
GOOS=windows GOARCH=amd64 go build -trimpath -ldflags "-H windowsgui -s -w" -o C64DPainter.exe ./cmd/c64dpainter
```

macOS (universal app for Apple silicon and Intel, can be built on any OS):

```
CGO_ENABLED=0 GOOS=darwin GOARCH=arm64 go build -trimpath -ldflags "-s -w" -o build/C64DPainter_arm64 ./cmd/c64dpainter
CGO_ENABLED=0 GOOS=darwin GOARCH=amd64 go build -trimpath -ldflags "-s -w" -o build/C64DPainter_amd64 ./cmd/c64dpainter
python3 tools/macbundle.py build/C64DPainter_arm64 build/C64DPainter_amd64 res/c64dpainter.png build 0.9
```

Linux (static binary, X11 or Wayland via XWayland, no libraries needed):

```
CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -trimpath -ldflags "-s -w" -o c64dpainter ./cmd/c64dpainter
CGO_ENABLED=0 GOOS=linux GOARCH=arm64 go build -trimpath -ldflags "-s -w" -o c64dpainter-arm64 ./cmd/c64dpainter
```

On Linux the file dialogs of `zenity` or `kdialog` are used when installed, otherwise a built-in file browser (force it with `C64DPAINTER_DIALOGS=builtin`).

On a Mac you can additionally sign the bundle ad hoc with `codesign --force --deep -s - "build/C64 DPainter.app"`.

The file `cmd/c64dpainter/rsrc_windows_amd64.syso` contains the icon and the manifest. It was created with `tools/genicon.py` and [rsrc](https://github.com/akavel/rsrc).

Tests (they also run on Linux):

```
go test ./internal/...
```

`go run ./cmd/shot <folder> <image.png> <image320x200.png>` drives the user interface with a script and saves screenshots.

## Layout

| Folder | Contents |
|---|---|
| `internal/c64` | Multicolor model, KLA/PRG/PNG, BMP reader, image conversion and dithering, palettes |
| `internal/app` | User interface, tools, selection, dialogs, help (platform independent) |
| `internal/gfx`, `internal/font` | Software renderer and embedded DejaVu font |
| `cmd/c64dpainter` | Front ends: Windows (Win32 via syscall), macOS (Cocoa via purego), Linux (X11 via xgb) |
| `packaging` | Extra files for the Linux and macOS packages |
| `tools` | Build script, macOS bundler, icon and font generators, dither analysis |
| `third_party` | purego and xgb (with their licenses) |

## License

C64 DPainter is free for private use. Created by IT-Dummköpfen.

The included libraries keep their own licenses: purego (Apache 2.0, `third_party/purego/LICENSE`), xgb (BSD, `third_party/xgb/LICENSE`). The embedded font glyphs are rendered from the DejaVu fonts (Bitstream Vera / DejaVu license).
