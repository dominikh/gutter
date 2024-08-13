// SPDX-FileCopyrightText: 2024 Dominik Honnef and contributors
//
// SPDX-License-Identifier: MIT

package lottie_renderer

import (
	_ "embed"
	"testing"

	"honnef.co/go/curve"
	"honnef.co/go/gutter/lottie/lottie_converter"
	"honnef.co/go/gutter/lottie/lottie_encoding"
	"honnef.co/go/jello"
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

	var scene jello.Scene
	var r Renderer
	for range b.N {
		for i := int(ccomp.FirstFrame); i < int(ccomp.LastFrame); i++ {
			scene.Reset()
			r.Append(ccomp, float64(i), curve.Identity, 1, &scene)
		}
	}
	numFrames := float64(int(ccomp.LastFrame) - int(ccomp.FirstFrame))
	b.ReportMetric(float64(b.Elapsed())/numFrames/float64(b.N), "ns/frame")
}
