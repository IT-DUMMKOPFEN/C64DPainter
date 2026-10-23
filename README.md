# C64 DPainter

[![Build](https://github.com/IT-DUMMKOPFEN/C64DPainter/actions/workflows/build.yml/badge.svg)](https://github.com/IT-DUMMKOPFEN/C64DPainter/actions/workflows/build.yml)

A paint program for Windows, macOS and Linux for pictures in the **multicolor bitmap mode of the Commodore 64**. Its look and controls follow Deluxe Paint and PyDPainter.

Version 0.99 beta · Created by IT-Dummköpfen · Free for private use.

## Download

- **Releases**: finished programs for every tagged version are on the [Releases](https://github.com/IT-DUMMKOPFEN/C64DPainter/releases) page.
- **Latest build**: every push to `main` is built automatically. Open the newest run under [Actions](https://github.com/IT-DUMMKOPFEN/C64DPainter/actions/workflows/build.yml) and download the programs under "Artifacts" (you must be signed in to GitHub).

| File | System |
|---|---|
| `C64DPainter.exe` | Windows 11 (64 bit), just start it – Windows 10 may crash |
| `C64DPainter-macOS.zip` | macOS 11 or newer, Apple silicon (M chips) only – read "How to open.txt" inside |
| `c64dpainter-linux-x86_64.tar.gz` | Linux on PCs (64 bit) |
| `c64dpainter-linux-arm64.tar.gz` | Linux on ARM (e.g. Raspberry Pi 4/5 with a 64-bit system) |

## Features

- Strict multicolor rules (160×200 pixels, 4×8 cells, bit pairs %00/%01/%10/%11). When a cell is full, the program can block the pixel, replace the rarest cell color or use the nearest cell color.
- Tools: dots, freehand, line, curve, fill, gradient fill, airbrush, dither, smear, shade, eraser, rectangle, square, circle, ellipse, triangle, hexagon and star (each outlined or filled), select and color picker.
- **Symmetry**: paint mirrored left ↔ right, top ↔ bottom or four-fold, with movable axes.
- **Shade** brush: lightens or darkens one step per stroke, by brightness or along a color ramp.
- **Mirrored view** (Shift+M) and a floating **CRT preview** window (100/200 %, scanlines and PAL color bleed).
- **Working folder** and **quick save** (F2): numbered PNGs without a dialog.
- **My brushes**: your saved brushes in one window, ready to stamp.
- **Text tool**: C64 wide, tall and narrow fonts in multicolor pixels, with shadow or outline, as brush.
- **PETSCII editor** (Tools menu): convert pictures to PETSCII, edit them and export PRG, ACME assembler source, PNG or Petmate files.
- **Sprite editor** (Tools menu): 64 hires or multicolor sprites, overlay, animation preview, import from the picture or a 320×200 image (single or as a raster), project files, ACME assembler export with all colors and settings. A sprite grid (24×21) in the View menu helps to draw sprites in the multicolor editor.
- **Font editor** (Tools menu): C64 character sets with 1×1 to 2×2 tiles, compare with the ROM, test text, import from the picture, a 320×200 image or the C64 font, project files, raw .bin/.64c and ACME assembler export.
- **Auto backup** (Options menu, off at start) into the working folder and a list of **recent files**.
- **Brushes** menu: load and save brushes as PNG with a transparent background.
- Round, square or diamond brushes from 1 to 16 pixels, plus custom brushes made from a selection.
- 42 patterns (density steps, dots, lines, diagonals, grids and textures).
- **Auto dither** (Edit menu) and a **Dither brush**, both experimental: blend the borders between flat color areas in clean steps (two colors per transition, 25 %/50 % bands, outlines and silhouettes stay sharp) in the styles Artist, Scanlines, Checker and Stripes, reconstructed from hand-pixelled C64 pictures – or shade with **Color**, which darkens with the colors of the cell.
- **Gradient fill**: 27 three-color, 14 four-color and 13 five-color C64 ramps plus your own custom ramps, plus five-band versions (dark → light → dark, light → dark → light), linear, circle or diamond, single click or drag for direction and width, optional dithered transitions.
- **Smear** brush: drags colors along the stroke like a smudge tool.
- Swap colors in the whole picture or with a cell brush (the background color is never touched).
- Selections can be copied (default) or moved (Shift), flipped, rotated by 90° or freely, scaled and sheared in both directions.
- Zoom from 1× to 32×, configurable grid (8×8 by default), pixel grid and C64 border.
- Only the 16 C64 colors, with 116 palette variants from c64gfx.com.
- Layers: three paint layers (layer 2 at the bottom, 4 on top; layers 3 + 4 have an on/off switch; show/hide, eraser) on which the multicolor rules apply together – every combination of visible layers is a valid C64 picture – plus a template layer (PNG/BMP/JPG/GIF, above or below the picture, with opacity) and a spare page.
- **Import & convert** for 320×200 images: live multicolor preview (multicolor / original / split view), brightness, contrast, saturation, hue and gamma, 13 dithering methods with adjustable strength, pixel-pair modes and background color choice, color matching in RGB, YUV, XYZ, Lab, OkLab or by brightness, and cell colors by best fit, most frequent or the same in all cells.
- Project files (.c64dp) keep all layers and the template.
- Loading: C64 DPainter projects, KLA/KOA, Advanced Art Studio (.ocp), Amica Paint (.ami), PNG, BMP.
- Saving: KLA, PNG (320×200, 160×200, 384×272 with border) and PRG (a runnable C64 program with a viewer).
- Foldable side panel sections, detailed built-in help (F1), light and dark theme, interface size 100/200/300 %, movable dialog windows.

## Building

All programs at once (Windows, macOS, Linux x86-64 and arm64, packaged into `dist/`):

```
tools/build-all.sh
```

This needs Go, Python 3 with Pillow (`pip install pillow`, for the macOS icon), `tar` and `zip`. The same script runs on GitHub (`.github/workflows/build.yml`).

### Making a release

Either on GitHub: **Actions → Build → Run workflow**, enter a tag such as `v0.99b` and start it. Or push a tag that starts with `v`:

```
git tag v0.99b
git push origin v0.99b
```

GitHub then builds everything and publishes a release with all four files.

### Single platforms

Requires Go 1.22 or newer and no cgo. The two dependencies are included in `third_party`: [purego](https://github.com/ebitengine/purego) (Apache 2.0, macOS front end) and [xgb](https://github.com/jezek/xgb) (BSD, Linux front end).

Windows:

```
GOOS=windows GOARCH=amd64 go build -trimpath -ldflags "-H windowsgui -s -w" -o C64DPainter.exe ./cmd/c64dpainter
```

macOS (app for Apple silicon, can be built on any OS):

```
CGO_ENABLED=0 GOOS=darwin GOARCH=arm64 go build -trimpath -ldflags "-s -w" -o build/C64DPainter_arm64 ./cmd/c64dpainter
python3 tools/macbundle.py build/C64DPainter_arm64 res/c64dpainter.png build 0.99
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

`go run ./cmd/screens <folder> <picture.kla> <image320x200.png>` loads a KLA picture and saves screenshots of the main functions (for web pages such as itch.io).

## Layout

| Folder | Contents |
|---|---|
| `internal/c64` | Multicolor model, KLA/PRG/PNG, BMP reader, image conversion and dithering, palettes |
| `internal/app` | User interface, tools, selection, dialogs, help (platform independent) |
| `internal/gfx`, `internal/font` | Software renderer and embedded DejaVu font |
| `cmd/c64dpainter` | Front ends: Windows (Win32 via syscall), macOS (Cocoa via purego), Linux (X11 via xgb) |
| `packaging` | Extra files for the Linux and macOS packages |
| `tools` | Build script, macOS bundler, icon, font and stamp generators, dither analysis |
| `third_party` | purego and xgb (with their licenses) |

## License

C64 DPainter is free for private use. Created by IT-Dummköpfen.

The included libraries keep their own licenses: purego (Apache 2.0, `third_party/purego/LICENSE`), xgb (BSD, `third_party/xgb/LICENSE`). The embedded font glyphs are rendered from the DejaVu fonts (Bitstream Vera / DejaVu license).

Licenses and credits of ideas and data:

| Source | License | Used for |
|---|---|---|
| C64 character ROM (`internal/c64/charset`) | © Commodore | PETSCII editor, font editor and text tool (as in Petmate and most PETSCII editors) |
| [Spritemate](https://github.com/Esshahn/spritemate) by Ingo Hinterding | MIT | sprite editor ideas (overlay, tools, ACME export) |
| [MagSpriteEd](https://github.com/MagnarHarestad/MagSpriteEd) by Magnar Harestad | MIT | sprite editor ideas (hires per sprite, color keys, animation) |
| [Sprite Studio 64](https://github.com/tstamborski/sprite-studio-64) by Tobiasz Stamborski | MIT | sprite editor ideas (64 sprites, animation range, overlay) |
| [KANJI](https://github.com/AMID64/KANJI) by AMID64 | MIT | font editor ideas (tiles, compare with the ROM, inverse half) |
| [C64 BDF font editor](https://github.com/garymeg/C64_BDF_font_editor) by garymeg | MIT | font editor ideas (raw font import, test view) |
| [Petmate](https://github.com/nurpax/petmate) by nurpax | MIT | ideas, the "sorted by shape" character order, the `.petmate` file format |
| [petsciirender](https://github.com/gnacu/petsciirender) by gnacu | no license given | ideas only, no code |
| [PETSCII editor](https://github.com/ventti/petscii) by Marq | WTFPL | ideas |
| [Image-to-PETSCII](https://github.com/amb/Image-to-PETSCII) by amb | GPL-3 | the idea of matching cells against characters, no code |
| [Retropixels](https://github.com/micheldebree/retropixels) by Michel de Bree | MIT | ideas for color matching and cell colors |
| [RECOIL](https://github.com/pfusik/recoil) by Piotr Fusik | MIT | format details of Advanced Art Studio and Amica Paint |
