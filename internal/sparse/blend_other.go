// SPDX-FileCopyrightText: 2026 Dominik Honnef and contributors
//
// SPDX-License-Identifier: MIT

//go:build !amd64 || !goexperiment.simd

package sparse

import "honnef.co/go/gutter/gfx"

func blendComplexComplex(
	dst Pixels,
	tos Pixels,
	alphas [][stripHeight]uint8,
	blend gfx.BlendMode,
	opacity float32,
) {
	blendComplexComplexScalar(dst, tos, alphas, blend, opacity)
}
