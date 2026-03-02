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

	"honnef.co/go/color"
	"honnef.co/go/curve"
	"honnef.co/go/gutter/gfx"
	"honnef.co/go/stuff/container/maybe"
	"honnef.co/go/stuff/syncutil"
)

type gfxState struct {
	numLayers int
	numClips  int
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

	lazy          bool
	pushedToTiles []struct{ x, y uint16 }
}

type Renderer struct {
	width  uint16
	height uint16
	// [y][x]wideTile
	tiles      [][]wideTile
	stateStack []gfxState
	layerStack []layer
	clipStack  []Path
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
		stateStack: []gfxState{{0, 0}},
		layerStack: []layer{
			{
				bbox: tileBbox{
					tileMin: tileCoord{0, 0},
					tileMax: tileCoord{widthTiles, heightTiles},
				},
				opacity: 1,
			},
		},
	}
}

func optimizeRecording(cmds gfx.Recording) {
	type layer struct {
		push               int
		needBackdrop       bool
		childNeedsBackdrop bool
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
		case gfx.CommandPushClip:
			// TODO(dh): should we handle empty clip paths here?
		case gfx.CommandPopClip:
		case gfx.CommandStroke:
		default:
			panic(fmt.Sprintf("unexpected gfx.Command: %#v", cmd))
		}
	}
}

func PlayRecording(cmds gfx.Recording, r *Renderer, aff curve.Affine) {
	const debugPrintRecording = false

	printRecording := func() {
		var indent string
		for _, cmd := range cmds {
			if cmd == nil {
				continue
			}
			log.Printf("%s%T", indent, cmd)
			switch cmd.(type) {
			case gfx.CommandPushLayer:
				indent += "  "
			case gfx.CommandPopLayer:
				indent = indent[:max(0, len(indent)-2)]
			case gfx.CommandPlayRecording:
				// FIXME(dh): print recordings recursively
			}
		}
	}

	if debugPrintRecording {
		log.Println("Recording before optimization:")
		printRecording()
	}
	optimizeRecording(cmds)
	if debugPrintRecording {
		log.Println()
		log.Println("Recording after optimization:")
		printRecording()
	}

	// OPT(dh): reuse this slice between multiple renders
	compiled := make([]Path, len(cmds))
	// OPT(dh): if we compiled paths as part of recording instead of playback
	// we'd be able to exploit more parallelism, between command generation and
	// path compilation.
	syncutil.Distribute(cmds, -1, func(group, step int, items gfx.Recording) error {
		for i, cmd := range items {
			switch cmd := cmd.(type) {
			case gfx.CommandFill:
				compiled[group*step+i] = CompileFillPath(cmd.Shape, aff.Mul(cmd.Transform), cmd.FillRule, r.width, r.height)
			case gfx.CommandPlayRecording:
				// Nothing to do
			case gfx.CommandPopClip:
				// Nothing to do
			case gfx.CommandPopLayer:
				// Nothing to do
			case gfx.CommandPushClip:
				compiled[group*step+i] = CompileFillPath(cmd.Clip, aff.Mul(cmd.Transform), cmd.FillRule, r.width, r.height)
			case gfx.CommandPushLayer:
				if cmd.Layer.Clip != nil {
					compiled[group*step+i] = CompileFillPath(cmd.Layer.Clip, aff.Mul(cmd.Transform), cmd.FillRule, r.width, r.height)
				}
			case gfx.CommandStroke:
				compiled[group*step+i] = CompileStrokedPath(cmd.Shape, aff.Mul(cmd.Transform), cmd.Stroke, r.width, r.height)
			case nil:
			default:
				panic(fmt.Sprintf("unexpected gfx.Command: %#v", cmd))
			}
		}
		return nil
	})

	for i, cmd := range cmds {
		switch cmd := cmd.(type) {
		case gfx.CommandFill:
			r.FillCompiled(compiled[i], aff.Mul(cmd.Transform), cmd.Paint)
		case gfx.CommandPopLayer:
			r.PopLayer()
		case gfx.CommandPushLayer:
			lc := LayerCompiled{
				BlendMode: cmd.Layer.BlendMode,
				Opacity:   cmd.Layer.Opacity,
			}
			if cmd.Layer.Clip != nil {
				lc.Clip = maybe.Some(compiled[i])
			}
			r.PushLayerCompiled(lc)
		case gfx.CommandPopClip:
			r.PopClip()
		case gfx.CommandPushClip:
			r.PushClipCompiled(compiled[i])
		case gfx.CommandStroke:
			r.StrokeCompiled(compiled[i], aff.Mul(cmd.Transform), cmd.Paint)
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
			clear(tile.fillArgs)
			// OPT(dh): technically there's no need to clear blendArgs, it
			// doesn't contain any pointers.
			clear(tile.blendArgs)
			clear(tile.alphaBlendArgs)
			clear(tile.alphaFillArgs)
			tile.cmds = tile.cmds[:0]
			tile.fillArgs = tile.fillArgs[:0]
			tile.blendArgs = tile.blendArgs[:0]
			tile.alphaFillArgs = tile.alphaFillArgs[:0]
			tile.alphaBlendArgs = tile.alphaBlendArgs[:0]
			tile.numZeroClips = 0
			tile.numLayers = 0
		}
	}

	clear(ctx.layerStack[1:])
	ctx.layerStack = ctx.layerStack[:1]
	ctx.stateStack = ctx.stateStack[:1]
	ctx.stateStack[0] = gfxState{0, 0}
	clear(ctx.clipStack)
	ctx.clipStack = ctx.clipStack[:0]
}

