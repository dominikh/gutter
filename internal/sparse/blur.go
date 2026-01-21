// SPDX-FileCopyrightText: 2025 the Vello Authors
// SPDX-FileCopyrightText: 2026 Dominik Honnef and contributors
//
// SPDX-License-Identifier: MIT

package sparse

import (
	"math"

	"honnef.co/go/curve"
	"honnef.co/go/gutter/gfx"
	"honnef.co/go/stuff/math/math32"
)

type blurredRoundedRectFiller struct {
	curPos curve.Point
	rect   *gfx.EncodedBlurredRoundedRectangle
}

func newBlurredRoundedRectFiller(
	rect *gfx.EncodedBlurredRoundedRectangle,
	startX uint16,
	startY uint16,
) *blurredRoundedRectFiller {
	return &blurredRoundedRectFiller{
		curPos: curve.Pt(float64(startX), float64(startY)).Transform(rect.Transform),
		rect:   rect,
	}
}

func (f *blurredRoundedRectFiller) reset(startX, startY uint16) {
	f.curPos = curve.Pt(float64(startX), float64(startY)).Transform(f.rect.Transform)
}

func (f *blurredRoundedRectFiller) run(dst [][stripHeight]gfx.PlainColor) {
	// Implementation is adapted from:
	// <https://git.sr.ht/~raph/blurrr/tree/master/src/distfield.rs>

	// OPT: Add optimized version for non-rotated rectangles. We can precompute all of the
	// variables that only depend on y.

	rect := f.rect

	for _x := range dst {
		column := &dst[_x]
		colPos := f.curPos
		for _y := range column {
			j := float32(colPos.Y)
			i := float32(colPos.X)

			var alphaVal float32

			{
				y := j + 0.5 - 0.5*rect.Height
				y0 := math32.Abs(y) - (rect.H*0.5 - rect.R1)
				y1 := max(y0, 0.0)

				x := i + 0.5 - 0.5*rect.Width
				x0 := math32.Abs(x) - (rect.W*0.5 - rect.R1)
				x1 := max(x0, 0.0)
				var dPos float32
				if rect.LowPrecision {
					dPos = fastPow(fastPow(x1, rect.Exponent)+fastPow(y1, rect.Exponent), rect.RecipExponent)
				} else {
					dPos = pow32(pow32(x1, rect.Exponent)+pow32(y1, rect.Exponent), rect.RecipExponent)
				}
				dNeg := min(max(x0, y0), 0.0)
				d := dPos + dNeg - rect.R1
				alphaVal = rect.Scale *
					(erf7(rect.RecipStdDev*(rect.MinEdge+d)) - erf7(rect.RecipStdDev*d))
			}

			column[_y] = gfx.PlainColor{
				f.rect.Color[0] * alphaVal,
				f.rect.Color[1] * alphaVal,
				f.rect.Color[2] * alphaVal,
				f.rect.Color[3] * alphaVal,
			}

			colPos = colPos.Translate(f.rect.YAdvance)
		}

		f.curPos = f.curPos.Translate(f.rect.XAdvance)
	}
}

const logPrec = 9

var logTable = func(n int) []float32 {
	table := make([]float32, 1<<n)
	var numlog float32
	x := uint32(0x3F800000)
	numlog = math.Float32frombits(x)
	incr := uint32(1 << (23 - n))
	p := 1 << n
	for i := range p {
		table[i] = float32(math.Log2(float64(numlog)))
		x += incr
		numlog = math.Float32frombits(x)
	}
	return table
}(logPrec)

func fastLog(x float32) float32 {
	// From "A fast logarithm implementation with adjustable accuracy"
	xb := math.Float32bits(x)
	log2 := ((xb >> 23) & 255) - 127
	xb &= 0x7FFFFF
	xb >>= 23 - logPrec
	val := logTable[xb]
	return (val + float32(log2)) * math.Ln2
}

func fastExp(f float32) float32 {
	// From https://specbranch.com/posts/fast-exp/
	return math.Float32frombits(uint32(int32(f*12102203) + 1064986823))
}

func fastPow(x, y float32) float32 {
	return fastExp(y * fastLog(x))
}
