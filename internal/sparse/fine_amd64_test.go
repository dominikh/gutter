// SPDX-FileCopyrightText: 2025 Dominik Honnef and contributors
//
// SPDX-License-Identifier: MIT

//go:build !noasm && goexperiment.simd

package sparse

import (
	"fmt"
	"testing"

	"honnef.co/go/gutter/gfx"
	"honnef.co/go/gutter/internal/arch"
)

func Benchmark_fineFillComplexAMD64(b *testing.B) {
	fns := []struct {
		fp      func(buf Pixels, color gfx.PlainColor)
		desc    string
		enabled bool
	}{
		{fineFillComplexScalar, "noasm", true},
		{fineFillComplexAVX, "AVX", arch.AVX()},
	}
	for _, fn := range fns {
		b.Run(fmt.Sprintf("instr=%s", fn.desc), func(b *testing.B) {
			if !fn.enabled {
				b.Skip()
			}
			c := gfx.PlainColor{0.5, 0.5, 0.5, 0.5}
			benchmarkFill(b, func(b *testing.B, buf Pixels) {
				for b.Loop() {
					fn.fp(buf, c)
				}
			})
		})
	}
}

func Benchmark_memsetColumnsAMD64(b *testing.B) {
	fns := []struct {
		fp      func(buf Pixels, color gfx.PlainColor)
		desc    string
		enabled bool
	}{
		{memsetColumnsScalar, "noasm", true},
		{memsetColumnsAVX, "AVX", arch.AVX()},
	}
	for _, fn := range fns {
		b.Run(fmt.Sprintf("instr=%s", fn.desc), func(b *testing.B) {
			if !fn.enabled {
				b.Skip()
			}
			c := gfx.PlainColor{1, 1, 1, 1}
			benchmarkFill(b, func(b *testing.B, buf Pixels) {
				for b.Loop() {
					fn.fp(buf, c)
				}
			})
		})
	}
}
