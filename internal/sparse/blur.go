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

func encodeBlurredRoundedRectangle(
	b *gfx.BlurredRoundedRectangle,
	transform curve.Affine,
) *encodedBlurredRoundedRectangle {
	rect := b.Rect
	// Ensure rectangle has positive width/height.
	if rect.X0 > rect.X1 {
		rect.X0, rect.X1 = rect.X1, rect.X0
	}
	if rect.Y0 > rect.Y1 {
		rect.Y0, rect.Y1 = rect.Y1, rect.Y0
	}

	width := float32(rect.Width())
	height := float32(rect.Height())
	minEdge := min(width, height)
	rmax := 0.5 * minEdge
	radius := min(b.Radius, rmax)

	// To avoid divide by 0; potentially should be a bigger number for antialiasing.
	stdDev := max(b.StdDev, 1e-6)
	recipStdDev := 1.0 / stdDev

	// Pull in long end (make less eccentric).
	delta := 1.25 *
		stdDev *
		(exp32(-pow2(0.5*recipStdDev*width)) -
			exp32(-pow2(0.5*recipStdDev*height)))
	w := width + min(delta, 0.0)
	h := height - max(delta, 0.0)

	scale := 0.5 * erf7(recipStdDev*0.5*(max(w, h)-0.5*radius))
	r0 := min(hypot32(radius, stdDev*1.15), rmax)
	r1 := min(hypot32(radius, stdDev*2.0), rmax)
	exponent := 2.0 * r1 / r0
	transform = transform.Invert().ThenTranslate(curve.Vec(-rect.X0, -rect.Y0))
	xAdvance, yAdvance := xyAdvances(transform)

	return &encodedBlurredRoundedRectangle{
		exponent:      exponent,
		recipExponent: 1.0 / exponent,
		width:         width,
		height:        height,
		scale:         scale,
		r1:            r1,
		recipStdDev:   recipStdDev,
		minEdge:       minEdge,
		color:         gfx.ColorToInternal(b.Color),
		w:             w,
		h:             h,
		transform:     transform,
		xAdvance:      xAdvance,
		yAdvance:      yAdvance,
		lowPrecision:  b.LowPrecision,
	}
}

var _ encodedPaint = (*encodedBlurredRoundedRectangle)(nil)

type encodedBlurredRoundedRectangle struct {
	exponent      float32
	recipExponent float32
	scale         float32
	recipStdDev   float32
	minEdge       float32
	w             float32
	h             float32
	width         float32
	height        float32
	r1            float32
	// The base color for the blurred rectangle.
	color gfx.PlainColor
	// A transform that needs to be applied to the position of the first
	// processed pixel.
	transform curve.Affine
	// How much to advance into the x/y direction for one step in the x
	// direction.
	xAdvance curve.Vec2
	// How much to advance into the x/y direction for one step in the y
	// direction.
	yAdvance     curve.Vec2
	lowPrecision bool
}

type blurredRoundedRectFiller struct {
	curPos curve.Point
	rect   *encodedBlurredRoundedRectangle
}

func (e *encodedBlurredRoundedRectangle) filler(startX, startY uint16) paintFiller {
	return &blurredRoundedRectFiller{
		curPos: curve.Pt(float64(startX), float64(startY)).Transform(e.transform),
		rect:   e,
	}
}

func (f *blurredRoundedRectFiller) reset(startX, startY uint16) {
	f.curPos = curve.Pt(float64(startX), float64(startY)).Transform(f.rect.transform)
}

func (f *blurredRoundedRectFiller) fill(dst Pixels) {
	// Implementation is adapted from:
	// <https://git.sr.ht/~raph/blurrr/tree/master/src/distfield.rs>

	// OPT: Add optimized version for non-rotated rectangles. We can precompute all of the
	// variables that only depend on y.

	rect := f.rect
	width := len(dst[0])

	for _x := range width {
		colPos := f.curPos
		for _y := range stripHeight {
			j := float32(colPos.Y)
			i := float32(colPos.X)

			var alphaVal float32

			{
				y := j + 0.5 - 0.5*rect.height
				y0 := math32.Abs(y) - (rect.h*0.5 - rect.r1)
				y1 := max(y0, 0.0)

				x := i + 0.5 - 0.5*rect.width
				x0 := math32.Abs(x) - (rect.w*0.5 - rect.r1)
				x1 := max(x0, 0.0)
				var dPos float32
				if rect.lowPrecision {
					dPos = fastPow(fastPow(x1, rect.exponent)+fastPow(y1, rect.exponent), rect.recipExponent)
				} else {
					dPos = pow32(pow32(x1, rect.exponent)+pow32(y1, rect.exponent), rect.recipExponent)
				}
				dNeg := min(max(x0, y0), 0.0)
				d := dPos + dNeg - rect.r1
				alphaVal = rect.scale *
					(erf7(rect.recipStdDev*(rect.minEdge+d)) - erf7(rect.recipStdDev*d))
			}

			dst[0][_x][_y] = f.rect.color[0] * alphaVal
			dst[1][_x][_y] = f.rect.color[1] * alphaVal
			dst[2][_x][_y] = f.rect.color[2] * alphaVal
			dst[3][_x][_y] = f.rect.color[3] * alphaVal

			colPos = colPos.Translate(f.rect.yAdvance)
		}

		f.curPos = f.curPos.Translate(f.rect.xAdvance)
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

// Opaque implements [encodedPaint].
func (e *encodedBlurredRoundedRectangle) Opaque() bool {
	// OPT(dh): with zero corner radius and <=1 std dev and an opaque color,
	// this paint is opaque, too.
	return false
}

// isEncodedPaint implements [encodedPaint].
func (e *encodedBlurredRoundedRectangle) isEncodedPaint() {}
