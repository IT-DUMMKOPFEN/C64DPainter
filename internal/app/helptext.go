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

File, edit, selection, layer and view commands, options (palette, color conflicts, theme, interface size) and this help. The sun/moon icon at the top right switches between the light and the dark theme.

**Options → Interface size** makes the whole interface 100 %, 200 % or 300 % large; the window is resized to match. Sizes that do not fit on your screen are greyed out.

## Toolbox (right)

- At the top: twelve **brush presets** (round, square and diamond, different sizes).
- Below: the **tools**. Split buttons (rectangle, circle, ellipse): click the upper left half for an outline, the lower right half for a filled shape.
- **Right-clicking** a tool button opens its help or settings (grid, template, zoom, gradient).
- The **color 1 / color 2** display. Clicking it swaps both colors.
- The **palette** with the 16 C64 colors. "B" marks the background color (Ctrl+click a color to make it the background).

## Side panel (left)

- **Selection** (only with the select tool): rotate, scale, shear.
- **Symmetry** (while it is on): axes and mirror mode.
- **Dither brush** / **Smear** / **Shade** / **Gradient fill** (only with these tools): their settings.
- **Click a section title** (Brush, Patterns, Colors, Layers, Cell under the mouse, Navigator …) to fold the section in and out. That saves space and brings calm; at every start all sections are open.
- **Brush**: shape (round, square, diamond, custom), size 1–16, preview, airbrush settings.
- **Patterns**: fill patterns for all tools (hidden for the dither brush).
- **Colors**: background ($D021), border ($D020) and the behavior for color conflicts.
- **Layers**: the template (layer 1) and the paint layers 2–4 (click = paint on it, eye = show/hide); the switch next to the title turns layers 3 and 4 on or off.
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

## [gradient] Gradient fill (Shift+F)

Fills the connected area of the same color with a **color gradient** (on the active layer). Clicking the tool button opens the **gradient window** with five tabs:

= 3 colors | 27 ramps light → dark (7 basic, 20 extended, e.g. Neon yellow, Mint, Sunset, Night sky)
= 4 colors | 14 ramps with four different colors, light → dark
= 5 colors | 13 ramps with five different colors, light → dark
= dark → light → dark | the 3-color ramps as 5 bands, dark at both ends
= light → dark → light | the 3-color ramps as 5 bands, light at both ends

> A C64 cell can only hold **3 colors plus the background color**. Gradients with 4 or 5 different colors work best with wide color bands: then only two colors meet in a cell. Narrow gradients or dithered transitions can put more colors into one cell – then the conflict mode decides. The background color does not count, so ramps that end in the background color (often black) are easier.

The last ramp of the 3-, 4- and 5-color tabs is **Custom** – your own ramp: select it, click one of its color fields and then a palette color (the next field is selected automatically). Your custom ramps are saved and are there again after a restart. The custom 3-color ramp also appears in the two five-band tabs, and the Shade brush can use all custom ramps.

In the window you also choose:

- **Shape**: Linear, Circle or Diamond.
- **Direction** for a single click with the linear shape (8 directions).
- **Reverse colors** and **Dithered transitions** (25 / 50 / 75 % patterns between the color bands instead of hard edges).

Close the window with **OK**; the side panel shows the chosen gradient and a button to open the window again.

- **Single click** into an area: the gradient starts at the click point. Linear runs in the chosen direction up to the far edge of the area; circle and diamond spread from the click point up to the farthest point of the area.
- **Click and drag**: the drag sets the direction and the width of the gradient. A line shows it while you drag. Beyond the end the last color continues.

A gradient uses at most three colors, so in empty cells it never runs into color conflicts. In cells that already hold other colors the conflict mode decides.

## [spray] Airbrush (A)

Sprays brush dabs into a round area while the mouse button is held. Set radius and density on the left in the brush panel while the airbrush is active.

## [dither] Dither brush (Shift+D) – experimental

Paint over the borders between flat color areas: they are blended with the chosen dither style (Artist, Scanlines, Checker or Stripes) in clean steps – outlines and object silhouettes stay sharp. With the style **Color** the brush darkens the colors you paint over instead. The style and its settings are on the left in the side panel; the pixels that would change are shown under the brush. Right-click the button for **Auto dither** of the whole picture. See "Dithering".

## [smear] Smear (Shift+S)

Smears the colors along the stroke, like the smudge tool of other paint programs: the brush picks up the colors under it and drags them along. **Strength** (side panel) sets how far: high values pull long streaks, low values blend softly. The brush shape and size come from the Brush panel. In full cells the nearest cell color is used, so the multicolor rules always hold.

## [eraser] Eraser (W)

Makes pixels of the **active layer** transparent again, so the layers below (or the background) show through. Brush shape and size come from the Brush panel; ` + "`Ctrl`" + ` gives a straight line. Without layers below, erasing gives the background color.

## [shade] Shade (Shift+W)

Lightens or darkens the colors under the brush by **one step**: left mouse button = lighter, right mouse button = darker. Each pixel changes only once per stroke, so you can paint over an area without it turning white or black – for the next step start a new stroke. In the side panel you choose the steps: **All colors by brightness** (the 16 C64 colors from black to white) or one of the **color ramps** of the gradient fill with 3, 4 or 5 colors (e.g. blue: blue → light blue → cyan) – ramps with more colors give finer steps; colors that are not in the chosen ramp stay as they are. In full cells the conflict mode decides.

## [cellswap] Cell color swap brush

