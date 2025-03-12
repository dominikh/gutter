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

func benchmarkClipFill(b *testing.B, fn func(b *testing.B, dst, src [][stripHeight]Color)) {
	dst := make([][stripHeight]Color, wideTileWidth)
	src := make([][stripHeight]Color, wideTileWidth)
	// make sure memory is paged in
	clear(dst)
	clear(src)

	// We test the full width to measure the best possible performance, and at
	// the smallest possible width to measure the per-call overhead.
	fillWidths := []int{wideTileWidth, 1}
	for _, width := range fillWidths {
		b.Run(fmt.Sprintf("width=%d", width), func(b *testing.B) {
			fn(b, dst, src)
			px := float64(width * tileHeight * b.N)
			d := float64(b.Elapsed()) / px
			bytes := px * 4 * 4
			r := bytes / float64(b.Elapsed().Seconds())
			b.ReportMetric(d, "ns/px")
			b.ReportMetric(r, "B/s")
		})
	}
}

func Benchmark_fineFillComplexNative(b *testing.B) {
	c := Color{0.5, 0.5, 0.5, 0.5}
	benchmarkFill(b, func(b *testing.B, buf [][stripHeight]Color) {
		for b.Loop() {
			fineFillComplexNative(buf, c)
		}
	})
}

func Benchmark_fineClipFillSimpleNosNative(b *testing.B) {
	nos := Color{1, 1, 1, 1}
	benchmarkClipFill(b, func(b *testing.B, dst, src [][stripHeight]Color) {
		for b.Loop() {
			fineClipFillSimpleNosNative(dst, nos, src)
		}
	})
}

func Benchmark_fineClipFillSimpleTosTranslucentNative(b *testing.B) {
	tos := Color{0.5, 0.5, 0.5, 0.5}
	benchmarkClipFill(b, func(b *testing.B, dst, src [][stripHeight]Color) {
		for b.Loop() {
			fineClipFillSimpleTosTranslucentNative(dst, tos)
		}
	})
}

func Benchmark_fineClipFillNative(b *testing.B) {
	benchmarkClipFill(b, func(b *testing.B, dst, src [][stripHeight]Color) {
		for b.Loop() {
			fineClipFillNative(dst, src)
		}
	})
}

func Benchmark_memsetColumnsNative(b *testing.B) {
	c := Color{1, 1, 1, 1}
	benchmarkFill(b, func(b *testing.B, buf [][4]Color) {
		for b.Loop() {
			memsetColumnsNative(buf, c)
		}
	})
}

func benchmarkFinePack(b *testing.B, complex bool) {
	tests := []struct {
		label  string
		width  uint16
		height uint16
	}{
		// 256*4 uses 16 KiB, which fits into L1 on somewhat modern CPUs.
		{"L1", 256, 4},
		// 256*128 uses 512 KiB, which fits into L2 on somewhat modern CPUs.
		{"L2", 256, 128},
		// 512*512 uses 4 MiB, which fits into L3 on somewhat modern CPUs.
		{"L3", 512, 512},
		// 4096*4096 uses 256 MiB, which does not fit into L3 on most CPUs.
		{"mem", 4096, 4096},
	}

	for _, tt := range tests {
		b.Run(tt.label, func(b *testing.B) {
			if tt.width%wideTileWidth != 0 {
				b.Fatalf("width %d isn't multiple of wideTileWidth", tt.width)
			}
			if tt.height%stripHeight != 0 {
				b.Fatalf("height %d isn't multiple of stripHeight", tt.height)
			}

			pixmap := make([]Color, int(tt.width)*int(tt.height))
			f := newFine(tt.width, tt.height, pixmap)
			clear(pixmap)
			if complex {
				clear(f.layers[len(f.layers)-1].scratch[:])
				f.layers[len(f.layers)-1].complex = true
			}

			for b.Loop() {
				for x := range tt.width / wideTileWidth {
					for y := range tt.height / stripHeight {
						f.pack(x, y)
					}
				}
			}

			px := float64(int(tt.width) * int(tt.height) * b.N)
			d := float64(b.Elapsed()) / px
			bytes := px * 4 * 4
			r := bytes / float64(b.Elapsed().Seconds())
			b.ReportMetric(d, "ns/px")
			b.ReportMetric(r, "B/s")
		})
	}
}

func Benchmark_fine_pack_simple(b *testing.B) {
	benchmarkFinePack(b, false)
}

func Benchmark_fine_pack_complex(b *testing.B) {
	benchmarkFinePack(b, true)
}
