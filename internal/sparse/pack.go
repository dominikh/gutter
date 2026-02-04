// SPDX-FileCopyrightText: 2025 Dominik Honnef and contributors
//
// SPDX-License-Identifier: MIT

package sparse

type PackerUint8SRGB struct {
	Out    [][4]byte
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

func (p *PackerUint8SRGB) PackSimple(x0, y0, x1, y1 uint16, c [4]float32) {
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
	var px [4]uint8
	if p.PremulAlpha {
		// OPT(dh): You'd expect a SIMD version to be faster, but
		// the mixing of SSE and AVX instructions in dev.simd is
		// fucking us. Also, the function doesn't inline, killing
		// performance even more.

		px = [4]uint8{
			uint8((float32ToSrgb8(c[0]))),
			uint8((float32ToSrgb8(c[1]))),
			uint8((float32ToSrgb8(c[2]))),
			uint8(c[3]*255 + 0.5),
		}
	} else {
		// OPT(dh): add SIMD path once it performs well

		// We fold unpremultiplying and scaling into a single factor.
		px = [4]uint8{
			float32ToSrgb8(c[0] / c[3]),
			float32ToSrgb8(c[1] / c[3]),
			float32ToSrgb8(c[2] / c[3]),
			uint8(c[3]*255 + 0.5),
		}
	}

	for range outHeight {
		row := out[:min(len(out), int(outWidth))]
		memsetUint8Pixels(row, px)
		out = out[min(uint(len(out)), uint(p.Width)):]
	}
}

func (p *PackerUint8SRGB) PackComplex(x0, y0, x1, y1 uint16, src [][4]float32) {
	// src is a single wide tile, stored in column major order. Right now it's a
	// [256][4][4]float32.
	//
	// The output buffer is the whole window's buffer, in row major order. It's
	// [p.Height][p.Width][4]uint8
	//
	// This method writes a single wide tile to the buffer, covering the buffer
	// region (x0, y0)--(x1, y1), possibly truncated to the buffer's bounds.

	srcHeight := y1 - y0
	// x0 and y0 are guaranteed to be in bounds, which means that even after
	// this, x1 and y1 are >= x0 and y0 and the computation of outWidth and
	// outHeight cannot wrap around.
	x1 = min(x1, uint16(p.Width))
	y1 = min(y1, uint16(p.Height))
	outWidth := x1 - x0
	outHeight := y1 - y0

	baseIdx := int(y0)*p.Width + int(x0)
	out := p.Out[baseIdx:]
	if p.PremulAlpha {
		// According to
		// https://web.archive.org/web/20250815165940/https://hacksoflife.blogspot.com/2022/06/srgb-pre-multiplied-alpha-and.html
		// whether the color needs to be premultiplied with alpha before or
		// after converting it to sRGB depends on the consumer of the data and
		// whether they will blend in linear or sRGB space. But we can't know
		// what our consumer (likely a display manager) will do… We'll assume
		// that they're modern and blend in linear space and premultiply our
		// colors before conversion to sRGB. Because our colors are already
		// stored premultiplied this saves us work, too.
		//
		// https://web.archive.org/web/20250829113330/https://ssp.impulsetrain.com/gamma-premult.html
		// covers the same topic and says that premultiplying before encoding in
		// sRGB is the right thing to do for GPU textures.

		for y := range outHeight {
			row := out[:min(len(out), int(outWidth))]
			for x := range row {
				// OPT(dh): You'd expect a SIMD version to be faster, but
				// the mixing of SSE and AVX instructions in dev.simd is
				// fucking us. Also, the function doesn't inline, killing
				// performance even more.

				px := &src[x*int(srcHeight)+int(y)]
				// This doesn't do proper gamut mapping. Doing it would be far too slow.
				row[x] = [4]uint8{
					float32ToSrgb8(px[0]),
					float32ToSrgb8(px[1]),
					float32ToSrgb8(px[2]),
					uint8(px[3]*255 + 0.5),
				}
			}
			out = out[min(uint(len(out)), uint(p.Width)):]
		}
	} else {
		for y := range outHeight {
			row := out[:min(len(out), int(outWidth))]
			for x := range row {
				// OPT(dh): add SIMD path once it performs well

				px := &src[x*int(srcHeight)+int(y)]
				// This doesn't do proper gamut mapping. Doing it would be far too slow.
				row[x] = [4]uint8{
					float32ToSrgb8(px[0] / px[3]),
					float32ToSrgb8(px[1] / px[3]),
					float32ToSrgb8(px[2] / px[3]),
					uint8(px[3]*255 + 0.5),
				}
			}
			out = out[min(uint(len(out)), uint(p.Width)):]
		}
	}
}

type PackerFloat32 struct {
	Out    [][4]float32
	Width  int
	Height int
}

func (p *PackerFloat32) PackSimple(x0, y0, x1, y1 uint16, c [4]float32) {
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

func (p *PackerFloat32) PackComplex(x0, y0, x1, y1 uint16, src [][4]float32) {
	srcHeight := y1 - y0
	x1 = min(x1, uint16(p.Width))
	y1 = min(y1, uint16(p.Height))
	outWidth := x1 - x0
	outHeight := y1 - y0

	baseIdx := int(y0)*p.Width + int(x0)
	out := p.Out[baseIdx:]
	for y := range outHeight {
		row := out[:min(len(out), int(outWidth))]
		for x := range row {
			px := src[x*int(srcHeight)+int(y)]
			row[x] = px
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

func (p *PackerUint16) PackSimple(x0, y0, x1, y1 uint16, c [4]float32) {
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

func (p *PackerUint16) PackComplex(x0, y0, x1, y1 uint16, src [][4]float32) {
	srcHeight := y1 - y0
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
				px := src[x*int(srcHeight)+int(y)]
				// Colors are already stored using premultiplied alpha. Since
				// we're not applying any gamma we don't have to unpremultiply.
				row[x] = [4]uint16{
					uint16(clamp(px[0], 0, 1)*65535 + 0.5),
					uint16(clamp(px[1], 0, 1)*65535 + 0.5),
					uint16(clamp(px[2], 0, 1)*65535 + 0.5),
					uint16(clamp(px[3], 0, 1)*65535 + 0.5),
				}
			}
			out = out[min(uint(len(out)), uint(p.Width)):]
		}
	} else {
		for y := range outHeight {
			row := out[:min(len(out), int(outWidth))]
			for x := range row {
				px := src[x*int(srcHeight)+int(y)]
				// We fold unpremultiplying and scaling into a single factor.
				alpha := max(px[3], 1e-10) / 65535
				row[x] = [4]uint16{
					uint16(clamp(px[0], 0, 1)/alpha + 0.5),
					uint16(clamp(px[1], 0, 1)/alpha + 0.5),
					uint16(clamp(px[2], 0, 1)/alpha + 0.5),
					uint16(clamp(px[3], 0, 1)*65535 + 0.5),
				}
			}
			out = out[min(uint(len(out)), uint(p.Width)):]
		}
	}
}