Opens the swap window in brush mode: choose the color to replace and the new color, then paint over cells – in every 4 × 8 cell you touch, the first color becomes the second. A color can also become the **background** color (its pixels turn into ` + "`%00`" + `). Choose any tool or press ` + "`Esc`" + ` to stop. See "Swap colors".

## [rect|rectf] Rectangle (R / Shift+R)

Drag from corner to corner. Outlines use the brush, filled shapes use the pattern. ` + "`Shift`" + ` = square (on screen).

## [circle|circlef] Circle (C / Shift+C)

Drag from the center. The circle is round on screen – so in pixels it is twice as high as wide.

## [ellipse|ellipsef] Ellipse (E / Shift+E)

Drag from the center. Width and height follow the mouse. ` + "`Shift`" + ` = circle.

## [square|squaref] Square (N / Shift+N)

Drag from corner to corner. The square is square on screen (in pixels it is twice as high as wide).

## [triangle|trianglef] Triangle (I / Shift+I)

## [hexagon|hexagonf] Hexagon (O / Shift+O)

## [star|starf] Star (L / Shift+L)

For all three: drag from the center: the distance sets the size, the direction the rotation – the first corner points to the mouse. Hold ` + "`Shift`" + ` to keep the shape upright. Like the circle, the shapes are regular on screen. Outlines use the brush, filled shapes the pattern.

## [symmetry] Symmetry (Shift+X)

Everything you paint is also painted mirrored: **left ↔ right**, **top ↔ bottom** or **both** (four-fold). The dashed lines show the axes; by default they lie in the middle of the picture, in the side panel you can move them (**Axis X / Axis Y**, **Axis to the center**). Symmetry works with dots, freehand, lines, curves, the airbrush, the dither and shade brushes, the eraser, all shapes and both fill tools – not with the selection and the smear brush. Switch it on and off with ` + "`Shift+X`" + `, the toolbox button or **View → Symmetry**.

## [select] Select (B)

Drag a frame, then move, copy, flip, rotate, scale and shear. See "Selection & transform".

## [picker] Color picker (,)

A click picks the color (left = color 1, right = color 2) and returns to the previous tool. ` + "`Alt`" + `+click does the same with any tool.

## [hand] Pan

Moves the view. Faster: hold ` + "`Space`" + ` or drag with the middle mouse button.

-> brushes | Brushes
-> patterns | Patterns
-> dither | Dithering
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
- As in Deluxe Paint, **color 2** (the color you choose with the **right** mouse button in the palette) becomes **transparent** in the brush. Choose the color around your motif as color 2 before you click "Use as brush".
- With **"Background color transparent"** (Selection menu), background-colored pixels are skipped when stamping.

## Brushes menu

- **My brushes …** – your saved brushes (see below).
- **Load brush …** / **Save brush …** – brushes are PNG files with a transparent background. Every multicolor pixel is saved two pixels wide, as on screen, so the files can also be edited in other paint programs. Loading accepts both double-wide and single pixels; other colors become the nearest C64 color. The dialogs start in the folder ` + "`brushes`" + ` in the working folder. A one-color brush is saved in color 1.
- **Selection as brush**, **Standard brush** (` + "`Shift+B`" + `), flip, rotate and resize the brush, and **Brush keeps its own colors**.

## [text] Text

The text button in the toolbox (or **Brushes → Text …**) writes text in multicolor pixels and turns it into the brush, so you place it like a stamp – and can flip and resize it with ` + "`X`" + `, ` + "`Y`" + `, ` + "`H`" + `.

= C64 wide (8×8) | the C64 characters, every pixel a wide multicolor pixel: 16 × 8 screen pixels per letter – the classic look of C64 titles
= C64 tall (8×16) | the same, twice as high
= Narrow (4×8) | an own font of 3 × 7 pixels that fits one cell: about 40 letters per line (upper case, digits and signs)

- **Letters**: upper case or lower case character set.
- **Effect**: none, **shadow** (one pixel to the lower right) or **outline** – in a second color.
- **Spacing** between the letters, **Shift+Enter** for a new line, **Enter** uses the text as brush.

Text and shadow/outline use only two colors, so the text always fits the multicolor rules (in full cells the conflict mode decides).

## [stamp] My brushes

The button in the toolbox (or **Brushes → My brushes …**) opens a window with all brushes you saved in the folder ` + "`brushes`" + ` of the working folder – your own little stamp collection. Click a brush and **Use as brush** (or double-click it): it becomes the brush and the tool switches to **Dots**, so every click stamps it once. **Reload** reads the folder again, e.g. after you copied PNG files into it.
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

> The **background color** ($D021) is marked with "B". It can never be the color to **replace**, but it can be the **new** color: then the pixels become background pixels (` + "`%00`" + `) and follow later changes of $D021. "Exchange both colors" is not possible with the background. To change the background itself, use **Colors → Background** in the side panel.

## Swap color in picture

**Edit → Swap color in picture …** replaces one color by another in the whole picture.

- Choose the color to replace in the upper row and the new color in the lower row.
- The dialog shows how many pixels and cells use the color.
- **Exchange both colors (A ↔ B)**: A becomes B and at the same time B becomes A.
- If a cell already contains the new color, the pixels are moved to it and the old color slot becomes free.
- One step: ` + "`Ctrl+Z`" + ` undoes it.

## Swap color in cells (brush)

**Edit → Swap color in cells (brush) …** or the **cell swap button** in the toolbox asks for the two colors and then turns the mouse into a **cell brush**:

