// SPDX-FileCopyrightText: 2026 Dominik Honnef and contributors
//
// SPDX-License-Identifier: MIT

//go:build !purego

package sparse

import "honnef.co/go/gutter/internal/arch"

func init() {
	linearRgbaF32ToSrgbU8Tests = append(linearRgbaF32ToSrgbU8Tests,
		srgbTest{
			name:     "polynomial",
			instr:    "avx",
			maxError: 0.5221,
			fn:       linearRgbaF32ToSrgbU8_Polynomial_AVX2,
			disabled: !(arch.AVX2() && arch.FMA()),
		},
	)
}
