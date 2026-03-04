// SPDX-FileCopyrightText: 2025 Dominik Honnef and contributors
//
// SPDX-License-Identifier: MIT

package sparse

import (
	"honnef.co/go/gutter/debug"
	"honnef.co/go/gutter/gfx"
)

type packUint8Fn func(
	in *WideTileBuffer,
	out [][4]uint8,
	stride int,
	outWidth int,
	outHeight int,
	unpremul bool,
)

type packUint8FnSimd func(
	in *WideTileBuffer,
	out *[4]uint8,
	stride int,
	outWidth int,
	outHeight int,
	unpremul bool,
)

type Packer interface {
	PackSimple(x0, y0, x1, y1 uint16, c gfx.PlainColor)
	PackComplex(x0, y0, x1, y1 uint16, tile *WideTileBuffer)
}

type PackerUint8SRGB struct {
	Out    [][4]uint8
	Width  int
	Height int
	// PremulAlpha applies alpha premultiplication to the electrical color
	// values.
	PremulAlpha bool
}

func clamp(x float32, lo, hi float32) float32 {
	// This is faster than max(min(x, hi), lo), but still produces more
	// instructions than necessary. Ideally, this would just be MINSS and MAXSS.
	// See https://golang.org/issue/72831
	if x < lo {
		return lo
	} else if x > hi {
		return hi
	} else {
		return x
	}
}

func (p *PackerUint8SRGB) PackSimple(x0, y0, x1, y1 uint16, c gfx.PlainColor) {
	// x0 and y0 are guaranteed to be in bounds, which means that even after
	// this, x1 and y1 are >= x0 and y0 and the computation of outWidth and
	// outHeight cannot wrap around.
	x1 = min(x1, uint16(p.Width))
	y1 = min(y1, uint16(p.Height))
	outWidth := x1 - x0
	outHeight := y1 - y0

	baseIdx := int(y0)*p.Width + int(x0)
	out := p.Out[baseIdx:]

	// This doesn't do proper gamut mapping. Doing it would be far too slow.
	px := linearRgbaF32ToSrgbU8One(c, !p.PremulAlpha)

	for range outHeight {
		row := out[:min(len(out), int(outWidth))]
		memsetUint8Pixels(row, px)
		out = out[min(uint(len(out)), uint(p.Width)):]
	}
}

func (p *PackerUint8SRGB) PackComplex(x0, y0, x1, y1 uint16, src *WideTileBuffer) {
	// src is a single wide tile, stored in column major order.
	//
	// The output buffer is the whole window's buffer, in row major order. It's
	// [p.Height][p.Width][4]uint8
	//
	// This method writes a single wide tile to the buffer, covering the buffer
	// region (x0, y0)--(x1, y1), possibly truncated to the buffer's bounds.

	x1 = min(x1, uint16(p.Width))
	y1 = min(y1, uint16(p.Height))
	outWidth := x1 - x0
	outHeight := y1 - y0
	baseIdx := int(y0)*p.Width + int(x0)
	packUint8SRGB(
		src,
		p.Out[baseIdx:],
		p.Width,
		int(outWidth),
		int(outHeight),
		!p.PremulAlpha,
	)
}

type PackerFloat32 struct {
	Out    []gfx.PlainColor
	Width  int
	Height int
}

func (p *PackerFloat32) PackSimple(x0, y0, x1, y1 uint16, c gfx.PlainColor) {
	x1 = min(x1, uint16(p.Width))
	y1 = min(y1, uint16(p.Height))
	outWidth := x1 - x0
	outHeight := y1 - y0

	baseIdx := int(y0)*p.Width + int(x0)
	out := p.Out[baseIdx:]
	for range outHeight {
		row := out[:min(len(out), int(outWidth))]
		for x := range row {
			row[x] = c
		}
		out = out[min(uint(len(out)), uint(p.Width)):]
	}
}

func (p *PackerFloat32) PackComplex(x0, y0, x1, y1 uint16, src *WideTileBuffer) {
	x1 = min(x1, uint16(p.Width))
	y1 = min(y1, uint16(p.Height))
	outWidth := x1 - x0
	outHeight := y1 - y0

	baseIdx := int(y0)*p.Width + int(x0)
	out := p.Out[baseIdx:]
	for y := range outHeight {
		row := out[:min(len(out), int(outWidth))]
		for x := range row {
			row[x] = gfx.PlainColor{src[0][x][y], src[1][x][y], src[2][x][y], src[3][x][y]}
		}
		out = out[min(uint(len(out)), uint(p.Width)):]
	}
}

type PackerUint16 struct {
	Out         [][4]uint16
	Width       int
	Height      int
	PremulAlpha bool
}

func (p *PackerUint16) PackSimple(x0, y0, x1, y1 uint16, c gfx.PlainColor) {
	x1 = min(x1, uint16(p.Width))
	y1 = min(y1, uint16(p.Height))
	outWidth := x1 - x0
	outHeight := y1 - y0

	baseIdx := int(y0)*p.Width + int(x0)
	out := p.Out[baseIdx:]

	var px [4]uint16
	if p.PremulAlpha {
		// Colors are already stored using premultiplied alpha. Since we're not
		// applying any gamma we don't have to unpremultiply.
		px = [4]uint16{
			uint16(clamp(c[0], 0, 1)*65535 + 0.5),
			uint16(clamp(c[1], 0, 1)*65535 + 0.5),
			uint16(clamp(c[2], 0, 1)*65535 + 0.5),
			uint16(clamp(c[3], 0, 1)*65535 + 0.5),
		}
	} else {
		// We fold unpremultiplying and scaling into a single factor.
		alpha := max(c[3], 1e-10) / 65535
		px = [4]uint16{
			uint16(clamp(c[0], 0, 1)/alpha + 0.5),
			uint16(clamp(c[1], 0, 1)/alpha + 0.5),
			uint16(clamp(c[2], 0, 1)/alpha + 0.5),
			uint16(clamp(c[3], 0, 1)*65535 + 0.5),
		}
	}

	for range outHeight {
		row := out[:min(len(out), int(outWidth))]
		for x := range row {
			row[x] = px
		}
		out = out[min(uint(len(out)), uint(p.Width)):]
	}
}