- Paint over the picture: in every **cell** (4 × 8 pixels) the brush touches, the first color becomes the second one. The whole cell changes, not only the pixels under the brush.
- The cells under the brush are highlighted before you click.
- Each cell is changed only once per stroke, so painting back and forth does no harm.
- The **right** mouse button swaps the other way round (second color → first color).
- The brush size from the brush panel decides how many cells you reach at once.
- The side panel shows both colors, with **Change colors …** and **Stop**.
- The brush ends as soon as you choose **any other tool** in the toolbox (or with its key), or press ` + "`Esc`" + `.
`},
	{id: "dither", title: "Dithering", body: `
# Auto dither and Dither brush

> Both functions are **experimental**. Check the result in the preview (or undo with ` + "`U`" + `) – they do not replace careful pixelling by hand.

Paint your picture with **flat colors** first – then let the program blend the borders between the colors, the way C64 artists do it by hand. Flat areas stay as they are; only the places where differently colored areas meet are mixed.

- **Edit → Auto dither …** (` + "`Shift+A`" + `) dithers the whole picture or the selection, with a live preview.
- The **Dither brush** (` + "`Shift+D`" + `, toolbox) does the same only where you paint. While you hover over the picture, the pixels the brush would change are shown under it.

## Where the styles come from

The dither styles were reconstructed from **16 hand-pixelled C64 multicolor pictures**. Every picture was turned back into its 160 × 200 multicolor pixels; then all 8 × 8 areas that mix exactly two colors (6381 of them) were collected and their patterns counted. For every group of pictures the fill order (dither matrix) was fitted to the patterns that occur most often.

What the analysis showed:

- Artists mix colors of **similar brightness**: 54 % of all mixed pairs are only one brightness step apart, 29 % two steps, almost never more. So C64 dithers run along **color ramps** like black → blue → purple → light red → yellow → white.
- Most dithers live on **every second row** ("interlaced") – the wide multicolor pixels look finer that way.
- Single pictures prefer clear families of patterns: scanlines, horizontal strokes, checkerboards, vertical columns or scattered dots.

## How it works

The program follows the rules pixel artists use for clean dithering:

- **Two colors per transition.** Where two areas meet, only these two colors are mixed – never three.
- **Few, clean steps.** The transition is built from bands of one pattern each: solid · 25 % · 50 % | 50 % · 25 % · solid. The pattern runs on across the border without a seam.
- **Only colors of similar brightness** that C64 artists actually mix are blended. Other borders – the silhouettes of objects – stay sharp.
- **Outlines and small details stay crisp.** Thin lines and small areas are never dithered, and nothing is dithered into them.
- **Narrow areas keep a solid core** – the transition never eats a whole area.

## The styles

= Artist | the most common technique of all pictures: 25 % = checker dots on every second row, 50 % = every second row
= Scanlines | alternate rows with spread dots – fine, calm gradients (portraits, shiny objects)
= Checker | the classic: 25 % = spread dots, 50 % = checkerboard – soft and even (skin, clouds)
= Stripes | lines that widen into stripes, always parallel to the border – graphic, poster-like
= Color | shading by darkening (see below)

## Color – shading by darkening

Instead of mixing two neighboring areas, **Color** darkens a color by dithering it with a **darker color**. It first looks for a darker color that is already in the 4 × 8 cell (or the background color); if there is none, it takes a darker color from the artists' color ramps that fits into a free slot of the cell (cell colors "Add").

- **Auto dither** darkens every area in clean steps towards its darker neighbors – like a soft shadow along edges and outlines.
- The **Dither brush** darkens everything you paint over. **Darkness** sets the share of the darker color (25 / 50 / 75 %); paint again to darken further.

## Settings

- **Width** – how far the transition reaches into each area, in pixel rows (a multicolor pixel counts as two rows wide).
- **Steps** – dither stages per side: 1 = only 50 %, 2 = 25 % and 50 % (recommended), 3 = 12 %, 25 % and 50 %. More steps need more width.
- **Keep edges** – higher = only colors of very similar brightness are blended; all other borders stay hard.
- **Details** – lines and areas thinner than this (outlines, eyes, highlights) are never dithered.
- **Artist pairs** – only color pairs that C64 artists mix are blended (measured in the 16 pictures). Higher = stricter. 0 = every pair of similar brightness.
- **In-between colors** – for strong contrasts: joins two colors that are too far apart with a color from the artists' ramps (for example black → dark grey → grey). Good for big gradients like skies, but it also puts color bands around objects, so it is off by default.

## Cell colors

Each 4 × 8 cell can only show four colors, so the program has to decide which colors a cell may use:

= Keep | every cell keeps exactly its colors. Where a cell lacks the other color (e.g. a border exactly on a cell border), nothing can be blended.
= Add | the existing colors always stay; in-between colors are only added in free slots. Never a color conflict. Recommended.
= Re-choose | (Auto dither only) the three colors of every cell are chosen anew – the smoothest blend, but small details may change color.

> The background color ($D021) is never changed.

## Color matching

Where a color has to be replaced because a cell is full, **Color matching** decides which of the allowed colors comes closest: RGB, YUV, XYZ, Lab, OkLab or only the brightness (see "Import & convert").

## Tips

- Paint big shapes with clean flat colors and outlines, then use **Auto dither** once, and refine single spots with the **Dither brush**.
- The **Changes** view of the dialog shows exactly which pixels change.
- Painting over the same place again with the Dither brush blends further – like a smudge tool.
- **Use settings for the brush** in the dialog hands style and settings over to the Dither brush.
- Use ` + "`Ctrl`" + ` with the Dither brush for a straight stroke.

