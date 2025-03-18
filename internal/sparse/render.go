// SPDX-FileCopyrightText: 2024 the Piet Authors
// SPDX-FileCopyrightText: 2025 Dominik Honnef and contributors
//
// SPDX-License-Identifier: MIT

package sparse

import (
	"fmt"
	"iter"
	"log"
	"slices"

	"honnef.co/go/curve"
)

type FillRule int

const (
	NonZero FillRule = iota
	EvenOdd
)

type Color [4]float32

type gfxState struct {
	numLayers int
}

type layer struct {
	// The intersected bounding box after clip
	bbox [4]uint16
	// The rendered path in sparse strip representation
	strips  []strip
	opacity float32
	blend   BlendMode
}

type Renderer struct {
	width  uint16
	height uint16
	// [y][x]wideTile
	tiles [][]wideTile
	// [sparse column][y]uint8
	alphas     [][stripHeight]uint8
	transform  curve.Affine
	stateStack []gfxState
	layerStack []layer

	// Scratch buffers
	tileBuf  []tile
	stripBuf []strip
	lineBuf  []flatLine
}

func NewRenderer(width, height uint16) *Renderer {
	widthTiles := divCeil(width, wideTileWidth)
	heightTiles := divCeil(height, stripHeight)
	tiles := make([][]wideTile, heightTiles)
	for i := range tiles {
		tiles[i] = make([]wideTile, widthTiles)
	}

	return &Renderer{
		width:      width,
		height:     height,
		tiles:      tiles,
		transform:  curve.Identity,
		stateStack: []gfxState{{0}},
	}
}

func (ctx *Renderer) Reset() {
	for _, row := range ctx.tiles {
		for x := range row {
			tile := &row[x]
			tile.bg = Color{}
			clear(tile.cmds)
			tile.cmds = tile.cmds[:0]
		}
	}

	ctx.alphas = ctx.alphas[:0]
}

// Finish the coarse rasterization prior to fine rendering.
//
// At the moment, this mostly involves resolving any open layers, but
// might extend to other things.
func (ctx *Renderer) finish() {
	ctx.popLayers()
}

func (ctx *Renderer) RenderToPixmap(width, height uint16, pixmap []Color) {
	ctx.finish()
	fine := newFine(width, height, pixmap)
	for y, row := range ctx.tiles {
		for x := range row {
			tile := &row[x]
			fine.topLayer().clear(tile.bg)

			if false && len(tile.cmds) > 0 {
				log.Println("tile", x, y)
				for i := range tile.cmds {
					log.Println(&tile.cmds[i])
				}
				log.Println()
			}

			for i := range tile.cmds {
				cmd := tile.cmds[i]
				fine.runCmd(cmd, ctx.alphas)
			}
			switch len(fine.layers) {
			case 0:
				panic("internal error: left with no layers")
			case 1:
			default:
				panic("internal error: left with more than one layer")
			}
			fine.pack(uint16(x), uint16(y))
		}
	}
	if false {
		log.Println(&fine.stats)
	}
}

func (ctx *Renderer) renderPathCommon(lineBuf []flatLine, fillRule FillRule) {
	ctx.tileBuf = makeTiles(lineBuf, ctx.tileBuf, uint16(ctx.width), uint16(ctx.height))
	slices.SortFunc(ctx.tileBuf, tile.cmp)
	ctx.stripBuf, ctx.alphas = renderStripsScalar(ctx.tileBuf, fillRule, ctx.lineBuf, ctx.stripBuf, ctx.alphas)
}

