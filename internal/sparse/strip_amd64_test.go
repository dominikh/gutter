// SPDX-FileCopyrightText: 2025 Dominik Honnef and contributors
//
// SPDX-License-Identifier: MIT

//go:build !purego

package sparse

import (
	"fmt"
	"math"
	"testing"

	"golang.org/x/sys/cpu"
)

func BenchmarkComputeAlphasNonZeroAMD64(b *testing.B) {
	fns := []struct {
		fp      func(tail *[tileWidth][tileHeight]uint8, locationWinding *[tileWidth][tileHeight]float32)
		desc    string
		enabled bool
	}{
		{computeAlphasNonZeroNative, "purego", true},
		{computeAlphasNonZeroSSE, "SSE", true},
		{computeAlphasNonZeroAVX, "AVX2", cpu.X86.HasAVX2},
	}

	// Ideally, these two variables would be in the scope of a single iteration,
	// but our use of function pointers causes them to escape to the heap, which
	// would dominate the benchmark results.
	var locationWinding [tileWidth][tileHeight]float32
	var tail [tileWidth][tileHeight]uint8
	for _, fn := range fns {
		b.Run(fmt.Sprintf("instr=%s", fn.desc), func(b *testing.B) {
			if !fn.enabled {
				b.Skip()
			}
			for b.Loop() {
				locationWinding = [tileWidth][tileHeight]float32{{0.25, 1, 1, 1}, {0, 0.75, 1, 1}, {0, 0.25, 1, 1}, {0, 0, 0.75, 1}}
				fn.fp(&tail, &locationWinding)
			}
		})
	}
}

func BenchmarkProcessOutOfBoundsWindingSSE(b *testing.B) {
	for b.Loop() {
		ymin := float32(4.4388885)
		ymax := float32(7.99)
		sign := float32(1)
		var locationWinding [tileWidth][tileHeight]float32
		var accumulatedWinding [tileHeight]float32
		processOutOfBoundsWindingSSE(ymin, ymax, sign, &locationWinding, &accumulatedWinding)
	}
}

func BenchmarkComputeWindingAMD64(b *testing.B) {
	fns := []struct {
		fp func(
			lineTopY float32,
			lineTopX float32,
			lineBottomY float32,
			sign float32,
			xSlope float32,
			ySlope float32,
			locationWinding *[tileWidth][tileHeight]float32,
			accumulatedWinding *[tileHeight]float32,
		)
		desc    string
		enabled bool
	}{
		{computeWindingScalar, "purego", true},
		{computeWindingAVX, "AVX", cpu.X86.HasAVX},
	}

	// Ideally, these two variables would be in the scope of a single iteration,
	// but our use of function pointers causes them to escape to the heap, which
	// would dominate the benchmark results.
	var locationWinding [tileWidth][tileHeight]float32
	var accumulatedWinding [tileHeight]float32
	for _, fn := range fns {
		b.Run(fmt.Sprintf("instr=%s", fn.desc), func(b *testing.B) {
			if !fn.enabled {
				b.Skip()
			}

			for b.Loop() {
				lineTopY := float32(0)
				lineTopX := float32(0)
				lineBottomY := float32(40)
				sign := float32(1)
				xSlope := float32(0)
				ySlope := float32(math.Inf(1))
				clear(locationWinding[:])
				clear(accumulatedWinding[:])
				fn.fp(
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
		})
	}
}