-> import | Import & convert (dithering of photos)
-> tips | Tips for C64 graphics
`},
	{id: "select", title: "Selection & transform", body: `
# Selection and transform

## Selecting

Choose the select tool (` + "`B`" + `) and drag a frame. ` + "`Ctrl+A`" + ` selects the whole picture. Right-click or ` + "`Esc`" + ` drops the selection.

## Moving and copying

- Dragging inside the selection moves a **copy** of the content – the original stays in the picture.
- Hold ` + "`Shift`" + ` when you start dragging to **move** the content instead: the old place becomes transparent (background ` + "`%00`" + `).
- Dragging an already floating selection with ` + "`Ctrl`" + ` leaves a copy behind (stamping).
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
- ` + "`0`" + `, ` + "`Ctrl+0`" + ` or ` + "`Home`" + `: fit to window. ` + "`Ctrl+1`" + `: 100 % (320 × 200 screen pixels).
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
- Presets: 8×8 (cell), 16×16, 32×32, 2×1 (one pixel), 4×4 and **Sprite 24×21** (one multicolor sprite of 12 × 21 pixels).

**View → Sprite grid (24×21)** switches directly between the sprite grid and the cell grid. Draw sprites in it and take them into the **sprite editor** with Import (as a raster).

{MACNOTE}

## Mirrored view (Shift+M)

` + "`Shift+M`" + ` shows the picture mirrored left ↔ right. Looking at a picture mirrored for a moment shows mistakes in proportions you no longer notice. Only the **view** changes – the picture is not changed, and you can keep painting. "MIRRORED VIEW" is shown in the corner; ` + "`Shift+M`" + ` switches back.

## CRT preview (Shift+V)

A separate window shows the picture with its border like on a TV. It does not block the program: keep painting, the preview follows every change. Drag it by its title bar.

- **100 %** – 384 × 272 screen pixels, the real size of a C64 picture with border; **200 %** twice as large. When the window does not fit, the middle part is shown.
- **CRT** on: scanlines, the slightly soft picture and the PAL color bleed – good for judging dithering. Off: sharp pixels.

## More display aids

- **Pixel grid** (` + "`M`" + `): thin lines around every pixel from zoom 4×.
- **Highlight cell under the mouse**: dashed frame around the current cell.
- **Show border**: shows the C64 border in the border color ($D020), like on a TV.
`},
	{id: "layers", title: "Layers & template", body: `
# Layers

A picture has **four layers** (side panel **Layers** or the **Layer** menu). They are listed from 1 at the top to 4 at the bottom, in the order they lie on top of each other:

= Layer 1 | the **template**: any image for tracing, its colors do not matter
= Layer 2 | paint layer **at the bottom** – loaded and imported pictures go here
= Layer 3 | paint layer above layer 2
= Layer 4 | paint layer **on top**

## Paint layers 2–4

- At the start only **layer 2** is active; layers 3 and 4 are **switched off**. The switch **Layers 3 + 4** next to the title of the Layers panel (or **Layer → Use layers 3 + 4**) turns them on and off. While they are on, every layer can be shown or hidden on its own.
- **Click a layer** (or press ` + "`2`" + `, ` + "`3`" + `, ` + "`4`" + `) to paint on it; ` + "`3`" + ` and ` + "`4`" + ` switch layers 3 + 4 on. The active layer is highlighted and shown in the status bar. All tools paint into the active layer; the selection, Auto dither, the Dither brush, Smear and Gradient fill work on it too.
- Pixels you have not painted on a layer are **transparent**: the layers behind show through. Where no layer has a pixel, you see the background color.
- The **Eraser** (` + "`W`" + `) makes pixels of the active layer transparent again. **Clear layer** empties the whole active layer.
- The **eye icon** shows or hides a layer. Switched-off layers 3 and 4 are hidden, but their colors still count for the cell rules.

> **The multicolor rules apply to layers 2–4 together.** In every 4 × 8 cell, all three layers together may use the background color plus at most three colors – **also when a layer is hidden**. So every combination of visible layers is always a valid C64 picture. The cell info marks colors that only come from hidden layers.

What you see (the visible layers) is what you save as KLA, PNG or PRG. To keep the layers, save as a **project (.c64dp)**.

Pixels painted in the background color follow the background: when you change $D021, they change too (like %00 pixels on a real C64).

## Layer 1: template

Load a PNG, BMP, JPG or GIF image as a template (**File → Load template …**, the button in the side panel or the **Layer** menu). The template does **not** become part of the C64 graphics – it is only for tracing.

- **Eye icon** or key ` + "`T`" + `: show/hide.
- **Opacity** 0–100 %.
- **Above picture**: the template lies semi-transparent over your drawing.
- **Below picture**: the template only shows through pixels with background bit pair ` + "`%00`" + `. So you see what has not been painted yet.
- **Stretch / Fit / Fill** decides how the template is mapped to 320 × 200.
- **Convert template to C64 picture …** opens the converter with all its settings. Like opening or importing a picture, this replaces all layers (undo brings them back).

## Spare page

Like Deluxe Paint there is a second page. ` + "`J`" + ` switches between both pages, **Edit → Copy picture to other page** keeps a backup. Each page has its own layers and undo history.
`},
	{id: "formats", title: "Saving & file formats", body: `
# Saving and file formats

## C64 DPainter project (.c64dp) – keeps the layers

Saves **all layers** (also hidden ones), the active layer, the background and border color, the palette and the **template** with its settings. Use it for work in progress; export KLA, PNG or PRG for the C64 – these contain the visible layers merged into one picture.

## Koala Painter (.kla / .koa) – standard format

