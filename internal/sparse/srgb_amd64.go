// SPDX-FileCopyrightText: 2026 Dominik Honnef and contributors
//
// SPDX-License-Identifier: MIT

//go:build !purego

package sparse

import (
	"honnef.co/go/gutter/gfx"
	"honnef.co/go/gutter/internal/arch"
)

var hasAVX2AndFMA3 = arch.AVX2() && arch.FMA()

func linearRgbaF32ToSrgbU8(
	in *WideTileBuffer,
	out *[wideTileWidth][stripHeight][4]uint8,
	unpremul bool,
) {

	// We always want to use the same algorithm (LUT or polynomial
	// approximation) to ensure consistent error and rounding, even when the
	// number of elements in a call is too small for SIMD/when processing the tail.
	if arch.GOAMD64 >= 3 || hasAVX2AndFMA3 {
		// AMD Piledriver supports FMA3 but not AVX2 and we could make use
		// of it in the scalar implementation, but it's not worth the
		// effort. There are no relevant CPUs that have AVX2 but no FMA3.

		linearRgbaF32ToSrgbU8_Polynomial_AVX2(in, out, unpremul)
	} else {
		linearRgbaF32ToSrgbU8_LUT_Scalar(in, out, unpremul)
	}
}

func linearRgbaF32ToSrgbU8One(in gfx.PlainColor, unpremul bool) [4]uint8 {
	if arch.GOAMD64 >= 3 || hasAVX2AndFMA3 {
		return linearRgbaF32ToSrgbU8_Polynomial_Scalar_One(in, unpremul)
	} else {
		return linearRgbaF32ToSrgbU8_LUT_Scalar_One(in, unpremul)
	}
}
