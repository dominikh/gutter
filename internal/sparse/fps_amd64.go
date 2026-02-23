// SPDX-FileCopyrightText: 2025 Dominik Honnef and contributors
//
// SPDX-License-Identifier: MIT

//go:build !purego

package sparse

import (
	"simd/archsimd"

	"golang.org/x/sys/cpu"
	"honnef.co/go/gutter/gfx"
)

func memsetColumns(buf [][stripHeight]gfx.PlainColor, c gfx.PlainColor) {
	if cpu.X86.HasAVX {
		memsetColumnsAVX(buf, c)
	} else {
		memsetColumnsNative(buf, c)
	}
}

func fineFillComplex(buf [][stripHeight]gfx.PlainColor, color gfx.PlainColor) {
	if cpu.X86.HasAVX {
		fineFillComplexAVX(buf, color)
	} else {
		fineFillComplexScalar(buf, color)
	}
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
	if archsimd.X86.AVX() {
		computeWindingAVX(
			lineTopY,
			lineTopX,
			lineBottomY,
			sign,
			xSlope,
			ySlope,
			locationWinding,
			accumulatedWinding,
		)
	} else {
		computeWindingScalar(
			lineTopY,
			lineTopX,
			lineBottomY,
			sign,
			xSlope,
			ySlope,
			locationWinding,
			accumulatedWinding,
		)
	}
}

func processOutOfBoundsWinding(
	ymin float32,
	ymax float32,
	sign float32,
	locationWinding *[tileWidth][tileHeight]float32,
	accumulatedWinding *[tileHeight]float32,
) {
	processOutOfBoundsWindingSSE(ymin, ymax, sign, locationWinding, accumulatedWinding)
}

func computeAlphasNonZero(
	tail *[tileWidth][tileHeight]uint8,
	locationWinding *[tileWidth][tileHeight]float32,
) {
	if cpu.X86.HasAVX && cpu.X86.HasAVX2 {
		computeAlphasNonZeroAVX(tail, locationWinding)
	} else {
		computeAlphasNonZeroNative(tail, locationWinding)
	}
}
