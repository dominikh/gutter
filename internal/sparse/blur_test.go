// SPDX-FileCopyrightText: 2026 Dominik Honnef and contributors
//
// SPDX-License-Identifier: MIT

package sparse

import (
	"math"
	"testing"

	"honnef.co/go/color"
	"honnef.co/go/curve"
	"honnef.co/go/gutter/gfx"
)

func BenchmarkBlurredRoundedRectFiller(b *testing.B) {
	br := &gfx.BlurredRoundedRectangle{
		Rect:   curve.NewRectFromOrigin(curve.Pt(100, 100), curve.Sz(100, 100)),
		Color:  color.Make(color.LinearSRGB, 1, 0, 0, 1),
		Radius: 15,
		StdDev: 10,
	}

	b.Run("normal", func(b *testing.B) {
		br.LowPrecision = false
		f := encodeBlurredRoundedRectangle(br, curve.Identity).filler(0, 0)
		dst := make([][stripHeight]gfx.PlainColor, 256)
		for b.Loop() {
			f.reset(0, 100)
			f.fill(dst)
		}
	})

	b.Run("low_precision", func(b *testing.B) {
		br.LowPrecision = true
		f := encodeBlurredRoundedRectangle(br, curve.Identity).filler(0, 0)
		dst := make([][stripHeight]gfx.PlainColor, 256)
		for b.Loop() {
			f.reset(0, 100)
			f.fill(dst)
		}
	})
}

func BenchmarkLog(b *testing.B) {
	const x = 3.456
	b.Run("stdlib", func(b *testing.B) {
		for b.Loop() {
			math.Log(x)
		}
	})

	b.Run("approximation", func(b *testing.B) {
		for b.Loop() {
			fastLog(x)
		}
	})
}

func BenchmarkExp(b *testing.B) {
	const x = 3.456
	b.Run("stdlib", func(b *testing.B) {
		for b.Loop() {
			math.Exp(x)
		}
	})

	b.Run("approximation", func(b *testing.B) {
		for b.Loop() {
			fastExp(x)
		}
	})
}

func BenchmarkPow(b *testing.B) {
	const x = 3.456
	b.Run("stdlib", func(b *testing.B) {
		for b.Loop() {
			math.Pow(x, x)
		}
	})

	b.Run("approximation", func(b *testing.B) {
		for b.Loop() {
			fastPow(x, x)
		}
	})
}
