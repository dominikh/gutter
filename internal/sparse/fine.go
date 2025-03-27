// SPDX-FileCopyrightText: 2024 the Piet Authors
// SPDX-FileCopyrightText: 2025 Dominik Honnef and contributors
//
// SPDX-License-Identifier: MIT

//go:generate go run ./_asm -out sparse_amd64.s

package sparse

import (
	"fmt"
	"strings"
	"unsafe"

	"honnef.co/go/safeish"
)

// [x][y]Color
type fineScratch = [wideTileWidth][stripHeight]Color

type fineStats struct {
	simplePacks  uint64
	complexPacks uint64
	pushClips    uint64
	popClips     uint64

	fullWidthOpaqueClearFills      uint64
	fullWidthComplexFills          uint64
	fullWidthTranslucentClearFills uint64
	opaqueFills                    uint64
	simpleFills                    uint64
	complexFills                   uint64

	simpleAlphaFills  uint64
	complexAlphaFills uint64

	simpleSimpleClipFills   uint64
	complexComplexClipFills uint64

	clipAlphaFills uint64

	materializedLayers uint64
	materializedPixels uint64
}

func (s *fineStats) String() string {
	lines := []string{

		fmt.Sprintf("Full-width opaque clear fills: %d", s.fullWidthOpaqueClearFills),
		fmt.Sprintf("Full-width translucent clear fills: %d", s.fullWidthTranslucentClearFills),
		fmt.Sprintf("Full-width complex fills: %d", s.fullWidthComplexFills),
		fmt.Sprintf("Partial opaque fills: %d", s.opaqueFills),
		fmt.Sprintf("Partial simple fills: %d", s.simpleFills),
		fmt.Sprintf("Partial complex fills: %d", s.complexFills),

		fmt.Sprintf("Simple alpha fills: %d", s.simpleAlphaFills),
		fmt.Sprintf("Complex alpha fills: %d", s.complexAlphaFills),

		fmt.Sprintf("PushClips: %d", s.pushClips),
		fmt.Sprintf("PopClips: %d", s.popClips),

		fmt.Sprintf("simple+simple clip fills: %d", s.simpleSimpleClipFills),
		fmt.Sprintf("complex+complex clip fills: %d", s.complexComplexClipFills),

		fmt.Sprintf("Clip alpha fills: %d", s.clipAlphaFills),

		fmt.Sprintf("Materialized pixels: %d", s.materializedPixels),
		fmt.Sprintf("Materialized layers: %d", s.materializedLayers),

		fmt.Sprintf("Simple packs: %d", s.simplePacks),
		fmt.Sprintf("Complex packs: %d", s.complexPacks),
	}

	return strings.Join(lines, "\n")
}

type fine struct {
	// the width and height of the output image, in pixels
	width, height uint16
	outBuf        []Color
	layers        []fineLayer

	// free list of scratch space
	freeScratches []*fineScratch

	stats fineStats
}

type fineLayer struct {
	scratch     *fineScratch
	singleColor Color
	// if complex is false, all pixels have the color stored in singleColor and
	// the contents of scratch may be undefined.
	complex bool
}

func newFine(width, height uint16, out []Color) *fine {
	f := &fine{
		width:  width,
		height: height,
		outBuf: out,
	}
	f.layers = []fineLayer{{scratch: f.newScratch()}}
	return f
}

func (f *fine) topLayer() *fineLayer {
	return &f.layers[len(f.layers)-1]
}

func (f *fine) newScratch() *fineScratch {
	// Align scratch memory to this many bytes. This should match the largest
	// vector width that we use in assembly. Currently that is 32 for AVX. Has
	// to be a power of 2.
	const align = 32

	scratch := make([]byte, unsafe.Sizeof(fineScratch{})+align)
	ptr := unsafe.Pointer(&scratch[0])
	alignedPtr := unsafe.Pointer((uintptr(ptr) + align - 1) &^ (align - 1))
	return (*fineScratch)(alignedPtr)
}

func (l *fineLayer) clear(c Color) {
	l.complex = false
	l.singleColor = c
}

// pack writes the tile at (tileX, tileY) to the output buffer.
func (f *fine) pack(tileX, tileY uint16) {
	l := f.topLayer()
	if l.complex {
		f.packComplex(l, tileX, tileY)
	} else {
		f.packSimple(l, tileX, tileY)
	}
}

func (f *fine) packSimple(l *fineLayer, tileX, tileY uint16) {
	f.stats.simplePacks++
	outWidth := max(0, min(wideTileWidth, f.width-tileX*wideTileWidth))
	outHeight := max(0, min(stripHeight, f.height-tileY*stripHeight))

	baseIdx := (int(tileY*stripHeight)*int(f.width) + int(tileX*wideTileWidth))
	out := f.outBuf[baseIdx:]
	for range outHeight {
		row := out[:min(len(out), int(outWidth))]

		// We're writing the same color to every pixel, so even though
		// memsetColumns operates on columns, we can just pretend that a
		// single row of pixels is a bunch of columns.
		outCols := safeish.SliceCast[[][stripHeight]Color](row)
		memsetColumnsFp(outCols, l.singleColor)
		for x := len(row) &^ 0b11; x < len(row); x++ {
			row[x] = l.singleColor
		}
		out = out[min(uint(len(out)), uint(f.width)):]
	}
}

