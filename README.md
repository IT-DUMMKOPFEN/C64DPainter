# C64 DPainter

[![Build](https://github.com/IT-DUMMKOPFEN/C64DPainter/actions/workflows/build.yml/badge.svg)](https://github.com/IT-DUMMKOPFEN/C64DPainter/actions/workflows/build.yml)

A paint program for Windows, macOS (Apple silicon and Intel) and Linux for pictures in the **bitmap modes of the Commodore 64**: multicolor, hires and "hires free". Its look and controls follow Deluxe Paint and PyDPainter.

Version 1.0 · Created by IT-Dummköpfen · Free for private use.

## Download

- **Releases**: finished programs for every tagged version are on the [Releases](https://github.com/IT-DUMMKOPFEN/C64DPainter/releases) page.
- **Latest build**: every push to `main` is built automatically. Open the newest run under [Actions](https://github.com/IT-DUMMKOPFEN/C64DPainter/actions/workflows/build.yml) and download the programs under "Artifacts" (you must be signed in to GitHub).

| File | System |
|---|---|
| `C64DPainter.exe` | Windows 11 (64 bit), just start it – Windows 10 may crash |
| `C64DPainter-macOS-Silicon.zip` | macOS 11 or newer, Macs with Apple silicon (M chips) – read "How to open.txt" inside |
| `C64DPainter-macOS-Intel.zip` | macOS 11 or newer, Macs with an Intel processor – read "How to open.txt" inside |
| `c64dpainter-linux-x86_64.tar.gz` | Linux on PCs (64 bit) |
| `c64dpainter-linux-arm64.tar.gz` | Linux on ARM (e.g. Raspberry Pi 4/5 with a 64-bit system) |

### First start

The program is not signed with a paid developer certificate, so Windows and macOS warn the first time:

- **Windows:** if "Windows protected your PC" appears, click **More info**, then **Run anyway**.
- **macOS:** drag "C64 DPainter.app" into Applications, then run `xattr -cr "/Applications/C64 DPainter.app"` once in Terminal. Without Terminal: try to open the app, then click **Open Anyway** in System Settings → Privacy & Security.
- **Linux:** unpack the archive and start `./c64dpainter`; `./install.sh` adds a menu entry and an icon.

## Features

- Three modes, chosen at the start and with File → New: **Multicolor** (160×200 pixels, 4×8 cells, bit pairs %00/%01/%10/%11), **Hires** (320×200, standard bitmap mode, 2 colors per 8×8 cell) and **Hires free** (320×200, all colors; reduced to 2 per cell on export). When a cell is full, the program can block the pixel, replace the rarest cell color or use the nearest cell color – optionally without touching the other layers.
- Tools: dots, freehand, line, curve, fill, gradient fill, airbrush, dither, smear, shade, eraser, rectangle, square, circle, ellipse, triangle, hexagon and star (each outlined or filled), select and color picker.
- **Symmetry**: paint mirrored left ↔ right, top ↔ bottom or four-fold, with movable axes.
- **Shade** brush: lightens or darkens one step per stroke, by brightness or along a color ramp.
- **Mirrored view** (Shift+M) and a floating **CRT preview** window (100/200 %, scanlines and PAL color bleed).
- **Working folder** and **quick save** (F2): numbered PNGs without a dialog.
- **My brushes**: your saved brushes in one window, ready to stamp.
- **Text tool**: C64 wide, tall and narrow fonts in multicolor pixels, with shadow or outline, as brush.
- **PETSCII editor** (Tools menu): convert pictures to PETSCII, edit them and export PRG, ACME assembler source, PNG or Petmate files.
- **Sprite editor** (Tools menu): 64 hires or multicolor sprites, overlay, animation preview, import from the picture or a 320×200 image (single or as a raster), project files, ACME assembler export with all colors and settings. A sprite grid (24×21) in the View menu helps to draw sprites in the multicolor editor.
- **Font editor** (Tools menu): hires or multicolor C64 character sets with 1×1 to 2×2 tiles, compare with the ROM, test text, import from the picture, a 320×200 image or the C64 font, project files, raw .bin/.64c and ACME assembler export.
- **Crash recovery**: changed pictures are saved quietly every minute and offered again after a crash. **Auto backup** (Options menu, off at start) into the working folder and a list of **recent files**. Window position and size and the import settings are kept for the next start.
- **Brushes** menu: load and save brushes as PNG with a transparent background.
- Round, square or diamond brushes from 1 to 16 pixels, plus custom brushes made from a selection.
- 42 patterns (density steps, dots, lines, diagonals, grids and textures).
- **Auto dither** (Edit menu) and a **Dither brush**, both experimental: blend the borders between flat color areas in clean steps (two colors per transition, 25 %/50 % bands, outlines and silhouettes stay sharp) in the styles Artist, Scanlines, Checker and Stripes, reconstructed from hand-pixelled C64 pictures – or shade with **Color**, which darkens with the colors of the cell.
- **Gradient fill**: 27 three-color, 14 four-color and 13 five-color C64 ramps plus your own custom ramps, plus five-band versions (dark → light → dark, light → dark → light), linear, circle or diamond, single click or drag for direction and width, optional dithered transitions.
- **Smear** brush: drags colors along the stroke like a smudge tool.
- Swap colors in the whole picture or with a cell brush (a color can become the background color, the background itself is never swapped away).
- Selections can be copied (default) or moved (Shift), flipped, rotated by 90° or freely, scaled, sheared in both directions and **distorted in perspective** (corners opposite or free, top/bottom or left/right).
- Zoom from 1× to 32×, configurable grid (8×8 by default), pixel grid and C64 border.
- Only the 16 C64 colors, with 116 palette variants from c64gfx.com.
- Layers: three paint layers (layer 2 at the bottom, 4 on top; layers 3 + 4 have an on/off switch; show/hide, eraser) on which the multicolor rules apply together – every combination of visible layers is a valid C64 picture – plus a template layer (PNG/BMP/JPG/GIF, above or below the picture, with opacity) and a spare page.
- **Import & convert** for 320×200 images, with presets shown as small previews (Recommended, Retropixels look, Smooth, Sharp) and automatic contrast: live preview (multicolor / original / split view), brightness, contrast, saturation, hue and gamma, 16 dithering methods (ordered patterns as in Retropixels, blue noise, cell pattern, Yliluoma and error diffusion) with adjustable strength, pixel-pair modes and background color choice, color matching in RGB, YUV, XYZ, Lab, OkLab or by brightness, and cell colors by best fit, most frequent or the same in all cells.
- **Side border sprites** (multicolor, experimental): paint left and right of the picture into the opened side border – 10 multicolor sprites per side, exported as a cycle-exact C64 program (.prg) and ACME source (PAL only).
- Project files (.c64dp) keep all layers and the template.
- Loading: C64 DPainter projects, KLA/KOA, Advanced Art Studio (.ocp), Amica Paint (.ami), Art Studio hires (.art), Doodle (.dd), PNG, BMP.
- Saving: KLA (multicolor), Art Studio hires and Doodle (hires), PNG (320×200, 160×200, 384×272 with border, 640×400 and 1280×800) and PRG (a runnable C64 program with a viewer for multicolor or hires; with the side border open also a PRG and ACME source that show the border sprites).
- Foldable side panel sections, detailed built-in help (F1), light and dark theme, interface size 100/200/300 %, movable dialog windows.

## Building

All programs at once (Windows, macOS, Linux x86-64 and arm64, packaged into `dist/`):

```
tools/build-all.sh
```

This needs Go, Python 3 with Pillow (`pip install pillow`, for the macOS icon), `tar` and `zip`. The same script runs on GitHub (`.github/workflows/build.yml`).

### Making a release

Either on GitHub: **Actions → Build → Run workflow**, enter a tag such as `v1.0` and start it. Or push a tag that starts with `v`:

```
git tag v1.0
git push origin v1.0
```

GitHub then builds everything and publishes a release with all four files.

### Single platforms

Requires Go 1.22 or newer and no cgo. The two dependencies are included in `third_party`: [purego](https://github.com/ebitengine/purego) (Apache 2.0, macOS front end) and [xgb](https://github.com/jezek/xgb) (BSD, Linux front end).

Windows:

```
GOOS=windows GOARCH=amd64 go build -trimpath -ldflags "-H windowsgui -s -w" -o C64DPainter.exe ./cmd/c64dpainter
```

macOS (app for Apple silicon or Intel, can be built on any OS):

```
CGO_ENABLED=0 GOOS=darwin GOARCH=arm64 go build -trimpath -ldflags "-s -w" -o build/C64DPainter_arm64 ./cmd/c64dpainter
python3 tools/macbundle.py build/C64DPainter_arm64 res/c64dpainter.png build 1.0
# Intel: GOARCH=amd64
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
| [Retropixels](https://github.com/micheldebree/retropixels) by Michel de Bree | MIT | ideas for color matching, cell colors, automatic contrast and brightness-shifting ordered dithering |
| [RECOIL](https://github.com/pfusik/recoil) by Piotr Fusik | MIT | format details of Advanced Art Studio and Amica Paint |
| VIC-II article by Christian Bauer | – | timing rules for the side border program |
| [VICE](https://github.com/VICE-Team/svn-mirror) (x64sc) | GPL-2 | testing the side border program, no code included |
| [ACME](https://github.com/meonwax/acme) cross assembler | GPL-2 | checking the built-in assembler, no code included |
