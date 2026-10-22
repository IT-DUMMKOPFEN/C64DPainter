package c64

// Layers
//
// In the editor a picture has three paint layers. Index 0 (layer 2 in the
// user interface) is at the bottom, index 2 (layer 4) on top. Every
// layer pixel is transparent, the background color or one of the 16 colors.
// The multicolor rules apply to all three layers together, visible or not:
// the colors of all layer pixels of a 4×8 cell are at most the background
// plus three colors. So any combination of visible layers is a valid C64
// picture. Bits, Screen and ColorRAM always hold the composite of the
// visible layers (the topmost visible opaque pixel wins), so rendering and
// export work as before. A picture without layers (Layered == false) is a
// plain C64 picture and behaves exactly as before.

// LayerCount is the number of paint layers.
const LayerCount = 3

// Layer pixel values.
const (
	LayerClear uint8 = 0  // transparent
	LayerBG    uint8 = 17 // the background color ($D021), follows its changes
)

// ToLayers turns a plain picture into a layered one: the whole picture goes
// into the front layer (index 0; background pixels become transparent), the
// other layers are empty and all layers are visible.
func (p *Picture) ToLayers() {
	for l := range p.L {
		p.L[l] = [W * H]uint8{}
	}
	if !p.Layered {
		for y := 0; y < H; y++ {
			for x := 0; x < W; x++ {
				i := y*W + x
				if p.Bits[i] != 0 {
					p.L[0][i] = p.ColorAt(x, y) + 1
				}
			}
		}
	}
	p.Layered = true
	p.Hide = [LayerCount]bool{}
	p.Active = 0
	p.Recomposite()
}

// layerColor decodes a layer pixel value.
func (p *Picture) layerColor(v uint8) (c uint8, opaque bool) {
	switch {
	case v == LayerClear:
		return 0, false
	case v == LayerBG:
		return p.BG, true
	}
	return v - 1, true
}

// LayerColorAt returns the color of a layer pixel and whether it is opaque.
func (p *Picture) LayerColorAt(l, x, y int) (uint8, bool) {
	if l < 0 || l >= LayerCount || x < 0 || y < 0 || x >= W || y >= H {
		return 0, false
	}
	return p.layerColor(p.L[l][y*W+x])
}

// LayerPixels counts the opaque pixels of a layer.
func (p *Picture) LayerPixels(l int) int {
	n := 0
	for _, v := range p.L[l] {
		if v != LayerClear {
			n++
		}
	}
	return n
}

// cellUnion counts, per color, the opaque non-background pixels of all
// layers in a cell. The pixel (ex, ey) of the active layer is left out.
func (p *Picture) cellUnion(cell, ex, ey int) (cnt [16]int, n int) {
	cx, cy := (cell%Cols)*4, (cell/Cols)*8
	for l := 0; l < LayerCount; l++ {
		for y := cy; y < cy+8; y++ {
			for x := cx; x < cx+4; x++ {
				if l == p.Active && x == ex && y == ey {
					continue
				}
				c, ok := p.layerColor(p.L[l][y*W+x])
				if !ok || c == p.BG {
					continue
				}
				if cnt[c] == 0 {
					n++
				}
				cnt[c]++
			}
		}
	}
	return
}

// rebuildCell assigns the colors of all layers to the bit pairs of a cell
// (colors keep their slot when possible) and composes the visible pixels.
func (p *Picture) rebuildCell(cell int) {
	cnt, _ := p.cellUnion(cell, -1, -1)
	var slotOf [16]int
	var taken [4]bool
	for s := 1; s <= 3; s++ {
		c := p.SlotColor(cell, s)
		if c != p.BG && cnt[c] > 0 && slotOf[c] == 0 {
			slotOf[c] = s
			taken[s] = true
		}
	}
	for c := uint8(0); c < 16; c++ {
		if cnt[c] == 0 || c == p.BG || slotOf[c] != 0 {
			continue
		}
		for s := 1; s <= 3; s++ {
			if !taken[s] {
				slotOf[c] = s
				taken[s] = true
				p.SetSlotColor(cell, s, c)
				break
			}
		}
	}
	for s := 1; s <= 3; s++ {
		if !taken[s] {
			p.SetSlotColor(cell, s, 0)
		}
	}
	cx, cy := (cell%Cols)*4, (cell/Cols)*8
	for y := cy; y < cy+8; y++ {
		for x := cx; x < cx+4; x++ {
			i := y*W + x
			bits := uint8(0)
			for l := LayerCount - 1; l >= 0; l-- { // the highest index is on top
				if p.Hide[l] {
					continue
				}
				c, ok := p.layerColor(p.L[l][i])
				if !ok {
					continue
				}
				if c != p.BG {
					bits = uint8(slotOf[c])
				}
				break
			}
			p.Bits[i] = bits
		}
	}
}

// Recomposite rebuilds the visible C64 picture from the layers.
func (p *Picture) Recomposite() {
	if !p.Layered {
		return
	}
	for cell := 0; cell < Cells; cell++ {
		p.rebuildCell(cell)
	}
}

// SetLayerHidden shows or hides a layer.
func (p *Picture) SetLayerHidden(l int, hide bool) {
	if l < 0 || l >= LayerCount || p.Hide[l] == hide {
		return
	}
	p.Hide[l] = hide
	p.Recomposite()
}

