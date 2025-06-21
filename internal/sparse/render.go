// SPDX-FileCopyrightText: 2024 the Piet Authors
// SPDX-FileCopyrightText: 2025 Dominik Honnef and contributors
//
// SPDX-License-Identifier: MIT

package sparse

import (
	"fmt"
	"log"
	"runtime"
	"slices"

	"honnef.co/go/curve"
	"honnef.co/go/gutter/gfx"
	"honnef.co/go/gutter/maybe"
)

type gfxState struct {
	numLayers int
}

type tileCoord struct {
	tileX, tileY uint16
}

type tileBbox struct {
	tileMin, tileMax tileCoord
}

func (bbox tileBbox) intersect(other tileBbox) tileBbox {
	return tileBbox{
		tileMin: tileCoord{
			max(bbox.tileMin.tileX, other.tileMin.tileX),
			max(bbox.tileMin.tileY, other.tileMin.tileY),
		},
		tileMax: tileCoord{
			min(bbox.tileMax.tileX, other.tileMax.tileX),
			min(bbox.tileMax.tileY, other.tileMax.tileY),
		},
	}
}

type layer struct {
	// The intersected bounding box after clip
	bbox tileBbox
	// The rendered path in sparse strip representation
	strips       []strip
	alphas       [][stripHeight]uint8
	opacity      float32
	blend        gfx.BlendMode
	copyBackdrop bool
	nonempty     bool
	blackholed   int
	noclip       bool
}

type Renderer struct {
	width  uint16
	height uint16
	// [y][x]wideTile
	tiles      [][]wideTile
	stateStack []gfxState
	layerStack []layer

	cmds []cmd
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
		stateStack: []gfxState{{0}},
		layerStack: []layer{
			{
				bbox: tileBbox{
					tileMin: tileCoord{0, 0},
					tileMax: tileCoord{widthTiles, heightTiles},
				},
				opacity: 1,
			},
		},
		cmds: []cmd{
			0: {},
			1: {typ: cmdPopLayer},
			2: {typ: cmdPushLayer},
			3: {typ: cmdCopyBackdrop},
		},
	}
}

func optimizeRecording(cmds gfx.Recording) {
	type layer struct {
		push               int
		needBackdrop       bool
		childNeedsBackdrop bool
	}

	const debug = false

	if debug {
		log.Println("before:")
		for _, cmd := range cmds {
			log.Printf("%T", cmd)
			if cmd, ok := cmd.(gfx.CommandPushLayer); ok {
				log.Println(cmd.Layer.BlendMode, cmd.Layer.Clip == nil, cmd.Layer.Opacity)
			}
		}
		log.Println()
	}

	var layers []layer
	for i, cmd := range cmds {
		switch cmd := cmd.(type) {
		case gfx.CommandFill:
		case gfx.CommandPlayRecording:
		case gfx.CommandPopLayer:
			l := layers[len(layers)-1]
			if !l.needBackdrop && !l.childNeedsBackdrop {
				cmds[i] = nil
				cmds[l.push] = nil
			}
			layers = layers[:len(layers)-1]
		case gfx.CommandPushLayer:
			// TODO(dh): investigate if there are more than the default blend
			// mode that don't care about the backdrop as such.
			l := layer{
				push: i,
				needBackdrop: cmd.Layer.BlendMode != gfx.BlendMode{} ||
					cmd.Layer.Opacity != 1 ||
					cmd.Layer.Clip != nil,
			}
			if len(layers) > 0 && cmd.Layer.BlendMode != (gfx.BlendMode{}) {
				layers[len(layers)-1].childNeedsBackdrop = true
			}
			layers = append(layers, l)
		case gfx.CommandStroke:
		default:
			panic(fmt.Sprintf("unexpected gfx.Command: %#v", cmd))
		}
	}

	if debug {
		log.Println("after:")
		for _, cmd := range cmds {
			log.Printf("%T", cmd)
		}
		log.Println()
	}
}

