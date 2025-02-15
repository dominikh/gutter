// SPDX-FileCopyrightText: 2025 Dominik Honnef and contributors
//
// SPDX-License-Identifier: MIT

//go:build !purego

package sparse

import (
	"testing"

	"golang.org/x/sys/cpu"
)

func BenchmarkFineFillSSE(b *testing.B) {
	if !cpu.X86.HasSSE2 {
		b.Skip()
	}
	benchmarkFineFill(b, fineFillSSE)
}

func BenchmarkFineFillAVX(b *testing.B) {
	if !cpu.X86.HasAVX {
		b.Skip()
	}
	benchmarkFineFill(b, fineFillAVX)
}
