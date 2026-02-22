// SPDX-FileCopyrightText: 2026 Dominik Honnef and contributors
//
// SPDX-License-Identifier: MIT

//go:build purego || !amd64

package sparse

import "honnef.co/go/gutter/gfx"

func linearRgbaF32ToSrgbU8(
	in *WideTileBuffer,
	out *[wideTileWidth][stripHeight][4]uint8,
	unpremul bool,
) {
	linearRgbaF32ToSrgbU8_LUT_Scalar(in, out, unpremul)
}

func linearRgbaF32ToSrgbU8One(in gfx.PlainColor, unpremul bool) [4]uint8 {
	return linearRgbaF32ToSrgbU8_LUT_Scalar_One(in, unpremul)
}