func (f *fine) packComplex(l *fineLayer, tileX, tileY uint16) {
	// OPT add SIMD implementation

	f.stats.complexPacks++
	outWidth := max(0, min(wideTileWidth, f.width-tileX*wideTileWidth))
	outHeight := max(0, min(stripHeight, f.height-tileY*stripHeight))

	baseIdx := (int(tileY*stripHeight)*int(f.width) + int(tileX*wideTileWidth))
	out := f.outBuf[baseIdx:]
	scratch := l.scratch[:outWidth]
	for y := range outHeight {
		row := out[:min(len(out), int(outWidth))]
		for x := range row {
			row[x] = scratch[x][y]
		}
		out = out[min(uint(len(out)), uint(f.width)):]
	}
}

func memsetColumnsNative(buf [][stripHeight]Color, c Color) {
	var col [stripHeight]Color
	for i := range col {
		col[i] = c
	}
	for x := range buf {
		buf[x] = col
	}
}

func (f *fine) materialize(l *fineLayer, start, end int) {
	if l.complex {
		return
	}

	f.stats.materializedPixels += uint64(end-start) * stripHeight
	f.stats.materializedLayers++

	memsetColumnsFp(l.scratch[start:end], l.singleColor)
}

func (f *fine) runCmd(cmd cmd) {
	switch cmd.typ {
	case cmdFill:
		f.fill(int(cmd.x), int(cmd.width), cmd.color)
	case cmdAlphaFill:
		aslice := cmd.alphas[cmd.alphaIdx:]
		f.alphaFill(int(cmd.x), int(cmd.width), aslice, cmd.color)
	case cmdPushClip:
		f.stats.pushClips++
		var scratch *fineScratch
		if len(f.freeScratches) > 0 {
			scratch = f.freeScratches[len(f.freeScratches)-1]
			f.freeScratches = f.freeScratches[:len(f.freeScratches)-1]
		} else {
			scratch = f.newScratch()
		}
		f.layers = append(f.layers, fineLayer{
			scratch: scratch,
		})
	case cmdPopClip:
		f.stats.popClips++
		f.freeScratches = append(f.freeScratches, f.layers[len(f.layers)-1].scratch)
		f.layers = f.layers[:len(f.layers)-1]
	case cmdClipFill:
		f.clipFill(int(cmd.x), int(cmd.width), cmd.blend, cmd.opacity)
	case cmdClipAlphaFill:
		aslice := cmd.alphas[cmd.alphaIdx:]
		f.clipAlphaFill(int(cmd.x), int(cmd.width), aslice, cmd.blend, cmd.opacity)
	default:
		panic(fmt.Sprintf("unreachable: %T", cmd))
	}
}

var (
	memsetColumnsFp = memsetColumnsNative

	fillComplexFp = fineFillComplexNative
)

func (f *fine) fill(x, width int, color Color) {
	l := f.topLayer()
	buf := l.scratch[x : x+width]

	if x == 0 && width == wideTileWidth {
		if color[3] == 1.0 {
			f.stats.fullWidthOpaqueClearFills++
			l.clear(color)
		} else if l.complex {
			f.stats.fullWidthComplexFills++
			fillComplexFp(buf, color)
		} else {
			f.stats.fullWidthTranslucentClearFills++
			oneMinusAlpha := 1.0 - color[3]
			color = Color{
				0: l.singleColor[0]*oneMinusAlpha + color[0],
				1: l.singleColor[1]*oneMinusAlpha + color[1],
				2: l.singleColor[2]*oneMinusAlpha + color[2],
				3: l.singleColor[3]*oneMinusAlpha + color[3],
			}
			l.clear(color)
		}
	} else {
		// If the tile isn't complex yet, it will be after we've processed this
		// fill. Materialize all the pixels that this fill isn't going to
		// overwrite.
		f.materialize(l, 0, x)
		f.materialize(l, x+width, wideTileWidth)

		if color[3] == 1.0 {
			f.stats.opaqueFills++
			// The fill color is opaque, so we use a fill function that doesn't care
			// about the background color.
			memsetColumnsFp(buf, color)
			l.complex = true
		} else if !l.complex {
			f.stats.simpleFills++
			// The tile is simple, which means the fill only has to blend colors
			// once, not for every pixel.
			oneMinusAlpha := 1.0 - color[3]
			color = Color{
				0: l.singleColor[0]*oneMinusAlpha + color[0],
				1: l.singleColor[1]*oneMinusAlpha + color[1],
				2: l.singleColor[2]*oneMinusAlpha + color[2],
				3: l.singleColor[3]*oneMinusAlpha + color[3],
			}
			memsetColumnsFp(buf, color)
			l.complex = true
		} else {
			f.stats.complexFills++
			// Do the general, per-pixel fill.
			fillComplexFp(buf, color)
		}
	}
}

