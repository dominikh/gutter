// SPDX-FileCopyrightText: 2026 Dominik Honnef and contributors
//
// SPDX-License-Identifier: MIT

//go:build !purego && amd64.v3

package sparse

func linearRgbaF32ToSrgbU8(in [][4]float32, out [][4]uint8, unpremul bool) {
	// Gracefully handly out being too small.
	n := min(len(in), len(out))

	// AVX implementations operate on 32 elements at a time and do no bounds
	// checks.
	nr := (n / 32) * 32

	// When processing the remainder, we use the scalar version of the
	// polynomial implementation, even though it is slower than the scalar LUT
	// implementation, to ensure consistent approximation error and rounding
	// behavior for all pixels.
	if nr > 0 {
		linearRgbaF32ToSrgbU8_Polynomial_AVX2(in[:nr], out[:nr], unpremul)
	}
	linearRgbaF32ToSrgbU8_Polynomial_Scalar(in[nr:n], out[nr:n], unpremul)
}

func linearRgbaF32ToSrgbU8One(in [4]float32, unpremul bool) [4]uint8 {
	return linearRgbaF32ToSrgbU8_Polynomial_Scalar_One(in, unpremul)
}
