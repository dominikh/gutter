// SPDX-FileCopyrightText: 2022 the Peniko Authors
// SPDX-FileCopyrightText: 2025 Dominik Honnef and contributors
//
// SPDX-License-Identifier: MIT

package gfx

import (
	"iter"

	"honnef.co/go/color"
	"honnef.co/go/curve"
)

// ColorSpace is the color space used to represent pixel values internally. This
// is the color space that is used by default for blending. All [color.Color]
// values provided by the user are converted to this color space for
// storage--however, colors do not get color mapped or clipped; out of gamut
// colors will be represented with values less than 0 or greater than 1.
//
// When values aren't constrained to the range [0, 1], all linear RGB color
// spaces as well as XYZ give identical results for linear operations.
// Multiplying two colors, however, is non-linear and will give different
// results in different linear RGB color spaces. To get predictable results, and
// to avoid color space conversions for the most common case (sRGB inputs and
// outputs), we choose linear sRGB (i.e. an RGB color space that uses the same
// primaries and white point as sRGB).
//
// Colors exposed as [4]float32 use this color space.
var ColorSpace = color.LinearSRGB

// PlainColor represents a premultiplied color in the [ColorSpace] color space.
type PlainColor [4]float32

func ColorToInternal(c color.Color) PlainColor {
	// Avoid call to c.Convert in the common case, which avoids copying c (as of
	// go1.25-devel_ed08d2ad09). This wouldn't be necessary with copy elision,
	// since c.Convert inlines and contains the same condition--but inlining
	// doesn't eliminate the copy made for the function call.
	if c.Space != ColorSpace {
		c = c.Convert(ColorSpace)
	}
	cc := PlainColor{
		float32(c.Values[0]) * float32(c.Values[3]),
		float32(c.Values[1]) * float32(c.Values[3]),
		float32(c.Values[2]) * float32(c.Values[3]),
		float32(c.Values[3]),
	}
	return cc
}

func InternalToColor(c PlainColor) color.Color {
	return color.Make(
		ColorSpace,
		float64(c[0]/max(c[3], 1e-10)),
		float64(c[1]/max(c[3], 1e-10)),
		float64(c[2]/max(c[3], 1e-10)),
		float64(c[3]),
	)
}

// isEncodedPaint implements encodedPaint.
func (s PlainColor) isEncodedPaint() {}

// Opaque implements encodedPaint.
func (s PlainColor) Opaque() bool {
	return s[3] == 1
}

type Shape interface {
	PathElements(precision float64) iter.Seq[curve.PathElement]
}

//go:generate go tool stringer -type=FillRule -trimprefix=FillRule
type FillRule int

const (
	NonZero FillRule = iota
	EvenOdd
)

//go:generate go tool stringer -type=Compose -trimprefix=Compose
type Compose uint8

// ComposeOps is the list of all supported composition operators.
var ComposeOps = []Compose{
	ComposeSrcOver, ComposeCopy, ComposeDest, ComposeClear, ComposeDestOver,
	ComposeSrcIn, ComposeDestIn, ComposeSrcOut, ComposeDestOut, ComposeSrcAtop,
	ComposeDestAtop, ComposeXor, ComposePlus,
}

// MixOps is the list of all supported color mixing operators.
var MixOps = []Mix{
	MixNormal, MixMultiply, MixScreen, MixOverlay, MixDarken,
	MixLighten, MixColorDodge, MixColorBurn, MixHardLight, MixSoftLight,
	MixDifference, MixExclusion,
}

const ComposeAffectsDestRegion = 1 << 7

const (
	// The source is placed over the destination.
	ComposeSrcOver Compose = 0
	// Only the source will be present.
	ComposeCopy Compose = 1 | ComposeAffectsDestRegion
	// Only the destination will be present.
	ComposeDest Compose = 2 | ComposeAffectsDestRegion
	// No regions are enabled.
	ComposeClear Compose = 3 | ComposeAffectsDestRegion
	// The destination is placed over the source.
	ComposeDestOver Compose = 4
	// The parts of the source that overlap with the destination are placed.
	ComposeSrcIn Compose = 5 | ComposeAffectsDestRegion
	// The parts of the destination that overlap with the source are placed.
	ComposeDestIn Compose = 6 | ComposeAffectsDestRegion
	// The parts of the source that fall outside of the destination are placed.
	ComposeSrcOut Compose = 7 | ComposeAffectsDestRegion
	// The parts of the destination that fall outside of the source are placed.
	ComposeDestOut Compose = 8 | ComposeAffectsDestRegion
	// The parts of the source which overlap the destination replace the
	// destination. The destination is placed everywhere else.
	ComposeSrcAtop Compose = 9
	// The parts of the destination which overlaps the source replace the
	// source. The source is placed everywhere else.
	ComposeDestAtop Compose = 10 | ComposeAffectsDestRegion
	// The non-overlapping regions of source and destination are combined.
	ComposeXor Compose = 11
	// The sum of the source image and destination image is displayed.
	ComposePlus Compose = 12
)

// Mix defines the color mixing function for a [blend operation](BlendMode).
//
//go:generate go tool stringer -type=Mix -trimprefix=Mix
type Mix uint8

const (
	// Default attribute which specifies no blending. The blending formula
	// simply selects the source color.
	MixNormal Mix = 0
	// Source color is multiplied by the destination color and replaces the
	// destination.
	MixMultiply Mix = 1
	// Multiplies the complements of the backdrop and source color values, then
	// complements the result.
	MixScreen Mix = 2
	// Multiplies or screens the colors, depending on the backdrop color value.
	MixOverlay Mix = 3
	// Selects the darker of the backdrop and source colors.
	MixDarken Mix = 4
	// Selects the lighter of the backdrop and source colors.
	MixLighten Mix = 5
	// Brightens the backdrop color to reflect the source color. Painting with
	// black produces no change.
	MixColorDodge Mix = 6
	// Darkens the backdrop color to reflect the source color. Painting with
	// white produces no change.
	MixColorBurn Mix = 7
	// Multiplies or screens the colors, depending on the source color value.
	// The effect is similar to shining a harsh spotlight on the backdrop.
	MixHardLight Mix = 8
	// Darkens or lightens the colors, depending on the source color value. The
	// effect is similar to shining a diffused spotlight on the backdrop.
	MixSoftLight Mix = 9
	// Subtracts the darker of the two constituent colors from the lighter
	// color.
	MixDifference Mix = 10
	// Produces an effect similar to that of the Difference mode but lower in
	// contrast. Painting with white inverts the backdrop color; painting with
	// black produces no change.
	MixExclusion Mix = 11
)

// BlendMode specifies the blend mode consisting of color mixing and composition functions.
type BlendMode struct {
	// The color mixing function
	Mix Mix
	// The layer composition function
	Compose Compose
}

type Paint interface {
	Encode(transform curve.Affine) EncodedPaint
}

type Solid color.Color

func (s Solid) Encode(_ curve.Affine) EncodedPaint {
	c := color.Color(s).Convert(ColorSpace)
	cc := ColorToInternal(c)
	return cc
}

type EncodedPaint interface {
	// Opaque reports whether it's impossible for the paint to be translucent.
	Opaque() bool
	isEncodedPaint()
}

type Layer struct {
	BlendMode BlendMode
	Opacity   float32
}
