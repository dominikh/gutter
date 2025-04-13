// SPDX-FileCopyrightText: 2025 Dominik Honnef and contributors
//
// SPDX-License-Identifier: MIT

package sparse

import (
	"fmt"
	"math/rand"
	"testing"

	"honnef.co/go/color"
	"honnef.co/go/curve"
)

var blends = []Mix{
	MixNormal,
	MixMultiply,
	MixScreen,
	MixOverlay,
	MixDarken,
	MixLighten,
	MixColorDodge,
	MixColorBurn,
	MixHardLight,
	MixSoftLight,
	MixDifference,
	MixExclusion,
}

func BenchmarkClipFillReuseGen(b *testing.B) {
	origDst := make([][stripHeight]Color, 256)
	origSrc := make([][stripHeight]Color, 256)
	for i := range origDst {
		for j := range origDst[i] {
			for k := range origDst[i][j] {
				origDst[i][j][k] = rand.Float32()
				origSrc[i][j][k] = rand.Float32()
			}
			for k := range origDst[i][j][:3] {
				origDst[i][j][k] *= origDst[i][j][3]
				origSrc[i][j][k] *= origSrc[i][j][3]
			}
		}
	}

	dst := make([][stripHeight]Color, 256)
	run := func(kind string, comp Compose, blend Mix, fn func()) {
		b.Run(fmt.Sprintf("kind=%s/comp=%s/blend=%s", kind, comp, blend), func(b *testing.B) {
			for b.Loop() {
				copy(dst, origDst)
				fn()
			}
			px := float64(256 * stripHeight * b.N)
			d := float64(b.Elapsed()) / px
			bytes := px * 4 * 4
			r := bytes / float64(b.Elapsed().Seconds())
			b.ReportMetric(d, "ns/px")
			b.ReportMetric(r, "B/s")
		})
	}

	for _, comp := range ComposeOps {
		for _, blend := range blends {
			run("ComplexComplex", comp, blend, func() {
				blendComplexComplex(dst, origSrc, nil, BlendMode{Compose: comp, Mix: blend}, 1)
			})
		}
	}
}

func TestCompose(t *testing.T) {
	dst := curve.NewRectFromOrigin(curve.Pt(0, 0), curve.Sz(40, 40))
	src := curve.NewRectFromOrigin(curve.Pt(24, 24), curve.Sz(40, 40))

	for _, op := range ComposeOps {
		t.Run("op="+op.String(), func(t *testing.T) {
			renderAndCompare(t, 64, 64, true, "compose_"+op.String(), func(ctx *Renderer) {
				ctx.Fill(dst.PathElements(0.1), curve.Identity, NonZero, Solid(color.Make(color.SRGB, 1, 0.86, 0, 1)))
				ctx.PushLayer(Layer{
					BlendMode: BlendMode{
						Compose: op,
					},
					Opacity: 1,
				})
				ctx.Fill(src.PathElements(0.1), curve.Identity, NonZero, Solid(color.Make(color.SRGB, 0.4, 0.78, 0.91, 1)))
			})
		})
	}
}

func TestMix(t *testing.T) {
	dst := curve.NewRectFromOrigin(curve.Pt(0, 0), curve.Sz(40, 40))
	src := curve.NewRectFromOrigin(curve.Pt(24, 24), curve.Sz(40, 40))

	for _, op := range MixOps {
		t.Run("op="+op.String(), func(t *testing.T) {
			renderAndCompare(t, 64, 64, true, "mix_"+op.String(), func(ctx *Renderer) {
				ctx.Fill(dst.PathElements(0.1), curve.Identity, NonZero, Solid(color.Make(color.SRGB, 1, 0.86, 0, 1)))
				ctx.PushLayer(Layer{
					BlendMode: BlendMode{
						Mix: op,
					},
					Opacity: 1,
				})
				ctx.Fill(src.PathElements(0.1), curve.Identity, NonZero, Solid(color.Make(color.SRGB, 0.4, 0.78, 0.91, 1)))
			})
		})
	}
}
