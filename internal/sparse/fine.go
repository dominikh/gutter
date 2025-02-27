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

const debugPack = false

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
	maxj := max(0, min(4, f.height-y*stripHeight))
	_ = f.outBuf[baseIdx+(maxj-1)*f.width+maxi-1]

	out := f.outBuf[baseIdx:]

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
				if debugPack {
					*safeish.Index(out, i) = Color{1, 0, 0, 1}
				} else {
					*safeish.Index(out, i) = f.singleColor
				}
			}
			out = out[min(len(out), f.width):]
		}
	}
}

func (f *fine) materialize(start, end int) {
	col := [4]Color{
		f.singleColor,
		f.singleColor,
		f.singleColor,
		f.singleColor,
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

var fillFp = fineFillNative

func (f *fine) fill(x, width int, color Color) {
	f.fillWithFp(x, width, color, fillFp)
}

func (f *fine) fillWithFp(x, width int, color Color, fillFp func(*fine, [][stripHeight]Color, Color)) {
	if x == 0 && width == wideTileWidth {
		if color[3] == 1.0 {
			f.clear(color)
			return
		} else if !f.complex {
			oneMinusAlpha := 1.0 - color[3]
			color = Color{
				0: color[0] + oneMinusAlpha*f.singleColor[0],
				1: color[1] + oneMinusAlpha*f.singleColor[1],
				2: color[2] + oneMinusAlpha*f.singleColor[2],
				3: color[3] + oneMinusAlpha*f.singleColor[3],
			}
			f.clear(color)
			return
		}
	} else if !f.complex {
		f.materialize(0, x)
		f.materialize(x+width, wideTileWidth)
		// Don't change state yet, the fill implementation can reduce blending
		// work if it knows the single color.
	}

	buf := f.scratch[x : x+width]
	fillFp(f, buf, color)

	if x != 0 || width != wideTileWidth {
		f.complex = true
	}
}

func fineFillNative(f *fine, buf [][stripHeight]Color, color Color) {
	if color[3] == 1.0 {
		for x := range buf {
			col := &buf[x]
			for y := range col {
				col[y] = color
			}
		}
	} else {
		oneMinusAlpha := 1.0 - color[3]
		if f.complex {
			for x := range buf {
				col := &buf[x]
				for y := range col {
					col[y][0] = color[0] + oneMinusAlpha*col[y][0]
					col[y][1] = color[1] + oneMinusAlpha*col[y][1]
					col[y][2] = color[2] + oneMinusAlpha*col[y][2]
					col[y][3] = color[3] + oneMinusAlpha*col[y][3]
				}
			}
		} else {
			color = Color{
				0: color[0] + oneMinusAlpha*f.singleColor[0],
				1: color[1] + oneMinusAlpha*f.singleColor[1],
				2: color[2] + oneMinusAlpha*f.singleColor[2],
				3: color[3] + oneMinusAlpha*f.singleColor[3],
			}
			col := [4]Color{color, color, color, color}
			for x := range buf {
				buf[x] = col
			}
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
