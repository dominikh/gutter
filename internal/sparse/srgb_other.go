// SPDX-FileCopyrightText: 2026 Dominik Honnef and contributors
//
// SPDX-License-Identifier: MIT

//go:build purego || !amd64

package sparse

func linearRgbaF32ToSrgbU8(in [][4]float32, out [][4]uint8, unpremul bool) {
	linearRgbaF32ToSrgbU8_LUT_Scalar(in, out, unpremul)
}

func linearRgbaF32ToSrgbU8One(in [4]float32, unpremul bool) [4]uint8 {
	return linearRgbaF32ToSrgbU8_LUT_Scalar_One(in, unpremul)
}
