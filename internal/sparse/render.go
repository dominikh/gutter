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
	"time"

	"honnef.co/go/curve"
)

type FillRule int

const (
	NonZero FillRule = iota
	EvenOdd
)

type Color [4]float32

type gfxState struct {
	numClips int
}

type clip struct {
	// The intersected bounding box after clip
	bbox [4]uint16
	// The rendered path in sparse strip representation
	strips []strip
}

type CsRenderCtx struct {
	width  uint16
	height uint16
	// [y][x]wideTile
	tiles [][]wideTile
	// [sparse column][y]uint8
	alphas     [][stripHeight]uint8
	transform  curve.Affine
	stateStack []gfxState
	clipStack  []clip

	// Scratch buffers
	tileBuf  []tile
	stripBuf []strip
	lineBuf  []flatLine
}

func NewCsRenderCtx(width, height uint16) *CsRenderCtx {
	widthTiles := divCeil(width, wideTileWidth)
	heightTiles := divCeil(height, stripHeight)
	tiles := make([][]wideTile, heightTiles)
	for i := range tiles {
		tiles[i] = make([]wideTile, widthTiles)
	}

	return &CsRenderCtx{
		width:      width,
		height:     height,
		tiles:      tiles,
		transform:  curve.Identity,
		stateStack: []gfxState{{0}},
	}
}

func (ctx *CsRenderCtx) Reset() {
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
// At the moment, this mostly involves resolving any open clips, but
// might extend to other things.
func (ctx *CsRenderCtx) finish() {
	ctx.popClips()
}

func (ctx *CsRenderCtx) RenderToPixmap(width, height uint16, pixmap []Color) {
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
			if len(fine.layers) != 1 {
				panic("internal error: left with more than one layer")
			}
			fine.pack(uint16(x), uint16(y))
		}
	}
	if false {
		log.Println(&fine.stats)
	}
}

func (ctx *CsRenderCtx) renderPathCommon(lineBuf []flatLine, fillRule FillRule) {
	ctx.tileBuf = makeTiles(lineBuf, ctx.tileBuf, uint16(ctx.width), uint16(ctx.height))
	slices.SortFunc(ctx.tileBuf, tile.cmp)
	t := time.Now()
	ctx.stripBuf, ctx.alphas = renderStripsScalar(ctx.tileBuf, fillRule, ctx.lineBuf, ctx.stripBuf, ctx.alphas)
	if false {
		log.Println("renderPathCommon:", time.Since(t))
	}
}

func (ctx *CsRenderCtx) renderPath(lineBuf []flatLine, fillRule FillRule, color Color) {
	// XXX support a brush

	ctx.renderPathCommon(lineBuf, fillRule)

	widthTiles := divCeil(ctx.width, wideTileWidth)
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
		// TODO: we are limiting xtile1 to widthTiles because strips aren't
		// being clipped to the viewport yet. Evaluate removing this once we
		// clip higher up the stack.
		xtile1 := min(divCeil(x1, wideTileWidth), widthTiles, bbox[2])
		x := x0
		if bbox[0]*wideTileWidth > x {
			col += uint32(bbox[0]*wideTileWidth - x)
			x = bbox[0] * wideTileWidth
		}
		for xtile := xtile0; xtile < xtile1; xtile++ {
			xTileRel := x % wideTileWidth
			lhs := min(x1, (uint16(xtile+1) * wideTileWidth))
			if lhs < x {
				panic(fmt.Sprintf("internal error: %v < %v", lhs, x))
			}
			width := lhs - x
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

		if activeFill && stripY == nextStrip.stripY() && nextStrip.x >= 0 {
			x = x1
			uproundedWidth := divCeil(ctx.width, wideTileWidth) * wideTileWidth
			x2 := min(nextStrip.x, uproundedWidth)
			fxt0 := max(x1/wideTileWidth, bbox[0])
			fxt1 := min(divCeil(x2, wideTileWidth), widthTiles, bbox[2])
			for xtile := fxt0; xtile < fxt1; xtile++ {
				xTileRel := x % wideTileWidth
				width := min(x2, (xtile+1)*wideTileWidth) - x
				x += width
				ctx.tiles[stripY][xtile].fill(xTileRel, width, color)
			}
		}
	}
}

