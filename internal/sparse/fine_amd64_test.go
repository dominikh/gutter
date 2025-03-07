// SPDX-FileCopyrightText: 2025 Dominik Honnef and contributors
//
// SPDX-License-Identifier: MIT

//go:build !purego

package sparse

import (
	"testing"

	"golang.org/x/sys/cpu"
)

func Benchmark_fineFillComplexAVX(b *testing.B) {
	if !cpu.X86.HasAVX {
		b.Skip()
	}
	c := Color{0.5, 0.5, 0.5, 0.5}
	benchmarkFill(b, func(b *testing.B, buf [][stripHeight]Color) {
		for b.Loop() {
			fineFillComplexAVX(buf, c)
		}
	})
}

func Benchmark_memsetColumnsAVX(b *testing.B) {
	if !cpu.X86.HasAVX {
		b.Skip()
	}
	c := Color{1, 1, 1, 1}
	benchmarkFill(b, func(b *testing.B, buf [][4]Color) {
		for b.Loop() {
			memsetColumnsAVX(buf, c)
		}
	})
}

func Benchmark_fineFillComplexSSE(b *testing.B) {
	if !cpu.X86.HasSSE2 {
		b.Skip()
	}
	c := Color{0.5, 0.5, 0.5, 0.5}
	benchmarkFill(b, func(b *testing.B, buf [][stripHeight]Color) {
		for b.Loop() {
			fineFillComplexSSE(buf, c)
		}
	})
}

func Benchmark_memsetColumnsSSE(b *testing.B) {
	if !cpu.X86.HasSSE2 {
		b.Skip()
	}
	c := Color{1, 1, 1, 1}
	benchmarkFill(b, func(b *testing.B, buf [][4]Color) {
		for b.Loop() {
			memsetColumnsSSE(buf, c)
		}
	})
}
