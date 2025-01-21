// SPDX-FileCopyrightText: 2024 the Piet Authors
// SPDX-FileCopyrightText: 2025 Dominik Honnef and contributors
//
// SPDX-License-Identifier: MIT

package sparse

import (
	"fmt"
)

type fine struct {
	width, height int
	outBuf        [][4]uint8
	scratch       [wideTileWidth * stripHeight][4]float32
}

func (f *fine) clear(c [4]float32) {
	for i := range f.scratch {
		f.scratch[i] = c
	}
}

func (f *fine) pack(x, y int) {
	if (x+1)*wideTileWidth > f.width {
		panic("unreachable")
	}
	if (y+1)*stripHeight > f.height {
		panic("unreachable")
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

			targetIdx := lineIdx + i
			out := &f.outBuf[targetIdx]
			src := f.scratch[(i*stripHeight + j)]
			*out = [4]uint8{
				uint8((src[0] * 255) + 0.5),
				uint8((src[1] * 255) + 0.5),
				uint8((src[2] * 255) + 0.5),
				uint8((src[3] * 255) + 0.5),
			}
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

func (f *fine) fill(x, width int, color [4]float32) {
	if color[3] == 1.0 {
		dst := f.scratch[x*stripHeight:][:stripHeight*width]
		for j := range dst {
			dst[j] = color
		}
	} else {
		oneMinusAlpha := 1.0 - color[3]
		dst := f.scratch[x*stripHeight:][:stripHeight*width]
		for j := range dst {
			dst[j][0] = color[0] + oneMinusAlpha*dst[j][0]
			dst[j][1] = color[1] + oneMinusAlpha*dst[j][1]
			dst[j][2] = color[2] + oneMinusAlpha*dst[j][2]
			dst[j][3] = color[3] + oneMinusAlpha*dst[j][3]
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
	for k := 0; k+4 <= len(dst); k += 4 {
		z := dst[k:][:4]
		a := alphas[n]
		n++
		for j := range 4 {
			maskAlpha := float32((a >> (j * 8)) & 0xFF)
			oneMinusAlpha := 1.0 - maskAlpha*color[3]
			z[j][0] = z[j][0]*oneMinusAlpha + maskAlpha*color[0]
			z[j][1] = z[j][1]*oneMinusAlpha + maskAlpha*color[1]
			z[j][2] = z[j][2]*oneMinusAlpha + maskAlpha*color[2]
			z[j][3] = z[j][3]*oneMinusAlpha + maskAlpha*color[3]
		}
	}
}
