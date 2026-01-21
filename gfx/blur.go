// SPDX-FileCopyrightText: 2025 the Vello Authors
// SPDX-FileCopyrightText: 2026 Dominik Honnef and contributors
//
// SPDX-License-Identifier: MIT

package gfx

import (
	"math"

	"honnef.co/go/color"
	"honnef.co/go/curve"
	"honnef.co/go/stuff/math/math32"
)

type BlurredRoundedRectangle struct {
	// The base rectangle to use for the blur effect.
	Rect curve.Rect
	// The color of the blurred rectangle.
	Color color.Color
	// The radius of the blur effect.
	Radius float32
	// The standard deviation of the blur effect.
	StdDev float32
	// Whether to opt into further optimizations.
	LowPrecision bool
}

var _ Paint = BlurredRoundedRectangle{}

// Encode implements [Paint].
func (b BlurredRoundedRectangle) Encode(transform curve.Affine) EncodedPaint {
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

	return &EncodedBlurredRoundedRectangle{
		Exponent:      exponent,
		RecipExponent: 1.0 / exponent,
		Width:         width,
		Height:        height,
		Scale:         scale,
		R1:            r1,
		RecipStdDev:   recipStdDev,
		MinEdge:       minEdge,
		Color:         ColorToInternal(b.Color),
		W:             w,
		H:             h,
		Transform:     transform,
		XAdvance:      xAdvance,
		YAdvance:      yAdvance,
		LowPrecision:  b.LowPrecision,
	}
}

type EncodedBlurredRoundedRectangle struct {
	Exponent      float32
	RecipExponent float32
	Scale         float32
	RecipStdDev   float32
	MinEdge       float32
	W             float32
	H             float32
	Width         float32
	Height        float32
	R1            float32
	// The base Color for the blurred rectangle.
	Color PlainColor
	// A Transform that needs to be applied to the position of the first
	// processed pixel.
	Transform curve.Affine
	// How much to advance into the x/y direction for one step in the x
	// direction.
	XAdvance curve.Vec2
	// How much to advance into the x/y direction for one step in the y
	// direction.
	YAdvance     curve.Vec2
	LowPrecision bool
}

var _ EncodedPaint = (*EncodedBlurredRoundedRectangle)(nil)

// Opaque implements [EncodedPaint].
func (e *EncodedBlurredRoundedRectangle) Opaque() bool {
	// OPT(dh): with zero corner radius and <=1 std dev and an opaque color,
	// this paint is opaque, too.
	return false
}

// isEncodedPaint implements [EncodedPaint].
func (e *EncodedBlurredRoundedRectangle) isEncodedPaint() {}

// erf7 approximates the erf function.
func erf7(x float32) float32 {
	// TODO(dh): this function is duplicated in internal/sparse

	// See https://raphlinus.github.io/audio/2018/09/05/sigmoid.html for a little
	// explanation of this approximation to the erf function.
	x = x * (2 / math.SqrtPi)
	xx := x * x
	x = x + (0.24295+(0.03395+0.0104*xx)*xx)*(x*xx)
	return x / math32.Sqrt(1.0+x*x)
}
