// SPDX-FileCopyrightText: 2025 the Vello Authors
// SPDX-FileCopyrightText: 2026 Dominik Honnef and contributors
//
// SPDX-License-Identifier: MIT

package sparse

import (
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
				dPos := pow32(pow32(x1, rect.Exponent)+pow32(y1, rect.Exponent), rect.RecipExponent)
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
