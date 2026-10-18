package app

import (
	"math"

	"c64dpainter/internal/c64"
)

// --- Square, triangle, hexagon and star ----------------------------------------------
//
// Shapes are round on screen (a multicolor pixel is two screen pixels wide),
// like the circle. Outlines use the brush, filled shapes the pattern.

type fpt struct{ x, y float64 } // screen units: x = pixel*2, y = pixel

func isShapeTool(t int) bool {
	switch t {
	case ToolSquare, ToolSquareF, ToolTriangle, ToolTriangleF, ToolHexagon, ToolHexagonF, ToolStar, ToolStarF:
		return true
	}
	return false
}

func shapeFilled(t int) bool {
	return t == ToolSquareF || t == ToolTriangleF || t == ToolHexagonF || t == ToolStarF
}

// shapeVertices returns the corners of a regular shape around the pixel
// (x0, y0), with its first corner pointing to (x1, y1). upright keeps it
// standing straight.
func shapeVertices(tool, x0, y0, x1, y1 int, upright bool) []fpt {
	cx, cy := float64(x0)*2+1, float64(y0)+0.5
	dx, dy := float64(x1-x0)*2, float64(y1-y0)
	r := math.Hypot(dx, dy)
	ang := math.Atan2(dy, dx)
	if upright {
		ang = -math.Pi / 2
	}
	var n int
	var inner float64
	switch tool {
	case ToolTriangle, ToolTriangleF:
		n = 3
	case ToolHexagon, ToolHexagonF:
		n = 6
	case ToolStar, ToolStarF:
		n, inner = 5, 0.42
	}
	var v []fpt
	if inner > 0 {
		for i := 0; i < 2*n; i++ {
			rr := r
			if i%2 == 1 {
				rr = r * inner
			}
			a := ang + float64(i)*math.Pi/float64(n)
			v = append(v, fpt{cx + rr*math.Cos(a), cy + rr*math.Sin(a)})
		}
		return v
	}
	for i := 0; i < n; i++ {
		a := ang + float64(i)*2*math.Pi/float64(n)
		v = append(v, fpt{cx + r*math.Cos(a), cy + r*math.Sin(a)})
	}
	return v
}

// toPixel converts screen units to the pixel that contains the point.
func toPixel(p fpt) (int, int) {
	return int(math.Floor(p.x / 2)), int(math.Floor(p.y))
}

// drawPolygon draws the outline (brush) or the filled area (pattern).
func (a *App) drawPolygon(v []fpt, filled bool, btn int) {
	if len(v) < 3 {
		return
	}
	if !filled {
		for i := range v {
			x0, y0 := toPixel(v[i])
			x1, y1 := toPixel(v[(i+1)%len(v)])
			a.lineDabs(a.pic, x0, y0, x1, y1, btn)
		}
		return
	}
	minY, maxY := v[0].y, v[0].y
	for _, p := range v {
		minY = math.Min(minY, p.y)
		maxY = math.Max(maxY, p.y)
	}
	for py := int(math.Floor(minY)); py <= int(math.Ceil(maxY)); py++ {
		if py < 0 || py >= c64.H {
			continue
		}
		sy := float64(py) + 0.5
		var xs []float64
		for i := range v {
			p, q := v[i], v[(i+1)%len(v)]
			if (p.y <= sy && q.y > sy) || (q.y <= sy && p.y > sy) {
				xs = append(xs, p.x+(sy-p.y)*(q.x-p.x)/(q.y-p.y))
			}
		}
		sortFloats(xs)
		for k := 0; k+1 < len(xs); k += 2 {
			for px := int(math.Ceil((xs[k] - 1) / 2)); float64(px)*2+1 <= xs[k+1]; px++ {
				a.plotPat(a.pic, px, py, btn)
			}
		}
	}
	// the outline in the pattern too, so thin corners are not lost
	for i := range v {
		x0, y0 := toPixel(v[i])
		x1, y1 := toPixel(v[(i+1)%len(v)])
		forLine(x0, y0, x1, y1, func(x, y int) { a.plotPat(a.pic, x, y, btn) })
	}
}

func sortFloats(f []float64) {
	for i := 1; i < len(f); i++ {
		for j := i; j > 0 && f[j] < f[j-1]; j-- {
			f[j], f[j-1] = f[j-1], f[j]
		}
	}
}

// drawShape draws the current shape tool from (x0, y0) to (x1, y1).
func (a *App) drawShape(x0, y0, x1, y1, btn int, shift bool) {
	filled := shapeFilled(a.tool)
	switch a.tool {
	case ToolSquare, ToolSquareF:
		x1, y1 = constrainSquare(x0, y0, x1, y1)
		if filled {
			a.fillRect(x0, y0, x1, y1, btn)
			return
		}
		a.lineDabs(a.pic, x0, y0, x1, y0, btn)
		a.lineDabs(a.pic, x1, y0, x1, y1, btn)
		a.lineDabs(a.pic, x1, y1, x0, y1, btn)
		a.lineDabs(a.pic, x0, y1, x0, y0, btn)
		return
	}
	if x0 == x1 && y0 == y1 {
		a.dab(a.pic, x0, y0, btn)
		return
	}
	a.drawPolygon(shapeVertices(a.tool, x0, y0, x1, y1, shift), filled, btn)
}