// Finish the coarse rasterization prior to fine rendering.
//
// At the moment, this mostly involves resolving any open layers, but
// might extend to other things.
func (ctx *Renderer) finish() {
	for len(ctx.stateStack) > 0 {
		ctx.restore()
	}
}

func (ctx *Renderer) Render(packer Packer) {
	ctx.finish()

	syncutil.Distribute(ctx.tiles, runtime.GOMAXPROCS(0), func(group int, step int, subitems [][]wideTile) error {
		stackScratch := make([]optLayer, 0, 32)
		fine := newFine(packer)
		for y, row := range subitems {
			y += group * step
			for x := range row {
				tile := &row[x]
				fine.setTile(tile, uint16(x), uint16(y))
				fine.topLayer().clear(tile.bg)

				if false && len(tile.cmds) > 0 {
					log.Println("tile", x, y)
					for i := range tile.cmds {
						log.Println(&tile.cmds[i])
					}
					log.Println()
				}

				// log.Println(x, y)
				tile.cmds, stackScratch = optimizeCommands(tile, tile.cmds, stackScratch[:0])

				for _, c := range tile.cmds {
					fine.runCmd(c)
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
	tileBufPool.Put(tileBuf[:0])
	return stripBuf, alphas
}

func (ctx *Renderer) renderPath(p Path, paint encodedPaint) {
	topLayer := &ctx.layerStack[len(ctx.layerStack)-1]
	if topLayer.blackholed > 0 {
		return
	}

	if len(ctx.clipStack) > 0 {
		p = ctx.clipStack[len(ctx.clipStack)-1].Intersect(p)
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
			c := cmd{typ: cmdAlphaFill}
			alphas := alphas[col:]
			if width <= alphaValueCutoff {
				allOne := true
				for _, a := range alphas[:width] {
					if a != [4]uint8{255, 255, 255, 255} {
						allOne = false
						break
					}
				}
				if allOne {
					if paint.Opaque() {
						c.typ = cmdClear
					} else {
						c.typ = cmdFill
					}
				}
			}
			x += width
			col += uint32(width)
			switch c.typ {
			case cmdClear, cmdFill:
				ctx.fillTile(xtile, stripY, xTileRel, width, paint)
			case cmdAlphaFill:
				args := alphaFillArgs{
					alphas: alphas,
					fillArgs: fillArgs{
						paint: paint,
						baseArgs: baseArgs{
							x:     xTileRel,
							width: width,
						},
					},
				}
				ctx.alphaFillTile(xtile, stripY, args)
			default:
				panic(fmt.Sprintf("unexpected sparse.cmdType: %#v", c.typ))
			}
		}

		if nextStrip.fillGap && stripY == nextStrip.stripY() {
			x = max(x1, bbox.tileMin.tileX*wideTileWidth)
			uproundedWidth := divCeil(ctx.width, wideTileWidth) * wideTileWidth
			x2 := min(nextStrip.x, uproundedWidth)
			fxt0 := max(x1/wideTileWidth, bbox.tileMin.tileX)
			fxt1 := min(divCeil(x2, wideTileWidth), bbox.tileMax.tileX)
			for xtile := fxt0; xtile < fxt1; xtile++ {
				xTileRel := x % wideTileWidth
				width := min(x2, (xtile+1)*wideTileWidth) - x
				x += width
				ctx.fillTile(xtile, stripY, xTileRel, width, paint)
			}
		}
	}
}

func (ctx *Renderer) bbox() tileBbox {
	return ctx.layerStack[len(ctx.layerStack)-1].bbox
}

func (ctx *Renderer) popLayer() {
	lastLayer := &ctx.layerStack[len(ctx.layerStack)-1]
	if lastLayer.blackholed > 0 {
		lastLayer.blackholed--
		return
	}

	defer func() {
		// Don't hold on to any data (such as layer.strips and layer.alphas)
		//
		// Note that this won't errornously reset the very first layer because
		// popLayer doesn't get called for it.
		*lastLayer = layer{}
	}()
	ctx.stateStack[len(ctx.stateStack)-1].numLayers--
	ctx.layerStack = ctx.layerStack[:len(ctx.layerStack)-1]
	bbox := lastLayer.bbox

	if lastLayer.noclip {
		// The layer didn't add a new clip, so we can just blend and pop all
		// layers in the bounding box. It doesn't matter that we might blend
		// pixels that lie outside the clip, the parent layer with the actual
		// clip on it will make sure to discard those pixels.
		if lastLayer.lazy {
			for _, tileCoords := range lastLayer.pushedToTiles {
				tile := &ctx.tiles[tileCoords.y][tileCoords.x]
				tile.blend(
					0,
					wideTileWidth,
					lastLayer.blend,
					lastLayer.opacity,
				)
				tile.popLayer()
			}
		} else {
			for tileY := bbox.tileMin.tileY; tileY < bbox.tileMax.tileY; tileY++ {
				for tileX := bbox.tileMin.tileX; tileX < bbox.tileMax.tileX; tileX++ {
					ctx.tiles[tileY][tileX].blend(
						0,
						wideTileWidth,
						lastLayer.blend,
						lastLayer.opacity,
					)
					ctx.tiles[tileY][tileX].popLayer()
				}
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
				ctx.tiles[tileY][tileX].popLayer()
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
				ctx.tiles[tileY][tileX].popLayer()
				tileX++
				popPending = false
			}
			// The winding check is probably not needed; if there was a fill,
			// the logic below should have advanced tileX.
			if !strip.fillGap {
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
				ctx.tiles[tileY][tileX].popLayer()
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
				ctx.tiles[tileY][xtile].blend(xTileRel, width, lastLayer.blend, lastLayer.opacity)
			} else {
				args := alphaBlendArgs{
					alphas: alphas[col:],
					blendArgs: blendArgs{
						blend:   lastLayer.blend,
						opacity: lastLayer.opacity,
						baseArgs: baseArgs{
							x:     xTileRel,
							width: width,
						},
					},
				}
				ctx.tiles[tileY][xtile].alphaBlend(args)
			}
			x += width
			col += uint32(width)
			tileX = xtile
			popPending = true
		}

		if nextStrip.fillGap && stripY == nextStrip.stripY() {
			x = max(x1, bbox.tileMin.tileX*wideTileWidth)
			uproundedWidth := divCeil(ctx.width, wideTileWidth) * wideTileWidth
			x2 := min(nextStrip.x, uproundedWidth)
			fxt0 := tileX
			fxt1 := min(divCeil(x2, wideTileWidth), bbox.tileMax.tileX)

			for xtile := fxt0; xtile < fxt1; xtile++ {
				if xtile > fxt0 && popPending {
					ctx.tiles[tileY][tileX].popLayer()
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
				ctx.tiles[tileY][xtile].blend(xTileRel, width, lastLayer.blend, lastLayer.opacity)
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
	// include bbox.tileMax.tileY, at least one strip has to cover it, doesn't it?
	for tileY < bbox.tileMax.tileY {
		for x := tileX; x < bbox.tileMax.tileX; x++ {
			ctx.tiles[tileY][x].popZeroClip()
		}
		tileX = bbox.tileMin.tileX
		tileY++
	}
}

func (ctx *Renderer) FillCompiled(p Path, transform curve.Affine, paint gfx.Paint) {
	ctx.renderPath(p, encodePaint(paint, transform))
}

func (ctx *Renderer) Fill(
	shape gfx.Shape,
	transform curve.Affine,
	fillRule gfx.FillRule,
	paint gfx.Paint,
) {
	p := CompileFillPath(shape, transform, fillRule, ctx.width, ctx.height)
	ctx.renderPath(p, encodePaint(paint, transform))
}

func (ctx *Renderer) StrokeCompiled(p Path, transform curve.Affine, paint gfx.Paint) {
	ctx.renderPath(p, encodePaint(paint, transform))
}

func (ctx *Renderer) Stroke(
	shape gfx.Shape,
	transform curve.Affine,
	stroke_ curve.Stroke,
	paint gfx.Paint,
) {
	p := CompileStrokedPath(shape, transform, stroke_, ctx.width, ctx.height)
	ctx.renderPath(p, encodePaint(paint, transform))
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
	Clip         maybe.Option[Path]
	CopyBackdrop bool
}

// PushClip pushes a new clip to the clip stack. The provided shape gets
// intersected with the current clip path, if any.
//
// All uses of [PushClip], [PushClipCompiled], [Fill], [FillCompiled], [Stroke],
// and [StrokeCompiled] have their shapes and paths intersected with the
// currently active clip path, if any.
//
// The clip stack is independent of the layer stack.
func (ctx *Renderer) PushClip(shape gfx.Shape, transform curve.Affine, fill gfx.FillRule) {
	ctx.PushClipCompiled(CompileFillPath(shape, transform, fill, ctx.width, ctx.height))
}

// PushClipCompiled is like [PushClip] but using an already compiled [Path].
func (ctx *Renderer) PushClipCompiled(p Path) {
	if len(ctx.clipStack) != 0 {
		p = ctx.clipStack[len(ctx.clipStack)-1].Intersect(p)
	}
	ctx.stateStack[len(ctx.stateStack)-1].numClips++
	ctx.clipStack = append(ctx.clipStack, p)
}

// PopClip pops one element off the clip stack.
func (ctx *Renderer) PopClip() {
	if len(ctx.clipStack) != 0 {
		ctx.clipStack[len(ctx.clipStack)-1] = Path{}
		ctx.clipStack = ctx.clipStack[:len(ctx.clipStack)-1]
		ctx.stateStack[len(ctx.stateStack)-1].numClips--
	}
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

func (ctx *Renderer) ensureLayerForTile(tileX, tileY uint16) {
	tile := &ctx.tiles[tileY][tileX]
	if tile.isZeroClip() {
		return
	}
	if tile.numLayers+1 < len(ctx.layerStack) {
		for i := tile.numLayers + 1; i < len(ctx.layerStack); i++ {
			l := &ctx.layerStack[i]
			if !l.lazy {
				panic("unexpected unlazy layer")
			}
			tile.pushLayer()
			l.pushedToTiles = append(l.pushedToTiles, struct {
				x uint16
				y uint16
			}{tileX, tileY})
		}
	}
	if tile.numLayers+1 != len(ctx.layerStack) {
		panic("unreachable")
	}
}

func (ctx *Renderer) fillTile(tileX, tileY, tileRelX, width uint16, paint encodedPaint) {
	ctx.ensureLayerForTile(tileX, tileY)
	ctx.tiles[tileY][tileX].fill(tileRelX, width, paint)
}

func (ctx *Renderer) alphaFillTile(tileX, tileY uint16, args alphaFillArgs) {
	ctx.ensureLayerForTile(tileX, tileY)
	ctx.tiles[tileY][tileX].alphaFill(args)
}

func (ctx *Renderer) PushLayerCompiled(l LayerCompiled) {
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

		// OPT(dh): allow all blend modes that we can
		lazy := l.BlendMode == gfx.BlendMode{} && !l.CopyBackdrop
		if !lazy {
			for tileY := bbox.tileMin.tileY; tileY < bbox.tileMax.tileY; tileY++ {
				for tileX := bbox.tileMin.tileX; tileX < bbox.tileMax.tileX; tileX++ {
					ctx.tiles[tileY][tileX].pushLayer()
					if l.CopyBackdrop {
						ctx.tiles[tileY][tileX].copyBackdrop()
					}
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
			lazy:         lazy,
		}
		ctx.layerStack = append(ctx.layerStack, clip)
		ctx.stateStack[len(ctx.stateStack)-1].numLayers++
		return
	}

	// Intersect clip bounding box.
	//
	// Note that we do not intersect the layer's clip path with the clip stack.
	// The clip stack can be popped independently of the layer stack and the
	// layer's full clip path should apply when that happens.
	//
	// TODO(dh): should PushLayer be affected by the current clip?
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
			if !strip.fillGap {
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
				// OPT(dh): make these layers lazy, too. complicated by zero clips.
				ctx.ensureLayerForTile(xtile, tileY)
				ctx.tiles[tileY][xtile].pushLayer()
				if l.CopyBackdrop {
					ctx.tiles[tileY][xtile].copyBackdrop()
				}
			}
			tileX = xtile1
		}

		// Push layers for all tiles covered by solid fill (except for the one
		// already covered by alpha, if any)
		if nextStrip.fillGap && tileY == nextStrip.stripY() {
			x2 := min(divCeil(nextStrip.x, wideTileWidth), bbox.tileMax.tileX)
			fxt0 := tileX
			fxt1 := x2
			for xtile := fxt0; xtile < fxt1; xtile++ {
				// OPT(dh): make these layers lazy, too. complicated by zero clips.
				ctx.ensureLayerForTile(xtile, tileY)
				ctx.tiles[tileY][xtile].pushLayer()
				if l.CopyBackdrop {
					ctx.tiles[tileY][xtile].copyBackdrop()
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
	var p maybe.Option[Path]
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

func (ctx *Renderer) PopLayer() {
	if len(ctx.layerStack) == 1 {
		// We start with one layer in the layer stack, which the user shouldn't
		// be able to pop.
		return
	}
	if ctx.stateStack[len(ctx.stateStack)-1].numLayers > 0 {
		ctx.popLayer()
	}
}

func (ctx *Renderer) Save() {
	ctx.stateStack = append(ctx.stateStack, gfxState{0, 0})
}

func (ctx *Renderer) Restore() {
	if len(ctx.stateStack) == 1 {
		// We start with one state in the state stack, so that PushClip and
		// PushLayer can unconditionally increase the counts in the topmost
		// state. This isn't a state that the user should be able to pop.
		return
	}
	ctx.restore()
}

func (ctx *Renderer) restore() {
	state := &ctx.stateStack[len(ctx.stateStack)-1]
	for state.numLayers > 0 {
		ctx.popLayer()
	}
	for state.numClips > 0 {
		ctx.PopClip()
	}
	ctx.stateStack = ctx.stateStack[:len(ctx.stateStack)-1]
}

type encodedPaint interface {
	// Opaque reports whether it's impossible for the paint to be translucent.
	Opaque() bool
	isEncodedPaint()
}

func encodePaint(p gfx.Paint, transform curve.Affine) encodedPaint {
	// OPT(dh): we should cache and reuse encoded paints. If the user uses the
	// same gradient many times, we shouldn't compute and store many LUTs.

	switch p := p.(type) {
	case gfx.Solid:
		return encodeColor(color.Color(p), transform)
	case *gfx.LinearGradient:
		return encodeLinearGradient(p, transform)
	case *gfx.RadialGradient:
		return encodeRadialGradient(p, transform)
	case *gfx.SweepGradient:
		return encodeSweepGradient(p, transform)
	case *gfx.BlurredRoundedRectangle:
		return encodeBlurredRoundedRectangle(p, transform)
	default:
		panic(fmt.Sprintf("unexpected gfx.Paint: %#v", p))
	}
}