The classic C64 format for multicolor pictures, 10 003 bytes:

= 2 bytes | load address $6000
= 8000 bytes | bitmap (bit pairs, in the VIC-II cell layout)
= 1000 bytes | screen RAM (colors %01 and %10)
= 1000 bytes | color RAM (color %11)
= 1 byte | background color

` + "`Ctrl+S`" + ` saves in the format of the current file; a new picture is saved as a project (.c64dp) – choose .kla, .png or .prg in the save dialog. Files without a load address (10 001 bytes) are also read.

## Working folder and quick save (F2)

At the start and with **File → New** the program asks for the **working folder** – the folder where your pictures and data are saved. The save and open dialogs start there. **File → Working folder …** changes it at any time.

` + "`F2`" + ` saves the picture at once, without a dialog, as a numbered PNG (320 × 200) into the working folder: ` + "`picture_0001.png`" + `, ` + "`picture_0002.png`" + ` … The numbers continue after the highest number already in the folder. When the picture already has a file name, that name is used instead of "picture". The status bar shows the file name.

## PNG

- **320 × 200**: as on screen (every pixel 2 pixels wide). Default for "Save as" with .png.
- **160 × 200**: one pixel per multicolor pixel.
- **384 × 272 with border**: like a PAL TV with border color.

PNGs are saved with the currently selected palette and 16 indexed colors.

## C64 program (.prg)

A **runnable program** with a built-in viewer. See "Running the C64 program".

## Loading

**C64 DPainter projects (.c64dp)**, **PNG**, **BMP** (1/4/8/16/24/32 bit, also RLE compressed), **KLA/KOA**, **Advanced Art Studio (.ocp)** and **Amica Paint (.ami)**. KLA, OCP, AMI, PNG and BMP go into layer 2. OCP and AMI files are only read: save the picture as project, KLA, PNG or PRG.

## Recent files

The **File** menu lists the last 8 files you opened or saved. A click opens the file again; files that no longer exist are removed from the list. **Clear list** empties it.

## Auto backup

**Options → Auto backup** saves your work every 3 minutes into the working folder as ` + "`Backup.c64dp`" + ` (with all layers and the template) – and ` + "`Backup (page 2).c64dp`" + ` when the spare page is used. A backup is only written when the picture has changed; the file is overwritten each time. Auto backup is **off** at every start of the program; switch it on when you need it. Templates can also be JPG and GIF.

-> prg | Running the C64 program
-> import | Import & convert
`},
	{id: "import", title: "Import & convert", body: `
# Import & convert

**File → Import image …** (` + "`Ctrl+I`" + `) turns any PNG, BMP, JPG or GIF image into a C64 multicolor picture. A live preview shows the result while you change the settings: **Multicolor**, **Original** or **Split** (left original, right result). Hold the mouse button on the preview to see the original.

## Use as

**C64 picture** replaces the picture (layer 2), **Template** puts the image into layer 1 for tracing, **Both** does both.

## Size and pixel pairs

- **Stretch / Fit / Fill** map the image to 320 × 200. Images of exactly 320 × 200 or 160 × 200 pixels are taken 1:1.
- **Average / Left pixel / Right pixel** – two screen pixels become one wide multicolor pixel.

## Adjustments

Brightness, contrast, saturation, hue and gamma – often a little more contrast and saturation helps a lot with the 16 C64 colors.

## Dithering

13 methods: ordered patterns (Bayer 2 × 2, 4 × 4, 8 × 8, clustered dot, line pattern, noise) and error diffusion (Floyd–Steinberg, Atkinson, Jarvis-Judice-Ninke, Stucki, Burkes, Sierra, Sierra Lite). **Strength** sets how much is dithered.

## Color matching

How "the most similar C64 color" is measured. Each method suits other images – just try them in the preview:

= RGB | weighted RGB distance – quick and good for most images (default)
= YUV | brightness and color difference as on a TV – keeps brightness steps
= XYZ | CIE XYZ – physical light, good for photos
= Lab | CIE Lab – close to how the eye sees differences
= OkLab | modern, even perception of hue and brightness
= Brightness | only the brightness counts – the colors follow the light and dark parts, a colorful, poster-like look

## Cell colors

= Best fit | every 4 × 8 cell gets the three colors that fit it best (default)
= Most frequent | every cell gets its three most frequent colors – often calmer, as in Retropixels
= Same in all cells | the whole picture uses the background color plus the same three colors – good for game graphics and charsets

## Background color ($D021)

**A** chooses it automatically (the program tries the most frequent colors), or pick one of the 16 colors.

> The result always follows the multicolor rules. The ideas for color matching and cell colors come from **Retropixels** by Michel de Bree.