// Erase makes a pixel of the active layer transparent. Without layers the
// pixel becomes background.
func (p *Picture) Erase(x, y int) {
	if x < 0 || y < 0 || x >= W || y >= H {
		return
	}
	i := y*W + x
	if !p.Layered {
		p.Bits[i] = 0
		return
	}
	if p.L[p.Active][i] == LayerClear {
		return
	}
	p.L[p.Active][i] = LayerClear
	p.rebuildCell(CellOf(x, y))
}

// ClearLayer makes a whole layer transparent.
func (p *Picture) ClearLayer(l int) {
	if !p.Layered || l < 0 || l >= LayerCount {
		return
	}
	p.L[l] = [W * H]uint8{}
	p.Recomposite()
}

// SetBG changes the background color ($D021). Like on the C64 all background
// pixels change with it; layer pixels that already have the new color become
// background pixels.
func (p *Picture) SetBG(c uint8) {
	c &= 15
	if p.Layered {
		for l := range p.L {
			for i, v := range p.L[l] {
				if v == c+1 {
					p.L[l][i] = LayerBG
				}
			}
		}
	}
	p.BG = c
	p.Recomposite()
}

// plotLayer is Plot for layered pictures: it paints into the active layer
// and checks the rules for all layers together.
func (p *Picture) plotLayer(x, y int, c uint8, mode ConflictMode, pal *Palette) PlotResult {
	i := y*W + x
	cell := CellOf(x, y)
	v := c + 1
	if c == p.BG {
		v = LayerBG
	}
	if p.L[p.Active][i] == v {
		return PlotOK
	}
	if v == LayerBG {
		p.L[p.Active][i] = v
		p.rebuildCell(cell)
		return PlotOK
	}
	cnt, n := p.cellUnion(cell, x, y)
	if cnt[c] > 0 || n < 3 {
		p.L[p.Active][i] = v
		p.rebuildCell(cell)
		return PlotOK
	}
	// conflict: the layers already use background + 3 colors in this cell
	switch mode {
	case ConflictReplace:
		rare := -1
		for k := 0; k < 16; k++ {
			if cnt[k] > 0 && (rare < 0 || cnt[k] < cnt[rare]) {
				rare = k
			}
		}
		cx, cy := (cell%Cols)*4, (cell/Cols)*8
		for l := 0; l < LayerCount; l++ {
			for yy := cy; yy < cy+8; yy++ {
				for xx := cx; xx < cx+4; xx++ {
					if p.L[l][yy*W+xx] == uint8(rare)+1 {
						p.L[l][yy*W+xx] = v
					}
				}
			}
		}
		p.L[p.Active][i] = v
		p.rebuildCell(cell)
		return PlotReplaced
	case ConflictNearest:
		if pal == nil {
			return PlotBlocked
		}
		tr, tg, tb := rgbOf(pal.Colors[c])
		best, bd := p.BG, 1<<62
		for k := 0; k < 16; k++ {
			if cnt[k] == 0 && uint8(k) != p.BG {
				continue
			}
			r, g, b := rgbOf(pal.Colors[k])
			if d := ColorDist(tr, tg, tb, r, g, b); d < bd {
				bd, best = d, uint8(k)
			}
		}
		nv := best + 1
		if best == p.BG {
			nv = LayerBG
		}
		p.L[p.Active][i] = nv
		p.rebuildCell(cell)
		return PlotSubstituted
	}
	return PlotBlocked
}

// usageLayers counts, per bit pair, the pixels of all layers in a cell;
// positions where no layer has a pixel count as background.
func (p *Picture) usageLayers(cell, ex, ey int) [4]int {
	var u [4]int
	cx, cy := (cell%Cols)*4, (cell/Cols)*8
	for y := cy; y < cy+8; y++ {
		for x := cx; x < cx+4; x++ {
			if x == ex && y == ey {
				continue
			}
			any := false
			for l := 0; l < LayerCount; l++ {
				c, ok := p.layerColor(p.L[l][y*W+x])
				if !ok {
					continue
				}
				any = true
				s := 0
				if c != p.BG {
					for k := 1; k <= 3; k++ {
						if p.SlotColor(cell, k) == c {
							s = k
							break
						}
					}
				}
				u[s]++
			}
			if !any {
				u[0]++
			}
		}
	}
	return u
}

// replaceCellColorLayers is ReplaceCellColor for layered pictures: the color
// changes in all layers of the cell.
func (p *Picture) replaceCellColorLayers(cell int, from, to uint8, both bool) int {
	cx, cy := (cell%Cols)*4, (cell/Cols)*8
	changed := 0
	for l := 0; l < LayerCount; l++ {
		for y := cy; y < cy+8; y++ {
			for x := cx; x < cx+4; x++ {
				i := y*W + x
				switch p.L[l][i] {
				case from + 1:
					p.L[l][i] = to + 1
					changed++
				case to + 1:
					if both {
						p.L[l][i] = from + 1
						changed++
					}
				}
			}
		}
	}
	if changed > 0 {
		p.rebuildCell(cell)
	}
	return changed
}

// HiddenOnly reports for each bit pair of a cell whether its color is used
// only by pixels of hidden layers (or covered pixels), so it is not visible.
func (p *Picture) HiddenOnly(cell int) (hidden [4]bool) {
	if !p.Layered {
		return
	}
	u := p.Usage(cell, -1, -1)
	var vis [4]int
	cx, cy := (cell%Cols)*4, (cell/Cols)*8
	for y := cy; y < cy+8; y++ {
		for x := cx; x < cx+4; x++ {
			vis[p.Bits[y*W+x]]++
		}
	}
	for s := 1; s <= 3; s++ {
		hidden[s] = u[s] > 0 && vis[s] == 0
	}
	return
}
