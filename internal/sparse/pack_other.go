// SPDX-FileCopyrightText: 2026 Dominik Honnef and contributors
//
// SPDX-License-Identifier: MIT

//go:build !amd64 || noasm || !goexperiment.simd

package sparse

func linearRgbaF32ToSrgbU8One(in [4]float32, unpremul bool) [4]uint8 {
	return linearRgbaF32ToSrgbU8_LUT_Scalar_One(in, unpremul)
}

func packUint8SRGB(
	in *WideTileBuffer,
	out [][4]uint8,
	stride int,
	outWidth int,
	outHeight int,
	unpremul bool,
) {
	packUint8SRGB_LUT_Scalar(in, out, stride, outWidth, outHeight, unpremul)
}