func (p *PackerUint16) PackComplex(x0, y0, x1, y1 uint16, src *WideTileBuffer) {
	x1 = min(x1, uint16(p.Width))
	y1 = min(y1, uint16(p.Height))
	outWidth := x1 - x0
	outHeight := y1 - y0

	baseIdx := int(y0)*p.Width + int(x0)
	out := p.Out[baseIdx:]

	if p.PremulAlpha {
		for y := range outHeight {
			row := out[:min(len(out), int(outWidth))]
			for x := range row {
				r, g, b, a := src[0][x][y], src[1][x][y], src[2][x][y], src[3][x][y]
				// Colors are already stored using premultiplied alpha. Since
				// we're not applying any gamma we don't have to unpremultiply.
				row[x] = [4]uint16{
					uint16(clamp(r, 0, 1)*65535 + 0.5),
					uint16(clamp(g, 0, 1)*65535 + 0.5),
					uint16(clamp(b, 0, 1)*65535 + 0.5),
					uint16(clamp(a, 0, 1)*65535 + 0.5),
				}
			}
			out = out[min(uint(len(out)), uint(p.Width)):]
		}
	} else {
		for y := range outHeight {
			row := out[:min(len(out), int(outWidth))]
			for x := range row {
				r, g, b, a := src[0][x][y], src[1][x][y], src[2][x][y], src[3][x][y]
				// We fold unpremultiplying and scaling into a single factor.
				alpha := max(a, 1e-10) / 65535
				row[x] = [4]uint16{
					uint16(clamp(r, 0, 1)/alpha + 0.5),
					uint16(clamp(g, 0, 1)/alpha + 0.5),
					uint16(clamp(b, 0, 1)/alpha + 0.5),
					uint16(clamp(a, 0, 1)*65535 + 0.5),
				}
			}
			out = out[min(uint(len(out)), uint(p.Width)):]
		}
	}
}

func packUint8SRGB_LUT_Scalar(
	in *WideTileBuffer,
	out [][4]uint8,
	stride int,
	outWidth int,
	outHeight int,
	unpremul bool,
) {
	packUint8SRGB_Scalar(
		in,
		out,
		stride,
		outWidth,
		outHeight,
		unpremul,
		linearRgbaF32ToSrgbU8_LUT_Scalar_One,
	)
}

func packUint8SRGB_Polynomial_Scalar(
	in *WideTileBuffer,
	out [][4]uint8,
	stride int,
	outWidth int,
	outHeight int,
	unpremul bool,
) {
	packUint8SRGB_Scalar(
		in,
		out,
		stride,
		outWidth,
		outHeight,
		unpremul,
		linearRgbaF32ToSrgbU8_Polynomial_Scalar_One,
	)
}

func packUint8SRGB_Scalar(
	in *WideTileBuffer,
	out [][4]uint8,
	stride int,
	outWidth int,
	outHeight int,
	unpremul bool,
	fn func(px gfx.PlainColor, unpremul bool) [4]uint8,
) {
	debug.Assert(outWidth <= stride)
	debug.Assert(outWidth <= len(in[0]))
	debug.Assert(outHeight <= len(in[0][0]))

	for y := range outHeight {
		row := out[y*stride:][:outWidth]
		for x := range outWidth {
			px := gfx.PlainColor{in[0][x][y], in[1][x][y], in[2][x][y], in[3][x][y]}
			row[x] = fn(px, unpremul)
		}
	}
}

func packUint8SRGB_SIMD(
	in *WideTileBuffer,
	out [][4]uint8,
	stride int,
	outWidth int,
	outHeight int,
	unpremul bool,
	widthMultiple int,
	fn packUint8FnSimd,
) {
	debug.Assert(stride > 0)
	debug.Assert(outWidth > 0)
	debug.Assert(outHeight > 0)
	debug.Assert(outWidth <= stride)
	debug.Assert(outWidth <= len(in[0]))
	debug.Assert(outHeight <= len(in[0][0]))

	if outHeight == stripHeight {
		// The AVX2 implementation operates on 32 pixels at a time.
		w := outWidth / widthMultiple * widthMultiple

		// This check doesn't help the optimizer, but protects the assembly
		// implementation.
		_ = out[(outHeight-1)*stride+outWidth-1]
		fn(in, &out[0], stride, w, outHeight, unpremul)

		if w < outWidth {
			for y := range outHeight {
				row := out[y*stride:][:outWidth]
				// Convert w to uint to help BCE.
				for x := uint(w); x < uint(outWidth); x++ {
					px := gfx.PlainColor{in[0][x][y], in[1][x][y], in[2][x][y], in[3][x][y]}
					row[x] = linearRgbaF32ToSrgbU8_Polynomial_Scalar_One(px, unpremul)
				}
			}
		}
	} else {
		packUint8SRGB_Polynomial_Scalar(in, out, stride, outWidth, outHeight, unpremul)
	}
}