func (f *fine) clipFill(x, width int, blend BlendMode, opacity float32) {
	if n := len(f.layers); n < 2 {
		panic(fmt.Sprintf("internal error: trying to clipFill but we only have %d layers", n))
	}

	tos := &f.layers[len(f.layers)-1]
	nos := &f.layers[len(f.layers)-2]

	// If nos isn't complex yet, it will be after we've processed this fill.
	// Materialize all the pixels that this fill isn't going to overwrite.
	f.materialize(nos, 0, x)
	f.materialize(nos, x+width, wideTileWidth)

	dst := nos.scratch[x : x+width]
	src := tos.scratch[x : x+width]
	if !nos.complex && !tos.complex {
		f.stats.simpleSimpleClipFills++
		c := tos.singleColor
		c[0] *= opacity
		c[1] *= opacity
		c[2] *= opacity
		c[3] *= opacity
		blendSimpleSimple(dst, nos.singleColor, c, blend)
	} else {
		f.materialize(nos, x, x+width)
		f.materialize(tos, x, x+width)
		f.stats.complexComplexClipFills++
		blendComplexComplex(dst, src, nil, blend, opacity)
	}
	nos.complex = true
}

func (f *fine) clipAlphaFill(x, width int, alphas [][stripHeight]uint8, blend BlendMode, opacity float32) {
	f.stats.clipAlphaFills++
	tos := &f.layers[len(f.layers)-1]
	nos := &f.layers[len(f.layers)-2]

	// OPT implement handling of layer.complex
	f.materialize(tos, 0, wideTileWidth)
	f.materialize(nos, 0, wideTileWidth)
	tos.complex = true
	nos.complex = true

	// OPT(dh): instead of modifying the source in place, teach blend functions
	// how to apply alpha values.
	dst := nos.scratch[x : x+width]
	src := tos.scratch[x : x+width]
	blendComplexComplex(dst, src, alphas, blend, opacity)
}

func fineFillComplexNative(buf [][stripHeight]Color, color Color) {
	oneMinusAlpha := 1.0 - color[3]
	for x := range buf {
		col := &buf[x]
		for y := range col {
			col[y][0] = col[y][0]*oneMinusAlpha + color[0]
			col[y][1] = col[y][1]*oneMinusAlpha + color[1]
			col[y][2] = col[y][2]*oneMinusAlpha + color[2]
			col[y][3] = col[y][3]*oneMinusAlpha + color[3]
		}
	}
}

func (f *fine) alphaFill(x, width int, alphas [][stripHeight]uint8, color Color) {
	// OPT implement SIMD versions

	if len(alphas) < width {
		panic(fmt.Sprintf("internal error: got %d alphas for a width of %d",
			len(alphas), width))
	}
	color[0] *= (1.0 / 255.0)
	color[1] *= (1.0 / 255.0)
	color[2] *= (1.0 / 255.0)
	color[3] *= (1.0 / 255.0)

	l := f.topLayer()
	dst := l.scratch[x : x+width]

	if l.complex {
		f.stats.complexAlphaFills++
		for x := range dst {
			col := &dst[x]
			a := &alphas[x]
			for y := range col {
				maskAlpha := float32(a[y])
				oneMinusAlpha := 1.0 - maskAlpha*color[3]
				col[y][0] = col[y][0]*oneMinusAlpha + maskAlpha*color[0]
				col[y][1] = col[y][1]*oneMinusAlpha + maskAlpha*color[1]
				col[y][2] = col[y][2]*oneMinusAlpha + maskAlpha*color[2]
				col[y][3] = col[y][3]*oneMinusAlpha + maskAlpha*color[3]
			}
		}
	} else {
		f.stats.simpleAlphaFills++
		bg := l.singleColor
		f.materialize(l, 0, x)
		f.materialize(l, x+width, wideTileWidth)
		l.complex = true

		for x := range dst {
			col := &dst[x]
			a := &alphas[x]
			for y := range col {
				maskAlpha := float32(a[y])
				oneMinusAlpha := 1.0 - maskAlpha*color[3]
				col[y][0] = bg[0]*oneMinusAlpha + maskAlpha*color[0]
				col[y][1] = bg[1]*oneMinusAlpha + maskAlpha*color[1]
				col[y][2] = bg[2]*oneMinusAlpha + maskAlpha*color[2]
				col[y][3] = bg[3]*oneMinusAlpha + maskAlpha*color[3]
			}
		}
	}

}
