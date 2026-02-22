// SPDX-FileCopyrightText: 2025 Dominik Honnef and contributors
//
// SPDX-License-Identifier: MIT

//go:build !purego

package sparse

import "honnef.co/go/gutter/gfx"

//go:noescape
func memsetColumnsAVX(buf [][stripHeight]gfx.PlainColor, color gfx.PlainColor)

//go:noescape
func memsetColumnsSSE(buf [][stripHeight]gfx.PlainColor, color gfx.PlainColor)

//go:noescape
func processOutOfBoundsWindingSSE(
	ymin float32,
	ymax float32,
	sign float32,
	locationWinding *[tileWidth][tileHeight]float32,
	accumulatedWinding *[tileHeight]float32,
)

//go:noescape
func computeAlphasNonZeroSSE(tail *[tileWidth][tileHeight]uint8, locationWinding *[tileWidth][tileHeight]float32)

//go:noescape
func computeAlphasNonZeroAVX(tail *[tileWidth][tileHeight]uint8, locationWinding *[tileWidth][tileHeight]float32)

// linearRgbaF32ToSrgbU8_Polynomial_AVX2 converts linear RGBA values to 8-bit sRGB using a
// degree 5 polynomial approximation. If unpremul is true, colors will be
// unpremultiplied before conversion to sRGB. The maximum error of the
// approximation is less than 0.5221.
//
// Needs AVX2 and FMA3. The scalar implementation is at
// [linearRgbaF32ToSrgbU8_Polynomial_Scalar].
//
// For a function that automatically selects the best implementation of linear
// to sRGB conversion and that handles sizes that aren't multiplies of 32 see
// [linearRgbaF32ToSrgbU8].
//
//go:noescape
func linearRgbaF32ToSrgbU8_Polynomial_AVX2(
	in *WideTileBuffer,
	out *[wideTileWidth][stripHeight][4]uint8,
	unpremul bool,
)
