// SPDX-FileCopyrightText: 2024 the Piet Authors
// SPDX-FileCopyrightText: 2025 Dominik Honnef and contributors
//
// SPDX-License-Identifier: MIT

package sparse

import (
	"fmt"
	"unsafe"

	"honnef.co/go/safeish"
)

// [x][y]Color
type fineScratch = [wideTileWidth][stripHeight]Color

type fine struct {
	// the width and height of the output image, in pixels
	width, height int
	outBuf        []Color
	layers        []fineLayer
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

func (f *fine) clear(c Color) {
	l := f.topLayer()
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
		for j := range maxj {
			for i := range maxi {
				*safeish.Index(out, i) = l.scratch[i][j]
			}
			out = out[min(len(out), f.width):]
		}
	} else {
		for range maxj {
			for i := range maxi {
				*safeish.Index(out, i) = l.singleColor
			}
			out = out[min(len(out), f.width):]
		}
	}
}

func (l *fineLayer) materialize(start, end int) {
	if l.complex {
		return
	}

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
		// OPT: reuse layers that were popped
		f.layers = append(f.layers, fineLayer{
			scratch: f.newScratch(),
		})
	case cmdPopClip:
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
			f.clear(color)
		} else if l.complex {
			fillComplexFp(buf, color)
		} else {
			oneMinusAlpha := 1.0 - color[3]
			color = Color{
				0: color[0] + oneMinusAlpha*l.singleColor[0],
				1: color[1] + oneMinusAlpha*l.singleColor[1],
				2: color[2] + oneMinusAlpha*l.singleColor[2],
				3: color[3] + oneMinusAlpha*l.singleColor[3],
			}
			f.clear(color)
		}
	} else {
		if !l.complex {
			// If the tile isn't complex yet, it will be after we've processed this
			// fill. Materialize all the pixels that this fill isn't going to
			// overwrite.
			l.materialize(0, x)
			l.materialize(x+width, wideTileWidth)
		}

		if color[3] == 1.0 {
			// The fill color is opaque, so we use a fill function that doesn't care
			// about the background color.
			fillSolidFp(buf, color)
			l.complex = true
		} else if !l.complex {
			// The tile is simple, which means the fill only has to blend colors
			// once, not for every pixel.
			fillSimpleFp(buf, color, l.singleColor)
			l.complex = true
		} else {
			// Do the general, per-pixel fill.
			fillComplexFp(buf, color)
		}
	}
}

func (f *fine) clipFill(x, width int) {
	tos := &f.layers[len(f.layers)-1]
	nos := &f.layers[len(f.layers)-2]

	// OPT implement handling of layer.complex
	tos.materialize(0, wideTileWidth)
	nos.materialize(0, wideTileWidth)
	tos.complex = true
	nos.complex = true

	// OPT add SIMD version
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

func (f *fine) clipStrip(x, width int, alphas [][stripHeight]uint8) {
	tos := &f.layers[len(f.layers)-1]
	nos := &f.layers[len(f.layers)-2]

	// OPT implement handling of layer.complex
	tos.materialize(0, wideTileWidth)
	nos.materialize(0, wideTileWidth)
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
		bg := l.singleColor
		l.materialize(0, x)
		l.materialize(x+width, wideTileWidth)
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
