// SPDX-FileCopyrightText: 2026 Dominik Honnef and contributors
//
// SPDX-License-Identifier: MIT

//go:build !purego

package sparse

import "honnef.co/go/gutter/internal/arch"

func init() {
	packUint8SrgbTests = append(packUint8SrgbTests,
		packUint8SrgbTest{
			name:     "polynomial",
			instr:    "avx",
			maxError: 0.5221,
			fn:       packUint8SRGB_AVX2,
			disabled: !(arch.AVX2() && arch.FMA()),
		},
	)
}
