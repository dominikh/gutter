// SPDX-FileCopyrightText: 2026 Dominik Honnef and contributors
//
// SPDX-License-Identifier: MIT

//go:build goexperiment.simd

package sparse

// TODO(dh): use VFNMADD and VFMSUB once archsimd makes it available

import (
	"fmt"
	"math"
	. "simd/archsimd"

	"honnef.co/go/gutter/debug"
	"honnef.co/go/gutter/gfx"
	"honnef.co/go/gutter/internal/arch"
	"honnef.co/go/safeish"
)

var alphasAllOnes = make([]uint64, wideTileWidth/2)

func init() {
	for i := range alphasAllOnes {
		alphasAllOnes[i] = math.MaxUint64
	}
}

func blendComplexComplex(
	dst Pixels,
	tos Pixels,
	alphas [][stripHeight]uint8,
	blend gfx.BlendMode,
	opacity float32,
) {
	if arch.GOAMD64 >= 3 || hasAVX2AndFMA3 {
		blendComplexComplexAVX(dst, tos, alphas, blend, opacity)
	} else {
		blendComplexComplexScalar(dst, tos, alphas, blend, opacity)
	}
}

func blendComplexComplexAVX(
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

	var alphas_ []uint64
	if alphas == nil {
		alphas_ = alphasAllOnes
	} else {
		alphas_ = safeish.SliceCast[[]uint64](alphas)
	}

	debug.Assert(len(dst.plane(0))/16 <= len(alphas_))

	if dst.width()%2 != 0 {
		// We do this first so that opacity is no longer live and doesn't get
		// spilled and restored, which would cause a non-VEX MOVSS instruction
		// to be emitted (unless we used additional hacky workarounds).
		alphas := alphas
		if alphas != nil {
			alphas = alphas[dst.width()-1:]
		}
		blendComplexComplexScalar(
			dst.slice(dst.width()-1, dst.width()),
			tos.slice(tos.width()-1, tos.width()),
			alphas,
			blend,
			opacity,
		)
	}

	dstR := safeish.SliceCast[[][8]float32](dst.plane(0))
	dstG := safeish.SliceCast[[][8]float32](dst.plane(1))
	dstB := safeish.SliceCast[[][8]float32](dst.plane(2))
	dstA := safeish.SliceCast[[][8]float32](dst.plane(3))

	tosR := safeish.SliceCast[[][8]float32](tos.plane(0))
	tosG := safeish.SliceCast[[][8]float32](tos.plane(1))
	tosB := safeish.SliceCast[[][8]float32](tos.plane(2))
	tosA := safeish.SliceCast[[][8]float32](tos.plane(3))

	// Help BCE
	_ = dstG[len(dstR)-1]
	_ = dstB[len(dstR)-1]
	_ = dstA[len(dstR)-1]
	_ = tosG[len(dstR)-1]
	_ = tosB[len(dstR)-1]
	_ = tosA[len(dstR)-1]

	// OPT(dh): implement shortcuts, like for ComposeClear

	one := BroadcastFloat32x8(1)
	maxByte := BroadcastFloat32x8(255)
	opacity_ := BroadcastFloat32x8(opacity)
	eps := BroadcastFloat32x8(1e-10)

	for x := range dstR {
		dstAx := LoadFloat32x8(&dstA[x])
		tosAx := LoadFloat32x8(&tosA[x])

		// OPT(dh): don't run this when fa is all zeros, assuming that even
		// matters for performance
		// OPT(dh): should we use the approximate reciprocal and a round of
		// Newton-Raphson instead?

		invas := one.Div(tosAx.Max(eps))
		invad := one.Div(dstAx.Max(eps))

		// OPT(dh): You would think it'd be a lot faster to load these on demand
		// in the individual switch cases, to reduce their lifetimes (for
		// example, Cd2 now has to be live even while we work with Cd0, and the
		// Cs vectors have to outlive the whole switch), but that is not the
		// case. Which probably means we're already bottlenecking on something
		// else. We didn't even measure slowdowns for mix modes that don't need
		// Cd at all, like MixNormal.
		Cs0 := LoadFloat32x8(&tosR[x]).Mul(invas)
		Cs1 := LoadFloat32x8(&tosG[x]).Mul(invas)
		Cs2 := LoadFloat32x8(&tosB[x]).Mul(invas)
		Cd0 := LoadFloat32x8(&dstR[x]).Mul(invad)
		Cd1 := LoadFloat32x8(&dstG[x]).Mul(invad)
		Cd2 := LoadFloat32x8(&dstB[x]).Mul(invad)

		var Cm0, Cm1, Cm2 Float32x8
		oneMinusDstA := one.Sub(dstAx)
		switch blend.Mix {
		case gfx.MixColorBurn:
			// 1 - min(1, (1-dst)/src)
			// Using max(src, eps) to avoid division by zero; this corr
			// produces 0 when src==0 and 1 when dst==1.
			Cm0 = one.Sub(one.Sub(Cd0).Div(Cs0.Max(eps)).Min(one))
			Cm1 = one.Sub(one.Sub(Cd1).Div(Cs1.Max(eps)).Min(one))
			Cm2 = one.Sub(one.Sub(Cd2).Div(Cs2.Max(eps)).Min(one))
		case gfx.MixColorDodge:
			// min(1, dst/(1-src))
			// Using max(1-src, eps) to avoid division by zero; this correctly
			// produces 0 when dst==0 and 1 when src==1.
			Cm0 = Cd0.Div(one.Sub(Cs0).Max(eps)).Min(one)
			Cm1 = Cd1.Div(one.Sub(Cs1).Max(eps)).Min(one)
			Cm2 = Cd2.Div(one.Sub(Cs2).Max(eps)).Min(one)
		case gfx.MixDarken:
			Cm0 = Cs0.Min(Cd0)
			Cm1 = Cs1.Min(Cd1)
			Cm2 = Cs2.Min(Cd2)
		case gfx.MixDifference:
			signMask := BroadcastUint32x8(1 << 31)
			Cm0 = Cd0.Sub(Cs0).AsUint32x8().AndNot(signMask).AsFloat32x8()
			Cm1 = Cd1.Sub(Cs1).AsUint32x8().AndNot(signMask).AsFloat32x8()
			Cm2 = Cd2.Sub(Cs2).AsUint32x8().AndNot(signMask).AsFloat32x8()
		case gfx.MixExclusion:
			tmp := Cd0.Mul(Cs0)
			Cm0 = Cd0.Add(Cs0).Sub(tmp.Add(tmp))

			tmp = Cd1.Mul(Cs1)
			Cm1 = Cd1.Add(Cs1).Sub(tmp.Add(tmp))

			tmp = Cd2.Mul(Cs2)
			Cm2 = Cd2.Add(Cs2).Sub(tmp.Add(tmp))
		case gfx.MixHardLight:
			// src <= 0.5: 2*src*dst
			// src > 0.5:  screen(dst, 2*src-1) = dst + (2*src-1) - dst*(2*src-1)
			half := BroadcastFloat32x8(0.5)
			twoCs0 := Cs0.Add(Cs0)
			lo0 := twoCs0.Mul(Cd0)
			s0 := twoCs0.Sub(one)
			hi0 := Cd0.Add(s0).Sub(Cd0.Mul(s0))
			mask0 := Cs0.LessEqual(half)
			Cm0 = lo0.Merge(hi0, mask0)

			twoCs1 := Cs1.Add(Cs1)
			lo1 := twoCs1.Mul(Cd1)
			s1 := twoCs1.Sub(one)
			hi1 := Cd1.Add(s1).Sub(Cd1.Mul(s1))
			mask1 := Cs1.LessEqual(half)
			Cm1 = lo1.Merge(hi1, mask1)

			twoCs2 := Cs2.Add(Cs2)
			lo2 := twoCs2.Mul(Cd2)
			s2 := twoCs2.Sub(one)
			hi2 := Cd2.Add(s2).Sub(Cd2.Mul(s2))
			mask2 := Cs2.LessEqual(half)
			Cm2 = lo2.Merge(hi2, mask2)
		case gfx.MixLighten:
			Cm0 = Cs0.Max(Cd0)
			Cm1 = Cs1.Max(Cd1)
			Cm2 = Cs2.Max(Cd2)
		case gfx.MixMultiply:
			Cm0 = Cs0.Mul(Cd0)
			Cm1 = Cs1.Mul(Cd1)
			Cm2 = Cs2.Mul(Cd2)
		case gfx.MixNormal:
			Cm0 = Cs0
			Cm1 = Cs1
			Cm2 = Cs2
		case gfx.MixOverlay:
			// Overlay is HardLight with src and dst swapped:
			// Cd <= 0.5: 2*Cd*Cs
			// Cd > 0.5:  screen(Cs, 2*Cd-1) = Cs + (2*Cd-1) - Cs*(2*Cd-1)
			half := BroadcastFloat32x8(0.5)
			twoCd0 := Cd0.Add(Cd0)
			lo0 := twoCd0.Mul(Cs0)
			d0 := twoCd0.Sub(one)
			hi0 := Cs0.Add(d0).Sub(Cs0.Mul(d0))
			mask0 := Cd0.LessEqual(half)
			Cm0 = lo0.Merge(hi0, mask0)

			twoCd1 := Cd1.Add(Cd1)
			lo1 := twoCd1.Mul(Cs1)
			d1 := twoCd1.Sub(one)
			hi1 := Cs1.Add(d1).Sub(Cs1.Mul(d1))
			mask1 := Cd1.LessEqual(half)
			Cm1 = lo1.Merge(hi1, mask1)

			twoCd2 := Cd2.Add(Cd2)
			lo2 := twoCd2.Mul(Cs2)
			d2 := twoCd2.Sub(one)
			hi2 := Cs2.Add(d2).Sub(Cs2.Mul(d2))
			mask2 := Cd2.LessEqual(half)
			Cm2 = lo2.Merge(hi2, mask2)
		case gfx.MixScreen:
			Cm0 = Cd0.Add(Cs0).Sub(Cd0.Mul(Cs0))
			Cm1 = Cd1.Add(Cs1).Sub(Cd1.Mul(Cs1))
			Cm2 = Cd2.Add(Cs2).Sub(Cd2.Mul(Cs2))
		case gfx.MixSoftLight:
			// src <= 0.5: dst - (1-2*src)*dst*(1-dst)
			// src > 0.5:  dst + (2*src-1)*(Ddst-dst)
			//   where Ddst = ((16*dst-12)*dst+4)*dst  if dst <= 0.25
			//              = 4*dst*(dst*(4*dst-3)) + 4*dst
			//                sqrt(dst)                if dst  > 0.25

			// We use 0.25 and 2 together with 1 to make 0.5, 3, and 4, to
			// reduce the number of registers that have to be live for all 3
			// planes.
			quarter := BroadcastFloat32x8(0.25)
			two := BroadcastFloat32x8(2)

			twoCs0 := Cs0.Add(Cs0)
			oneMinusCd0 := one.Sub(Cd0)
			lo0 := Cd0.Sub(one.Sub(twoCs0).Mul(Cd0).Mul(oneMinusCd0))
			fourCd0 := two.Add(two).Mul(Cd0)
			DdstLo0 := fourCd0.Sub(two.Add(one)).Mul(Cd0).MulAdd(fourCd0, fourCd0)
			DdstHi0 := Cd0.Sqrt()
			dstMask0 := Cd0.LessEqual(quarter)
			Ddst0 := DdstLo0.Merge(DdstHi0, dstMask0)
			// Cd0 + (twoCs0 - one) * Ddst0 - Cd0
			hi0 := Ddst0.Sub(Cd0).MulAdd(twoCs0.Sub(one), Cd0)
			half := quarter.Add(quarter)
			srcMask0 := Cs0.LessEqual(half)
			Cm0 = lo0.Merge(hi0, srcMask0)

			twoCs1 := Cs1.Add(Cs1)
			oneMinusCd1 := one.Sub(Cd1)
			lo1 := Cd1.Sub(one.Sub(twoCs1).Mul(Cd1).Mul(oneMinusCd1))
			fourCd1 := two.Add(two).Mul(Cd1)
			DdstLo1 := fourCd1.Sub(two.Add(one)).Mul(Cd1).MulAdd(fourCd1, fourCd1)
			DdstHi1 := Cd1.Sqrt()
			dstMask1 := Cd1.LessEqual(quarter)
			Ddst1 := DdstLo1.Merge(DdstHi1, dstMask1)
			hi1 := Ddst1.Sub(Cd1).MulAdd(twoCs1.Sub(one), Cd1)
			half = quarter.Add(quarter)
			srcMask1 := Cs1.LessEqual(half)
			Cm1 = lo1.Merge(hi1, srcMask1)

			twoCs2 := Cs2.Add(Cs2)
			oneMinusCd2 := one.Sub(Cd2)
			lo2 := Cd2.Sub(one.Sub(twoCs2).Mul(Cd2).Mul(oneMinusCd2))
			fourCd2 := two.Add(two).Mul(Cd2)
			DdstLo2 := fourCd2.Sub(two.Add(one)).Mul(Cd2).MulAdd(fourCd2, fourCd2)
			DdstHi2 := Cd2.Sqrt()
			dstMask2 := Cd2.LessEqual(quarter)
			Ddst2 := DdstLo2.Merge(DdstHi2, dstMask2)
			hi2 := Ddst2.Sub(Cd2).MulAdd(twoCs2.Sub(one), Cd2)
			half = quarter.Add(quarter)
			srcMask2 := Cs2.LessEqual(half)
			Cm2 = lo2.Merge(hi2, srcMask2)
		}

		Cr0 := dstAx.MulAdd(Cm0, Cs0.Mul(oneMinusDstA))
		Cr1 := dstAx.MulAdd(Cm1, Cs1.Mul(oneMinusDstA))
		Cr2 := dstAx.MulAdd(Cm2, Cs2.Mul(oneMinusDstA))

		var fa, fb Float32x8
		switch blend.Compose {
		case gfx.ComposeClear:
			// Nothing to do
		case gfx.ComposeCopy:
			fa = one
		case gfx.ComposeDest:
			fb = one
		case gfx.ComposePlus:
			fa = one
			fb = one
		case gfx.ComposeDestAtop:
			fa = one.Sub(dstAx)
			fb = tosAx.Mul(opacity_)
		case gfx.ComposeDestIn:
			fb = tosAx.Mul(opacity_)
		case gfx.ComposeDestOut:
			fb = one.Sub(tosAx.Mul(opacity_))
		case gfx.ComposeDestOver:
			fa = one.Sub(dstAx)
			fb = one
		case gfx.ComposeSrcAtop:
			fa = dstAx
			fb = one.Sub(tosAx.Mul(opacity_))
		case gfx.ComposeSrcIn:
			fa = dstAx
		case gfx.ComposeSrcOut:
			fa = one.Sub(dstAx)
		case gfx.ComposeSrcOver:
			fa = one
			fb = one.Sub(tosAx.Mul(opacity_))
		case gfx.ComposeXor:
			fa = one.Sub(dstAx)
			fb = one.Sub(tosAx.Mul(opacity_))
		default:
			panic(fmt.Sprintf("unexpected gfx.Compose: %#v", blend.Compose))
		}

		a := Uint64x2{}.
			SetElem(0, alphas_[x]).
			AsUint8x16().
			ExtendLo8ToUint32().
			AsInt32x8().
			ConvertToFloat32().
			Div(maxByte)
		ma := a.Mul(opacity_).Mul(fa)
		mb := a.Mul(fb)
		oneMinusAlpha := one.Sub(a)
		// OPT(dh): is it worth checking for ma being all zeros?

		// Cr0 * tosAx * ma + dstRx * mb + oneMinusAlpha * dstRx
		dstRx := LoadFloat32x8(&dstR[x])
		dstRx.
			MulAdd(oneMinusAlpha, Cr0.Mul(tosAx).MulAdd(ma, dstRx.Mul(mb))).
			Store(&dstR[x])

		dstGx := LoadFloat32x8(&dstG[x])
		dstGx.
			MulAdd(oneMinusAlpha, Cr1.Mul(tosAx).MulAdd(ma, dstGx.Mul(mb))).
			Store(&dstG[x])

		dstBx := LoadFloat32x8(&dstB[x])
		dstBx.
			MulAdd(oneMinusAlpha, Cr2.Mul(tosAx).MulAdd(ma, dstBx.Mul(mb))).
			Store(&dstB[x])

		// tosAx * ma + dstAx * mb + oneMinusAlpha * dstAx
		dstAx.
			MulAdd(oneMinusAlpha, mb.MulAdd(dstAx, ma.Mul(tosAx))).
			Min(one).
			Max(Float32x8{}).Store(&dstA[x])
	}

	ClearAVXUpperBits()
}
