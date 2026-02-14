// SPDX-FileCopyrightText: 2026 Dominik Honnef and contributors
//
// SPDX-License-Identifier: MIT

//go:build !purego

package sparse

import (
	"simd/archsimd"
)

func init() {
	linearRgbaF32ToSrgbU8Tests = append(linearRgbaF32ToSrgbU8Tests,
		srgbTest{
			name:     "polynomial",
			instr:    "avx",
			maxError: 0.5221,
			fn:       linearRgbaF32ToSrgbU8_Polynomial_AVX2_FMA3,
			disabled: !(archsimd.X86.AVX2() && archsimd.X86.FMA()),
		},

		srgbTest{
			name:     "lut",
			instr:    "avx",
			maxError: 0.545,
			fn:       linearRgbaF32ToSrgbU8_LUT_AVX2,
			disabled: !archsimd.X86.AVX2(),
		},
	)
}