func PlayRecording(cmds gfx.Recording, r *Renderer, aff curve.Affine) {
	optimizeRecording(cmds)

	// OPT parallelism

	for _, cmd := range cmds {
		switch cmd := cmd.(type) {
		case gfx.CommandFill:
			r.Fill(cmd.Shape, aff.Mul(cmd.Transform), cmd.FillRule, cmd.Paint)
		case gfx.CommandPopLayer:
			r.Restore()
		case gfx.CommandPushLayer:
			r.Save()
			r.PushLayer(Layer{
				BlendMode:     cmd.Layer.BlendMode,
				Opacity:       cmd.Layer.Opacity,
				Clip:          cmd.Layer.Clip,
				ClipTransform: aff.Mul(cmd.Transform),
				ClipFillRule:  cmd.FillRule,
			})
		case gfx.CommandStroke:
			r.Stroke(cmd.Shape, aff.Mul(cmd.Transform), cmd.Stroke, cmd.Paint)
		case gfx.CommandPlayRecording:
			PlayRecording(cmd.Recording, r, aff.Mul(cmd.Transform))
		case nil:
		default:
			panic(fmt.Sprintf("unexpected Command: %#v", cmd))
		}
	}
}

func (ctx *Renderer) Width() uint16  { return ctx.width }
func (ctx *Renderer) Height() uint16 { return ctx.height }

func (ctx *Renderer) Reset() {
	for _, row := range ctx.tiles {
		for x := range row {
			tile := &row[x]
			tile.bg = gfx.PlainColor{}
			clear(tile.cmds)
			tile.cmds = tile.cmds[:0]
		}
	}
	clear(ctx.cmds)
	ctx.cmds = ctx.cmds[:4]
	ctx.cmds[0] = cmd{}
	ctx.cmds[1] = cmd{typ: cmdPopLayer}
	ctx.cmds[2] = cmd{typ: cmdPushLayer}
	ctx.cmds[3] = cmd{typ: cmdCopyBackdrop}
}

// Finish the coarse rasterization prior to fine rendering.
//
// At the moment, this mostly involves resolving any open layers, but
// might extend to other things.
func (ctx *Renderer) finish() {
	ctx.popLayers()
}

func (ctx *Renderer) Render(width, height uint16, packer Packer) {
	ctx.finish()

	distribute(ctx.tiles, runtime.GOMAXPROCS(0), func(group int, step int, subitems [][]wideTile) error {
		stackScratch := make([]optLayer, 0, 32)
		fine := newFine(width, height, packer)
		for y, row := range subitems {
			y += group * step
			for x := range row {
				tile := &row[x]
				fine.setTile(uint16(x), uint16(y))
				fine.topLayer().clear(tile.bg)

				if false && len(tile.cmds) > 0 {
					log.Println("tile", x, y)
					for i := range tile.cmds {
						log.Println(&tile.cmds[i])
					}
					log.Println()
				}

				// log.Println(x, y)
				newCmdIdxs, newStackScratch := optimizeCommands(ctx.cmds, tile.cmds, stackScratch[:0])
				tile.cmds = newCmdIdxs
				stackScratch = newStackScratch

				for _, cmdIdx := range tile.cmds {
					fine.runCmd(ctx.cmds[cmdIdx])
				}
				switch len(fine.layers) {
				case 0:
					panic("internal error: left with no layers")
				case 1:
				default:
					panic("internal error: left with more than one layer")
				}
				fine.pack()
			}
		}
		return nil
	})
	if false {
		// log.Println(&fine.stats)
	}
}

func renderPathCommon(lineBuf []flatLine, fillRule gfx.FillRule, width, height uint16) ([]strip, [][stripHeight]uint8) {
	tileBuf := makeTiles(lineBuf, width, height)
	slices.Sort(tileBuf)
	stripBuf, alphas := renderStripsScalar(tileBuf, fillRule, lineBuf)
	return stripBuf, alphas
}

