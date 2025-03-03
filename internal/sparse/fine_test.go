// SPDX-FileCopyrightText: 2025 Dominik Honnef and contributors
//
// SPDX-License-Identifier: MIT

package sparse

import (
	"fmt"
	"testing"
)

func benchmarkFill(b *testing.B, fn func(b *testing.B, buf [][stripHeight]Color)) {
	buf := make([][stripHeight]Color, wideTileWidth)
	// make sure memory is paged in
	clear(buf)

	// We test the full width to measure the best possible performance, and at
	// the smallest possible width to measure the per-call overhead.
	fillWidths := []int{wideTileWidth, 1}
	for _, width := range fillWidths {
		b.Run(fmt.Sprintf("width=%d", width), func(b *testing.B) {
			fn(b, buf)
			px := float64(width * tileHeight * b.N)
			d := float64(b.Elapsed()) / px
			bytes := px * 4 * 4
			r := bytes / float64(b.Elapsed().Seconds())
			b.ReportMetric(d, "ns/px")
			b.ReportMetric(r, "B/s")
		})
	}
}

func Benchmark_fineFillSolidNative(b *testing.B) {
	c := Color{1, 1, 1, 1}
	benchmarkFill(b, func(b *testing.B, buf [][stripHeight]Color) {
		for b.Loop() {
			fineFillSolidNative(buf, c)
		}
	})
}

func Benchmark_fineFillSimpleNative(b *testing.B) {
	c := Color{0.5, 0.5, 0.5, 0.5}
	bg := Color{1, 0, 0, 1}
	benchmarkFill(b, func(b *testing.B, buf [][stripHeight]Color) {
		for b.Loop() {
			fineFillSimpleNative(buf, c, bg)
		}
	})
}

func Benchmark_fineFillComplexNative(b *testing.B) {
	c := Color{0.5, 0.5, 0.5, 0.5}
	benchmarkFill(b, func(b *testing.B, buf [][stripHeight]Color) {
		for b.Loop() {
			fineFillComplexNative(buf, c)
		}
	})
}
