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

// DefaultLayers is the number of paint layers of a new picture, MaxLayers
// the most a picture can have (the template layer of the editor makes 16).
const (
	DefaultLayers = 3
	MaxLayers     = 15
)

// LayerCount returns the number of paint layers.
func (p *Picture) LayerCount() int { return len(p.L) }

// IsFree reports whether layer l ignores the color rules.
func (p *Picture) IsFree(l int) bool { return l >= 0 && l < len(p.Free) && p.Free[l] }

// HasFreeLayers reports whether a visible layer with pixels ignores the rules
// (the picture then has to be converted for C64 formats).
func (p *Picture) HasFreeLayers() bool {
	if !p.Layered {
		return false
	}
	for l := range p.L {
		if p.Free[l] && !p.Hide[l] && p.LayerPixels(l) > 0 {
			return true
		}
	}
	return false
}

// ensureLayers makes sure the layer slices exist and have the same length.
func (p *Picture) ensureLayers() {
	if len(p.L) == 0 {
		p.L = make([][HW * H]uint8, DefaultLayers)
	}
	for len(p.Hide) < len(p.L) {
		p.Hide = append(p.Hide, false)
	}
	for len(p.Free) < len(p.L) {
		p.Free = append(p.Free, false)
	}
	for len(p.Names) < len(p.L) {
		p.Names = append(p.Names, "")
	}
	p.Hide, p.Free, p.Names = p.Hide[:len(p.L)], p.Free[:len(p.L)], p.Names[:len(p.L)]
	if p.Active >= len(p.L) {
		p.Active = len(p.L) - 1
	}
	if p.Active < 0 {
		p.Active = 0
	}
}

// Layer pixel values.
const (
	LayerClear uint8 = 0  // transparent
	LayerBG    uint8 = 17 // the background color ($D021), follows its changes
)

