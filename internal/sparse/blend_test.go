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
	"honnef.co/go/gutter/gfx"
)

var blends = []gfx.Mix{
	gfx.MixNormal,
	gfx.MixMultiply,
	gfx.MixScreen,
	gfx.MixOverlay,
	gfx.MixDarken,
	gfx.MixLighten,
	gfx.MixColorDodge,
	gfx.MixColorBurn,
	gfx.MixHardLight,
	gfx.MixSoftLight,
	gfx.MixDifference,
	gfx.MixExclusion,
}

func BenchmarkClipFillReuseGen(b *testing.B) {
	var origDst, origSrc WideTileBuffer
	for ch := range 4 {
		for i := range origDst[ch] {
			for j := range origDst[ch][i] {
				origDst[ch][i][j] = rand.Float32()
				origSrc[ch][i][j] = rand.Float32()
			}
		}
	}
	// Premultiply RGB by A
	for ch := range 3 {
		for i := range origDst[ch] {
			for j := range origDst[ch][i] {
				origDst[ch][i][j] *= origDst[3][i][j]
				origSrc[ch][i][j] *= origSrc[3][i][j]
			}
		}
	}

	var dst WideTileBuffer
	run := func(kind string, comp gfx.Compose, blend gfx.Mix, fn func()) {
		b.Run(fmt.Sprintf("kind=%s/comp=%s/blend=%s", kind, comp, blend), func(b *testing.B) {
			for b.Loop() {
				dst = origDst
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

	dstPixels := dst.allPixels()
	srcPixels := origSrc.allPixels()
	for _, comp := range gfx.ComposeOps {
		for _, blend := range blends {
			run("ComplexComplex", comp, blend, func() {
				blendComplexComplex(dstPixels, srcPixels, nil, gfx.BlendMode{Compose: comp, Mix: blend}, 1)
			})
		}
	}
}

func TestCompose(t *testing.T) {
	dst := curve.NewRectFromOrigin(curve.Pt(0, 0), curve.Sz(40, 40))
	src := curve.NewRectFromOrigin(curve.Pt(24, 24), curve.Sz(40, 40))

	for _, op := range gfx.ComposeOps {
		t.Run("op="+op.String(), func(t *testing.T) {
			renderAndCompare(t, 64, 64, true, "compose_"+op.String(), func(ctx *Renderer) {
				ctx.Fill(dst, curve.Identity, gfx.NonZero, gfx.Solid(color.Make(color.SRGB, 1, 0.86, 0, 1)))
				ctx.PushLayer(Layer{
					BlendMode: gfx.BlendMode{
						Compose: op,
					},
					Opacity: 1,
				})
				ctx.Fill(src, curve.Identity, gfx.NonZero, gfx.Solid(color.Make(color.SRGB, 0.4, 0.78, 0.91, 1)))
			})
		})
	}
}

func TestMix(t *testing.T) {
	dst := curve.NewRectFromOrigin(curve.Pt(0, 0), curve.Sz(40, 40))
	src := curve.NewRectFromOrigin(curve.Pt(24, 24), curve.Sz(40, 40))

	for _, op := range gfx.MixOps {
		t.Run("op="+op.String(), func(t *testing.T) {
			renderAndCompare(t, 64, 64, true, "mix_"+op.String(), func(ctx *Renderer) {
				ctx.Fill(dst, curve.Identity, gfx.NonZero, gfx.Solid(color.Make(color.SRGB, 1, 0.86, 0, 1)))
				ctx.PushLayer(Layer{
					BlendMode: gfx.BlendMode{
						Mix: op,
					},
					Opacity: 1,
				})
				ctx.Fill(src, curve.Identity, gfx.NonZero, gfx.Solid(color.Make(color.SRGB, 0.4, 0.78, 0.91, 1)))
			})
		})
	}
}
