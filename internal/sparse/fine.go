// SPDX-FileCopyrightText: 2024 the Piet Authors
// SPDX-FileCopyrightText: 2025 Dominik Honnef and contributors
//
// SPDX-License-Identifier: MIT

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

	simpleStrips  uint64
	complexStrips uint64

	nostosSimpleClipFills         uint64
	nosSimpleClipFills            uint64
	tosSimpleOpaqueClipFills      uint64
	tosSimpleTranslucentClipFills uint64
	complexClipFills              uint64

	clipStrips uint64

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

		fmt.Sprintf("Simple strips: %d", s.simpleStrips),
		fmt.Sprintf("Complex strips: %d", s.complexStrips),

		fmt.Sprintf("PushClips: %d", s.pushClips),
		fmt.Sprintf("PopClips: %d", s.popClips),

		fmt.Sprintf("nos+tos simple clip fills: %d", s.nostosSimpleClipFills),
		fmt.Sprintf("nos simple clip fills: %d", s.nosSimpleClipFills),
		fmt.Sprintf("tos simple opaque clip fills: %d", s.tosSimpleOpaqueClipFills),
		fmt.Sprintf("tos simple translucent clip fills: %d", s.tosSimpleTranslucentClipFills),
		fmt.Sprintf("Complex clip fills: %d", s.complexClipFills),

		fmt.Sprintf("Clip strips: %d", s.clipStrips),

		fmt.Sprintf("Materialized pixels: %d", s.materializedPixels),
		fmt.Sprintf("Materialized layers: %d", s.materializedLayers),

		fmt.Sprintf("Simple packs: %d", s.simplePacks),
		fmt.Sprintf("Complex packs: %d", s.complexPacks),
	}

	return strings.Join(lines, "\n")
}

type fine struct {
	// the width and height of the output image, in pixels
	width, height int
	outBuf        []Color
	layers        []fineLayer

	stats fineStats
}

type fineLayer struct {
	scratch     *fineScratch
	singleColor Color
	// if complex is false, all pixels have the color stored in singleColor and
	// the contents of scratch may be undefined.
	complex bool
}

func newFine(width, height int, out []Color) *fine {
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

// pack writes the tile at (x, y) to the output buffer. x and y are tile
// indices, not pixels.
func (f *fine) pack(x, y int) {
	baseIdx := (y*stripHeight*f.width + x*wideTileWidth)

	maxi := max(0, min(wideTileWidth, f.width-x*wideTileWidth))
	maxj := max(0, min(stripHeight, f.height-y*stripHeight))

	out := f.outBuf[baseIdx:]
	_ = out[(maxj-1)*f.width+maxi-1]
	l := f.topLayer()
	if l.complex {
		f.stats.complexPacks++
		for j := range maxj {
			for i := range maxi {
				*safeish.Index(out, i) = l.scratch[i][j]
			}
			out = out[min(len(out), f.width):]
		}
	} else {
		f.stats.simplePacks++
		for range maxj {
			for i := range maxi {
				*safeish.Index(out, i) = l.singleColor
			}
			out = out[min(len(out), f.width):]
		}
	}
}

func (f *fine) materialize(l *fineLayer, start, end int) {
	if l.complex {
		return
	}

	f.stats.materializedPixels += uint64(end-start) * stripHeight
	f.stats.materializedLayers++

	var col [stripHeight]Color
	for i := range col {
		col[i] = l.singleColor
	}
	buf := l.scratch[start:end]
	for x := range buf {
		buf[x] = col
	}
}

func (f *fine) runCmd(cmd cmd, alphas [][stripHeight]uint8) {
	switch cmd.typ {
	case cmdFill:
		f.fill(int(cmd.x), int(cmd.width), cmd.color)
	case cmdStrip:
		aslice := alphas[cmd.alphaIdx:]
		f.strip(int(cmd.x), int(cmd.width), aslice, cmd.color)
	case cmdPushClip:
		f.stats.pushClips++
		// OPT: reuse layers that were popped
		f.layers = append(f.layers, fineLayer{
			scratch: f.newScratch(),
		})
	case cmdPopClip:
		f.stats.popClips++
		f.layers = f.layers[:len(f.layers)-1]
	case cmdClipFill:
		f.clipFill(int(cmd.x), int(cmd.width))
	case cmdClipStrip:
		aslice := alphas[cmd.alphaIdx:]
		f.clipStrip(int(cmd.x), int(cmd.width), aslice)
	default:
		panic(fmt.Sprintf("unreachable: %T", cmd))
	}
}

var (
	fillSolidFp   = fineFillSolidNative
	fillSimpleFp  = fineFillSimpleNative
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
				0: color[0] + oneMinusAlpha*l.singleColor[0],
				1: color[1] + oneMinusAlpha*l.singleColor[1],
				2: color[2] + oneMinusAlpha*l.singleColor[2],
				3: color[3] + oneMinusAlpha*l.singleColor[3],
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
			fillSolidFp(buf, color)
			l.complex = true
		} else if !l.complex {
			f.stats.simpleFills++
			// The tile is simple, which means the fill only has to blend colors
			// once, not for every pixel.
			fillSimpleFp(buf, color, l.singleColor)
			l.complex = true
		} else {
			f.stats.complexFills++
			// Do the general, per-pixel fill.
			fillComplexFp(buf, color)
		}
	}
}

