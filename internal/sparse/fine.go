// SPDX-FileCopyrightText: 2024 the Piet Authors
// SPDX-FileCopyrightText: 2025 Dominik Honnef and contributors
//
// SPDX-License-Identifier: MIT

//go:generate go run ./_asm -out sparse_amd64.s

package sparse

import (
	"fmt"
	"unsafe"

	"honnef.co/go/gutter/gfx"
	"honnef.co/go/safeish"
)

// [x][y]Color
type WideTileBuffer = [wideTileWidth][stripHeight]gfx.PlainColor

type fine struct {
	tile   *wideTile
	tileX  uint16
	tileY  uint16
	layers []fineLayer
	packer Packer

	// free list of scratch space
	freeScratches []*WideTileBuffer
}

type fineLayer struct {
	scratch     *WideTileBuffer
	singleColor gfx.PlainColor
	// if complex is false, all pixels have the color stored in singleColor and
	// the contents of scratch may be undefined.
	complex bool
}

func newFine(packer Packer) *fine {
	f := &fine{
		packer: packer,
	}
	f.layers = []fineLayer{{scratch: f.newScratch()}}
	return f
}

func (f *fine) setTile(tile *wideTile, x, y uint16) {
	f.tile = tile
	f.tileX = x
	f.tileY = y
}

func (f *fine) topLayer() *fineLayer {
	return &f.layers[len(f.layers)-1]
}

func (f *fine) newScratch() *WideTileBuffer {
	// Align scratch memory to this many bytes. This should match the largest
	// vector width that we use in assembly. Currently that is 32 for AVX. Has
	// to be a power of 2.
	const align = 32

	scratch := make([]byte, unsafe.Sizeof(WideTileBuffer{})+align)
	ptr := unsafe.Pointer(&scratch[0])
	alignedPtr := unsafe.Pointer((uintptr(ptr) + align - 1) &^ (align - 1))
	return (*WideTileBuffer)(alignedPtr)
}

func (l *fineLayer) clear(c gfx.PlainColor) {
	l.complex = false
	l.singleColor = c
}

// pack writes the tile at (tileX, tileY) to the output buffer.
func (f *fine) pack() {
	l := f.topLayer()
	if l.complex {
		f.packComplex(l, f.tileX, f.tileY)
	} else {
		f.packSimple(l, f.tileX, f.tileY)
	}
}

func (f *fine) packSimple(l *fineLayer, tileX, tileY uint16) {
	outWidth := uint16(wideTileWidth)
	outHeight := uint16(stripHeight)
	x0 := tileX * wideTileWidth
	x1 := x0 + outWidth
	y0 := tileY * stripHeight
	y1 := y0 + outHeight
	f.packer.PackSimple(x0, y0, x1, y1, l.singleColor)
}

func (f *fine) packComplex(l *fineLayer, tileX, tileY uint16) {
	outWidth := uint16(wideTileWidth)
	outHeight := uint16(stripHeight)

	x0 := tileX * wideTileWidth
	x1 := x0 + outWidth
	y0 := tileY * stripHeight
	y1 := y0 + outHeight
	f.packer.PackComplex(x0, y0, x1, y1, l.scratch)
}

func memsetColumnsNative(buf [][stripHeight]gfx.PlainColor, c gfx.PlainColor) {
	var col [stripHeight]gfx.PlainColor
	for i := range col {
		col[i] = c
	}
	for x := range buf {
		buf[x] = col
	}
}

func (l *fineLayer) materialize() {
	if l.complex {
		return
	}

	memsetColumns(l.scratch[:], l.singleColor)
	l.complex = true
}

func (f *fine) allocScratch(width int) *WideTileBuffer {
	if len(f.freeScratches) > 0 {
		scratch := f.freeScratches[len(f.freeScratches)-1]
		clear(scratch[:width])
		f.freeScratches = f.freeScratches[:len(f.freeScratches)-1]
		return scratch
	} else {
		return f.newScratch()
	}
}

func (f *fine) freeScratch(scratch *WideTileBuffer) {
	f.freeScratches = append(f.freeScratches, scratch)
}