func (ctx *Renderer) renderPath(lineBuf []flatLine, fillRule FillRule, color Color) {
	// XXX support a brush

	ctx.renderPathCommon(lineBuf, fillRule)

	bbox := ctx.bbox()
	for i := range len(ctx.stripBuf) - 1 {
		strip := &ctx.stripBuf[i]

		if strip.x >= ctx.width {
			// Don't render strips that are outside the viewport.
			continue
		}

		nextStrip := &ctx.stripBuf[i+1]
		x0 := strip.x
		stripY := strip.stripY()
		if stripY < bbox[1] {
			continue
		}
		if stripY >= bbox[3] {
			break
		}
		col := strip.col
		// Can potentially be 0, if the next strip's x values is also < 0.
		var stripWidth uint16
		if v := nextStrip.col - col; v <= nextStrip.col {
			stripWidth = uint16(v)
		}
		x1 := x0 + stripWidth
		xtile0 := max(x0/wideTileWidth, bbox[0])
		xtile1 := min(divCeil(x1, wideTileWidth), bbox[2])
		x := x0
		if bbox[0]*wideTileWidth > x {
			col += uint32(bbox[0]*wideTileWidth - x)
			x = bbox[0] * wideTileWidth
		}
		for xtile := xtile0; xtile < xtile1; xtile++ {
			xTileRel := x % wideTileWidth
			width := min(x1, (xtile+1)*wideTileWidth) - x
			cmd := cmd{
				typ:      cmdAlphaFill,
				x:        xTileRel,
				width:    width,
				alphaIdx: int(col),
				color:    color,
			}
			x += width
			col += uint32(width)
			ctx.tiles[stripY][xtile].alphaFill(cmd)
		}

		var activeFill bool
		switch fillRule {
		case NonZero:
			activeFill = nextStrip.winding != 0
		case EvenOdd:
			activeFill = nextStrip.winding%2 != 0
		default:
			panic(fmt.Sprintf("unexpected sparse.FillRule: %#v", fillRule))
		}

		if activeFill && stripY == nextStrip.stripY() {
			x = max(x1, bbox[0]*wideTileWidth)
			uproundedWidth := divCeil(ctx.width, wideTileWidth) * wideTileWidth
			x2 := min(nextStrip.x, uproundedWidth)
			fxt0 := max(x1/wideTileWidth, bbox[0])
			fxt1 := min(divCeil(x2, wideTileWidth), bbox[2])
			for xtile := fxt0; xtile < fxt1; xtile++ {
				xTileRel := x % wideTileWidth
				width := min(x2, (xtile+1)*wideTileWidth) - x
				x += width
				ctx.tiles[stripY][xtile].fill(xTileRel, width, color)
			}
		}
	}
}

func (ctx *Renderer) bbox() [4]uint16 {
	if len(ctx.layerStack) > 0 {
		return ctx.layerStack[len(ctx.layerStack)-1].bbox
	} else {
		widthTiles := divCeil(ctx.width, wideTileWidth)
		heightTiles := divCeil(ctx.height, stripHeight)
		return [4]uint16{
			0,
			0,
			widthTiles,
			heightTiles,
		}
	}
}

