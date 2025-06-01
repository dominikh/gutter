// SPDX-FileCopyrightText: 2022 the Peniko Authors
// SPDX-FileCopyrightText: 2025 Dominik Honnef and contributors
//
// SPDX-License-Identifier: MIT

//go:generate go tool stringer -type=Mix -trimprefix=Mix
//go:generate go tool stringer -type=Compose -trimprefix=Compose

package sparse

import (
	"math"

	"honnef.co/go/safeish"
)

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

const composeAffectsDestRegion = 1 << 7

const (
	// The source is placed over the destination.
	ComposeSrcOver Compose = 0
	// Only the source will be present.
	ComposeCopy Compose = 1 | composeAffectsDestRegion
	// Only the destination will be present.
	ComposeDest Compose = 2 | composeAffectsDestRegion
	// No regions are enabled.
	ComposeClear Compose = 3 | composeAffectsDestRegion
	// The destination is placed over the source.
	ComposeDestOver Compose = 4
	// The parts of the source that overlap with the destination are placed.
	ComposeSrcIn Compose = 5 | composeAffectsDestRegion
	// The parts of the destination that overlap with the source are placed.
	ComposeDestIn Compose = 6 | composeAffectsDestRegion
	// The parts of the source that fall outside of the destination are placed.
	ComposeSrcOut Compose = 7 | composeAffectsDestRegion
	// The parts of the destination that fall outside of the source are placed.
	ComposeDestOut Compose = 8 | composeAffectsDestRegion
	// The parts of the source which overlap the destination replace the
	// destination. The destination is placed everywhere else.
	ComposeSrcAtop Compose = 9
	// The parts of the destination which overlaps the source replace the
	// source. The source is placed everywhere else.
	ComposeDestAtop Compose = 10 | composeAffectsDestRegion
	// The non-overlapping regions of source and destination are combined.
	ComposeXor Compose = 11
	// The sum of the source image and destination image is displayed.
	ComposePlus Compose = 12
)

// Mix defines the color mixing function for a [blend operation](BlendMode).
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

func mixColorDodge(dst, src float32) float32 {
	switch {
	case dst == 0:
		return 0
	case src == 1:
		return 1
	default:
		return min(1, dst/(1-src))
	}
}

func mixColorBurn(dst, src float32) float32 {
	switch {
	case dst == 1:
		return 1
	case src == 0:
		return 0
	default:
		return 1 - min(1, (1-dst)/src)
	}
}

func mixHardLight(dst, src float32) float32 {
	if src <= 0.5 {
		return 2 * src * dst
	} else {
		src = 2*src - 1
		return dst + src - (dst * src)
	}
}

func mixSoftLight(dst, src float32) float32 {
	if src <= 0.5 {
		return dst - (1-2*src)*dst*(1-dst)
	} else {
		var Ddst float32
		if dst <= 0.25 {
			Ddst = ((16*dst-12)*dst + 4) * dst
		} else {
			Ddst = float32(math.Sqrt(float64(dst)))
		}
		return dst + (2*src-1)*(Ddst-dst)
	}
}