func (f *fine) runCmd(cmd cmd) {
	switch cmd.typ {
	case cmdFill:
		args := f.tile.fillArgs[cmd.args]
		f.fill(int(args.x), int(args.width), args.paint)
	case cmdAlphaFill:
		args := f.tile.alphaFillArgs[cmd.args]
		f.alphaFill(int(args.x), int(args.width), args.alphas, args.paint)
	case cmdPushLayer:
		f.layers = append(f.layers, fineLayer{
			scratch: f.allocScratch(wideTileWidth),
		})
	case cmdPopLayer:
		f.freeScratch(f.layers[len(f.layers)-1].scratch)
		f.layers = f.layers[:len(f.layers)-1]
	case cmdCopyBackdrop:
		if len(f.layers) < 2 {
			panic(fmt.Sprintf("internal error: trying to copy from parent layer, but there are only %d layers",
				len(f.layers)))
		}
		parent := f.layers[len(f.layers)-2]
		l := f.topLayer()
		if parent.complex {
			l.complex = true
			copy(l.scratch[:], parent.scratch[:])
		} else {
			l.clear(parent.singleColor)
		}
	case cmdBlend:
		// OPT(dh): we should probably just pass *cmd to blend and alphaBlend
		args := f.tile.blendArgs[cmd.args]
		f.blend(int(args.x), int(args.width), args.blend, args.opacity)
	case cmdAlphaBlend:
		args := f.tile.alphaBlendArgs[cmd.args]
		f.alphaBlend(int(args.x), int(args.width), args.alphas, args.blend, args.opacity)
	case cmdClear:
		args := f.tile.fillArgs[cmd.args]
		if p, ok := args.paint.(encodedColor); ok {
			f.clear(int(args.x), int(args.width), gfx.PlainColor(p))
		} else {
			f.fill(int(args.x), int(args.width), args.paint)
		}
	case cmdNop:
	default:
		panic(fmt.Sprintf("unreachable: %T", cmd))
	}
}

func (f *fine) clear(x, width int, paint gfx.PlainColor) {
	l := f.topLayer()
	if x == 0 && width == wideTileWidth {
		l.clear(paint)
	} else {
		l.materialize()
		buf := l.scratch[x : x+width]
		memsetColumns(buf, paint)
	}
}

// TODO(dh): change types of x and width to uint16.
func (f *fine) fill(x, width int, paint encodedPaint) {
	l := f.topLayer()
	buf := l.scratch[x : x+width]

	switch paint := paint.(type) {
	case encodedColor:
		color := gfx.PlainColor(paint)
		if x == 0 && width == wideTileWidth {
			if color[3] == 1.0 {
				l.clear(color)
			} else if l.complex {
				fineFillComplex(buf, color)
			} else {
				oneMinusAlpha := 1.0 - color[3]
				color = gfx.PlainColor{
					color[0] + l.singleColor[0]*oneMinusAlpha,
					color[1] + l.singleColor[1]*oneMinusAlpha,
					color[2] + l.singleColor[2]*oneMinusAlpha,
					color[3] + l.singleColor[3]*oneMinusAlpha,
				}
				l.clear(color)
			}
		} else {
			// If the tile isn't complex yet, it will be after we've processed this
			// fill.
			complex := l.complex
			l.materialize()

			if color[3] == 1.0 {
				// The fill color is opaque, so we use a fill function that doesn't care
				// about the background color.
				memsetColumns(buf, color)
			} else if !complex {
				// The tile is simple, which means the fill only has to blend colors
				// once, not for every pixel.
				oneMinusAlpha := 1.0 - color[3]
				color = gfx.PlainColor{
					color[0] + l.singleColor[0]*oneMinusAlpha,
					color[1] + l.singleColor[1]*oneMinusAlpha,
					color[2] + l.singleColor[2]*oneMinusAlpha,
					color[3] + l.singleColor[3]*oneMinusAlpha,
				}
				memsetColumns(buf, color)
			} else {
				// Do the general, per-pixel fill.
				fineFillComplex(buf, color)
			}
		}

	case fillablePaint:
		startX := f.tileX*wideTileWidth + uint16(x)
		startY := f.tileY * tileHeight
		pf := paint.filler(startX, startY)
		if paint.(encodedPaint).Opaque() {
			// The gradient is opaque, so we don't have to blend.
			if x == 0 && width == wideTileWidth {
				// We're going to overwrite the entire tile, so the old contents
				// don't matter.
				pf.fill(buf)
				l.complex = true
			} else {
				l.materialize()
				pf.fill(buf)
			}
		} else {
			// OPT(dh): when the layer is simple, we don't have to read pixels
			// from memory to blend with the gradient
			l.materialize()
			colors := f.allocScratch(width)
			pf.fill(colors[:width])
			blendComplexComplex(buf, colors[:width], nil, gfx.BlendMode{}, 1)
			f.freeScratch(colors)
		}

	default:
		panic(fmt.Sprintf("internal error: unhandled type %T", paint))
	}
}

