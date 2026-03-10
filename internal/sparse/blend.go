// SPDX-FileCopyrightText: 2022 the Peniko Authors
// SPDX-FileCopyrightText: 2025 Dominik Honnef and contributors
//
// SPDX-License-Identifier: MIT

package sparse

import (
	"math"

	"honnef.co/go/gutter/gfx"
	"honnef.co/go/safeish"
	"honnef.co/go/stuff/math/math32"
)

func mixColorDodge(dst, src float32) float32 {
	switch {
	case src == 1:
		return 1
	default:
		return min(1, dst/(1-src))
	}
}

func mixColorBurn(dst, src float32) float32 {
	switch {
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

func blendComplexComplexScalar(
	dst Pixels,
	tos Pixels,
	alphas [][stripHeight]uint8,
	blend gfx.BlendMode,
	opacity float32,
) {
	switch blend.Compose {
	case gfx.ComposeClear:
		if alphas == nil {
			clear(dst.plane(0))
			clear(dst.plane(1))
			clear(dst.plane(2))
			clear(dst.plane(3))
			return
		}
	case gfx.ComposeDest:
		// TODO(dh): is this actually correct, or does it depend on opacity? or
		// on alphas?
		return
	case gfx.ComposeCopy:
		if blend.Mix == gfx.MixNormal && alphas == nil && opacity == 1 {
			copy(dst.plane(0), tos.plane(0))
			copy(dst.plane(1), tos.plane(1))
			copy(dst.plane(2), tos.plane(2))
			copy(dst.plane(3), tos.plane(3))
			return
		}
	}

	if alphas != nil {
		_ = alphas[len(dst.plane(0))-1]
	}
	getAlpha := func(i, j int) float32 {
		if alphas == nil {
			return 1
		} else {
			return (float32(safeish.Index(alphas, i)[j]) / 255.0)
		}
	}

	for i := range dst.plane(0) {
		for j := range stripHeight {
			dstA := dst.plane(3)[i][j]
			tosA := tos.plane(3)[i][j]

			var fa, fb float32
			switch blend.Compose {
			case gfx.ComposeClear:
				fa = 0
				fb = 0
			case gfx.ComposeCopy:
				fa = 1
				fb = 0
			case gfx.ComposeDest:
				fa = 0
				fb = 1
			case gfx.ComposePlus:
				fa = 1
				fb = 1
			case gfx.ComposeDestAtop:
				fa = 1 - dstA
				fb = tosA * opacity
			case gfx.ComposeDestIn:
				fa = 0
				fb = tosA * opacity
			case gfx.ComposeDestOut:
				fa = 0
				fb = 1 - tosA*opacity
			case gfx.ComposeDestOver:
				fa = 1 - dstA
				fb = 1
			case gfx.ComposeSrcAtop:
				fa = dstA
				fb = 1 - tosA*opacity
			case gfx.ComposeSrcIn:
				fa = dstA
				fb = 0
			case gfx.ComposeSrcOut:
				fa = 1 - dstA
				fb = 0
			case gfx.ComposeSrcOver:
				fa = 1
				fb = 1 - tosA*opacity
			case gfx.ComposeXor:
				fa = 1 - dstA
				fb = 1 - tosA*opacity
			}

			var Cr0, Cr1, Cr2 float32
			if fa != 0 && blend.Mix != gfx.MixNormal {
				invas := 1.0 / max(tosA, 1e-10)
				invad := 1.0 / max(dstA, 1e-10)
				Cd0 := dst.plane(0)[i][j] * invad
				Cd1 := dst.plane(1)[i][j] * invad
				Cd2 := dst.plane(2)[i][j] * invad
				Cs0 := tos.plane(0)[i][j] * invas
				Cs1 := tos.plane(1)[i][j] * invas
				Cs2 := tos.plane(2)[i][j] * invas

				var Cm0, Cm1, Cm2 float32
				switch blend.Mix {
				case gfx.MixColorBurn:
					Cm0 = mixColorBurn(Cd0, Cs0)
					Cm1 = mixColorBurn(Cd1, Cs1)
					Cm2 = mixColorBurn(Cd2, Cs2)
				case gfx.MixColorDodge:
					Cm0 = mixColorDodge(Cd0, Cs0)
					Cm1 = mixColorDodge(Cd1, Cs1)
					Cm2 = mixColorDodge(Cd2, Cs2)
				case gfx.MixDarken:
					Cm0 = min(Cs0, Cd0)
					Cm1 = min(Cs1, Cd1)
					Cm2 = min(Cs2, Cd2)
				case gfx.MixDifference:
					Cm0 = math32.Abs(Cd0 - Cs0)
					Cm1 = math32.Abs(Cd1 - Cs1)
					Cm2 = math32.Abs(Cd2 - Cs2)
				case gfx.MixExclusion:
					Cm0 = Cd0 + Cs0 - 2*Cd0*Cs0
					Cm1 = Cd1 + Cs1 - 2*Cd1*Cs1
					Cm2 = Cd2 + Cs2 - 2*Cd2*Cs2
				case gfx.MixHardLight:
					Cm0 = mixHardLight(Cd0, Cs0)
					Cm1 = mixHardLight(Cd1, Cs1)
					Cm2 = mixHardLight(Cd2, Cs2)
				case gfx.MixLighten:
					Cm0 = max(Cs0, Cd0)
					Cm1 = max(Cs1, Cd1)
					Cm2 = max(Cs2, Cd2)
				case gfx.MixMultiply:
					Cm0 = Cs0 * Cd0
					Cm1 = Cs1 * Cd1
					Cm2 = Cs2 * Cd2
				case gfx.MixNormal:
					Cm0 = Cs0
					Cm1 = Cs1
					Cm2 = Cs2
				case gfx.MixOverlay:
					Cm0 = mixHardLight(Cs0, Cd0)
					Cm1 = mixHardLight(Cs1, Cd1)
					Cm2 = mixHardLight(Cs2, Cd2)
				case gfx.MixScreen:
					Cm0 = Cd0 + Cs0 - (Cd0 * Cs0)
					Cm1 = Cd1 + Cs1 - (Cd1 * Cs1)
					Cm2 = Cd2 + Cs2 - (Cd2 * Cs2)
				case gfx.MixSoftLight:
					Cm0 = mixSoftLight(Cd0, Cs0)
					Cm1 = mixSoftLight(Cd1, Cs1)
					Cm2 = mixSoftLight(Cd2, Cs2)
				}
				Cr0 = (1-dstA)*Cs0 + dstA*Cm0
				Cr1 = (1-dstA)*Cs1 + dstA*Cm1
				Cr2 = (1-dstA)*Cs2 + dstA*Cm2
			}

			// The general composition formula is
			//
			// 	outₖ = srcₖ × opacity × fa + dstₖ × fb
			//
			// The result of compositing then has to be SrcOver-blended with
			// dst using the clip's alpha:
			//
			// 	aadₖ = α × outₖ + (1-α) × dstₖ
			//
			// Substituting out, we get:
			//
			// 	aadₖ = α × (srcₖ × opacity × fa + dstₖ × fb) + (1-α) × dstₖ
			// 	     = α × srcₖ × opacity × fa + α × dstₖ × fb + (1-α) × dstₖ
			//
			// α, opacity, fa, and fb don't vary per color channel, so to avoid
			// unnecessary multiplications, we let
			//
			// 	ma = α × opacity × fa
			// 	mb = α × fb
			//
			// and use
			//
			// 	aadₖ = srcₖ × ma + dstₖ × mb + (1-α) × dstₖ
			a := getAlpha(i, j)
			ma := a * opacity * fa
			mb := a * fb
			oneMinusAlpha := 1 - a
			if ma == 0 {
				dst.plane(0)[i][j] = dst.plane(0)[i][j]*mb + oneMinusAlpha*dst.plane(0)[i][j]
				dst.plane(1)[i][j] = dst.plane(1)[i][j]*mb + oneMinusAlpha*dst.plane(1)[i][j]
				dst.plane(2)[i][j] = dst.plane(2)[i][j]*mb + oneMinusAlpha*dst.plane(2)[i][j]
				dst.plane(3)[i][j] = clamp(dstA*mb+oneMinusAlpha*dstA, 0, 1)
			} else if blend.Mix == gfx.MixNormal {
				dst.plane(0)[i][j] = tos.plane(0)[i][j]*ma + dst.plane(0)[i][j]*mb + oneMinusAlpha*dst.plane(0)[i][j]
				dst.plane(1)[i][j] = tos.plane(1)[i][j]*ma + dst.plane(1)[i][j]*mb + oneMinusAlpha*dst.plane(1)[i][j]
				dst.plane(2)[i][j] = tos.plane(2)[i][j]*ma + dst.plane(2)[i][j]*mb + oneMinusAlpha*dst.plane(2)[i][j]
				dst.plane(3)[i][j] = clamp(tosA*ma+dstA*mb+oneMinusAlpha*dstA, 0, 1)
			} else {
				dst.plane(0)[i][j] = (Cr0*tosA)*ma + dst.plane(0)[i][j]*mb + oneMinusAlpha*dst.plane(0)[i][j]
				dst.plane(1)[i][j] = (Cr1*tosA)*ma + dst.plane(1)[i][j]*mb + oneMinusAlpha*dst.plane(1)[i][j]
				dst.plane(2)[i][j] = (Cr2*tosA)*ma + dst.plane(2)[i][j]*mb + oneMinusAlpha*dst.plane(2)[i][j]
				dst.plane(3)[i][j] = clamp(tosA*ma+dstA*mb+oneMinusAlpha*dstA, 0, 1)
			}
		}
	}
}

func blendSimpleSimple(
	dst Pixels,
	nos gfx.PlainColor,
	tos gfx.PlainColor,
	blend gfx.BlendMode,
) {
	switch blend.Compose {
	case gfx.ComposeClear:
		clear(dst.plane(0))
		clear(dst.plane(1))
		clear(dst.plane(2))
		clear(dst.plane(3))
		return
	case gfx.ComposeDest:
		return
	case gfx.ComposeCopy:
		if blend.Mix == gfx.MixNormal {
			memsetColumns(dst, tos)
			return
		}
	}

	var fa, fb float32
	switch blend.Compose {
	case gfx.ComposeClear:
		fa, fb = 0, 0
	case gfx.ComposeCopy:
		fa, fb = 1, 0
	case gfx.ComposeDest:
		fa, fb = 0, 1
	case gfx.ComposePlus:
		fa, fb = 1, 1
	case gfx.ComposeDestAtop:
		fa = 1 - nos[3]
		fb = tos[3]
	case gfx.ComposeDestIn:
		fa = 0
		fb = tos[3]
	case gfx.ComposeDestOut:
		fa = 0
		fb = 1 - tos[3]
	case gfx.ComposeDestOver:
		fa = 1 - nos[3]
		fb = 1
	case gfx.ComposeSrcAtop:
		fa = nos[3]
		fb = 1 - tos[3]
	case gfx.ComposeSrcIn:
		fa = nos[3]
		fb = 0
	case gfx.ComposeSrcOut:
		fa = 1 - nos[3]
		fb = 0
	case gfx.ComposeSrcOver:
		fa = 1
		fb = 1 - tos[3]
	case gfx.ComposeXor:
		fa = 1 - nos[3]
		fb = 1 - tos[3]
	}

	var Cr0, Cr1, Cr2 float32
	if fa != 0 && blend.Mix != gfx.MixNormal {
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
		case gfx.MixColorBurn:
			Cm0 = mixColorBurn(Cd0, Cs0)
			Cm1 = mixColorBurn(Cd1, Cs1)
			Cm2 = mixColorBurn(Cd2, Cs2)
		case gfx.MixColorDodge:
			Cm0 = mixColorDodge(Cd0, Cs0)
			Cm1 = mixColorDodge(Cd1, Cs1)
			Cm2 = mixColorDodge(Cd2, Cs2)
		case gfx.MixDarken:
			Cm0 = min(Cs0, Cd0)
			Cm1 = min(Cs1, Cd1)
			Cm2 = min(Cs2, Cd2)
		case gfx.MixDifference:
			Cm0 = math32.Abs(Cd0 - Cs0)
			Cm1 = math32.Abs(Cd1 - Cs1)
			Cm2 = math32.Abs(Cd2 - Cs2)
		case gfx.MixExclusion:
			Cm0 = Cd0 + Cs0 - 2*Cd0*Cs0
			Cm1 = Cd1 + Cs1 - 2*Cd1*Cs1
			Cm2 = Cd2 + Cs2 - 2*Cd2*Cs2
		case gfx.MixHardLight:
			Cm0 = mixHardLight(Cd0, Cs0)
			Cm1 = mixHardLight(Cd1, Cs1)
			Cm2 = mixHardLight(Cd2, Cs2)
		case gfx.MixLighten:
			Cm0 = max(Cs0, Cd0)
			Cm1 = max(Cs1, Cd1)
			Cm2 = max(Cs2, Cd2)
		case gfx.MixMultiply:
			Cm0 = Cs0 * Cd0
			Cm1 = Cs1 * Cd1
			Cm2 = Cs2 * Cd2
		case gfx.MixNormal:
			Cm0 = Cs0
			Cm1 = Cs1
			Cm2 = Cs2
		case gfx.MixOverlay:
			Cm0 = mixHardLight(Cs0, Cd0)
			Cm1 = mixHardLight(Cs1, Cd1)
			Cm2 = mixHardLight(Cs2, Cd2)
		case gfx.MixScreen:
			Cm0 = Cd0 + Cs0 - (Cd0 * Cs0)
			Cm1 = Cd1 + Cs1 - (Cd1 * Cs1)
			Cm2 = Cd2 + Cs2 - (Cd2 * Cs2)
		case gfx.MixSoftLight:
			Cm0 = mixSoftLight(Cd0, Cs0)
			Cm1 = mixSoftLight(Cd1, Cs1)
			Cm2 = mixSoftLight(Cd2, Cs2)
		}
		Cr0 = (1-nos[3])*Cs0 + nos[3]*Cm0
		Cr1 = (1-nos[3])*Cs1 + nos[3]*Cm1
		Cr2 = (1-nos[3])*Cs2 + nos[3]*Cm2
	}

	var nc gfx.PlainColor
	if fa == 0 {
		nc = gfx.PlainColor{
			nos[0] * fb,
			nos[1] * fb,
			nos[2] * fb,
			clamp(nos[3]*fb, 0, 1),
		}
	} else if blend.Mix == gfx.MixNormal {
		nc = gfx.PlainColor{
			tos[0]*fa + nos[0]*fb,
			tos[1]*fa + nos[1]*fb,
			tos[2]*fa + nos[2]*fb,
			clamp(tos[3]*fa+nos[3]*fb, 0, 1),
		}
	} else {
		nc = gfx.PlainColor{
			(Cr0*tos[3])*fa + nos[0]*fb,
			(Cr1*tos[3])*fa + nos[1]*fb,
			(Cr2*tos[3])*fa + nos[2]*fb,
			clamp(tos[3]*fa+nos[3]*fb, 0, 1),
		}
	}

	memsetColumns(dst, nc)
}