func blendComplexComplex(dst, tos [][stripHeight]plainColor, alphas [][stripHeight]uint8, blend BlendMode, opacity float32) {
	switch blend.Compose {
	case ComposeClear:
		clear(dst)
		return
	case ComposeDest:
		return
	case ComposeCopy:
		if blend.Mix == MixNormal && alphas == nil && opacity == 1 {
			copy(dst, tos)
			return
		}
	}

	if alphas != nil {
		_ = alphas[len(dst)-1]
	}
	getAlpha := func(i, j int) float32 {
		if alphas == nil {
			return opacity
		} else {
			return (float32(safeish.Index(alphas, i)[j]) / 255.0) * opacity
		}
	}

	_ = tos[len(dst)-1]
	for i := range dst {
		dstCol := &dst[i]
		tosCol := &tos[i]
		for j := range stripHeight {
			var fa, fb float32
			switch blend.Compose {
			case ComposeClear:
				fa, fb = 0, 0
			case ComposeCopy:
				fa, fb = 1, 0
			case ComposeDest:
				fa, fb = 0, 1
			case ComposePlus:
				fa, fb = 1, 1
			case ComposeDestAtop:
				fa = 1 - dstCol[j][3]
				fb = tosCol[j][3] * getAlpha(i, j)
			case ComposeDestIn:
				fa = 0
				fb = tosCol[j][3] * getAlpha(i, j)
			case ComposeDestOut:
				fa = 0
				fb = 1 - tosCol[j][3]*getAlpha(i, j)
			case ComposeDestOver:
				fa = 1 - dstCol[j][3]
				fb = 1
			case ComposeSrcAtop:
				fa = dstCol[j][3]
				fb = 1 - tosCol[j][3]*getAlpha(i, j)
			case ComposeSrcIn:
				fa = dstCol[j][3]
				fb = 0
			case ComposeSrcOut:
				fa = 1 - dstCol[j][3]
				fb = 0
			case ComposeSrcOver:
				fa = 1
				fb = 1 - tosCol[j][3]*getAlpha(i, j)
			case ComposeXor:
				fa = 1 - dstCol[j][3]
				fb = 1 - tosCol[j][3]*getAlpha(i, j)
			}

			var Cr0, Cr1, Cr2 float32
			if fa != 0 && blend.Mix != MixNormal {
				invas := 1.0 / max(tosCol[j][3], 1e-10)
				invad := 1.0 / max(dstCol[j][3], 1e-10)
				Cd0 := dstCol[j][0] * invad
				Cd1 := dstCol[j][1] * invad
				Cd2 := dstCol[j][2] * invad
				Cs0 := tosCol[j][0] * invas
				Cs1 := tosCol[j][1] * invas
				Cs2 := tosCol[j][2] * invas

				var Cm0, Cm1, Cm2 float32
				switch blend.Mix {
				case MixColorBurn:
					Cm0 = mixColorBurn(Cd0, Cs0)
					Cm1 = mixColorBurn(Cd1, Cs1)
					Cm2 = mixColorBurn(Cd2, Cs2)
				case MixColorDodge:
					Cm0 = mixColorDodge(Cd0, Cs0)
					Cm1 = mixColorDodge(Cd1, Cs1)
					Cm2 = mixColorDodge(Cd2, Cs2)
				case MixDarken:
					Cm0 = min(Cs0, Cd0)
					Cm1 = min(Cs1, Cd1)
					Cm2 = min(Cs2, Cd2)
				case MixDifference:
					Cm0 = abs32(Cd0 - Cs0)
					Cm1 = abs32(Cd1 - Cs1)
					Cm2 = abs32(Cd2 - Cs2)
				case MixExclusion:
					Cm0 = Cd0 + Cs0 - 2*Cd0*Cs0
					Cm1 = Cd1 + Cs1 - 2*Cd1*Cs1
					Cm2 = Cd2 + Cs2 - 2*Cd2*Cs2
				case MixHardLight:
					Cm0 = mixHardLight(Cd0, Cs0)
					Cm1 = mixHardLight(Cd1, Cs1)
					Cm2 = mixHardLight(Cd2, Cs2)
				case MixLighten:
					Cm0 = max(Cs0, Cd0)
					Cm1 = max(Cs1, Cd1)
					Cm2 = max(Cs2, Cd2)
				case MixMultiply:
					Cm0 = Cs0 * Cd0
					Cm1 = Cs1 * Cd1
					Cm2 = Cs2 * Cd2
				case MixNormal:
					Cm0 = Cs0
					Cm1 = Cs1
					Cm2 = Cs2
				case MixOverlay:
					Cm0 = mixHardLight(Cs0, Cd0)
					Cm1 = mixHardLight(Cs1, Cd1)
					Cm2 = mixHardLight(Cs2, Cd2)
				case MixScreen:
					Cm0 = Cd0 + Cs0 - (Cd0 * Cs0)
					Cm1 = Cd1 + Cs1 - (Cd1 * Cs1)
					Cm2 = Cd2 + Cs2 - (Cd2 * Cs2)
				case MixSoftLight:
					Cm0 = mixSoftLight(Cd0, Cs0)
					Cm1 = mixSoftLight(Cd1, Cs1)
					Cm2 = mixSoftLight(Cd2, Cs2)
				}
				Cr0 = (1-dstCol[j][3])*Cs0 + dstCol[j][3]*Cm0
				Cr1 = (1-dstCol[j][3])*Cs1 + dstCol[j][3]*Cm1
				Cr2 = (1-dstCol[j][3])*Cs2 + dstCol[j][3]*Cm2
			}

			if fa == 0 {
				dstCol[j][0] = dstCol[j][0] * fb
				dstCol[j][1] = dstCol[j][1] * fb
				dstCol[j][2] = dstCol[j][2] * fb
				dstCol[j][3] = min(dstCol[j][3]*fb, 1)
			} else if blend.Mix == MixNormal {
				fa *= getAlpha(i, j)
				dstCol[j][0] = tosCol[j][0]*fa + dstCol[j][0]*fb
				dstCol[j][1] = tosCol[j][1]*fa + dstCol[j][1]*fb
				dstCol[j][2] = tosCol[j][2]*fa + dstCol[j][2]*fb
				dstCol[j][3] = min(tosCol[j][3]*fa+dstCol[j][3]*fb, 1)
			} else {
				fa *= getAlpha(i, j)
				dstCol[j][0] = (Cr0*tosCol[j][3])*fa + dstCol[j][0]*fb
				dstCol[j][1] = (Cr1*tosCol[j][3])*fa + dstCol[j][1]*fb
				dstCol[j][2] = (Cr2*tosCol[j][3])*fa + dstCol[j][2]*fb
				dstCol[j][3] = min(tosCol[j][3]*fa+dstCol[j][3]*fb, 1)
			}
		}
	}
}

