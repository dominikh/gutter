// SPDX-FileCopyrightText: 2025 Dominik Honnef and contributors
//
// SPDX-License-Identifier: MIT

//go:build noasm || !amd64 || !goexperiment.simd

package sparse

import "honnef.co/go/gutter/gfx"

var (
	computeAlphasNonZeroFp = computeAlphasNonZeroNative
)

func memsetColumns(buf Pixels, c gfx.PlainColor) {
	memsetColumnsScalar(buf, c)
}

func fineFillComplex(buf Pixels, color gfx.PlainColor) {
	fineFillComplexScalar(buf, color)
}

func computeWinding(
	lineTopY float32,
	lineTopX float32,
	lineBottomY float32,
	sign float32,
	xSlope float32,
	ySlope float32,
	locationWinding *[tileWidth][tileHeight]float32,
	accumulatedWinding *[tileHeight]float32,
) {
	computeWindingScalar(lineTopY,
		lineTopX,
		lineBottomY,
		sign,
		xSlope,
		ySlope,
		locationWinding,
		accumulatedWinding,
	)
}

func processOutOfBoundsWinding(
	ymin float32,
	ymax float32,
	sign float32,
	locationWinding *[tileWidth][tileHeight]float32,
	accumulatedWinding *[tileHeight]float32,
) {
	processOutOfBoundsWindingNative(ymin, ymax, sign, locationWinding, accumulatedWinding)
}

func computeAlphasNonZero(
	tail *[tileWidth][tileHeight]uint8,
	locationWinding *[tileWidth][tileHeight]float32,
) {
	computeAlphasNonZeroNative(tail, locationWinding)
}
