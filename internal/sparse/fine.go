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

type fine struct {
	// the width and height of the output image, in pixels
	width, height int
	outBuf        []Color
	// [x][y]Color
	scratch     *[wideTileWidth][stripHeight]Color
	singleColor Color

	// if complex is false, all pixels have the color stored in singleColor and
	// the contents of scratch may be undefined.
	complex bool
}

func newFine(width, height int, out []Color) *fine {
	// Align scratch memory to this many bytes. This should match the largest
	// vector width that we use in assembly. Currently that is 32 for AVX. Has
	// to be a power of 2.
	const align = 32

	var f fine
	scratch := make([]byte, unsafe.Sizeof(*f.scratch)+align)
	ptr := unsafe.Pointer(&scratch[0])
	alignedPtr := unsafe.Pointer((uintptr(ptr) + align - 1) &^ (align - 1))
	scratch2 := (*[wideTileWidth][stripHeight]Color)(alignedPtr)
	return &fine{
		width:   width,
		height:  height,
		outBuf:  out,
		scratch: scratch2,
	}
}

func (f *fine) clear(c Color) {
	f.complex = false
	f.singleColor = c
}

// pack writes the tile at (x, y) to the output buffer. x and y are tile
// indices, not pixels.
func (f *fine) pack(x, y int) {
	baseIdx := (y*stripHeight*f.width + x*wideTileWidth)

	maxi := max(0, min(wideTileWidth, f.width-x*wideTileWidth))
	maxj := max(0, min(stripHeight, f.height-y*stripHeight))

	out := f.outBuf[baseIdx:]
	_ = out[(maxj-1)*f.width+maxi-1]
	if f.complex {
		for j := range maxj {
			for i := range maxi {
				*safeish.Index(out, i) = f.scratch[i][j]
			}
			out = out[min(len(out), f.width):]
		}
	} else {
		for range maxj {
			for i := range maxi {
				*safeish.Index(out, i) = f.singleColor
			}
			out = out[min(len(out), f.width):]
		}
	}
}

func (f *fine) materialize(start, end int) {
	var col [stripHeight]Color
	for i := range col {
		col[i] = f.singleColor
	}
	buf := f.scratch[start:end]
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
	buf := f.scratch[x : x+width]

	if x == 0 && width == wideTileWidth {
		// If the fill covers the whole tile, then the fill color is never opaque,
		// due to an optimization when building the command list. If the tile is
		// already complex, do a complex fill. Else, compute the new simple color.
		if f.complex {
			fillComplexFp(buf, color)
		} else {
			oneMinusAlpha := 1.0 - color[3]
			color = Color{
				0: color[0] + oneMinusAlpha*f.singleColor[0],
				1: color[1] + oneMinusAlpha*f.singleColor[1],
				2: color[2] + oneMinusAlpha*f.singleColor[2],
				3: color[3] + oneMinusAlpha*f.singleColor[3],
			}
			f.clear(color)
		}
	} else {
		if !f.complex {
			// If the tile isn't complex yet, it will be after we've processed this
			// fill. Materialize all the pixels that this fill isn't going to
			// overwrite.
			f.materialize(0, x)
			f.materialize(x+width, wideTileWidth)
		}

		if color[3] == 1.0 {
			// The fill color is opaque, so we use a fill function that doesn't care
			// about the background color.
			fillSolidFp(buf, color)
			f.complex = true
		} else if !f.complex {
			// The tile is simple, which means the fill only has to blend colors
			// once, not for every pixel.
			fillSimpleFp(buf, color, f.singleColor)
			f.complex = true
		} else {
			// Do the general, per-pixel fill.
			fillComplexFp(buf, color)
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

	dst := f.scratch[x : x+width]

	if f.complex {
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
		bg := f.singleColor
		f.materialize(0, x)
		f.materialize(x+width, wideTileWidth)
		f.complex = true

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
