// SPDX-FileCopyrightText: 2026 Dominik Honnef and contributors
//
// SPDX-License-Identifier: MIT

//go:build !purego && !amd64.v3

package sparse

import (
	"simd/archsimd"
	"unsafe"
)

var linearRgbaF32ToSrgbU8ScalarImpl func(in [][4]float32, out [][4]uint8, unpremul bool)
var linearRgbaF32ToSrgbU8SIMDImpl func(in [][4]float32, out [][4]uint8, unpremul bool)
var linearRgbaF32ToSrgbU8ScalarOneImpl func(in [4]float32, unpremul bool) [4]uint8

func init() {
	// We always want to use the same algorithm (LUT or polynomial
	// approximation) to ensure consistent error and rounding, even when the
	// number of elements in a call is too small for SIMD.
	//
	// To avoid 3 levels of branching on each call to linearRgbaF32ToSrgbU8, we use
	// function pointers instead.
	if archsimd.X86.AVX2() {
		if archsimd.X86.FMA() {
			// AMD Piledriver supports FMA3 but not AVX2 and we could make use
			// of it in the scalar implementation, but it's not worth the
			// effort.

			linearRgbaF32ToSrgbU8SIMDImpl = linearRgbaF32ToSrgbU8_Polynomial_AVX2_FMA3
			linearRgbaF32ToSrgbU8ScalarImpl = linearRgbaF32ToSrgbU8_Polynomial_Scalar
			linearRgbaF32ToSrgbU8ScalarOneImpl = linearRgbaF32ToSrgbU8_Polynomial_Scalar_One
		} else {
			linearRgbaF32ToSrgbU8SIMDImpl = linearRgbaF32ToSrgbU8_LUT_AVX2
			linearRgbaF32ToSrgbU8ScalarImpl = linearRgbaF32ToSrgbU8_LUT_Scalar
			linearRgbaF32ToSrgbU8ScalarOneImpl = linearRgbaF32ToSrgbU8_LUT_Scalar_One
		}
	} else {
		linearRgbaF32ToSrgbU8SIMDImpl = linearRgbaF32ToSrgbU8_LUT_Scalar
		linearRgbaF32ToSrgbU8ScalarImpl = linearRgbaF32ToSrgbU8_LUT_Scalar
		linearRgbaF32ToSrgbU8ScalarOneImpl = linearRgbaF32ToSrgbU8_LUT_Scalar_One
	}
}

func linearRgbaF32ToSrgbU8(in [][4]float32, out [][4]uint8, unpremul bool) {
	// Gracefully handly out being too small.
	n := min(len(in), len(out))

	// AVX implementations operate on 32 elements at a time and do no bounds
	// checks.
	nr := (n / 32) * 32

	// Don't cause our arguments to escape even though we use function pointers.
	in2 := *(*[][4]float32)(noEscape(unsafe.Pointer(&in)))
	out2 := *(*[][4]uint8)(noEscape(unsafe.Pointer(&out)))

	if nr > 0 {
		linearRgbaF32ToSrgbU8SIMDImpl(in2[:nr], out2[:nr], unpremul)
	}
	linearRgbaF32ToSrgbU8ScalarImpl(in2[nr:n], out2[nr:n], unpremul)
}

func linearRgbaF32ToSrgbU8One(in [4]float32, unpremul bool) [4]uint8 {
	return linearRgbaF32ToSrgbU8ScalarOneImpl(in, unpremul)
}