func blendSimpleSimple(dst [][stripHeight]plainColor, nos, tos plainColor, blend BlendMode) {
	switch blend.Compose {
	case ComposeClear:
		clear(dst)
		return
	case ComposeDest:
		return
	case ComposeCopy:
		if blend.Mix == MixNormal {
			memsetColumnsFp(dst, tos)
			return
		}
	}

	var fa, fb float32
	switch blend.Compose {
	case ComposeClear:
		fa, fb = 0, 0
	case ComposeCopy:
		fa, fb = 1, 0
	case ComposeDest:
		fa, fb = 0, 1
	case ComposePlus:
		fa, fb = 1, 1
	case ComposeDestAtop:
		fa = 1 - nos[3]
		fb = tos[3]
	case ComposeDestIn:
		fa = 0
		fb = tos[3]
	case ComposeDestOut:
		fa = 0
		fb = 1 - tos[3]
	case ComposeDestOver:
		fa = 1 - nos[3]
		fb = 1
	case ComposeSrcAtop:
		fa = nos[3]
		fb = 1 - tos[3]
	case ComposeSrcIn:
		fa = nos[3]
		fb = 0
	case ComposeSrcOut:
		fa = 1 - nos[3]
		fb = 0
	case ComposeSrcOver:
		fa = 1
		fb = 1 - tos[3]
	case ComposeXor:
		fa = 1 - nos[3]
		fb = 1 - tos[3]
	}

	var Cr0, Cr1, Cr2 float32
	if fa != 0 && blend.Mix != MixNormal {
		invas := 1.0 / max(tos[3], 1e-10)
		invad := 1.0 / max(nos[3], 1e-10)
		Cd0 := nos[0] * invad
		Cd1 := nos[1] * invad
		Cd2 := nos[2] * invad
		Cs0 := tos[0] * invas
		Cs1 := tos[1] * invas
		Cs2 := tos[2] * invas

		var Cm0, Cm1, Cm2 float32
		switch blend.Mix {
		case MixColorBurn:
			Cm0 = mixColorBurn(Cd0, Cs0)
			Cm1 = mixColorBurn(Cd1, Cs1)
			Cm2 = mixColorBurn(Cd2, Cs2)
		case MixColorDodge:
			Cm0 = mixColorDodge(Cd0, Cs0)
			Cm1 = mixColorDodge(Cd1, Cs1)
			Cm2 = mixColorDodge(Cd2, Cs2)
		case MixDarken:
			Cm0 = min(Cs0, Cd0)
			Cm1 = min(Cs1, Cd1)
			Cm2 = min(Cs2, Cd2)
		case MixDifference:
			Cm0 = abs32(Cd0 - Cs0)
			Cm1 = abs32(Cd1 - Cs1)
			Cm2 = abs32(Cd2 - Cs2)
		case MixExclusion:
			Cm0 = Cd0 + Cs0 - 2*Cd0*Cs0
			Cm1 = Cd1 + Cs1 - 2*Cd1*Cs1
			Cm2 = Cd2 + Cs2 - 2*Cd2*Cs2
		case MixHardLight:
			Cm0 = mixHardLight(Cd0, Cs0)
			Cm1 = mixHardLight(Cd1, Cs1)
			Cm2 = mixHardLight(Cd2, Cs2)
		case MixLighten:
			Cm0 = max(Cs0, Cd0)
			Cm1 = max(Cs1, Cd1)
			Cm2 = max(Cs2, Cd2)
		case MixMultiply:
			Cm0 = Cs0 * Cd0
			Cm1 = Cs1 * Cd1
			Cm2 = Cs2 * Cd2
		case MixNormal:
			Cm0 = Cs0
			Cm1 = Cs1
			Cm2 = Cs2
		case MixOverlay:
			Cm0 = mixHardLight(Cs0, Cd0)
			Cm1 = mixHardLight(Cs1, Cd1)
			Cm2 = mixHardLight(Cs2, Cd2)
		case MixScreen:
			Cm0 = Cd0 + Cs0 - (Cd0 * Cs0)
			Cm1 = Cd1 + Cs1 - (Cd1 * Cs1)
			Cm2 = Cd2 + Cs2 - (Cd2 * Cs2)
		case MixSoftLight:
			Cm0 = mixSoftLight(Cd0, Cs0)
			Cm1 = mixSoftLight(Cd1, Cs1)
			Cm2 = mixSoftLight(Cd2, Cs2)
		}
		Cr0 = (1-nos[3])*Cs0 + nos[3]*Cm0
		Cr1 = (1-nos[3])*Cs1 + nos[3]*Cm1
		Cr2 = (1-nos[3])*Cs2 + nos[3]*Cm2
	}

	var nc plainColor
	if fa == 0 {
		nc = plainColor{
			nos[0] * fb,
			nos[1] * fb,
			nos[2] * fb,
			min(nos[3]*fb, 1),
		}
	} else if blend.Mix == MixNormal {
		nc = plainColor{
			tos[0]*fa + nos[0]*fb,
			tos[1]*fa + nos[1]*fb,
			tos[2]*fa + nos[2]*fb,
			min(tos[3]*fa+nos[3]*fb, 1),
		}
	} else {
		nc = plainColor{
			(Cr0*tos[3])*fa + nos[0]*fb,
			(Cr1*tos[3])*fa + nos[1]*fb,
			(Cr2*tos[3])*fa + nos[2]*fb,
			min(tos[3]*fa+nos[3]*fb, 1),
		}
	}

	memsetColumnsFp(dst, nc)
}
