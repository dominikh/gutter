// SPDX-FileCopyrightText: 2025 Dominik Honnef and contributors
//
// SPDX-License-Identifier: MIT

//go:build !purego

package sparse

import (
	"fmt"
	"testing"

	"golang.org/x/sys/cpu"
)

func Benchmark_fineFillComplexAMD64(b *testing.B) {
	fns := []struct {
		fp      func(buf [][stripHeight]plainColor, color plainColor)
		desc    string
		enabled bool
	}{
		{fineFillComplexNative, "purego", true},
		{fineFillComplexSSE, "SSE", true},
		{fineFillComplexAVX, "AVX", cpu.X86.HasAVX},
	}
	for _, fn := range fns {
		b.Run(fmt.Sprintf("instr=%s", fn.desc), func(b *testing.B) {
			if !fn.enabled {
				b.Skip()
			}
			c := plainColor{0.5, 0.5, 0.5, 0.5}
			benchmarkFill(b, func(b *testing.B, buf [][stripHeight]plainColor) {
				for b.Loop() {
					fn.fp(buf, c)
				}
			})
		})
	}
}

func Benchmark_memsetColumnsAMD64(b *testing.B) {
	fns := []struct {
		fp      func(buf [][stripHeight]plainColor, color plainColor)
		desc    string
		enabled bool
	}{
		{memsetColumnsNative, "purego", true},
		{memsetColumnsSSE, "SSE", true},
		{memsetColumnsAVX, "AVX", cpu.X86.HasAVX},
	}
	for _, fn := range fns {
		b.Run(fmt.Sprintf("instr=%s", fn.desc), func(b *testing.B) {
			if !fn.enabled {
				b.Skip()
			}
			c := plainColor{1, 1, 1, 1}
			benchmarkFill(b, func(b *testing.B, buf [][4]plainColor) {
				for b.Loop() {
					fn.fp(buf, c)
				}
			})
		})
	}
}
