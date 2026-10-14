package app

var helpTopics = []helpTopic{
	{id: "start", group: "Getting started", title: "First steps", body: `
# Welcome to C64 DPainter

C64 DPainter is a paint program for pictures in the **multicolor bitmap mode** of the Commodore 64. Everything you paint automatically follows the rules of the C64's VIC-II graphics chip. What you see on screen is exactly what a real C64 can display.

## Your first picture in five steps

- Pick a tool in the toolbox on the right, for example **Freehand** (key ` + "`D`" + `).
- Click a color in the palette at the bottom right with the **left** mouse button – that is **color 1**. The **right** mouse button picks **color 2**.
- Paint with the left mouse button in color 1, with the right button in color 2.
- The mouse wheel zooms in and out. Hold **Space** and drag to move the view.
- Save with ` + "`Ctrl+S`" + ` as a Koala file (.kla), or use **File → Export** for PNG or a runnable C64 program (.prg).

> **Important:** every 4×8 pixel cell can contain at most 4 colors, and one of them is the background color, which is shared by the whole picture. If a cell is full and you paint a fifth color, the cell flashes red. See "Multicolor rules".

{MACNOTE}

-> rules | Understand the multicolor rules
-> import | Import and convert a 320×200 picture
-> ui | The user interface at a glance
-> keys | All keyboard shortcuts
`},
	{id: "ui", title: "The user interface", body: `
# The user interface

The layout follows **Deluxe Paint** and **PyDPainter**: the menu bar at the top, the toolbox with the palette on the right and the canvas in the middle. On the left there is a side panel with all settings (show or hide it with ` + "`Tab`" + `).

## Menu bar

File, edit, selection and view commands, options (palette, color conflicts, theme, interface size) and this help. The sun/moon icon at the top right switches between the light and the dark theme.

**Options → Interface size** makes the whole interface 100 %, 200 % or 300 % large; the window is resized to match. Sizes that do not fit on your screen are greyed out.

## Toolbox (right)

- At the top: twelve **brush presets** (round, square and diamond, different sizes).
- Below: the **tools**. Split buttons (rectangle, circle, ellipse): click the upper left half for an outline, the lower right half for a filled shape.
- **Right-clicking** a tool button opens its help or settings (grid, template, zoom).
- The **color 1 / color 2** display. Clicking it swaps both colors.
- The **palette** with the 16 C64 colors. "B" marks the background color (Ctrl+click a color to make it the background).

## Side panel (left)

- **Selection** (only with the select tool): rotate, scale, shear.
- **Brush**: shape (round, square, diamond, custom), size 1–16, preview, airbrush settings.
- **Patterns**: fill patterns for all tools.
- **Colors**: background ($D021), border ($D020) and the behavior for color conflicts.
- **Layers**: template layer and picture layer.
- **Cell under the mouse**: the four colors of the cell and which bit pairs are free.
- **Navigator**: the whole picture in small; click or drag to choose the visible area.

## Dialog windows

All dialog windows (help, about, palette, grid, import, transform …) can be moved inside the program window: drag them by their **title bar**. A **double-click** on the title bar centers the window again. Each window remembers its place until you quit the program.

The **Rotate, scale and shear** window does not darken the picture, so you can move it aside and watch the selection change.

## Status bar (bottom)

On the left you see hints for the active tool or messages. In the middle: position, cell, bit pair and color under the mouse. On the right: zoom, palette and conflict mode.
`},
	{id: "rules", title: "Multicolor rules", body: `
# The multicolor rules

In multicolor bitmap mode the picture has **160 × 200 pixels**. Every pixel consists of two bits (a **bit pair**) and is therefore twice as wide as it is high. The picture is divided into **40 × 25 cells** of **4 × 8 pixels** each.

The bit pair of a pixel decides **where** its color comes from:

@cells

- ` + "`%00`" + ` – background color from register **$D021**. It is shared by the whole picture.
- ` + "`%01`" + ` – bits 4–7 of the matching byte in **screen RAM** (normally $0400–$07E7).
- ` + "`%10`" + ` – bits 0–3 of the same screen RAM byte.
- ` + "`%11`" + ` – the matching byte in **color RAM** ($D800–$DBE7).

So: **every cell can have at most 4 colors** – the shared background color plus three freely chosen colors.

## How the program keeps the rules

You just paint with one of the 16 colors. The program finds the right bit pair for every pixel automatically:

- If the color is the background color, ` + "`%00`" + ` is used.
- If the color already belongs to a bit pair of the cell, that bit pair is used.
- If a bit pair is free (no other pixel of the cell uses it), it gets the new color.
- Otherwise the cell is **full**. What happens then is set under "Color conflicts".

> When you paint over a pixel that was the only one using a color, its bit pair becomes free. So you can always "swap" a color by painting over all of its pixels in the cell.

## Changing the background color

Set the background color on the left under **Colors** (or ` + "`Ctrl`" + `+click a palette color). Just like on a real C64, **all** pixels with bit pair ` + "`%00`" + ` change at once.

-> conflict | Color conflict settings
-> tech | Technical background (memory, registers)
`},
	{id: "conflict", title: "Color conflicts", body: `
# Color conflicts

A conflict happens when you want to paint a **fifth** color into a cell that already uses the background plus 3 colors. The cell flashes red. Under **Options → Color conflict** or on the left under **Colors** you choose what happens:

## Block (strict, default)

The pixel is **not** set. Nothing else in the picture changes. This is the safest setting when you want exact control over which colors end up in which cell.

## Replace rarest cell color

The color used by the **fewest pixels** in the cell is replaced by the new color. The other pixels of that color in the cell change too. Handy for quick recoloring.

## Use nearest cell color

Instead of the requested color, the **most similar** of the four existing cell colors is painted. Good for shading and the airbrush.

> Whatever you choose: the picture never breaks the rules. The mode only decides how the conflict is resolved.

## Tips to avoid conflicts

- Use the most frequent color of your picture as the background.
- Turn on the 8×8 grid (` + "`G`" + `). Every square is exactly one cell.
- Watch **Cell under the mouse** on the left: you see right away which bit pairs are still free.
- Put color transitions on cell borders where possible.
`},
	{id: "colors", title: "Colors & palettes", body: `
# Colors and palettes

The C64 has exactly **16 fixed colors**. The program cannot use any other colors.

@palette

## Color 1 and color 2

- **Left-click** a palette color to choose color 1 (painted with the left mouse button).
- **Right-click** chooses color 2 (painted with the right mouse button).
- ` + "`Ctrl`" + `+click sets the **background color** ($D021). The background color is marked with **B** in the palette.
- ` + "`[`" + ` and ` + "`]`" + ` step color 1 through the palette.
- The **color picker** (` + "`,`" + ` or ` + "`Alt`" + `+click) picks a color from the picture.

## Palettes

The 16 colors look a little different on every TV and in every emulator. That is why there are many **interpretations** of the palette (Pepto, Colodore, VICE, CCS64 and many more; source: c64gfx.com). Choose one with **Options → Choose palette** or the ` + "`P`" + ` key. The arrow keys move through the list and show the effect immediately.

> The palette only changes the **display** and the **PNG export**. KLA and PRG files store color numbers (0–15) – a C64 always shows its own colors.

**Importing** PNG/BMP files compares against the selected palette. A PNG exported with the same palette can therefore be opened again without any loss.

## Border color

The border color ($D020) is not part of the picture. It is only shown on screen and used for the "PNG with border" export and the C64 program (.prg).
`},
	{id: "tools", group: "Painting", title: "Tools", body: `
# Tools

@tools

## [dots] Dots (S)

Sets single brush dabs while the mouse button is held – without connecting lines. Fast movements leave gaps. Hold ` + "`Ctrl`" + ` for a straight row of dabs.

## [draw] Freehand (D)

Paints continuous lines. Left mouse button = color 1, right mouse button = color 2.

**Straight lines:** hold ` + "`Ctrl`" + ` while painting. A straight line runs from the point where you pressed ` + "`Ctrl`" + ` (or started the stroke) to the mouse and follows it until you let go. ` + "`Shift`" + ` additionally snaps it to horizontal, vertical or 45°. Release ` + "`Ctrl`" + ` during the stroke and you continue painting freehand from the end of the line.

## [line] Line (V)

Click and drag. Hold ` + "`Shift`" + ` to snap the line horizontally, vertically or to 45° (measured on screen).

## [curve] Curve (Q)

First drag the base line and release. Then move the mouse – the curve bends towards the pointer. A click fixes it, ` + "`Esc`" + ` cancels.

## [fill] Fill (F)

Fills the connected area of the same color (connected horizontally and vertically). The selected **pattern** is used. In full cells the conflict mode decides.

## [spray] Airbrush (A)

Sprays brush dabs into a round area while the mouse button is held. Set radius and density on the left in the brush panel while the airbrush is active.

## [rect|rectf] Rectangle (R / Shift+R)

Drag from corner to corner. Outlines use the brush, filled shapes use the pattern. ` + "`Shift`" + ` = square (on screen).

## [circle|circlef] Circle (C / Shift+C)

Drag from the center. The circle is round on screen – so in pixels it is twice as high as wide.

## [ellipse|ellipsef] Ellipse (E / Shift+E)

Drag from the center. Width and height follow the mouse. ` + "`Shift`" + ` = circle.

## [select] Select (B)

Drag a frame, then move, copy, flip, rotate, scale and shear. See "Selection & transform".

## [picker] Color picker (,)

A click picks the color (left = color 1, right = color 2) and returns to the previous tool. ` + "`Alt`" + `+click does the same with any tool.

## [hand] Pan

Moves the view. Faster: hold ` + "`Space`" + ` or drag with the middle mouse button.

-> brushes | Brushes
-> patterns | Patterns
-> select | Selection & transform
`},
	{id: "brushes", title: "Brushes", body: `
# Brushes

All tools except Fill and Select paint with the current brush.

## Shape and size

- **Round**, **Square** or **Diamond**, size **1 to 16 pixels** (on the left in the brush panel or at the top of the toolbox).
- **Diamond** is a square rotated by 45° – good for sharp corners, sparkles and calligraphic strokes. Very small diamonds (size 1–2, or 3–4 with "Round on screen") look like a square, because there are not enough pixels for the tips.
- ` + "`+`" + ` and ` + "`-`" + ` change the size, ` + "`.`" + ` selects the 1-pixel brush.
- The size is the **height** in pixels.

## Round on screen (2:1)

Multicolor pixels are twice as wide as high. With **"Round on screen"** turned on (default), the width is halved. A round size-16 brush is then 8 × 16 pixels and really looks round on screen. Without the option it is 16 × 16 pixels (twice as wide as high on screen). The same applies to square and diamond brushes: with the option, a diamond has equal sides on screen.

## Custom brush

Select an area with the select tool and click **"Use as brush"**. Now you paint with this part of the picture – just like in Deluxe Paint.

- **Brush keeps its own colors**: on = the area is stamped with its own colors; off = it is painted in color 1 (and the pattern).
- The **right** mouse button always paints the brush in color 2.
- ` + "`X`" + `, ` + "`Y`" + `, ` + "`Z`" + `, ` + "`H`" + ` also flip, rotate and shrink the custom brush.
- ` + "`Shift+B`" + ` returns to the standard brush.
- With **"Background color transparent"** (Selection menu), background-colored pixels are skipped when stamping.
`},
	{id: "patterns", title: "Patterns", body: `
# Patterns

Patterns decide **which pixels** are painted. They work with every tool: brushes, lines, shapes, fill and airbrush.

- Set pixels of the pattern are painted in the current color (color 1 with the left button, color 2 with the right button). Pixels in the **gaps stay unchanged** – so you can lay a pattern over existing artwork.
- To get a two-colored pattern, first fill the area with one color, then paint the pattern on top in the other color.

## The 42 patterns

The panel shows them in six rows of seven:

- **Density**: Solid, 1/16, 1/8, 1/4, 3/8, Checkerboard 1/2, 5/8
- **More density and dots**: 3/4, 7/8, 15/16, Stipple, Noise, Fine dots, Coarse dots
- **Lines**: horizontal, sparse horizontal, wide stripes, vertical, sparse vertical, wide columns, dashes
- **Diagonals**: diagonal / and \, wide diagonal / and \, zigzag, chevrons, stairs
- **Grids and checkers**: grid, cross-hatch, crosses, diamonds, checker 2×2, checker 4×4, bricks
- **Textures**: tiles, weave, basket weave, scales, waves, studs, rings

The density patterns from 1/16 to 15/16 are ideal for gradients with only a few colors. Hover over a pattern to see its name.

> Patterns are fixed to the pixel grid (8 × 8 pixels). That way neighboring areas fit together seamlessly.
`},
	{id: "swap", title: "Swap colors", body: `
# Swap colors

Two functions in the **Edit** menu exchange colors without breaking the multicolor rules. Both only change where a cell takes its colors from (screen RAM and color RAM), so the number of colors per cell never grows.

> The **background color** ($D021) is never changed by these functions. It is marked with "B" and cannot be chosen. To change it, use **Colors → Background** in the side panel.

## Swap color in picture

**Edit → Swap color in picture …** replaces one color by another in the whole picture.

- Choose the color to replace in the upper row and the new color in the lower row.
- The dialog shows how many pixels and cells use the color.
- **Exchange both colors (A ↔ B)**: A becomes B and at the same time B becomes A.
- If a cell already contains the new color, the pixels are moved to it and the old color slot becomes free.
- One step: ` + "`Ctrl+Z`" + ` undoes it.

## Swap color in cells (brush)

**Edit → Swap color in cells (brush) …** asks for the two colors and then turns the mouse into a **cell brush**:

- Paint over the picture: in every **cell** (4 × 8 pixels) the brush touches, the first color becomes the second one. The whole cell changes, not only the pixels under the brush.
- The cells under the brush are highlighted before you click.
- Each cell is changed only once per stroke, so painting back and forth does no harm.
- The **right** mouse button swaps the other way round (second color → first color).
- The brush size from the brush panel decides how many cells you reach at once.
- The side panel shows both colors, with **Change colors …** and **Stop**.
- The brush ends as soon as you choose **any other tool** in the toolbox (or with its key), or press ` + "`Esc`" + `.
`},
	{id: "select", title: "Selection & transform", body: `
# Selection and transform

## Selecting

Choose the select tool (` + "`B`" + `) and drag a frame. ` + "`Ctrl+A`" + ` selects the whole picture. Right-click or ` + "`Esc`" + ` drops the selection.

## Moving and copying

- Dragging inside the selection **moves** the content. The old place becomes background (` + "`%00`" + `).
- With ` + "`Ctrl`" + ` held while dragging you **copy**. Dragging an already floating selection with ` + "`Ctrl`" + ` leaves a copy behind (stamping).
- The arrow keys move by 1 pixel, with ` + "`Shift`" + ` by 4 pixels.
- ` + "`Enter`" + ` applies the selection to the picture, ` + "`Esc`" + ` cancels and restores the previous state.

## Handles

- The eight white handles change the size (` + "`Shift`" + ` on a corner keeps the aspect ratio).
- The round handle above the selection **rotates** it (` + "`Shift`" + ` = 15° steps).

## Transformations

On the left in the side panel (with the select tool active) or with **Selection → Rotate, scale, shear …**:

- **↔ / ↕** flip (` + "`X`" + ` / ` + "`Y`" + `)
- **↻ / ↺** rotate by 90° (` + "`Z`" + ` / ` + "`Shift+Z`" + `)
- **½ / 2×** half or double size (` + "`H`" + ` / ` + "`Shift+H`" + `) – for quick shrinking
- **Rotate** by any angle (−180° to 180°)
- **Width / height** from 5 % to 400 %
- **Shear ↔ / ↕** – slant in both directions

All values always refer to the **original** content, so trying things out never loses quality. Rotations are calculated on screen and take the wide multicolor pixels into account.

> The result of a transformation also follows the multicolor rules. In full cells the conflict mode decides.

## Clipboard

` + "`Ctrl+C`" + ` copies, ` + "`Ctrl+X`" + ` cuts, ` + "`Ctrl+V`" + ` pastes at the mouse position, ` + "`Del`" + ` deletes the content of the selection.
`},
	{id: "view", group: "View", title: "Zoom, view & grid", body: `
# Zoom, view and grid

## Zooming

- **Mouse wheel** over the picture: zoom in/out at the mouse position.
- ` + "`>`" + ` and ` + "`<`" + ` or the magnifier buttons in the toolbox.
- ` + "`Ctrl+0`" + ` or ` + "`Home`" + `: fit to window. ` + "`Ctrl+1`" + `: 100 % (320 × 200 screen pixels).
- Zoom levels from 1× to 32×.

## Moving the view

- Hold ` + "`Space`" + ` and drag, drag with the **middle mouse button** or use the pan tool.
- ` + "`Shift`" + `+mouse wheel scrolls horizontally, ` + "`Ctrl`" + `+mouse wheel vertically.
- The arrow keys move the view when there is no selection.
- Click or drag in the **navigator** at the bottom left.

## Grid

` + "`G`" + ` (or the grid button) turns the grid on and off. The default **8 × 8** shows exactly the color cells of multicolor mode.

With **View → Grid settings …** (` + "`Shift+G`" + ` or right-click the grid button) you set:

- **Width and height in screen pixels** (320 × 200 grid). A multicolor pixel is 2 screen pixels wide, so the width changes in steps of 2.
- **Offset**, **opacity** and **color** (automatic = inverted, so it is visible on every color).
- Presets: 8×8 (cell), 16×16, 32×32, 2×1 (one pixel), 4×4.

{MACNOTE}

## More display aids

- **Pixel grid** (` + "`M`" + `): thin lines around every pixel from zoom 4×.
- **Highlight cell under the mouse**: dashed frame around the current cell.
- **Show border**: shows the C64 border in the border color ($D020), like on a TV.
`},
	{id: "layers", title: "Layers & template", body: `
# Layers and template

There are two layers: the **template** and the **picture** (the actual C64 graphics).

## Template layer

Load a PNG, BMP, JPG or GIF image as a template (**File → Load template …**, the button in the side panel or right-click the layer button). The template does **not** become part of the C64 graphics – it is only for tracing.

- **Eye icon** or key ` + "`T`" + `: show/hide.
- **Opacity** 0–100 %.
- **Above picture**: the template lies semi-transparent over your drawing.
- **Below picture**: the template only shows through pixels with background bit pair ` + "`%00`" + `. So you see what has not been painted yet.
- **Stretch / Fit / Fill** decides how the template is mapped to 320 × 200.
- **Convert template to C64 picture …** opens the converter with all its settings.

## Picture layer

The eye icon of the picture layer hides your drawing so you can see the template alone. You can still paint.

## Spare page

Like Deluxe Paint there is a second page. ` + "`J`" + ` switches between both pages, **Edit → Copy picture to other page** keeps a backup. Each page has its own undo history.
`},
	{id: "import", group: "Files", title: "Import & convert", body: `
# Import and convert pictures

**File → Import image (320×200) …** (` + "`Ctrl+I`" + `) opens the converter. It also opens when you open a PNG or BMP file with **File → Open** or drop one onto the window.

The converter is made for pictures with **320 × 200 pixels** – the size of the C64 screen. Such pictures are taken over 1:1. Other sizes are scaled first. 160 × 200 pictures are treated as multicolor pixels.

## Preview

The large preview shows the result **in multicolor**, exactly as the C64 will show it, and updates immediately with every change.

- **Multicolor** – the converted result.
- **Original** – the source picture with your adjustments.
- **Split** – left half original, right half multicolor.
- **Hold the mouse button** on the preview to see the original for a moment.

Below the preview you see the chosen background color and how many cells use all four colors.

## Use as

- **C64 picture**: the picture is converted following the multicolor rules.
- **Template**: the picture becomes the template layer for tracing.
- **Both**: converted picture plus template.

## Size and pixel pairs

- **Stretch / Fit / Fill** – only for pictures that are not 320 × 200.
- **Pixel pairs**: a multicolor pixel is as wide as two screen pixels. **Average** mixes both, **Left pixel** or **Right pixel** takes only one of them. For pixel art drawn in 320 × 200 the left or right pixel often gives sharper results.

## Adjustments

= Brightness | −100 to +100, makes the picture darker or lighter
= Contrast | −100 to +100, flattens or strengthens light/dark differences
= Saturation | −100 (grey) to +100 (twice as colorful)
= Hue | −180° to +180°, rotates all colors around the color wheel
= Gamma | −100 to +100, darkens or brightens the mid-tones only

**Reset adjustments** sets everything back to 0.

## Dithering

Dithering mixes neighboring pixels of the cell colors to simulate colors the C64 does not have. **Strength** (0–100 %) controls how strong the effect is. 0 % equals no dithering.

**Ordered (pattern) methods** – regular patterns, very "C64-like", stable when pictures move:

= None | every pixel gets the nearest allowed color
= Bayer 2×2 | very fine, coarse steps
= Bayer 4×4 | classic dither pattern, good all-rounder
= Bayer 8×8 | finest gradients
= Clustered dot | dots grow like newspaper print
= Line pattern | horizontal lines, typical for C64 demos
= Noise | random pattern, natural look

**Error diffusion** – distributes the color error to the neighboring pixels, more detail for photos:

= Floyd–Steinberg | the classic, sharp and detailed
= Atkinson | lighter, more contrast, loses some detail in dark/light areas
= Jarvis-Judice-Ninke | spreads further, smoother
= Stucki | similar to Jarvis, a little sharper
= Burkes | fast and smooth
= Sierra | balanced between Floyd–Steinberg and Jarvis
= Sierra Lite | very compact, little spreading

**Serpentine scanning** (error diffusion only) processes every other row from right to left. This avoids diagonal streaks.

## Background color

**A** chooses the background color automatically (the one with the smallest total error). Or pick one of the 16 colors yourself.

## How the conversion works

For every 4×8 cell, the three colors are chosen from all useful combinations that give the smallest color error together with the background color. Then every pixel is assigned to one of these four colors – with the chosen dithering. The result always follows the multicolor rules.

> The converter remembers your last settings while the program is running.
`},
	{id: "formats", title: "Saving & file formats", body: `
# Saving and file formats

## Koala Painter (.kla / .koa) – standard format

The classic C64 format for multicolor pictures, 10 003 bytes:

= 2 bytes | load address $6000
= 8000 bytes | bitmap (bit pairs, in the VIC-II cell layout)
= 1000 bytes | screen RAM (colors %01 and %10)
= 1000 bytes | color RAM (color %11)
= 1 byte | background color

` + "`Ctrl+S`" + ` saves in the format of the current file, a new picture as .kla. Files without a load address (10 001 bytes) are also read.

## PNG

- **320 × 200**: as on screen (every pixel 2 pixels wide). Default for "Save as" with .png.
- **160 × 200**: one pixel per multicolor pixel.
- **384 × 272 with border**: like a PAL TV with border color.

PNGs are saved with the currently selected palette and 16 indexed colors.

## C64 program (.prg)

A **runnable program** with a built-in viewer. See "Running the C64 program".

## Loading

**PNG**, **BMP** (1/4/8/16/24/32 bit, also RLE compressed) and **KLA/KOA**. Templates can also be JPG and GIF.

-> prg | Running the C64 program
-> import | Import & convert
`},
	{id: "prg", title: "Running the C64 program", body: `
# The C64 program (.prg)

**File → Export → C64 program** (` + "`Ctrl+E`" + `) creates a file that starts directly on a real C64 or in an emulator (e.g. VICE).

## Starting

- In the emulator: drag the file onto the window (autostart), or
- on the C64: ` + "`LOAD\"PICTURE\",8`" + ` and then ` + "`RUN`" + `.

The picture appears immediately. **Space** resets the C64.

## What is inside

= $0801 | BASIC line 2026 SYS2061
= $080D | viewer in machine code
= $2000 | bitmap (8000 bytes)
= $3F40 | screen colors – copied to $0400
= $4328 | color RAM data – copied to $D800

The viewer disables interrupts (` + "`SEI`" + `), sets border ($D020) and background ($D021), copies the colors and then switches:

= $D011 = $3B | bitmap mode on, screen on, 25 rows
= $D016 = $18 | multicolor on, 40 columns
= $D018 = $18 | screen at $0400, bitmap at $2000
= $DD00 | VIC bank 0 ($0000–$3FFF)
`},
	{id: "keys", group: "Reference", title: "Keyboard shortcuts", body: `
# Keyboard shortcuts

As in Deluxe Paint, letter keys work directly, without Ctrl.

## Tools

= S | Dots
= D | Freehand
= V | Line
= Q | Curve
= F | Fill
= A | Airbrush
= R / Shift+R | Rectangle / filled
= C / Shift+C | Circle / filled
= E / Shift+E | Ellipse / filled
= B | Select
= , | Color picker (also Alt+click)
= Space | hold: move the view
= Ctrl | while painting (freehand, dots): straight line, with Shift at 45°

## Brush, colors, patterns

= + / - | brush larger / smaller
= . | 1-pixel brush
= Shift+B | standard brush (instead of custom brush)
= [ / ] | color 1 back / forward
= P | choose palette

## Selection and custom brush

= X / Y | flip horizontally / vertically
= Z / Shift+Z | rotate 90° / back
= H / Shift+H | half / double size
= Enter | apply selection
= Esc | cancel selection, curve or shape
= Del | delete the content of the selection
= Arrow keys | move selection (Shift = 4 pixels)
= Ctrl+C / Ctrl+X / Ctrl+V | copy / cut / paste
= Ctrl+A | select all

## View

= Mouse wheel | zoom at the mouse position
= > / < | zoom in / out
= Ctrl+0 or Home | fit to window
= Ctrl+1 | 100 %
= G / Shift+G | grid on/off / settings
= M | pixel grid
= T | template on/off
= Tab | side panel on/off

## Edit and files

= U or Ctrl+Z | undo
= Shift+U or Ctrl+Y | redo
= J | swap page (spare page)
= Shift+K | clear picture
= Ctrl+N | new
= Ctrl+O | open
= Ctrl+I | import & convert image
= Ctrl+S / Ctrl+Shift+S | save / save as
= Ctrl+E | export as C64 program
= F1 / ? | help / keyboard shortcuts
= Ctrl+Q | exit
`},
	{id: "tech", title: "Technical background", body: `
# Technical background

## Memory areas

= Bitmap | 8000 bytes, default $2000–$3F3F (configurable)
= Screen RAM | 1000 bytes, default $0400–$07E7 (configurable)
= Color RAM | 1000 nibbles, fixed at $D800–$DBE7
= Background | register $D021

## Bitmap layout

The bitmap is not stored line by line but **cell by cell**: 8 bytes for cell 0 (top left), then 8 bytes for cell 1 next to it, and so on. One row of cells takes 40 × 8 = 320 bytes. For a pixel (X = 0–159, Y = 0–199):

= Address | base + (Y / 8) × 320 + (X / 4) × 8 + (Y AND 7)
= Bit pair | bits 7–6 for X AND 3 = 0, bits 5–4 for 1, bits 3–2 for 2, bits 1–0 for 3

The matching byte in screen and color RAM has the index (Y / 8) × 40 + (X / 4).

## Registers

= $D011 bit 5 | bitmap mode (BMM) on
= $D011 bit 6 | extended background mode off (must be 0)
= $D016 bit 4 | multicolor (MCM) on
= $D018 | bits 4–7: screen RAM location, bit 3: bitmap at $0000 or $2000 (inside the bank)
= $DD00 bits 0–1 | VIC bank (inverted: %11 = bank 0)

In BASIC you can turn the mode on with ` + "`POKE 53265,PEEK(53265) OR 32`" + ` and ` + "`POKE 53270,PEEK(53270) OR 16`" + `.

## Simons' BASIC

The command ` + "`MULTI c1,c2,c3`" + ` sets the colors for ` + "`%01`" + `, ` + "`%10`" + ` and ` + "`%11`" + `; the background color ` + "`%00`" + ` is set with ` + "`COLOUR`" + `.

## Sprites

When sprites are combined with the picture, bit pairs ` + "`%00`" + ` and ` + "`%01`" + ` count as background and ` + "`%10`" + `/` + "`%11`" + ` as foreground (for sprite priority and collisions).
`},
	{id: "tips", title: "Tips for C64 graphics", body: `
# Tips for C64 graphics

- **Choose the background first.** The background color is the only color that is free in every cell. Use the most frequent color of your subject.
- **Think in cells.** Turn on the 8×8 grid. Plan color changes on cell borders.
- **Few colors, many dither patterns.** The density patterns (1/16 … 15/16) create soft transitions without adding new colors to the cell.
- **Use luminance pairs.** The C64 has color pairs of similar brightness: blue/brown, red/dark grey, purple/orange, grey/light blue, green/light red, cyan/light grey and yellow/light green. They are great for smooth dithered transitions.
- **Trace a template.** Load a photo as a template, set 40–60 % opacity and trace the outlines. Or let the converter do it and touch up the result.
- **Convert with care.** In the converter, raise the contrast a little and try Bayer 4×4 or Floyd–Steinberg at 40–70 % strength.
- **Use the spare page.** With ` + "`J`" + ` you have a second picture for experiments or as a parts bin for brushes.
- **Play with the conflict setting.** "Block" for clean work, "Use nearest color" for quick shading with the airbrush.
- **Check on the real thing.** Export a PRG and start it in an emulator. Depending on the palette, colors can look different.
`},
	{id: "about", title: "About C64 DPainter", body: `
# About C64 DPainter

**Version 0.8 beta** – a paint program for C64 multicolor bitmaps for Windows, macOS and Linux.

Created by **IT-Dummköpfen**. Free for private use.

Inspired by **Deluxe Paint** (Electronic Arts, 1985) and **PyDPainter**. Controls and keyboard shortcuts follow these programs.

## Sources and thanks

- Technical basics: C64-Wiki (multicolor bitmap mode, MULTI) and retro-programming.de (VIC-II graphics modes).
- Palettes: collection from c64gfx.com/palettes. Two palettes were missing one color on the source page; it was taken from Pepto and is marked with * in the palette dialog.
- Font: DejaVu Sans (free license, Bitstream Vera).

## Settings

Settings (theme, palette, grid, conflict mode …) are stored in ` + "`{SETTINGS}`" + `.

-> start | Back to "First steps"
`},
}
