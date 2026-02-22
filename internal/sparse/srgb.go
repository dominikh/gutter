// SPDX-FileCopyrightText: 2025 Dominik Honnef and contributors
//
// SPDX-License-Identifier: MIT
// SPDX-FileAttributionText: https://github.com/thomcc/fast-srgb8

package sparse

import (
	"math"

	"honnef.co/go/gutter/gfx"
)

var toSrgbTable = [104]uint32{
	0x0073000d, 0x007a000d, 0x0080000d, 0x0087000d, 0x008d000d, 0x0094000d, 0x009a000d, 0x00a1000d,
	0x00a7001a, 0x00b4001a, 0x00c1001a, 0x00ce001a, 0x00da001a, 0x00e7001a, 0x00f4001a, 0x0101001a,
	0x010e0033, 0x01280033, 0x01410033, 0x015b0033, 0x01750033, 0x018f0033, 0x01a80033, 0x01c20033,
	0x01dc0067, 0x020f0067, 0x02430067, 0x02760067, 0x02aa0067, 0x02dd0067, 0x03110067, 0x03440067,
	0x037800ce, 0x03df00ce, 0x044600ce, 0x04ad00ce, 0x051400ce, 0x057b00c5, 0x05dd00bc, 0x063b00b5,
	0x06970158, 0x07420142, 0x07e30130, 0x087b0120, 0x090b0112, 0x09940106, 0x0a1700fc, 0x0a9500f2,
	0x0b0f01cb, 0x0bf401ae, 0x0ccb0195, 0x0d950180, 0x0e56016e, 0x0f0d015e, 0x0fbc0150, 0x10630143,
	0x11070264, 0x1238023e, 0x1357021d, 0x14660201, 0x156601e9, 0x165a01d3, 0x174401c0, 0x182401af,
	0x18fe0331, 0x1a9602fe, 0x1c1502d2, 0x1d7e02ad, 0x1ed4028d, 0x201a0270, 0x21520256, 0x227d0240,
	0x239f0443, 0x25c003fe, 0x27bf03c4, 0x29a10392, 0x2b6a0367, 0x2d1d0341, 0x2ebe031f, 0x304d0300,
	0x31d105b0, 0x34a80555, 0x37520507, 0x39d504c5, 0x3c37048b, 0x3e7c0458, 0x40a8042a, 0x42bd0401,
	0x44c20798, 0x488e071e, 0x4c1c06b6, 0x4f76065d, 0x52a50610, 0x55ac05cc, 0x5892058f, 0x5b590559,
	0x5e0c0a23, 0x631c0980, 0x67db08f6, 0x6c55087f, 0x70940818, 0x74a007bd, 0x787d076c, 0x7c330723,
}

func linearRgbaF32ToSrgbU8_LUT_Scalar_One(px gfx.PlainColor, unpremul bool) [4]uint8 {
	var out [4]uint8
	if unpremul {
		px[0] /= px[3]
		px[1] /= px[3]
		px[2] /= px[3]
	}
	for k, f := range px[:3] {
		const maxvBits = 0x3f7fffff // 1.0 - f32::EPSILON
		const minvBits = 0x39000000 // 2^(-13)
		minv := math.Float32frombits(minvBits)
		maxv := math.Float32frombits(maxvBits)
		// written like this to handle nans.
		if !(f > minv) {
			f = minv
		}
		if f > maxv {
			f = maxv
		}
		fu := math.Float32bits(f)
		// Safety: all input floats are clamped into the {minv, maxv} range, which
		// turns out in this case to guarantee that their bitwise reprs are clamped
		// to the {MINV_BITS, MAXV_BITS} range (guaranteed by the fact that
		// minv/maxv are the normal, finite, the same sign, and not zero).
		//
		// Because of that, the smallest result of `fu - MINV_BITS` is 0 (when `fu`
		// is `MINV_BITS`), and the largest is `0x067fffff`, (when `fu` is
		// `MAXV_BITS`). `0x067fffff >> 20` is 0x67, i.e. 103, and thus all possible
		// results are inbounds for the (104 item) table. This is all verified in
		// test code.
		//
		// Note that the compiler can't figure this out on it's own, so the
		// get_unchecked does help some.
		// OPT(dh): use safeish.Index?
		i := ((fu - minvBits) >> 20) // as usize;
		entry := toSrgbTable[i]
		// bottom 16 bits are bias, top 9 are scale.
		// lerp to the next highest mantissa bits.

		// We should use some local variables, but those cost inlining budget...
		out[k] = uint8((((entry >> 16) << 9) + (entry&0xffff)*((fu>>12)&0xff)) >> 16)
	}
	out[3] = uint8(px[3]*255 + 0.5)

	return out
}

func linearRgbaF32ToSrgbU8_LUT_Scalar(
	in *WideTileBuffer,
	out *[wideTileWidth][stripHeight][4]uint8,
	unpremul bool,
) {
	// OPT(dh): avoid bounds checks on out
	for x := range in {
		for y, px := range &in[x] {
			// OPT(dh): god I hope this inlines
			out[x][y] = linearRgbaF32ToSrgbU8_LUT_Scalar_One(px, unpremul)
		}
	}
}

func linearRgbaF32ToSrgbU8_Polynomial_Scalar_One(px gfx.PlainColor, unpremul bool) [4]uint8 {
	var out [4]uint8

	if unpremul {
		px[0] /= px[3]
		px[1] /= px[3]
		px[2] /= px[3]
	}

	for k, v := range px[:3] {
		if v <= 0.0031308 {
			lin := v * 12.92
			out[k] = uint8(lin*255 + 0.5)
			continue
		}

		x := v - 5.35862651e-04
		x2 := x * x
		even1 := x*-9.12795913e-01 + -2.88143143e-02
		even2 := x2*-7.29192910e-01 + even1
		odd1 := x*1.06133172e+00 + 1.40194533e+00
		odd2 := x2*2.07758287e-01 + odd1
		poly := odd2*float32(math.Sqrt(float64(x))) + even2
		out[k] = uint8(poly*255 + 0.5)
	}
	out[3] = uint8(px[3]*255 + 0.5)
	return out
}

func linearRgbaF32ToSrgbU8_Polynomial_Scalar(
	in *WideTileBuffer,
	out *[wideTileWidth][stripHeight][4]uint8,
	unpremul bool,
) {
	// See linearF32ToSrgbU8_Polynomial_AVX2_FMA3 in ./_asm/sparse_amd64_asm.go for an
	// explanation of this algorithm.

	for x := range in {
		for y, px := range &in[x] {
			// OPT(dh): god I hope this inlines
			out[x][y] = linearRgbaF32ToSrgbU8_Polynomial_Scalar_One(px, unpremul)
		}
	}
}
