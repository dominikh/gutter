// SPDX-FileCopyrightText: 2025 Dominik Honnef and contributors
//
// SPDX-License-Identifier: MIT

package sparse

import (
	"fmt"
	"testing"
)

func benchmarkFineFill(
	b *testing.B,
	fn func(out [][stripHeight]Color, color Color),
) {
	for _, t := range []struct {
		name  string
		width int
		short bool
		color Color
	}{
		{"opaque", wideTileWidth, true, Color{0.5, 0.5, 0.5, 1}},
		{"translucent", wideTileWidth, true, Color{0.1, 0.1, 0.1, 0.1}},

		{"opaque", 16, false, Color{0.5, 0.5, 0.5, 1}},
		{"translucent", 16, false, Color{0.1, 0.1, 0.1, 0.1}},

		{"opaque", 2, false, Color{0.5, 0.5, 0.5, 1}},
		{"translucent", 2, false, Color{0.1, 0.1, 0.1, 0.1}},

		{"opaque", 1, false, Color{0.5, 0.5, 0.5, 1}},
		{"translucent", 1, false, Color{0.1, 0.1, 0.1, 0.1}},
	} {
		if testing.Short() && !t.short {
			b.Skip("skipping benchmark in short mode.")
		}
		b.Run(fmt.Sprintf("kind=%s/width=%d", t.name, t.width), func(b *testing.B) {
			f := newFine(t.width, tileHeight, nil)
			for b.Loop() {
				f.fillWithFp(1, t.width-1, t.color, fn)
			}
			px := float64(t.width * tileHeight * b.N)
			d := float64(b.Elapsed()) / px
			bytes := px * 4 * 4
			r := bytes / float64(b.Elapsed().Seconds())
			b.ReportMetric(d, "ns/px")
			b.ReportMetric(r, "B/s")
		})
	}
}

func BenchmarkFineFillAuto(b *testing.B) {
	benchmarkFineFill(b, fillFp)
}

func BenchmarkFineFillNative(b *testing.B) {
	benchmarkFineFill(b, fineFillNative)
}