func (f *fine) blend(x, width int, blend gfx.BlendMode, opacity float32) {
	if n := len(f.layers); n < 2 {
		panic(fmt.Sprintf("internal error: trying to clipFill but we only have %d layers", n))
	}

	tos := &f.layers[len(f.layers)-1]
	nos := &f.layers[len(f.layers)-2]

	// OPT(dh): can x==0 and width==256? in that case, why would we materialize
	// the pixels when nos and tos are simple? the result would still be simple.

	// If nos isn't complex yet, it will be after we've processed this fill.
	// Materialize all the pixels that this fill isn't going to overwrite.
	nosComplex := nos.complex
	nos.materialize()

	dst := nos.scratch[x : x+width]
	src := tos.scratch[x : x+width]
	if !nosComplex && !tos.complex {
		c := tos.singleColor

		c[0] *= opacity
		c[1] *= opacity
		c[2] *= opacity
		c[3] *= opacity
		blendSimpleSimple(dst, nos.singleColor, c, blend)
	} else {
		tos.materialize()
		blendComplexComplex(dst, src, nil, blend, opacity)
	}
}

func (f *fine) alphaBlend(
	x int,
	width int,
	alphas [][stripHeight]uint8,
	blend gfx.BlendMode,
	opacity float32,
) {
	tos := &f.layers[len(f.layers)-1]
	nos := &f.layers[len(f.layers)-2]

	// OPT implement handling of layer.complex
	tos.materialize()
	nos.materialize()

	dst := nos.scratch[x : x+width]
	src := tos.scratch[x : x+width]
	blendComplexComplex(dst, src, alphas, blend, opacity)
}

func fineFillComplexScalar(buf [][stripHeight]gfx.PlainColor, color gfx.PlainColor) {
	oneMinusAlpha := 1.0 - color[3]
	for x := range buf {
		col := &buf[x]
		for y := range col {
			col[y] = gfx.PlainColor{
				color[0] + col[y][0]*oneMinusAlpha,
				color[1] + col[y][1]*oneMinusAlpha,
				color[2] + col[y][2]*oneMinusAlpha,
				color[3] + col[y][3]*oneMinusAlpha,
			}
		}
	}
}

