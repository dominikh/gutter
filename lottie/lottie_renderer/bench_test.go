// SPDX-FileCopyrightText: 2024 Dominik Honnef and contributors
//
// SPDX-License-Identifier: MIT

package lottie_renderer

import (
	_ "embed"
	"testing"

	"honnef.co/go/gutter/gfx"
	"honnef.co/go/gutter/lottie/lottie_converter"
	"honnef.co/go/gutter/lottie/lottie_encoding"
)

//go:embed testdata/zipper.json
var zipperJSON []byte

func BenchmarkRender(b *testing.B) {
	comp, err := lottie_encoding.Parse(zipperJSON)
	if err != nil {
		b.Fatal(err)
	}
	ccomp := lottie_converter.ConvertAnimation(comp)
	b.ResetTimer()

	var r Renderer
	for range b.N {
		for i := int(ccomp.FirstFrame); i < int(ccomp.LastFrame); i++ {
			rec := gfx.NewRecorder()
			r.Render(ccomp, float64(i), 1, rec)
		}
	}
	numFrames := float64(int(ccomp.LastFrame) - int(ccomp.FirstFrame))
	b.ReportMetric(float64(b.Elapsed())/numFrames/float64(b.N), "ns/frame")
}
