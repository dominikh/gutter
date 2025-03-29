// SPDX-FileCopyrightText: 2025 Dominik Honnef and contributors
//
// SPDX-License-Identifier: MIT

//go:build !purego

package sparse

import (
	"math"
	"testing"

	"golang.org/x/sys/cpu"
)

func BenchmarkComputeAlphasNonZeroSSE(b *testing.B) {
	locationWinding := [4][4]float32{{0.25, 1, 1, 1}, {0, 0.75, 1, 1}, {0, 0.25, 1, 1}, {0, 0, 0.75, 1}}
	var tail [4][4]uint8
	for b.Loop() {
		computeAlphasNonZeroSSE(&tail, &locationWinding)
	}
}

func BenchmarkComputeAlphasNonZeroAVX(b *testing.B) {
	if !cpu.X86.HasAVX || !cpu.X86.HasAVX2 {
		b.Skip()
	}
	locationWinding := [4][4]float32{{0.25, 1, 1, 1}, {0, 0.75, 1, 1}, {0, 0.25, 1, 1}, {0, 0, 0.75, 1}}
	var tail [4][4]uint8
	for b.Loop() {
		computeAlphasNonZeroAVX(&tail, &locationWinding)
	}
}

func BenchmarkProcessOutOfBoundsWindingSSE(b *testing.B) {
	for b.Loop() {
		ymin := float32(4.4388885)
		ymax := float32(7.99)
		sign := float32(1)
		var locationWinding [4][4]float32
		var accumulatedWinding [4]float32
		processOutOfBoundsWindingSSE(ymin, ymax, sign, &locationWinding, &accumulatedWinding)
	}
}

func BenchmarkComputeWindingAVX(b *testing.B) {
	if !cpu.X86.HasAVX || !cpu.X86.HasFMA {
		b.Skip()
	}
	for b.Loop() {
		lineTopY := float32(0)
		lineTopX := float32(0)
		lineBottomY := float32(40)
		sign := float32(1)
		xSlope := float32(0)
		ySlope := float32(math.Inf(1))
		var locationWinding [4][4]float32
		var accumulatedWinding [4]float32
		computeWindingAVX(
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

func BenchmarkComputeWindingSSE(b *testing.B) {
	for b.Loop() {
		lineTopY := float32(0)
		lineTopX := float32(0)
		lineBottomY := float32(40)
		sign := float32(1)
		xSlope := float32(0)
		ySlope := float32(math.Inf(1))
		var locationWinding [4][4]float32
		var accumulatedWinding [4]float32
		computeWindingSSE(
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