type CompiledPath struct {
	strips   []strip
	alphas   [][stripHeight]uint8
	fillRule gfx.FillRule
}

func CompileFillPath(
	shape gfx.Shape,
	affine curve.Affine,
	fillRule gfx.FillRule,
	width uint16,
	height uint16,
) CompiledPath {
	// The transformation mustn't skew the shape for our optimizations to apply.
	if affine.N1 == 0 && affine.N2 == 0 {
		switch shape := shape.(type) {
		case curve.Rect:
			// OPT(dh): all rectangles of the same size that fall on integer
			// coordinates are the same, especially if their Y coordinates only
			// differ in multiples of the strip height.

			a, d, e, f := affine.N0, affine.N3, affine.N4, affine.N5
			shape = curve.Rect{
				X0: shape.X0*a + e,
				Y0: shape.Y0*d + f,
				X1: shape.X1*a + e,
				Y1: shape.Y1*d + f,
			}

			strips, alphas := renderRect(shape, width, height)
			return CompiledPath{
				strips:   strips,
				alphas:   alphas,
				fillRule: gfx.NonZero,
			}
		}
	}

	// TODO(dh): scale precision based on transformation
	lines := fill(shape.PathElements(0.1), affine)
	strips, alphas := renderPathCommon(lines, fillRule, width, height)
	return CompiledPath{strips, alphas, fillRule}
}

func CompileStrokedPath(
	shape gfx.Shape,
	affine curve.Affine,
	stroke_ curve.Stroke,
	width uint16,
	height uint16,
) CompiledPath {
	// TODO(dh): scale precision based on transformation
	path := shape.PathElements(0.1)
	lines := stroke(path, stroke_, affine)
	strips, alphas := renderPathCommon(lines, gfx.NonZero, width, height)
	return CompiledPath{strips, alphas, gfx.NonZero}
}

func (ctx *Renderer) renderPath(p CompiledPath, paint gfx.EncodedPaint) {
	topLayer := &ctx.layerStack[len(ctx.layerStack)-1]
	if topLayer.blackholed > 0 {
		return
	}

	topLayer.nonempty = true

	stripBuf := p.strips
	alphas := p.alphas

	bbox := ctx.bbox()
	for i := range len(stripBuf) - 1 {
		strip := &stripBuf[i]

		if strip.x >= ctx.width {
			// Don't render strips that are outside the viewport.
			continue
		}

		nextStrip := &stripBuf[i+1]
		x0 := strip.x
		stripY := strip.stripY()
		if stripY < bbox.tileMin.tileY {
			continue
		}
		if stripY >= bbox.tileMax.tileY {
			break
		}
		col := strip.col
		// Can potentially be 0, if the next strip's x values is also < 0.
		var stripWidth uint16
		if v := nextStrip.col - col; v <= nextStrip.col {
			stripWidth = uint16(v)
		}
		x1 := x0 + stripWidth
		xtile0 := max(x0/wideTileWidth, bbox.tileMin.tileX)
		xtile1 := min(divCeil(x1, wideTileWidth), bbox.tileMax.tileX)
		x := x0
		if bbox.tileMin.tileX*wideTileWidth > x {
			col += uint32(bbox.tileMin.tileX*wideTileWidth - x)
			x = bbox.tileMin.tileX * wideTileWidth
		}
		for xtile := xtile0; xtile < xtile1; xtile++ {
			xTileRel := x % wideTileWidth
			width := min(x1, (xtile+1)*wideTileWidth) - x
			c := cmd{
				typ:    cmdAlphaFill,
				x:      xTileRel,
				width:  width,
				paint:  paint,
				alphas: alphas[col:],
			}
			if width <= alphaValueCutoff {
				allOne := true
				for _, a := range c.alphas[:c.width] {
					if a != [4]uint8{255, 255, 255, 255} {
						allOne = false
						break
					}
				}
				if allOne {
					if c.paint.Opaque() {
						c.typ = cmdClear
					} else {
						c.typ = cmdFill
					}
				}
			}
			x += width
			col += uint32(width)
			ctx.cmds = ctx.tiles[stripY][xtile].alphaFill(ctx.cmds, c)
		}

		var activeFill bool
		switch p.fillRule {
		case gfx.NonZero:
			activeFill = nextStrip.winding != 0
		case gfx.EvenOdd:
			activeFill = nextStrip.winding%2 != 0
		default:
			panic(fmt.Sprintf("unexpected sparse.FillRule: %#v", p.fillRule))
		}

		if activeFill && stripY == nextStrip.stripY() {
			x = max(x1, bbox.tileMin.tileX*wideTileWidth)
			uproundedWidth := divCeil(ctx.width, wideTileWidth) * wideTileWidth
			x2 := min(nextStrip.x, uproundedWidth)
			fxt0 := max(x1/wideTileWidth, bbox.tileMin.tileX)
			fxt1 := min(divCeil(x2, wideTileWidth), bbox.tileMax.tileX)
			for xtile := fxt0; xtile < fxt1; xtile++ {
				xTileRel := x % wideTileWidth
				width := min(x2, (xtile+1)*wideTileWidth) - x
				x += width
				ctx.cmds = ctx.tiles[stripY][xtile].fill(ctx.cmds, xTileRel, width, paint)
			}
		}
	}
}

