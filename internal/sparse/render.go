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

type CsRenderCtx struct {
	width  int
	height int
	// [y][x]wideTile
	tiles [][]wideTile
	// [sparse column][y]uint8
	alphas    [][stripHeight]uint8
	transform curve.Affine

	// Scratch buffers
	tileBuf  []tile
	stripBuf []strip
}

func NewCsRenderCtx(width, height int) *CsRenderCtx {
	widthTiles := (width + wideTileWidth - 1) / wideTileWidth
	heightTiles := (height + stripHeight - 1) / stripHeight
	tiles := make([][]wideTile, heightTiles)
	for i := range tiles {
		tiles[i] = make([]wideTile, widthTiles)
	}

	return &CsRenderCtx{
		width:     width,
		height:    height,
		tiles:     tiles,
		transform: curve.Identity,
	}
}

func (ctx *CsRenderCtx) Reset() {
	for _, row := range ctx.tiles {
		for x := range row {
			tile := &row[x]
			tile.bg = [4]float32{}
			clear(tile.cmds)
			tile.cmds = tile.cmds[:0]
		}
	}

	ctx.alphas = ctx.alphas[:0]
}

func (ctx *CsRenderCtx) RenderToPixmap(width, height int, pixmap [][4]float32) {
	fine := newFine(width, height, pixmap)
	for y, row := range ctx.tiles {
		for x := range row {
			tile := &row[x]
			fine.clear(tile.bg)
			for i := range tile.cmds {
				cmd := tile.cmds[i]
				fine.runCmd(cmd, ctx.alphas)
			}
			fine.pack(x, y)
		}
	}
}

func (ctx *CsRenderCtx) renderPath(path iter.Seq[flatLine], fillRule FillRule, color [4]float32) {
	// XXX support a brush

	t1 := time.Now()
	ctx.tileBuf = makeTiles(path, ctx.tileBuf)

	t2 := time.Now()
	slices.SortFunc(ctx.tileBuf, tile.cmp)

	t3 := time.Now()
	ctx.stripBuf, ctx.alphas = renderStripsScalar(ctx.tileBuf, fillRule, ctx.stripBuf, ctx.alphas)

	t4 := time.Now()
	widthTiles := (ctx.width + wideTileWidth - 1) / wideTileWidth
	for i := range len(ctx.stripBuf) - 1 {
		strip := &ctx.stripBuf[i]

		// Don't render strips that are outside the viewport.
		if int(strip.x()) >= ctx.width {
			continue
		}
		if int(strip.y()) >= ctx.height {
			break
		}

		nextStrip := &ctx.stripBuf[i+1]
		x0 := strip.x()
		y := strip.stripY()
		stripWidth := nextStrip.col - strip.col
		x1 := x0 + stripWidth
		xtile0 := int(x0) / wideTileWidth
		// TODO: we are limiting xtile1 to widthTiles because strips aren't
		// being clipped to the viewport yet. Evaluate removing this once we
		// clip higher up the stack.
		xtile1 := min((int(x1)+wideTileWidth-1)/wideTileWidth, widthTiles)
		x := x0
		col := strip.col
		for xtile := xtile0; xtile < xtile1; xtile++ {
			xTileRel := x % wideTileWidth
			lhs := min(x1, uint32((xtile+1)*wideTileWidth))
			if lhs < x {
				panic(fmt.Sprintf("internal error: %v < %v", lhs, x))
			}
			width := lhs - x
			cmd := cmd{
				typ:      cmdStrip,
				x:        xTileRel,
				width:    width,
				alphaIdx: int(col),
				color:    color,
			}
			x += width
			col += width
			ctx.tiles[y][xtile].push(cmd)
		}
		if nextStrip.winding != 0 && y == nextStrip.stripY() {
			x = x1
			x2 := nextStrip.x()
			fxt0 := x1 / wideTileWidth
			// TODO: we are limiting fxt1 to widthTiles because strips aren't
			// being clipped to the viewport yet. Evaluate removing this once we
			// clip higher up the stack.
			fxt1 := min((x2+wideTileWidth-1)/wideTileWidth, uint32(widthTiles))
			for xtile := fxt0; xtile < fxt1; xtile++ {
				xTileRel := x % wideTileWidth
				width := min(x2, ((xtile+1)*wideTileWidth)) - x
				x += width
				ctx.tiles[y][xtile].fill(xTileRel, width, color)
			}
		}
	}
	t5 := time.Now()

	if false {
		log.Printf("make tiles: %s (%d); sort: %s; render strips: %s; make wide tiles: %s",
			t2.Sub(t1), len(ctx.tileBuf), t3.Sub(t2), t4.Sub(t3), t5.Sub(t4))
	}
}

func (ctx *CsRenderCtx) SetAffine(aff curve.Affine) {
	ctx.transform = aff
}

func (ctx *CsRenderCtx) getAffine() curve.Affine {
	return ctx.transform
}

func (ctx *CsRenderCtx) Fill(path iter.Seq[curve.PathElement], fillRule FillRule, color [4]float32) {
	// XXX support brushes
	affine := ctx.getAffine()
	it := fill(path, affine)
	ctx.renderPath(it, fillRule, color)
}

func (ctx *CsRenderCtx) Stroke(path iter.Seq[curve.PathElement], stroke_ curve.Stroke, color [4]float32) {
	// XXX support brushes
	affine := ctx.getAffine()
	it := stroke(path, stroke_, affine)
	ctx.renderPath(it, NonZero, color)
}
