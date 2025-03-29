// SPDX-FileCopyrightText: 2025 Dominik Honnef and contributors
//
// SPDX-License-Identifier: MIT

package sparse

import (
	"math"
	"testing"
)

func BenchmarkComputeAlphasNonZeroNative(b *testing.B) {
	locationWinding := [4][4]float32{{0.25, 1, 1, 1}, {0, 0.75, 1, 1}, {0, 0.25, 1, 1}, {0, 0, 0.75, 1}}
	var tail [4][4]uint8
	for b.Loop() {
		computeAlphasNonZeroNative(&tail, &locationWinding)
	}
}

func BenchmarkProcessOutOfBoundsWindingNative(b *testing.B) {
	for b.Loop() {
		ymin := float32(4.4388885)
		ymax := float32(7.99)
		sign := float32(1)
		var locationWinding [4][4]float32
		var accumulatedWinding [4]float32
		processOutOfBoundsWindingNative(ymin, ymax, sign, &locationWinding, &accumulatedWinding)
	}
}

func BenchmarkComputeWindingNative(b *testing.B) {
	for b.Loop() {
		lineTopY := float32(0)
		lineTopX := float32(0)
		lineBottomY := float32(40)
		sign := float32(1)
		xSlope := float32(0)
		ySlope := float32(math.Inf(1))
		var locationWinding [4][4]float32
		var accumulatedWinding [4]float32
		computeWindingNative(
			lineTopY,
			lineTopX,
			lineBottomY,
			sign,
			xSlope,
			ySlope,
			&locationWinding,
			&accumulatedWinding,
		)
	}
}