func (f *fine) clipFill(x, width int) {
	if x == 0 && width == wideTileWidth {
		// This shouldn't be possible because we don't push a clip when the
		// entire wide tile is inside the clipped area. Though this will change
		// once we support different blend modes.
		panic("internal error: clipFill for whole wide tile")
	}
	if n := len(f.layers); n < 2 {
		panic(fmt.Sprintf("internal error: trying to clipFill but we only have %d layers", n))
	}

	tos := &f.layers[len(f.layers)-1]
	nos := &f.layers[len(f.layers)-2]

	// If nos isn't complex yet, it will be after we've processed this fill.
	// Materialize all the pixels that this fill isn't going to overwrite.
	f.materialize(nos, 0, x)
	f.materialize(nos, x+width, wideTileWidth)

	// OPT see if storing nos and tos fields in local variables reduces memory
	// bandwidth significantly. Go might well assume that nos.scratch aliases
	// stuff.
	switch {
	case !nos.complex && !tos.complex:
		f.stats.nostosSimpleClipFills++
		// OPT add SIMD version
		oneMinusAlpha := 1.0 - tos.singleColor[3]
		color := Color{
			nos.singleColor[0]*oneMinusAlpha + tos.singleColor[0],
			nos.singleColor[1]*oneMinusAlpha + tos.singleColor[1],
			nos.singleColor[2]*oneMinusAlpha + tos.singleColor[2],
			nos.singleColor[3]*oneMinusAlpha + tos.singleColor[3],
		}
		var col [stripHeight]Color
		for i := range col {
			col[i] = color
		}
		for i := range width {
			nos.scratch[x+i] = col
		}
	case !nos.complex:
		f.stats.nosSimpleClipFills++
		// OPT add SIMD version
		for i := range width {
			for j := range stripHeight {
				oneMinusAlpha := 1.0 - tos.scratch[x+i][j][3]
				nos.scratch[x+i][j][0] = nos.singleColor[0]*oneMinusAlpha + tos.scratch[x+i][j][0]
				nos.scratch[x+i][j][1] = nos.singleColor[1]*oneMinusAlpha + tos.scratch[x+i][j][1]
				nos.scratch[x+i][j][2] = nos.singleColor[2]*oneMinusAlpha + tos.scratch[x+i][j][2]
				nos.scratch[x+i][j][3] = nos.singleColor[3]*oneMinusAlpha + tos.scratch[x+i][j][3]
			}
		}
	case !tos.complex:
		// OPT add SIMD version
		if tos.singleColor[3] == 1 {
			f.stats.tosSimpleOpaqueClipFills++
			var col [stripHeight]Color
			for i := range col {
				col[i] = tos.singleColor
			}
			for i := range width {
				nos.scratch[x+i] = col
			}
		} else {
			f.stats.tosSimpleTranslucentClipFills++
			oneMinusAlpha := 1.0 - tos.singleColor[3]
			for i := range width {
				for j := range stripHeight {
					nos.scratch[x+i][j][0] = nos.scratch[x+i][j][0]*oneMinusAlpha + tos.singleColor[0]
					nos.scratch[x+i][j][1] = nos.scratch[x+i][j][1]*oneMinusAlpha + tos.singleColor[1]
					nos.scratch[x+i][j][2] = nos.scratch[x+i][j][2]*oneMinusAlpha + tos.singleColor[2]
					nos.scratch[x+i][j][3] = nos.scratch[x+i][j][3]*oneMinusAlpha + tos.singleColor[3]
				}
			}
		}
	default:
		// OPT add SIMD version
		f.stats.complexClipFills++
		for i := range width {
			for j := range stripHeight {
				oneMinusAlpha := 1.0 - tos.scratch[x+i][j][3]
				nos.scratch[x+i][j][0] = nos.scratch[x+i][j][0]*oneMinusAlpha + tos.scratch[x+i][j][0]
				nos.scratch[x+i][j][1] = nos.scratch[x+i][j][1]*oneMinusAlpha + tos.scratch[x+i][j][1]
				nos.scratch[x+i][j][2] = nos.scratch[x+i][j][2]*oneMinusAlpha + tos.scratch[x+i][j][2]
				nos.scratch[x+i][j][3] = nos.scratch[x+i][j][3]*oneMinusAlpha + tos.scratch[x+i][j][3]
			}
		}
	}

	nos.complex = true
}

