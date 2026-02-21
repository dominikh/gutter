// SPDX-FileCopyrightText: 2026 Dominik Honnef and contributors
//
// SPDX-License-Identifier: MIT

//go:build !purego && !amd64.v3

package sparse

import (
	"simd/archsimd"
)

var hasAVX2AndFMA3 = archsimd.X86.AVX2() && archsimd.X86.FMA()

func linearRgbaF32ToSrgbU8(in [][4]float32, out [][4]uint8, unpremul bool) {
	// Gracefully handly out being too small.
	n := min(len(in), len(out))

	// We always want to use the same algorithm (LUT or polynomial
	// approximation) to ensure consistent error and rounding, even when the
	// number of elements in a call is too small for SIMD/when processing the tail.
	if hasAVX2AndFMA3 {
		// AVX implementations operate on 32 elements at a time and do no bounds
		// checks.
		nr := (n / 32) * 32

		// AMD Piledriver supports FMA3 but not AVX2 and we could make use
		// of it in the scalar implementation, but it's not worth the
		// effort. There are no relevant CPUs that have AVX2 but no FMA3.

		if nr > 0 {
			linearRgbaF32ToSrgbU8_Polynomial_AVX2(in[:nr], out[:nr], unpremul)
		}
		linearRgbaF32ToSrgbU8_Polynomial_Scalar(in[nr:n], out[nr:n], unpremul)
	} else {
		linearRgbaF32ToSrgbU8_LUT_Scalar(in[:n], out[:n], unpremul)
	}
}

func linearRgbaF32ToSrgbU8One(in [4]float32, unpremul bool) [4]uint8 {
	if hasAVX2AndFMA3 {
		return linearRgbaF32ToSrgbU8_Polynomial_Scalar_One(in, unpremul)
	} else {
		return linearRgbaF32ToSrgbU8_LUT_Scalar_One(in, unpremul)
	}
}
