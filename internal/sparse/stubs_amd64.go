// SPDX-FileCopyrightText: 2025 Dominik Honnef and contributors
//
// SPDX-License-Identifier: MIT

//go:build !noasm

package sparse

//go:noescape
func processOutOfBoundsWindingSSE(
	ymin float32,
	ymax float32,
	sign float32,
	locationWinding *[tileWidth][tileHeight]float32,
	accumulatedWinding *[tileHeight]float32,
)

//go:noescape
func computeAlphasNonZeroAVX(
	tail *[tileWidth][tileHeight]uint8,
	locationWinding *[tileWidth][tileHeight]float32,
)

//go:noescape
func packUint8SRGB_AVX2_Impl(
	in *WideTileBuffer,
	out *[4]uint8,
	stride int,
	outWidth int,
	outHeight int,
	unpremul bool,
)

//go:noescape
func gradientLUTGatherAVX2(
	dst0 *[stripHeight]float32,
	dst1 *[stripHeight]float32,
	dst2 *[stripHeight]float32,
	dst3 *[stripHeight]float32,
	lut *[4]float32,
	lutScale float32,
	tBuf *[stripHeight]float32,
	masks *[stripHeight]int32,
	width int,
)

//go:noescape
func gradientCascadeMergeAVX2(
	dst0 *[stripHeight]float32,
	dst1 *[stripHeight]float32,
	dst2 *[stripHeight]float32,
	dst3 *[stripHeight]float32,
	tBuf *[stripHeight]float32,
	sr *simdGradientRanges,
	masks *[stripHeight]int32,
	width int,
)

func packUint8SRGB_AVX2(
	in *WideTileBuffer,
	out [][4]uint8,
	stride int,
	outWidth int,
	outHeight int,
	unpremul bool,
) {
	packUint8SRGB_SIMD(
		in,
		out,
		stride,
		outWidth,
		outHeight,
		unpremul,
		32,
		packUint8SRGB_AVX2_Impl,
	)
}