func (ctx *Renderer) popLayer() {
	ctx.stateStack[len(ctx.stateStack)-1].numLayers--
	lastLayer := ctx.layerStack[len(ctx.layerStack)-1]
	ctx.layerStack = ctx.layerStack[:len(ctx.layerStack)-1]
	bbox := lastLayer.bbox
	strips := lastLayer.strips

	// The next bit of code accomplishes the following. For each tile in
	// the intersected bounding box, it does one of two things depending
	// on the contents of the clip path in that tile.
	// If all-zero: pop a zero_clip.
	// If contains one or more strips: render strips and fills, then pop a clip.
	// This logic is the inverse of the push logic in `clip()`, and the stack
	// should be balanced after running both.
	tileX := bbox[0]
	tileY := bbox[1]
	popPending := false
	for i := range len(strips) - 1 {
		strip := &strips[i]
		stripY := strip.stripY()
		if stripY < tileY {
			continue
		}
		for tileY < min(stripY, bbox[3]) {
			if popPending {
				ctx.tiles[tileY][tileX].popLayer()
				tileX++
				popPending = false
			}
			for x := tileX; x < bbox[2]; x++ {
				ctx.tiles[tileY][x].popZeroClip()
			}
			tileX = bbox[0]
			tileY++
		}
		if tileY == bbox[3] {
			break
		}
		x0 := strip.x
		xClamped := min(x0/wideTileWidth, bbox[2])
		if tileX < xClamped {
			if popPending {
				ctx.tiles[tileY][tileX].popLayer()
				tileX++
				popPending = false
			}
			// The winding check is probably not needed; if there was a fill,
			// the logic below should have advanced tileX.
			if strip.winding == 0 {
				for x := tileX; x < xClamped; x++ {
					ctx.tiles[tileY][x].popZeroClip()
				}
			}
			tileX = xClamped
		}

		nextStrip := &strips[i+1]
		width := uint16(nextStrip.col - strip.col)
		x1 := x0 + width
		xtile1 := min(divCeil(x1, wideTileWidth), bbox[2])
		x := x0
		col := strip.col
		if bbox[0]*wideTileWidth > x {
			col += uint32(bbox[0]*wideTileWidth - x)
			x = bbox[0] * wideTileWidth
		}
		for xtile := tileX; xtile < xtile1; xtile++ {
			if xtile > tileX && popPending {
				ctx.tiles[tileY][tileX].popLayer()
				popPending = false
			}
			xTileRel := x % wideTileWidth
			width := min(x1, (xtile+1)*wideTileWidth) - x
			cmd := cmd{
				typ:      cmdClipAlphaFill,
				x:        xTileRel,
				width:    width,
				alphaIdx: int(col),
				blend:    lastLayer.blend,
				opacity:  lastLayer.opacity,
			}
			x += width
			col += uint32(width)
			ctx.tiles[tileY][xtile].clipAlphaFill(cmd)
			tileX = xtile
			popPending = true
		}

		// XXX add even/odd winding rule support
		if nextStrip.winding != 0 && stripY == nextStrip.stripY() {
			x = max(x1, bbox[0]*wideTileWidth)
			uproundedWidth := divCeil(ctx.width, wideTileWidth) * wideTileWidth
			x2 := min(nextStrip.x, uproundedWidth)
			fxt0 := tileX
			fxt1 := min(divCeil(x2, wideTileWidth), bbox[2])

			for xtile := fxt0; xtile < fxt1; xtile++ {
				if xtile > fxt0 && popPending {
					ctx.tiles[tileY][tileX].popLayer()
					popPending = false
				}
				xTileRel := x % wideTileWidth
				width := min(x2, (xtile+1)*wideTileWidth) - x
				if width == 0 {
					continue
				}
				x += width
				ctx.tiles[tileY][xtile].clipFill(xTileRel, width, lastLayer.blend, lastLayer.opacity)
				tileX = xtile
				popPending = true
			}
		}
	}

	if popPending {
		ctx.tiles[tileY][tileX].popLayer()
		tileX++
		popPending = false
	}

	// TODO(dh): is this condition actually possible? For the bounding box to
	// include bbox[3], at least one strip has to cover it, doesn't it?
	for tileY < bbox[3] {
		for x := tileX; x < bbox[2]; x++ {
			ctx.tiles[tileY][x].popZeroClip()
		}
		tileX = bbox[0]
		tileY++
	}
}

func (ctx *Renderer) popLayers() {
	for ctx.stateStack[len(ctx.stateStack)-1].numLayers > 0 {
		ctx.popLayer()
	}
}

func (ctx *Renderer) SetAffine(aff curve.Affine) {
	ctx.transform = aff
}

func (ctx *Renderer) getAffine() curve.Affine {
	return ctx.transform
}

func (ctx *Renderer) Fill(path iter.Seq[curve.PathElement], fillRule FillRule, color Color) {
	// XXX support brushes
	affine := ctx.getAffine()
	ctx.lineBuf = fill(path, affine, ctx.lineBuf)
	ctx.renderPath(ctx.lineBuf, fillRule, color)
}

func (ctx *Renderer) Stroke(path iter.Seq[curve.PathElement], stroke_ curve.Stroke, color Color) {
	// XXX support brushes
	affine := ctx.getAffine()
	ctx.lineBuf = stroke(path, stroke_, affine, ctx.lineBuf)
	ctx.renderPath(ctx.lineBuf, NonZero, color)
}

type Layer struct {
	BlendMode    BlendMode
	Opacity      float32
	Clip         iter.Seq[curve.PathElement]
	ClipFillRule FillRule
}

func (ctx *Renderer) PushClip(path iter.Seq[curve.PathElement], fill FillRule) {
	ctx.PushLayer(Layer{Opacity: 1, Clip: path, ClipFillRule: fill})
}