func (ctx *CsRenderCtx) bbox() [4]uint16 {
	if len(ctx.clipStack) > 0 {
		return ctx.clipStack[len(ctx.clipStack)-1].bbox
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

func (ctx *CsRenderCtx) popClip() {
	ctx.stateStack[len(ctx.stateStack)-1].numClips--
	lastClip := ctx.clipStack[len(ctx.clipStack)-1]
	ctx.clipStack = ctx.clipStack[:len(ctx.clipStack)-1]
	clipBbox := lastClip.bbox
	strips := lastClip.strips

	// The next bit of code accomplishes the following. For each tile in
	// the intersected bounding box, it does one of three things depending
	// on the contents of the clip path in that tile.
	// If all-zero: pop a zero_clip.
	// If all-one: do nothing.
	// If contains one or more strips: render strips and fills, then pop a clip.
	// This logic is the inverse of the push logic in `clip()`, and the stack
	// should be balanced after running both.
	tileX := clipBbox[0]
	tileY := clipBbox[1]
	popPending := false
	for i := range len(strips) - 1 {
		strip := &strips[i]
		y := strip.stripY()
		if y < tileY {
			continue
		}
		for tileY < min(y, clipBbox[3]) {
			if popPending {
				popPending = false
				ctx.tiles[tileY][tileX].popClip()
				tileX++
			}
			for x := tileX; x < clipBbox[2]; x++ {
				ctx.tiles[tileY][x].popZeroClip()
			}
			tileX = clipBbox[0]
			tileY++
		}
		if tileY == clipBbox[3] {
			break
		}
		x0 := strip.x
		xClamped := min(x0/wideTileWidth, clipBbox[2])
		if tileX < xClamped {
			if popPending {
				popPending = false
				ctx.tiles[tileY][tileX].popClip()
				tileX++
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
		stripWidth := uint16(nextStrip.col - strip.col)
		x1 := x0 + stripWidth
		xtile0 := max(x0/wideTileWidth, clipBbox[0])
		xtile1 := min(divCeil(x1, wideTileWidth), clipBbox[2])
		x := x0
		alphaIdx := int(strip.col)
		if clipBbox[0]*wideTileWidth > x {
			alphaIdx += int(clipBbox[0]*wideTileWidth - x)
			x = clipBbox[0] * wideTileWidth
		}
		for xtile := xtile0; xtile < xtile1; xtile++ {
			if xtile > tileX && popPending {
				popPending = false
				ctx.tiles[tileY][tileX].popClip()
			}
			xTileRel := x % wideTileWidth
			width := min(x1, (xtile+1)*wideTileWidth) - x
			cmd := cmd{
				typ:      cmdClipAlphaFill,
				x:        xTileRel,
				width:    width,
				alphaIdx: alphaIdx,
			}
			x += width
			alphaIdx += int(width)
			ctx.tiles[tileY][xtile].clipAlphaFill(cmd)
			tileX = xtile
			popPending = true
		}
		if nextStrip.winding != 0 && y == nextStrip.stripY() {
			x2 := nextStrip.x
			tileX2 := min(x2, (tileX+1)*wideTileWidth)
			width := tileX2 - x1
			if width > 0 {
				xTileRel := x1 % wideTileWidth
				ctx.tiles[tileY][tileX].clipFill(xTileRel, width)
			}
			if x2 > (tileX+1)*wideTileWidth && x2 < ctx.width {
				ctx.tiles[tileY][tileX].popClip()
				width2 := x2 % wideTileWidth
				tileX = x2 / wideTileWidth
				if width2 > 0 {
					ctx.tiles[tileY][tileX].clipFill(0, width2)
				}
			}
		}
	}
	if popPending {
		popPending = false
		ctx.tiles[tileY][tileX].popClip()
		tileX++
	}
	for tileY < clipBbox[3] {
		for x := tileX; x < clipBbox[2]; x++ {
			ctx.tiles[tileY][x].popZeroClip()
		}
		tileX = clipBbox[0]
		tileY++
	}
}

func (ctx *CsRenderCtx) popClips() {
	for ctx.stateStack[len(ctx.stateStack)-1].numClips > 0 {
		ctx.popClip()
	}
}

func (ctx *CsRenderCtx) SetAffine(aff curve.Affine) {
	ctx.transform = aff
}

func (ctx *CsRenderCtx) getAffine() curve.Affine {
	return ctx.transform
}

func (ctx *CsRenderCtx) Fill(path iter.Seq[curve.PathElement], fillRule FillRule, color Color) {
	// XXX support brushes
	affine := ctx.getAffine()
	ctx.lineBuf = fill(path, affine, ctx.lineBuf)
	ctx.renderPath(ctx.lineBuf, fillRule, color)
}

func (ctx *CsRenderCtx) Stroke(path iter.Seq[curve.PathElement], stroke_ curve.Stroke, color Color) {
	// XXX support brushes
	affine := ctx.getAffine()
	ctx.lineBuf = stroke(path, stroke_, affine, ctx.lineBuf)
	ctx.renderPath(ctx.lineBuf, NonZero, color)
}

func (ctx *CsRenderCtx) Clip(path iter.Seq[curve.PathElement], fillRule FillRule) {
	affine := ctx.getAffine()
	ctx.lineBuf = fill(path, affine, ctx.lineBuf)
	ctx.renderPathCommon(ctx.lineBuf, fillRule)
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
	clipBbox := [4]uint16{
		max(parentBbox[0], pathBbox[0]),
		max(parentBbox[1], pathBbox[1]),
		min(parentBbox[2], pathBbox[2]),
		min(parentBbox[3], pathBbox[3]),
	}
	// The next bit of code accomplishes the following. For each tile in
	// the intersected bounding box, it does one of three things depending
	// on the contents of the clip path in that tile.
	// If all-zero: push a zero_clip
	// If all-one: do nothing
	// If contains one or more strips: push a clip
	tileX := clipBbox[0]
	tileY := clipBbox[1]
	for i := range len(strips) - 1 {
		strip := &strips[i]
		y := strip.stripY()
		if y < tileY {
			continue
		}
		for tileY < min(y, clipBbox[3]) {
			for x := tileX; x < clipBbox[2]; x++ {
				ctx.tiles[tileY][x].pushZeroClip()
			}
			tileX = clipBbox[0]
			tileY++
		}
		if tileY == clipBbox[3] {
			break
		}
		xPixels := strip.x
		xClamped := min(xPixels/wideTileWidth, clipBbox[2])
		if tileX < xClamped {
			if strip.winding == 0 {
				for x := tileX; x < xClamped; x++ {
					ctx.tiles[tileY][x].pushZeroClip()
				}
			}
			// If winding is nonzero, then wide tiles covered entirely
			// by sparse fill are no-op (no clipping is applied).
			tileX = xClamped
		}
		nextStrip := &strips[i+1]
		width := uint16(nextStrip.col - strip.col)
		x1 := min(divCeil(xPixels+width, wideTileWidth), clipBbox[2])
		if tileX < x1 {
			for x := tileX; x < x1; x++ {
				ctx.tiles[tileY][x].pushClip()
			}
			tileX = x1
		}
	}
	for tileY < clipBbox[3] {
		for x := tileX; x < clipBbox[2]; x++ {
			ctx.tiles[tileY][x].pushZeroClip()
		}
		tileX = clipBbox[0]
		tileY++
	}
	clip := clip{
		bbox:   clipBbox,
		strips: strips,
	}
	ctx.clipStack = append(ctx.clipStack, clip)
	ctx.stateStack[len(ctx.stateStack)-1].numClips++
}

func (ctx *CsRenderCtx) Save() {
	ctx.stateStack = append(ctx.stateStack, gfxState{0})
}

func (ctx *CsRenderCtx) Restore() {
	ctx.popClips()
	ctx.stateStack = ctx.stateStack[:len(ctx.stateStack)-1]
}
