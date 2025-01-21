// SPDX-FileCopyrightText: 2024 the Piet Authors
// SPDX-FileCopyrightText: 2025 Dominik Honnef and contributors
//
// SPDX-License-Identifier: MIT

package sparse

import (
	"image"
	"iter"
	"log"
	"slices"
	"time"

	"honnef.co/go/curve"
	"honnef.co/go/safeish"
)

type CsRenderCtx struct {
	width     int
	height    int
	tiles     []wideTile
	alphas    []uint32
	transform curve.Affine

	// Scratch buffers
	tileBuf  []tile
	stripBuf []strip
}

func NewCsRenderCtx(width, height int) *CsRenderCtx {
	widthTiles := (width + wideTileWidth - 1) / wideTileWidth
	heightTiles := (height + stripHeight - 1) / stripHeight
	tiles := make([]wideTile, widthTiles*heightTiles)

	return &CsRenderCtx{
		width:     width,
		height:    height,
		tiles:     tiles,
		transform: curve.Identity,
	}
}

func (ctx *CsRenderCtx) Reset() {
	for i := range ctx.tiles {
		tile := &ctx.tiles[i]
		tile.bg = [4]float32{}
		clear(tile.cmds)
		tile.cmds = tile.cmds[:0]
	}

	ctx.alphas = ctx.alphas[:0]
}

func (ctx *CsRenderCtx) RenderToPixmap(pixmap *image.RGBA) {
	fine := fine{
		width:  pixmap.Bounds().Dx(),
		height: pixmap.Bounds().Dy(),
		outBuf: safeish.SliceCast[[][4]byte](pixmap.Pix),
	}
	widthTiles := (ctx.width + wideTileWidth - 1) / wideTileWidth
	heightTiles := (ctx.height + stripHeight - 1) / stripHeight
	for y := range heightTiles {
		for x := range widthTiles {
			tile := &ctx.tiles[y*widthTiles+x]
			fine.clear(tile.bg)
			for i := range tile.cmds {
				cmd := tile.cmds[i]
				fine.runCmd(cmd, ctx.alphas)
			}
			fine.pack(x, y)
		}
	}
}

func (ctx *CsRenderCtx) renderPath(path iter.Seq[flatLine], color [4]float32) {
	// XXX support a brush

	// TODO: need to make sure tiles contained in viewport - we'll likely
	// panic otherwise.
	t1 := time.Now()
	ctx.tileBuf = makeTiles(path, ctx.tileBuf)
	t2 := time.Now()
	slices.SortFunc(ctx.tileBuf, tile.cmp)
	t3 := time.Now()
	ctx.stripBuf, ctx.alphas = renderStripsScalar(ctx.tileBuf, ctx.stripBuf, ctx.alphas)
	t4 := time.Now()
	widthTiles := (ctx.width + wideTileWidth - 1) / wideTileWidth
	// XXX can this be a range over ctx.stripBuf or does its length change during the loop?
	for i := range ctx.stripBuf {
		strip := &ctx.stripBuf[i]

		// Don't render strips that are outside the viewport vertically.
		if int(strip.y()) >= ctx.height {
			break
		}

		nextStrip := &ctx.stripBuf[i+1]
		x0 := strip.x()
		y := strip.stripY()
		rowStart := int(y) * widthTiles
		stripWidth := nextStrip.col - strip.col
		x1 := x0 + stripWidth
		xtile0 := int(x0) / wideTileWidth
		xtile1 := (int(x1) + wideTileWidth - 1) / wideTileWidth
		x := x0
		col := strip.col
		for xtile := xtile0; xtile < xtile1; xtile++ {
			xTileRel := x % wideTileWidth
			width := min(x1, uint32((xtile+1)*wideTileWidth)) - x
			cmd := cmd{
				typ:      cmdStrip,
				x:        xTileRel,
				width:    width,
				alphaIdx: int(col),
				color:    color,
			}
			x += width
			col += width
			ctx.tiles[rowStart+xtile].push(cmd)
		}
		if nextStrip.winding != 0 && y == nextStrip.stripY() {
			x = x1
			x2 := nextStrip.x()
			fxt0 := x1 / wideTileWidth
			fxt1 := (x2 + wideTileWidth - 1) / wideTileWidth
			for xtile := fxt0; xtile < fxt1; xtile++ {
				xTileRel := x % wideTileWidth
				width := min(x2, ((xtile+1)*wideTileWidth)) - x
				x += width
				ctx.tiles[rowStart+int(xtile)].fill(xTileRel, width, color)
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

func (ctx *CsRenderCtx) Fill(path iter.Seq[curve.PathElement], color [4]float32) {
	// XXX support brushes
	affine := ctx.getAffine()
	it := fill(path, affine)
	ctx.renderPath(it, color)
}

func (ctx *CsRenderCtx) Stroke(path iter.Seq[curve.PathElement], stroke_ curve.Stroke, color [4]float32) {
	// XXX support brushes
	affine := ctx.getAffine()
	it := stroke(path, stroke_, affine)
	ctx.renderPath(it, color)
}
