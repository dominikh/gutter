// SPDX-FileCopyrightText: 2026 Dominik Honnef and contributors
//
// SPDX-License-Identifier: MIT

//go:build !purego

package sparse

import (
	"honnef.co/go/gutter/internal/arch"
)

var hasAVX2AndFMA3 = arch.AVX2() && arch.FMA()

func packUint8SRGB(
	in *WideTileBuffer,
	out [][4]uint8,
	stride int,
	outWidth int,
	outHeight int,
	unpremul bool,
) {
	if arch.GOAMD64 >= 3 || hasAVX2AndFMA3 {
		packUint8SRGB_AVX2(in, out, stride, outWidth, outHeight, unpremul)
	} else {
		packUint8SRGB_LUT_Scalar(in, out, stride, outWidth, outHeight, unpremul)
	}
}

func linearRgbaF32ToSrgbU8One(in [4]float32, unpremul bool) [4]uint8 {
	if arch.GOAMD64 >= 3 || hasAVX2AndFMA3 {
		return linearRgbaF32ToSrgbU8_Polynomial_Scalar_One(in, unpremul)
	} else {
		return linearRgbaF32ToSrgbU8_LUT_Scalar_One(in, unpremul)
	}
}