func (ctx *Renderer) PushLayer(l Layer) {
	clipPath := l.Clip
	if clipPath == nil {
		// OPT(dh): instead of going through the whole clipping logic (computing
		// and processing strips), we should have a special case for layers
		// without clips that just processes all tiles in the current bounding
		// box.
		clipPath = curve.NewRectFromOrigin(
			curve.Pt(0, 0),
			curve.Sz(float64(ctx.width), float64(ctx.height)),
		).PathElements(0.1)
	}

	affine := ctx.getAffine()
	ctx.lineBuf = fill(clipPath, affine, ctx.lineBuf)
	ctx.renderPathCommon(ctx.lineBuf, l.ClipFillRule)
	strips := ctx.stripBuf
	ctx.stripBuf = nil
	var pathBbox [4]uint16
	if len(strips) > 1 {
		y0 := strips[0].stripY()
		y1 := strips[len(strips)-1].stripY() + 1
		x0 := strips[0].x / wideTileWidth
		x1 := x0
		for i := range len(strips) - 1 {
			strip := &strips[i]
			nextStrip := &strips[i+1]
			width := uint16(nextStrip.col - strip.col)
			x := strip.x
			x0 = min(x0, x/wideTileWidth)
			x1 = max(x1, divCeil(x+width, wideTileWidth))
		}
		pathBbox = [4]uint16{x0, y0, x1, y1}
	}
	parentBbox := ctx.bbox()
	// intersect clip bounding box
	bbox := [4]uint16{
		max(parentBbox[0], pathBbox[0]),
		max(parentBbox[1], pathBbox[1]),
		min(parentBbox[2], pathBbox[2]),
		min(parentBbox[3], pathBbox[3]),
	}

	// The next bit of code accomplishes the following. For each tile in
	// the intersected bounding box, it does one of two things depending
	// on the contents of the clip path in that tile.
	// If all-zero: push a zero_clip
	// If all-ones: push a clip
	// If contains one or more strips: push a clip
	tileX := bbox[0]
	tileY := bbox[1]
	for i := range len(strips) - 1 {
		strip := &strips[i]
		stripY := strip.stripY()
		if stripY < tileY {
			continue
		}
		for tileY < min(stripY, bbox[3]) {
			for x := tileX; x < bbox[2]; x++ {
				ctx.tiles[tileY][x].pushZeroClip()
			}
			tileX = bbox[0]
			tileY++
		}
		if tileY == bbox[3] {
			break
		}
		x0 := strip.x
		xClamped := min(x0/wideTileWidth, bbox[2])
		if tileX < xClamped {
			if strip.winding == 0 {
				for x := tileX; x < xClamped; x++ {
					ctx.tiles[tileY][x].pushZeroClip()
				}
			}
			tileX = xClamped
		}

		nextStrip := &strips[i+1]
		// Push layers for all tiles covered by alpha
		width := uint16(nextStrip.col - strip.col)
		x1 := x0 + width
		xtile1 := min(divCeil(x1, wideTileWidth), bbox[2])
		if tileX < xtile1 {
			for xtile := tileX; xtile < xtile1; xtile++ {
				ctx.tiles[tileY][xtile].pushLayer()
			}
			tileX = xtile1
		}

		// Push layers for all tiles covered by solid fill (except for the one
		// already covered by alpha, if any)
		//
		// XXX support even/odd fill rule
		if nextStrip.winding != 0 && tileY == nextStrip.stripY() {
			x2 := min(divCeil(nextStrip.x, wideTileWidth), bbox[2])
			fxt0 := tileX
			fxt1 := x2
			for xtile := fxt0; xtile < fxt1; xtile++ {
				ctx.tiles[tileY][xtile].pushLayer()
			}
			tileX = fxt1
		}
	}

	// TODO(dh): is this condition actually possible? For the bounding box to
	// include bbox[3], at least one strip has to cover it, doesn't it?
	for tileY < bbox[3] {
		for x := tileX; x < bbox[2]; x++ {
			ctx.tiles[tileY][x].pushZeroClip()
		}
		tileX = bbox[0]
		tileY++
	}

	clip := layer{
		bbox:    bbox,
		strips:  strips,
		opacity: l.Opacity,
		blend:   l.BlendMode,
	}
	ctx.layerStack = append(ctx.layerStack, clip)
	ctx.stateStack[len(ctx.stateStack)-1].numLayers++
}

func (ctx *Renderer) Save() {
	ctx.stateStack = append(ctx.stateStack, gfxState{0})
}

func (ctx *Renderer) Restore() {
	ctx.popLayers()
	ctx.stateStack = ctx.stateStack[:len(ctx.stateStack)-1]
}