func (f *fine) clipStrip(x, width int, alphas [][stripHeight]uint8) {
	f.stats.clipStrips++
	tos := &f.layers[len(f.layers)-1]
	nos := &f.layers[len(f.layers)-2]

	// OPT implement handling of layer.complex
	f.materialize(tos, 0, wideTileWidth)
	f.materialize(nos, 0, wideTileWidth)
	tos.complex = true
	nos.complex = true

	dst := nos.scratch[x : x+width]
	src := tos.scratch[x : x+width]
	for x := range dst {
		col := &dst[x]
		a := &alphas[x]
		for y := range col {
			maskAlpha := float32(a[y]) * (1.0 / 255.0)
			oneMinusAlpha := 1.0 - maskAlpha*src[x][y][3]
			col[y][0] = col[y][0]*oneMinusAlpha + maskAlpha*src[x][y][0]
			col[y][1] = col[y][1]*oneMinusAlpha + maskAlpha*src[x][y][1]
			col[y][2] = col[y][2]*oneMinusAlpha + maskAlpha*src[x][y][2]
			col[y][3] = col[y][3]*oneMinusAlpha + maskAlpha*src[x][y][3]
		}
	}
}

func fineFillSolidNative(buf [][stripHeight]Color, color Color) {
	for x := range buf {
		col := &buf[x]
		for y := range col {
			col[y] = color
		}
	}
}

func fineFillSimpleNative(buf [][stripHeight]Color, color Color, bg Color) {
	oneMinusAlpha := 1.0 - color[3]
	color = Color{
		0: color[0] + oneMinusAlpha*bg[0],
		1: color[1] + oneMinusAlpha*bg[1],
		2: color[2] + oneMinusAlpha*bg[2],
		3: color[3] + oneMinusAlpha*bg[3],
	}
	var col [stripHeight]Color
	for i := range col {
		col[i] = color
	}
	for x := range buf {
		buf[x] = col
	}
}

func fineFillComplexNative(buf [][stripHeight]Color, color Color) {
	oneMinusAlpha := 1.0 - color[3]
	for x := range buf {
		col := &buf[x]
		for y := range col {
			col[y][0] = color[0] + oneMinusAlpha*col[y][0]
			col[y][1] = color[1] + oneMinusAlpha*col[y][1]
			col[y][2] = color[2] + oneMinusAlpha*col[y][2]
			col[y][3] = color[3] + oneMinusAlpha*col[y][3]
		}
	}
}

func (f *fine) strip(x, width int, alphas [][stripHeight]uint8, color Color) {
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
		f.stats.complexStrips++
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
		f.stats.simpleStrips++
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