func (ctx *Renderer) bbox() tileBbox {
	if len(ctx.layerStack) > 0 {
		return ctx.layerStack[len(ctx.layerStack)-1].bbox
	} else {
		widthTiles := divCeil(ctx.width, wideTileWidth)
		heightTiles := divCeil(ctx.height, stripHeight)
		return tileBbox{
			tileMin: tileCoord{0, 0},
			tileMax: tileCoord{widthTiles, heightTiles},
		}
	}
}

func (ctx *Renderer) popLayer() {
	const popLayerCmdIdx = 1

	lastLayer := &ctx.layerStack[len(ctx.layerStack)-1]
	if lastLayer.blackholed > 0 {
		lastLayer.blackholed--
		return
	}

	ctx.stateStack[len(ctx.stateStack)-1].numLayers--
	ctx.layerStack = ctx.layerStack[:len(ctx.layerStack)-1]
	bbox := lastLayer.bbox

	if lastLayer.noclip {
		// The layer didn't add a new clip, so we can just blend and pop all
		// layers in the bounding box. It doesn't matter that we might blend
		// pixels that lie outside the clip, the parent layer with the actual
		// clip on it will make sure to discard those pixels.
		for tileY := bbox.tileMin.tileY; tileY < bbox.tileMax.tileY; tileY++ {
			for tileX := bbox.tileMin.tileX; tileX < bbox.tileMax.tileX; tileX++ {
				ctx.cmds = ctx.tiles[tileY][tileX].blend(
					ctx.cmds,
					0,
					wideTileWidth,
					lastLayer.blend,
					lastLayer.opacity,
				)
				ctx.tiles[tileY][tileX].popLayer(ctx.cmds, popLayerCmdIdx)
			}
		}
		return
	}

	strips := lastLayer.strips
	alphas := lastLayer.alphas

	// The next bit of code accomplishes the following. For each tile in
	// the intersected bounding box, it does one of two things depending
	// on the contents of the clip path in that tile.
	// If all-zero: pop a zero_clip.
	// If contains one or more strips: render strips and fills, then pop a clip.
	// This logic is the inverse of the push logic in `clip()`, and the stack
	// should be balanced after running both.
	tileX := bbox.tileMin.tileX
	tileY := bbox.tileMin.tileY
	popPending := false
	for i := range len(strips) - 1 {
		strip := &strips[i]
		stripY := strip.stripY()
		if stripY < tileY {
			continue
		}
		for tileY < min(stripY, bbox.tileMax.tileY) {
			if popPending {
				ctx.tiles[tileY][tileX].popLayer(ctx.cmds, popLayerCmdIdx)
				tileX++
				popPending = false
			}
			for x := tileX; x < bbox.tileMax.tileX; x++ {
				ctx.tiles[tileY][x].popZeroClip()
			}
			tileX = bbox.tileMin.tileX
			tileY++
		}
		if tileY == bbox.tileMax.tileY {
			break
		}
		x0 := strip.x
		xClamped := min(x0/wideTileWidth, bbox.tileMax.tileX)
		if tileX < xClamped {
			if popPending {
				ctx.tiles[tileY][tileX].popLayer(ctx.cmds, popLayerCmdIdx)
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
		xtile1 := min(divCeil(x1, wideTileWidth), bbox.tileMax.tileX)
		x := x0
		col := strip.col
		if bbox.tileMin.tileX*wideTileWidth > x {
			col += uint32(bbox.tileMin.tileX*wideTileWidth - x)
			x = bbox.tileMin.tileX * wideTileWidth
		}
		for xtile := tileX; xtile < xtile1; xtile++ {
			if xtile > tileX && popPending {
				ctx.tiles[tileY][tileX].popLayer(ctx.cmds, popLayerCmdIdx)
				popPending = false
			}
			xTileRel := x % wideTileWidth
			width := min(x1, (xtile+1)*wideTileWidth) - x
			allOne := false
			if width <= alphaValueCutoff {
				allOne = true
				for _, a := range alphas[col:][:width] {
					if a != [4]uint8{255, 255, 255, 255} {
						allOne = false
						break
					}
				}
			}
			if allOne {
				ctx.cmds = ctx.tiles[tileY][xtile].blend(ctx.cmds, xTileRel, width, lastLayer.blend, lastLayer.opacity)
			} else {
				cmd := cmd{
					typ:     cmdAlphaBlend,
					x:       xTileRel,
					width:   width,
					blend:   lastLayer.blend,
					opacity: lastLayer.opacity,
					alphas:  alphas[col:],
				}
				ctx.cmds = append(ctx.cmds, cmd)
				ctx.tiles[tileY][xtile].alphaBlend(ctx.cmds, int32(len(ctx.cmds)-1))
			}
			x += width
			col += uint32(width)
			tileX = xtile
			popPending = true
		}

		// XXX add even/odd winding rule support
		if nextStrip.winding != 0 && stripY == nextStrip.stripY() {
			x = max(x1, bbox.tileMin.tileX*wideTileWidth)
			uproundedWidth := divCeil(ctx.width, wideTileWidth) * wideTileWidth
			x2 := min(nextStrip.x, uproundedWidth)
			fxt0 := tileX
			fxt1 := min(divCeil(x2, wideTileWidth), bbox.tileMax.tileX)

			for xtile := fxt0; xtile < fxt1; xtile++ {
				if xtile > fxt0 && popPending {
					ctx.tiles[tileY][tileX].popLayer(ctx.cmds, popLayerCmdIdx)
					popPending = false
				}
				xTileRel := x % wideTileWidth
				if x > min(x2, (xtile+1)*wideTileWidth) {
					panic("internal error: width overflow")
				}
				width := min(x2, (xtile+1)*wideTileWidth) - x
				if width == 0 {
					continue
				}
				x += width
				ctx.cmds = ctx.tiles[tileY][xtile].blend(ctx.cmds, xTileRel, width, lastLayer.blend, lastLayer.opacity)
				tileX = xtile
				popPending = true
			}
		}
	}

	if popPending {
		ctx.tiles[tileY][tileX].popLayer(ctx.cmds, popLayerCmdIdx)
		tileX++
		popPending = false
	}

	// TODO(dh): is this condition actually possible? For the bounding box to
	// include bbox.tileMax.tileY, at least one strip has to cover it, doesn't it?
	for tileY < bbox.tileMax.tileY {
		for x := tileX; x < bbox.tileMax.tileX; x++ {
			ctx.tiles[tileY][x].popZeroClip()
		}
		tileX = bbox.tileMin.tileX
		tileY++
	}
}

func (ctx *Renderer) popLayers() {
	for ctx.stateStack[len(ctx.stateStack)-1].numLayers > 0 {
		ctx.popLayer()
	}
}

func (ctx *Renderer) FillCompiled(p CompiledPath, transform curve.Affine, paint gfx.Paint) {
	ctx.renderPath(p, paint.Encode(transform))
}

func (ctx *Renderer) Fill(
	shape gfx.Shape,
	transform curve.Affine,
	fillRule gfx.FillRule,
	paint gfx.Paint,
) {
	p := CompileFillPath(shape, transform, fillRule, ctx.width, ctx.height)
	ctx.renderPath(p, paint.Encode(transform))
}

func (ctx *Renderer) Stroke(
	shape gfx.Shape,
	transform curve.Affine,
	stroke_ curve.Stroke,
	paint gfx.Paint,
) {
	p := CompileStrokedPath(shape, transform, stroke_, ctx.width, ctx.height)
	ctx.renderPath(p, paint.Encode(transform))
}

type Layer struct {
	BlendMode     gfx.BlendMode
	Opacity       float32
	Clip          gfx.Shape
	ClipTransform curve.Affine
	ClipFillRule  gfx.FillRule
	CopyBackdrop  bool
}

type LayerCompiled struct {
	BlendMode    gfx.BlendMode
	Opacity      float32
	Clip         maybe.Option[CompiledPath]
	CopyBackdrop bool
}

func (ctx *Renderer) PushClip(shape gfx.Shape, transform curve.Affine, fill gfx.FillRule) {
	ctx.PushLayer(Layer{
		Opacity:       1,
		Clip:          shape,
		ClipFillRule:  fill,
		ClipTransform: transform,
		CopyBackdrop:  true,
		BlendMode:     gfx.BlendMode{Compose: gfx.ComposeCopy},
	})
}

func (ctx *Renderer) PushClipCompiled(p CompiledPath) {
	ctx.PushLayerCompiled(LayerCompiled{Opacity: 1, Clip: maybe.Some(p), CopyBackdrop: true})
}

func stripsBoundingBox(strips []strip) tileBbox {
	if len(strips) == 0 {
		return tileBbox{}
	}

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
	return tileBbox{
		tileMin: tileCoord{x0, y0},
		tileMax: tileCoord{x1, y1},
	}
}

func (ctx *Renderer) PushLayerCompiled(l LayerCompiled) {
	const pushLayerCmdIdx = 2
	const copyBackdropIdx = 3

	topLayer := &ctx.layerStack[len(ctx.layerStack)-1]
	if topLayer.blackholed > 0 || (l.BlendMode.Compose == gfx.ComposeSrcIn && !topLayer.nonempty) {
		topLayer.blackholed++
		return
	}

	topLayer.nonempty = true

	bbox := ctx.bbox()

	if !l.Clip.Set() {
		// When there is no new clip, all tiles that need to be zero-clipped
		// already have been by the last clip. We can just push layers for all
		// tiles in the bounding box; tiles that have been zero-clipped will
		// ignore the new layer.
		for tileY := bbox.tileMin.tileY; tileY < bbox.tileMax.tileY; tileY++ {
			for tileX := bbox.tileMin.tileX; tileX < bbox.tileMax.tileX; tileX++ {
				ctx.tiles[tileY][tileX].pushLayer(pushLayerCmdIdx)
				if l.CopyBackdrop {
					ctx.tiles[tileY][tileX].copyBackdrop(copyBackdropIdx)
				}
			}
		}
		clip := layer{
			bbox:         bbox,
			strips:       nil,
			opacity:      l.Opacity,
			blend:        l.BlendMode,
			alphas:       nil,
			copyBackdrop: l.CopyBackdrop,
			nonempty:     l.CopyBackdrop && topLayer.nonempty,
			noclip:       true,
		}
		ctx.layerStack = append(ctx.layerStack, clip)
		ctx.stateStack[len(ctx.stateStack)-1].numLayers++
		return
	}

	// intersect clip bounding box
	strips := l.Clip.Unwrap().strips
	bbox = bbox.intersect(stripsBoundingBox(strips))

	// The next bit of code accomplishes the following. For each tile in
	// the intersected bounding box, it does one of two things depending
	// on the contents of the clip path in that tile.
	// If all-zero: push a zero_clip
	// If all-ones: push a clip
	// If contains one or more strips: push a clip
	tileX := bbox.tileMin.tileX
	tileY := bbox.tileMin.tileY

	for i := range len(strips) - 1 {
		strip := &strips[i]
		stripY := strip.stripY()
		if stripY < tileY {
			continue
		}
		for tileY < min(stripY, bbox.tileMax.tileY) {
			for x := tileX; x < bbox.tileMax.tileX; x++ {
				ctx.tiles[tileY][x].pushZeroClip()
			}
			tileX = bbox.tileMin.tileX
			tileY++
		}
		if tileY == bbox.tileMax.tileY {
			break
		}
		x0 := strip.x
		xClamped := min(x0/wideTileWidth, bbox.tileMax.tileX)
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
		xtile1 := min(divCeil(x1, wideTileWidth), bbox.tileMax.tileX)
		if tileX < xtile1 {
			for xtile := tileX; xtile < xtile1; xtile++ {
				ctx.tiles[tileY][xtile].pushLayer(pushLayerCmdIdx)
				if l.CopyBackdrop {
					ctx.tiles[tileY][xtile].copyBackdrop(copyBackdropIdx)
				}
			}
			tileX = xtile1
		}

		// Push layers for all tiles covered by solid fill (except for the one
		// already covered by alpha, if any)
		//
		// XXX support even/odd fill rule
		if nextStrip.winding != 0 && tileY == nextStrip.stripY() {
			x2 := min(divCeil(nextStrip.x, wideTileWidth), bbox.tileMax.tileX)
			fxt0 := tileX
			fxt1 := x2
			for xtile := fxt0; xtile < fxt1; xtile++ {
				ctx.tiles[tileY][xtile].pushLayer(pushLayerCmdIdx)
				if l.CopyBackdrop {
					ctx.tiles[tileY][xtile].copyBackdrop(copyBackdropIdx)
				}
			}
			tileX = fxt1
		}
	}

	// TODO(dh): is this condition actually possible? For the bounding box to
	// include bbox.tileMax.tileY, at least one strip has to cover it, doesn't it?
	for tileY < bbox.tileMax.tileY {
		for x := tileX; x < bbox.tileMax.tileX; x++ {
			ctx.tiles[tileY][x].pushZeroClip()
		}
		tileX = bbox.tileMin.tileX
		tileY++
	}

	clip := layer{
		bbox:         bbox,
		strips:       strips,
		opacity:      l.Opacity,
		blend:        l.BlendMode,
		alphas:       l.Clip.Unwrap().alphas,
		copyBackdrop: l.CopyBackdrop,
		nonempty:     l.CopyBackdrop && topLayer.nonempty,
	}
	ctx.layerStack = append(ctx.layerStack, clip)
	ctx.stateStack[len(ctx.stateStack)-1].numLayers++
}

func (ctx *Renderer) PushLayer(l Layer) {
	var p maybe.Option[CompiledPath]
	if l.Clip != nil {
		p = maybe.Some(CompileFillPath(l.Clip, l.ClipTransform, l.ClipFillRule, ctx.width, ctx.height))
	}

	ctx.PushLayerCompiled(LayerCompiled{
		BlendMode:    l.BlendMode,
		Opacity:      l.Opacity,
		Clip:         p,
		CopyBackdrop: l.CopyBackdrop,
	})

}

func (ctx *Renderer) Save() {
	ctx.stateStack = append(ctx.stateStack, gfxState{0})
}

func (ctx *Renderer) Restore() {
	ctx.popLayers()
	ctx.stateStack = ctx.stateStack[:len(ctx.stateStack)-1]
}