func (f *fine) alphaFill(x, width int, alphas [][stripHeight]uint8, paint encodedPaint) {
	// OPT implement SIMD versions

	// TODO(dh): there is a lot of duplication between fine.fill and
	// fine.alphaFill. can we combine the two functions without hurting
	// performance much?

	if len(alphas) < width {
		panic(fmt.Sprintf("internal error: got %d alphas for a width of %d",
			len(alphas), width))
	}

	l := f.topLayer()
	dst := l.scratch[x : x+width]

	alphaFillInnerSingleColor := func(color gfx.PlainColor) {
		// Scale color by 1/255 because our alpha values are in [0, 255]
		color[0] *= (1.0 / 255.0)
		color[1] *= (1.0 / 255.0)
		color[2] *= (1.0 / 255.0)
		color[3] *= (1.0 / 255.0)

		if l.complex {
			for x := range dst {
				col := &dst[x]
				a := &alphas[x]
				for y := range col {
					maskAlpha := float32(a[y])
					oneMinusAlpha := 1.0 - maskAlpha*color[3]
					col[y] = gfx.PlainColor{
						color[0]*maskAlpha + col[y][0]*oneMinusAlpha,
						color[1]*maskAlpha + col[y][1]*oneMinusAlpha,
						color[2]*maskAlpha + col[y][2]*oneMinusAlpha,
						color[3]*maskAlpha + col[y][3]*oneMinusAlpha,
					}
				}
			}
		} else {
			bg := l.singleColor
			l.materialize()

			for x := range dst {
				col := &dst[x]
				a := &alphas[x]
				for y := range col {
					maskAlpha := float32(a[y])
					oneMinusAlpha := 1.0 - maskAlpha*color[3]
					col[y] = gfx.PlainColor{
						color[0]*maskAlpha + bg[0]*oneMinusAlpha,
						color[1]*maskAlpha + bg[1]*oneMinusAlpha,
						color[2]*maskAlpha + bg[2]*oneMinusAlpha,
						color[3]*maskAlpha + bg[3]*oneMinusAlpha,
					}
				}
			}
		}
	}

	alphaFillInner := func(colors []gfx.PlainColor) {
		colorIdx := 0
		nextColor := func() gfx.PlainColor {
			c := colors[colorIdx]
			colorIdx = (colorIdx + 1) % len(colors)
			return c
		}

		if l.complex {
			for x := range dst {
				col := &dst[x]
				a := &alphas[x]
				for y := range col {
					color := nextColor()
					color[0] *= (1.0 / 255.0)
					color[1] *= (1.0 / 255.0)
					color[2] *= (1.0 / 255.0)
					color[3] *= (1.0 / 255.0)
					maskAlpha := float32(a[y])
					oneMinusAlpha := 1.0 - maskAlpha*color[3]
					col[y] = gfx.PlainColor{
						color[0]*maskAlpha + col[y][0]*oneMinusAlpha,
						color[1]*maskAlpha + col[y][1]*oneMinusAlpha,
						color[2]*maskAlpha + col[y][2]*oneMinusAlpha,
						color[3]*maskAlpha + col[y][3]*oneMinusAlpha,
					}
				}
			}
		} else {
			bg := l.singleColor
			l.materialize()

			for x := range dst {
				col := &dst[x]
				a := &alphas[x]
				for y := range col {
					color := nextColor()
					color[0] *= (1.0 / 255.0)
					color[1] *= (1.0 / 255.0)
					color[2] *= (1.0 / 255.0)
					color[3] *= (1.0 / 255.0)
					maskAlpha := float32(a[y])
					oneMinusAlpha := 1.0 - maskAlpha*color[3]
					col[y] = gfx.PlainColor{
						color[0]*maskAlpha + bg[0]*oneMinusAlpha,
						color[1]*maskAlpha + bg[1]*oneMinusAlpha,
						color[2]*maskAlpha + bg[2]*oneMinusAlpha,
						color[3]*maskAlpha + bg[3]*oneMinusAlpha,
					}
				}
			}
		}
	}

	switch paint := paint.(type) {
	case encodedColor:
		alphaFillInnerSingleColor(gfx.PlainColor(paint))

	case fillablePaint:
		startX := f.tileX*wideTileWidth + uint16(x)
		startY := f.tileY * tileHeight
		pf := paint.filler(startX, startY)
		// OPT(dh): reuse memory
		colors := make([][stripHeight]gfx.PlainColor, width)
		pf.fill(colors)
		alphaFillInner(safeish.SliceCast[[]gfx.PlainColor](colors))

	default:
		panic(fmt.Sprintf("internal error: unhandled type %T", paint))
	}
}

type fillablePaint interface {
	filler(startX, startY uint16) paintFiller
}

type paintFiller interface {
	fill(dst [][stripHeight]gfx.PlainColor)
	reset(startX, startY uint16)
}
