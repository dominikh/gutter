// SPDX-FileCopyrightText: 2024 the Piet Authors
// SPDX-FileCopyrightText: 2025 Dominik Honnef and contributors
//
// SPDX-License-Identifier: MIT

package sparse

import (
	"fmt"
	"unsafe"
)

type fine struct {
	width, height int
	outBuf        [][4]float32
	scratch       *[wideTileWidth * stripHeight][4]float32
}

func newFine(width, height int, out [][4]float32) *fine {
	// Align scratch memory to this many bytes. This should match the largest
	// vector width that we use in assembly. Currently that is 32 for AVX.
	const align = 32

	var f fine
	scratch := make([]byte, unsafe.Sizeof(*f.scratch)+align)
	ptr := unsafe.Pointer(&scratch[0])
	alignedPtr := unsafe.Pointer(((uintptr(ptr) + align - 1) / align) * align)
	scratch2 := (*[wideTileWidth * stripHeight][4]float32)(alignedPtr)
	return &fine{width, height, out, scratch2}
}

func (f *fine) clear(c [4]float32) {
	for i := range f.scratch {
		f.scratch[i] = c
	}
}

func (f *fine) pack(x, y int) {
	if px, py := x*wideTileWidth, y*stripHeight; px > f.width || py > f.height {
		panic(fmt.Sprintf("tile (%d, %d) starts at pixel (%d, %d), which is out of bounds for size (%d, %d)",
			x, y, px, py, f.width, f.height))
	}
	baseIdx := (y*stripHeight*f.width + x*wideTileWidth)
	for j := range stripHeight {
		lineIdx := baseIdx + j*f.width

		// Continue if the current row is outside the range of the pixmap.
		if y*stripHeight+j >= f.height {
			break
		}

		for i := range wideTileWidth {
			// Abort if the current column is outside the range of the pixmap.
			if x*wideTileWidth+i >= f.width {
				break
			}

			f.outBuf[lineIdx+i] = f.scratch[(i*stripHeight + j)]
		}
	}
}

func (f *fine) runCmd(cmd cmd, alphas []uint32) {
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

func (f *fine) fill(x, width int, color [4]float32) {
	f.fillWithFp(x, width, color, fillFp)
}

func (f *fine) fillWithFp(x, width int, color [4]float32, fillFp func([][4]float32, [4]float32)) {
	buf := f.scratch[x*stripHeight:][:stripHeight*width]
	fillFp(buf, color)
}

func fineFillNative(buf [][4]float32, color [4]float32) {
	if color[3] == 1.0 {
		for j := range buf {
			buf[j] = color
		}
	} else {
		oneMinusAlpha := 1.0 - color[3]
		for j := range buf {
			buf[j][0] = color[0] + oneMinusAlpha*buf[j][0]
			buf[j][1] = color[1] + oneMinusAlpha*buf[j][1]
			buf[j][2] = color[2] + oneMinusAlpha*buf[j][2]
			buf[j][3] = color[3] + oneMinusAlpha*buf[j][3]
		}
	}
}

func (f *fine) strip(x, width int, alphas []uint32, color [4]float32) {
	if len(alphas) < width {
		panic(fmt.Sprintf("internal error: got %d alphas for a width of %d",
			len(alphas), width))
	}
	color[0] *= (1.0 / 255.0)
	color[1] *= (1.0 / 255.0)
	color[2] *= (1.0 / 255.0)
	color[3] *= (1.0 / 255.0)
	dst := f.scratch[x*stripHeight:][:stripHeight*width]
	n := 0
	for k := 0; k+stripHeight <= len(dst); k += stripHeight {
		z := dst[k:][:stripHeight]
		a := alphas[n]
		n++
		for j := range stripHeight {
			maskAlpha := float32((a >> (j * 8)) & 0xFF)
			oneMinusAlpha := 1.0 - maskAlpha*color[3]
			z[j][0] = z[j][0]*oneMinusAlpha + maskAlpha*color[0]
			z[j][1] = z[j][1]*oneMinusAlpha + maskAlpha*color[1]
			z[j][2] = z[j][2]*oneMinusAlpha + maskAlpha*color[2]
			z[j][3] = z[j][3]*oneMinusAlpha + maskAlpha*color[3]
		}
	}
}