-> dither | Dithering (Auto dither and Dither brush)
-> formats | Saving & file formats
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
	{id: "petscii", group: "C64 tools", title: "PETSCII editor", body: `
# PETSCII editor

**Tools → PETSCII editor …** opens an editor for **PETSCII** pictures: the normal text mode of the C64 with 40 × 25 characters from the character ROM. Every character has its own color, the background ($D021) is shared. PETSCII is independent of the multicolor rules. The editor fills the whole window; **Close PETSCII editor** brings back the multicolor editor. Your PETSCII picture stays in the editor until you quit the program.

## Convert a picture

**Convert …** turns the multicolor picture – or any image (**Load an image …**) – into PETSCII. For every 8 × 8 cell the character and color that match it best are chosen.

- **Character set**: upper case (with graphics characters) or lower case.
- **Characters**: all, no letters (only graphic characters), or only blocks and shades. **Also inverted characters** doubles the choice.
- **Color matching**: RGB, YUV, XYZ, Lab, OkLab or brightness (see "Import & convert").
- **Brightness / contrast** and the **background color** (A = automatic).

**New** starts with an empty screen.

## Tools

= 1  Draw | sets the character in the chosen color – right mouse button: erase
= 2  Colorize | only changes the color of the characters
= 3  Character only | only changes the characters, their colors stay
= 4  Eraser | sets spaces
= 5  Brush | drag a rectangle to copy it, then click to stamp it; right-click or ` + "`Esc`" + ` drops it
= 6  Text | click a cell and type; arrows move, Enter jumps to the next line

- ` + "`Alt`" + `+click picks the character and color under the mouse.
- Hold ` + "`Shift`" + ` while drawing for a straight horizontal or vertical line.
- ` + "`Alt`" + `+arrow keys shift the whole screen by one character.

## Characters and colors

- On the right you see the 256 characters. The dropdown changes their **arrangement**: **Standard PETSCII** (the order of the character ROM, always the first), sorted by shape (as in Petmate), letters & digits, lines & frames, blocks & shades, curves & symbols.
- ` + "`W A S D`" + ` move in the characters, ` + "`F`" + ` inverts the character, ` + "`Q`" + ` / ` + "`E`" + ` choose the previous / next color.
- On the left: grid (` + "`G`" + `), upper or lower case set, background and border color.

## Keys

= 1 – 6 | tools
= Ctrl+Z / Ctrl+Y | undo / redo (also U / Shift+U)
= Ctrl+S / Ctrl+O | save / open a Petmate file
= G | grid
= F | invert the character
= W A S D | move in the characters
= Q / E | previous / next color
= Alt+arrows | shift the screen
= Esc | drop the brush or selection, end text input

## Files and export

- **Save / Open**: Petmate workspace files (` + "`.petmate`" + `) – you can go on working in Petmate.
- **Export PRG**: a runnable C64 program. It shows the picture until you press a key, then resets the C64.
- **Export ASM**: source code for the **ACME** cross assembler with the screen codes and colors as ` + "`!byte`" + ` lines and the display code. Assemble it with ` + "`acme --cpu 6510 --format cbm --outfile picture.prg picture.asm`" + `.
- **Export PNG**: 384 × 272 pixels with border.

> Ideas from **Petmate** (nurpax, MIT license – also the "sorted by shape" order), **petsciirender** (gnacu), **PETSCII editor** (Marq, ventti/petscii, WTFPL) and **Image-to-PETSCII** (amb; only the idea, no code). The characters are the original C64 character ROM © Commodore.
`},
	{id: "sprites", title: "Sprite editor", body: `
# Sprite editor

**Tools → Sprite editor …** opens an editor for up to **64 C64 sprites**. It fills the whole window like the PETSCII editor; **Close sprite editor** brings back the multicolor editor. The sprites stay in the editor until you quit the program.

## Sprites on the C64

A sprite is 24 × 21 pixels (63 bytes). Every sprite is either

- **Hires**: 24 × 21 pixels in the sprite color, or
- **Multicolor**: 12 × 21 double wide pixels in three colors: the **sprite color** (own for every sprite, $D027+n) and **multicolor 1 and 2** ($D025, $D026), which all sprites share.

Transparent pixels show the background ($D021). Switching between hires and multicolor keeps the bits, exactly as on the C64 – the sprite looks different afterwards.

## Drawing

- Tools: **Pencil** (` + "`D`" + `), **Line** (` + "`L`" + `), **Rectangle** (` + "`R`" + `), **Filled box** (` + "`B`" + `), **Fill** (` + "`F`" + `).
- The **left** button paints with the chosen color, the **right** button paints transparent. **Alt+click** picks the color under the mouse, **Shift** draws a straight line with the pencil.
- Choose the color with ` + "`0`" + ` – ` + "`3`" + ` or on the right: 0 transparent, 1 sprite color, 2 multicolor 1, 3 multicolor 2. Click a color of the palette to change the chosen color (0 changes the background).
- **Mirror left/right** (` + "`M`" + `) paints both halves at once.
- **Overlay sprite** (` + "`O`" + `) shows a second sprite faded behind the one you draw – for example a hires outline over a multicolor sprite. The previews show both as on the C64 (the lower sprite number is in front).
- **Edit**: flip, shift (also ` + "`Alt+arrows`" + `), invert, clear, copy and paste.
- **Expand X / Y** doubles width or height (only in the preview and the export).
- **Use as brush** hands a multicolor sprite to the multicolor editor as brush.

## Sprite list and animation

On the right are all sprites. **Add**, **Duplicate**, **Delete** and **Move** change the list; the arrow keys, ` + "`Page Up`" + ` / ` + "`Page Down`" + ` or ` + "`,`" + ` / ` + "`.`" + ` choose the sprite.

Below the drawing area: a preview at 1× and 2×, and the **animation**: from sprite, to sprite, speed in frames per second and **Ping-pong** (forward and back). **Play** (` + "`Space`" + `) starts and stops it.

## Import from a picture

**Import …** takes sprites from the **multicolor picture** or from an **image of 320 × 200 pixels**:

- Click or drag on the picture to place the frame (it snaps to the sprite grid; the arrow keys move it pixel by pixel).
- **Columns / Rows** take a whole raster of sprites at once, **Gap** is the space between them.
- **Hires** or **Multicolor**, the **background** (transparent, A = the most frequent color), and the shared multicolors: found automatically (the two colors used in the most sprites) or kept.
- **Target**: replace from the current sprite on, or append.

Tip: in the multicolor editor, **View → Sprite grid (24×21)** shows a grid of exactly one multicolor sprite (12 × 21 pixels). Draw your sprites in the grid and import them here as a raster.

## Files and export

- **Save / Open**: sprite project (` + "`.c64sprites`" + `) with all sprites, colors and animation settings.
- **Export ASM**: source code for the **ACME** assembler: constants for the background, multicolor 1 and 2 and the animation, every sprite as 21 lines of ` + "`!byte`" + ` (hires drawn with ` + "`.`" + ` and ` + "`#`" + `), the 64th byte with multicolor flag and color (as in SpritePad), and tables of colors, multicolor and expand flags. ` + "`acme --cpu 6510 sprites.asm`" + ` writes ` + "`sprites.prg`" + `. The data starts at $2000; change ` + "`*`" + ` to place it elsewhere (a multiple of 64).
- **Export BIN**: 64 bytes per sprite, raw. **Export PNG**: all sprites as a sheet.

## Keys

= D L R B F | pencil, line, rectangle, filled box, fill
= 0 – 3 | color: transparent, sprite color, multicolor 1, multicolor 2
= C | hires ↔ multicolor
= X / Y | flip horizontally / vertically
= I | invert
= M / O / G | mirror, overlay, grid
= Alt+arrows | shift the sprite
= ← → / Page Up / Page Down | previous / next sprite
= Space | play / stop the animation
= Ctrl+C / Ctrl+V | copy / paste the sprite
= Ctrl+Z / Ctrl+Y | undo / redo
= Ctrl+S / Ctrl+O | save / open

> Ideas from **Spritemate** (Ingo Hinterding, MIT), **MagSpriteEd** (Magnar Harestad, MIT) and **Sprite Studio 64** (Tobiasz Stamborski, MIT).
`},
	{id: "fonts", title: "Font editor", body: `
# Font editor

**Tools → Font editor …** opens an editor for **C64 character sets**: 256 characters of 8 × 8 hires pixels (2048 bytes). It starts with the C64 font; **Close font editor** brings back the multicolor editor.

## Drawing

- Tools: **Pencil** (` + "`D`" + `), **Line** (` + "`L`" + `), **Rectangle** (` + "`R`" + `), **Fill** (` + "`F`" + `).
- The **left** button sets pixels – starting on a set pixel it clears them. The **right** button clears. **Shift** draws a straight line with the pencil.
- Choose the character on the right or with the **arrow keys**.
- **Tile size** 1×1, 2×1, 1×2 or 2×2 edits several characters as one big character. **Neighbors**: the next character to the right (+1) and below (+16, as in the grid). **Quarters**: right +64, below +128 – the classic layout of 2×2 fonts.
- **Edit**: flip, rotate (square tiles), invert, shift (` + "`Alt+arrows`" + `), copy and paste, clear and **Original** (back to the character of the C64 ROM).
- **Compare with ROM** (` + "`B`" + `) shows new pixels green and removed ones red.
- **Inverse half** fills the characters 128–255 with the inverted characters 0–127, as in the C64 ROM.
- **Reference ROM**: upper or lower case – used by Original, Compare and the test text.
- **Test text**: click the field and type; the text is shown in your font (as tiles when a tile size is chosen).
- The character and background colors are only for the display.

## New and import

- **New …**: the C64 font (upper or lower case) or an empty set.
- **Import …** reads characters from the **multicolor picture**, an **image of 320 × 200 pixels**, or the **C64 font** (upper or lower case). The picture is cut into 8 × 8 cells, read row by row: choose the **first cell** (or click it), the **number** and the **first character** to fill. Every pixel that is not the background color becomes a set pixel (hires; multicolor pixels count as two).

## Files and export

- **Save / Open**: font project (` + "`.c64font`" + `). **Open** also reads raw character sets (` + "`.bin`" + `, ` + "`.64c`" + `, 2048 or 4096 bytes, with or without load address).
- **Export ASM**: source code for the **ACME** assembler: all 256 characters as ` + "`!byte`" + ` lines drawn with ` + "`.`" + ` and ` + "`#`" + `, with comments and the value for $D018. ` + "`acme --cpu 6510 font.asm`" + ` writes ` + "`font.prg`" + ` (data at $3000, a multiple of $800).
- **Export BIN**: the raw 2048 bytes. **Export PNG**: all characters as 128 × 128 pixels.

## Keys

= D L R F | pencil, line, rectangle, fill
= arrows | choose the character
= Alt+arrows | shift the tile
= X / Y / T | flip / rotate
= I | invert
= B | compare with the ROM
= G | grid
= Del | clear
= Ctrl+C / Ctrl+V | copy / paste
= Ctrl+Z / Ctrl+Y | undo / redo
= Ctrl+S / Ctrl+O | save / open

> Ideas from **KANJI** (AMID64, MIT) and the **C64 BDF font editor** (garymeg, MIT). The start font is the original C64 character ROM © Commodore.
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
= Shift+F | Gradient fill (opens the gradient window)
= Shift+W | Shade (left lighter, right darker)
= N / Shift+N | Square / filled
= I / Shift+I | Triangle / filled
= O / Shift+O | Hexagon / filled
= L / Shift+L | Star / filled
= Shift+X | Symmetry on/off
= A | Airbrush
= Shift+D | Dither brush (experimental)
= Shift+S | Smear
= W | Eraser (active layer)
= R / Shift+R | Rectangle / filled
= C / Shift+C | Circle / filled
= E / Shift+E | Ellipse / filled
= B | Select
= , | Color picker (also Alt+click)
= Space | hold: move the view
= Ctrl | while painting (freehand, dots): straight line, with Shift at 45°

## Layers

= 2 / 3 / 4 | paint on layer 2 / 3 / 4 (3 and 4 switch layers 3 + 4 on)
= T | template (layer 1) on/off

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
= 0, Ctrl+0 or Home | fit to window
= Ctrl+1 | 100 %
= G / Shift+G | grid on/off / settings
= M | pixel grid
= Shift+M | mirrored view on/off
= Shift+V | CRT preview window
= Tab | side panel on/off

## Edit and files

= U or Ctrl+Z | undo
= Shift+U or Ctrl+Y | redo
= J | swap page (spare page)
= Shift+A | Auto dither (experimental)
= Shift+K | clear picture
= Ctrl+N | new
= Ctrl+O | open (also .c64dp projects)
= Ctrl+I | import & convert image
= Ctrl+S / Ctrl+Shift+S | save / save as
= F2 | quick save: numbered PNG into the working folder
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
- **Flat first, dither later.** Paint shapes in clean flat colors, then blend the borders with **Edit → Auto dither** or the **Dither brush** (both experimental), and shade with the style **Color**.
- **Smear for soft forms.** The **Smear** brush pulls colors into each other – handy for clouds, hair and fabric folds.
- **Trace a template.** Load a photo as a template, set 40–60 % opacity and trace the outlines. Or let the converter do it and touch up the result.
- **Convert with care.** In the converter, raise the contrast a little and try Bayer 4×4 or Floyd–Steinberg at 40–70 % strength.
- **Use the spare page.** With ` + "`J`" + ` you have a second picture for experiments or as a parts bin for brushes.
- **Play with the conflict setting.** "Block" for clean work, "Use nearest color" for quick shading with the airbrush.
- **Check on the real thing.** Export a PRG and start it in an emulator. Depending on the palette, colors can look different.
`},
	{id: "about", title: "About C64 DPainter", body: `
# About C64 DPainter

**Version 0.99 beta** – a paint program for C64 multicolor bitmaps.

Runs on **Windows 11** (Windows 10 may crash), **macOS** (Apple silicon and Intel) and **Linux**.

Created by **IT-Dummköpfen**. Free for private use.

Inspired by **Deluxe Paint** (Electronic Arts, 1985), **PyDPainter** and **Multipaint**. Controls and keyboard shortcuts follow these programs.

I was also inspired by many comments from users. Thank you so much! ❤

## Why This Software Is Free for Personal Use

When I develop software using artificial intelligence, I don’t view AI as a replacement for human work or merely as an automated tool. To me, AI is like a brilliant colleague—a partner who helps me develop ideas and brings them to life exactly as I envision them.

Since this software was created through this collaborative process, I firmly believe that it belongs to the community. When code is developed side by side with AI, it shouldn’t be hidden behind paywalls or kept strictly to oneself. Technology should empower people, and software developed with the help of artificial intelligence should be free for personal, non-commercial use.

This project is my contribution to this way of thinking. I hope you enjoy using it as much as I’ve enjoyed developing it over the past few weeks!

## Sources and thanks

Thank you all for making your work available and letting us use it:

- **Palettes:** collection from c64gfx.com/palettes. Two palettes were missing one color on the source page; it was taken from Pepto's palette and is marked with * in the palette dialog.
- **Technical basics:** C64-Wiki (multicolor bitmap mode, MULTI) and retro-programming.de (VIC-II graphics modes).
- **Font:** DejaVu Sans (Bitstream Vera / DejaVu fonts license).
- **macOS:** purego by Ebitengine (Apache 2.0) – the Cocoa window without cgo.
- **Linux:** xgb by BurntSushi and jezek (BSD) – the X11 connection.
- **Windows:** rsrc by akavel (MIT) – the program icon resource.
- **Language:** Go and its standard library (BSD), The Go Authors.
- **Dithering:** articles and guides on pixelparmesan.com, pixel-editor.com, pixnote.net, divoom.com and pixie.haus.
- **File formats:** the details of Advanced Art Studio and Amica Paint files from RECOIL by Piotr Fusik.
- **Import:** the ideas for color matching and cell colors from Retropixels by Michel de Bree (MIT license).
- **PETSCII editor:** ideas and the "sorted by shape" character order from Petmate by nurpax (MIT license); ideas from petsciirender by gnacu, PETSCII editor by Marq (ventti/petscii, WTFPL) and Image-to-PETSCII by amb (GPL-3, only the idea, no code).
- **Sprite editor:** ideas from Spritemate by Ingo Hinterding, MagSpriteEd by Magnar Harestad and Sprite Studio 64 by Tobiasz Stamborski (all MIT license).
- **Font editor:** ideas from KANJI by AMID64 and the C64 BDF font editor by garymeg (both MIT license).
- **C64 character ROM** © Commodore – used by the PETSCII editor, the font editor and the text tool, as in Petmate and most PETSCII editors.
- **Dither styles:** 16 hand-pixelled C64 scene pictures, analyzed pattern by pattern.
- **Gradients:** color ramp table by IT-Dummköpfen, extended with more C64 ramps.
- **Inspiration:** Deluxe Paint (Electronic Arts, 1985), PyDPainter and Multipaint (Marq/Kameli).

## Settings

Settings (theme, palette, grid, conflict mode …) are stored in ` + "`{SETTINGS}`" + `.

-> start | Back to "First steps"
`},
}