// ToLayers turns a plain picture into a layered one: the whole picture goes
// into the front layer (index 0; background pixels become transparent), the
// other layers are empty and all layers are visible.
func (p *Picture) ToLayers() {
	if !p.Layered {
		p.L, p.Hide, p.Free, p.Names = nil, nil, nil, nil
	}
	p.ensureLayers()
	for l := range p.L {
		p.L[l] = [HW * H]uint8{}
	}
	if p.Hires() {
		if !p.Layered {
			for i, c := range p.HP {
				if c != p.BG {
					p.L[0][i] = c + 1
				}
			}
		}
		p.Layered = true
		for l := range p.Hide {
			p.Hide[l] = false
		}
		p.Active = 0
		p.hRecomposite()
		return
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
	for l := range p.Hide {
		p.Hide[l] = false
	}
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
	if l < 0 || l >= len(p.L) || x < 0 || y < 0 || x >= p.Width() || y >= H {
		return 0, false
	}
	return p.layerColor(p.L[l][y*p.Width()+x])
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
	for l := 0; l < len(p.L); l++ {
		if p.Free[l] {
			continue
		}
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
			for l := len(p.L) - 1; l >= 0; l-- { // the highest index is on top
				if p.Hide[l] || p.Free[l] {
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
	if p.Hires() {
		p.hRecomposite()
		return
	}
	if !p.Layered {
		return
	}
	for cell := 0; cell < Cells; cell++ {
		p.rebuildCell(cell)
	}
}

// SetLayerHidden shows or hides a layer.
func (p *Picture) SetLayerHidden(l int, hide bool) {
	if l < 0 || l >= len(p.L) || p.Hide[l] == hide {
		return
	}
	p.Hide[l] = hide
	p.Recomposite()
}

// Erase makes a pixel of the active layer transparent. Without layers the
// pixel becomes background.
func (p *Picture) Erase(x, y int) {
	if x < 0 || y < 0 || x >= p.Width() || y >= H {
		if side, sx, ok := SideAt(x); ok && p.SideActive() {
			p.EraseSide(side, sx, y)
		}
		return
	}
	if p.Hires() {
		p.hErase(x, y)
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
	if !p.Free[p.Active] {
		p.rebuildCell(CellOf(x, y))
	}
}

// ClearLayer makes a whole layer transparent.
func (p *Picture) ClearLayer(l int) {
	if !p.Layered || l < 0 || l >= len(p.L) {
		return
	}
	p.L[l] = [HW * H]uint8{}
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
	protect := mode&ConflictProtect != 0
	mode &^= ConflictProtect
	i := y*W + x
	cell := CellOf(x, y)
	v := c + 1
	if c == p.BG {
		v = LayerBG
	}
	if p.L[p.Active][i] == v {
		return PlotOK
	}
	if p.Free[p.Active] {
		p.L[p.Active][i] = v // free layer: no rules
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
		cx, cy := (cell%Cols)*4, (cell/Cols)*8
		var other [16]int // pixels of the other layers per color
		if protect {
			for l := 0; l < len(p.L); l++ {
				if l == p.Active || p.Free[l] {
					continue
				}
				for yy := cy; yy < cy+8; yy++ {
					for xx := cx; xx < cx+4; xx++ {
						if v := p.L[l][yy*W+xx]; v != LayerClear && v != LayerBG {
							other[v-1]++
						}
					}
				}
			}
		}
		rare := -1
		for k := 0; k < 16; k++ {
			if cnt[k] > 0 && other[k] == 0 && (rare < 0 || cnt[k] < cnt[rare]) {
				rare = k
			}
		}
		if rare < 0 {
			return PlotBlocked // every color is used by another layer
		}
		for l := 0; l < len(p.L); l++ {
			if p.Free[l] {
				continue
			}
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
			for l := 0; l < len(p.L); l++ {
				if p.Free[l] {
					continue
				}
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
	for l := 0; l < len(p.L); l++ {
		if p.Free[l] {
			continue
		}
		for y := cy; y < cy+8; y++ {
			for x := cx; x < cx+4; x++ {
				i := y*W + x
				switch p.L[l][i] {
				case from + 1:
					if to == p.BG {
						p.L[l][i] = LayerBG
					} else {
						p.L[l][i] = to + 1
					}
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

// --- layer list operations (1.1) ----------------------------------------------

// InsertLayer adds an empty layer at index at (0 = bottom). The new layer
// becomes the active one.
func (p *Picture) InsertLayer(at int) bool {
	p.ensureLayers()
	if len(p.L) >= MaxLayers || at < 0 || at > len(p.L) {
		return false
	}
	p.L = append(p.L, [HW * H]uint8{})
	copy(p.L[at+1:], p.L[at:])
	p.L[at] = [HW * H]uint8{}
	p.Hide = append(p.Hide[:at], append([]bool{false}, p.Hide[at:]...)...)
	p.Free = append(p.Free[:at], append([]bool{false}, p.Free[at:]...)...)
	p.Names = append(p.Names[:at], append([]string{""}, p.Names[at:]...)...)
	p.Active = at
	return true
}

// DeleteLayer removes a layer (at least one stays).
func (p *Picture) DeleteLayer(l int) bool {
	if len(p.L) <= 1 || l < 0 || l >= len(p.L) {
		return false
	}
	p.L = append(p.L[:l], p.L[l+1:]...)
	p.Hide = append(p.Hide[:l], p.Hide[l+1:]...)
	p.Free = append(p.Free[:l], p.Free[l+1:]...)
	p.Names = append(p.Names[:l], p.Names[l+1:]...)
	if p.Active >= len(p.L) || p.Active > l {
		p.Active--
	}
	p.Active = max(0, min(p.Active, len(p.L)-1))
	p.Recomposite()
	return true
}

// MoveLayer moves layer l one step up (dir = 1) or down (dir = -1).
func (p *Picture) MoveLayer(l, dir int) bool {
	k := l + dir
	if l < 0 || k < 0 || l >= len(p.L) || k >= len(p.L) {
		return false
	}
	p.L[l], p.L[k] = p.L[k], p.L[l]
	p.Hide[l], p.Hide[k] = p.Hide[k], p.Hide[l]
	p.Free[l], p.Free[k] = p.Free[k], p.Free[l]
	p.Names[l], p.Names[k] = p.Names[k], p.Names[l]
	switch p.Active {
	case l:
		p.Active = k
	case k:
		p.Active = l
	}
	p.Recomposite()
	return true
}

// DuplicateLayer copies layer l to a new layer above it.
func (p *Picture) DuplicateLayer(l int) bool {
	if l < 0 || l >= len(p.L) || !p.InsertLayer(l+1) {
		return false
	}
	p.L[l+1] = p.L[l]
	p.Hide[l+1] = p.Hide[l]
	p.Free[l+1] = p.Free[l]
	if p.Names[l] != "" {
		p.Names[l+1] = p.Names[l] + " copy"
	}
	p.Recomposite()
	return true
}

// reapplyRules paints the pixels of layer l again with the color rules
// (used when a free layer becomes a normal one); colors that do not fit
// take the nearest color of the cell.
func (p *Picture) reapplyRules(l int, pal *Palette) {
	px := p.L[l]
	p.L[l] = [HW * H]uint8{}
	p.Free[l] = false
	act := p.Active
	p.Active = l
	p.Recomposite()
	w := p.Width()
	for i, v := range px {
		if v == LayerClear {
			continue
		}
		c := p.BG
		if v != LayerBG {
			c = v - 1
		}
		p.Plot(i%w, i/w, c, ConflictNearest, pal)
	}
	p.Active = act
	p.Recomposite()
}

// SetFree switches the color rules of a layer off (free) or on again.
func (p *Picture) SetFree(l int, free bool, pal *Palette) {
	if l < 0 || l >= len(p.L) || p.Free[l] == free || p.Mode == ModeFree {
		return
	}
	if free {
		p.Free[l] = true
		p.Recomposite()
		return
	}
	p.reapplyRules(l, pal)
}

// MergeDown paints layer l onto the layer below it and removes l. A normal
// layer below keeps its color rules (colors that do not fit take the
// nearest color of the cell).
func (p *Picture) MergeDown(l int, pal *Palette) bool {
	if l <= 0 || l >= len(p.L) {
		return false
	}
	below := l - 1
	top := p.L[l]
	wasFree := p.Free[l]
	p.DeleteLayer(l)
	if p.Free[below] || !wasFree {
		for i, v := range top {
			if v != LayerClear {
				p.L[below][i] = v
			}
		}
		if !p.Free[below] {
			// merged rule pixels: check the cells again
			px := p.L[below]
			p.L[below] = [HW * H]uint8{}
			p.Recomposite()
			act := p.Active
			p.Active = below
			w := p.Width()
			for i, v := range px {
				if v == LayerClear {
					continue
				}
				c := p.BG
				if v != LayerBG {
					c = v - 1
				}
				p.Plot(i%w, i/w, c, ConflictNearest, pal)
			}
			p.Active = act
		}
	} else {
		act := p.Active
		p.Active = below
		w := p.Width()
		for i, v := range top {
			if v == LayerClear {
				continue
			}
			c := p.BG
			if v != LayerBG {
				c = v - 1
			}
			p.Plot(i%w, i/w, c, ConflictNearest, pal)
		}
		p.Active = act
	}
	p.Active = below
	p.Recomposite()
	return true
}
