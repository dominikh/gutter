// SPDX-FileCopyrightText: 2024 the Piet Authors
// SPDX-FileCopyrightText: 2025 Dominik Honnef and contributors
//
// SPDX-License-Identifier: MIT

package sparse

import (
	"fmt"
)

//! Fine rasterization

// use crate::wide_tile::{Cmd, STRIP_HEIGHT, WIDE_TILE_WIDTH};

const STRIP_HEIGHT_F32 = STRIP_HEIGHT * 4

type fine struct {
	width, height int
	out_buf       []uint8
	// f32 RGBA pixels
	// That said, if we use u8, then this is basically a block of
	// untyped memory.
	scratch [WIDE_TILE_WIDTH * STRIP_HEIGHT * 4]float32
}

func (f *fine) clear(c [4]float32) {
	for i := 0; i < len(f.scratch); i += 4 {
		copy(f.scratch[i:], c[:])
	}
}

func (f *fine) pack(x, y int) {
	if (x+1)*WIDE_TILE_WIDTH > f.width {
		panic("unreachable")
	}
	if (y+1)*STRIP_HEIGHT > f.height {
		panic("unreachable")
	}
	base_ix := (y*STRIP_HEIGHT*f.width + x*WIDE_TILE_WIDTH) * 4
	for j := range STRIP_HEIGHT {
		line_ix := base_ix + j*f.width*4

		// Continue if the current row is outside the range of the pixmap.
		if y*STRIP_HEIGHT+j >= f.height {
			break
		}

		for i := range WIDE_TILE_WIDTH {
			// Abort if the current column is outside the range of the pixmap.
			if x*WIDE_TILE_WIDTH+i >= f.width {
				break
			}

			target_ix := line_ix + i*4
			out := f.out_buf[target_ix:][:4]
			for k, x := range f.scratch[(i*STRIP_HEIGHT+j)*4:][:4] {
				out[k] = uint8((x * 255) + 0.5)
			}
		}
	}
}

func (f *fine) run_cmd(cmd cmd, alphas []uint32) {
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
		dst := f.scratch[x*STRIP_HEIGHT_F32:][:STRIP_HEIGHT_F32*width]
		for j := 0; j < len(dst); j += 4 {
			copy(dst[j:], color[:])
		}
	} else {
		one_minus_alpha := 1.0 - color[3]
		dst := f.scratch[x*STRIP_HEIGHT_F32:][:STRIP_HEIGHT_F32*width]
		for j := 0; j < len(dst); j += 4 {
			for i := range 4 {
				dst[j+i] = color[i] + one_minus_alpha*dst[j+i]
			}
		}
	}
}

func (f *fine) strip(x, width int, alphas []uint32, color [4]float32) {
	if len(alphas) < width {
		panic(fmt.Sprintf("internal error: got %d alphas for a width of %d",
			len(alphas), width))
	}
	for i := range color {
		color[i] *= (1.0 / 255.0)
	}
	dst := f.scratch[x*STRIP_HEIGHT_F32:][:STRIP_HEIGHT_F32*width]
	n := 0
	for k := 0; k+16 <= len(dst); k += 16 {
		z := dst[k:][:16]
		a := alphas[n]
		n++
		for j := range 4 {
			mask_alpha := float32((a >> (j * 8)) & 0xFF)
			one_minus_alpha := 1.0 - mask_alpha*color[3]
			for i := range 4 {
				z[j*4+i] = z[j*4+i]*one_minus_alpha + mask_alpha*color[i]
			}
		}
	}
}
