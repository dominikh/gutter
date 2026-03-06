// SPDX-FileCopyrightText: 2026 Dominik Honnef and contributors
//
// SPDX-License-Identifier: MIT

//go:build goexperiment.simd

package sparse

import (
	"math"
	. "simd/archsimd"

	"honnef.co/go/gutter/gfx"
	"honnef.co/go/gutter/internal/arch"
)

const maxSIMDRanges = 16

func (gf *gradientFiller) fillSIMD(dst Pixels) bool {
	if !arch.AVX2() || !arch.FMA() {
		return false
	}
	if len(gf.gradient.ranges) > maxSIMDRanges {
		return false
	}

	switch kind := gf.gradient.kind.(type) {
	case encodedLinearGradient:
		gf.fillLinearSIMD(dst, kind)
	case encodedRadialGradient:
		gf.fillRadialSIMD(dst, kind)
	case encodedStripGradient:
		gf.fillStripSIMD(dst, kind)
	case encodedFocalGradient:
		gf.fillFocalSIMD(dst, kind)
	case encodedSweepGradient:
		gf.fillSweepSIMD(dst, kind)
	default:
		return false
	}
	ClearAVXUpperBits()
	return true
}

// applyExtendSIMD applies the gradient extend mode to a vector of t values.
func applyExtendSIMD(
	t Float32x4,
	extend gfx.GradientExtend,
) Float32x4 {
	zero := BroadcastFloat32x4(0)
	one := BroadcastFloat32x4(1)
	switch extend {
	case gfx.GradientExtendPad:
		return t.Max(zero).Min(one)
	case gfx.GradientExtendRepeat:
		// t - floor(t)
		return t.Sub(t.Floor())
	case gfx.GradientExtendReflect:
		// min(max(abs((t-1) - 2*floor((t-1)*0.5) - 1), 0), 1)
		half := BroadcastFloat32x4(0.5)
		two := BroadcastFloat32x4(2)
		tMinus1 := t.Sub(one)
		floored := tMinus1.Mul(half).Floor()
		reflected := tMinus1.Sub(two.Mul(floored)).Sub(one)
		// abs via clearing sign bit
		signMask := BroadcastInt32x4(0x7FFFFFFF)
		absReflected := reflected.AsInt32x4().
			And(signMask).
			AsFloat32x4()
		return absReflected.Max(zero).Min(one)
	}
	panic("unreachable")
}

// cascadeMerge evaluates the gradient color for a vector of t values
// by merging all ranges using conditional selection.
func cascadeMerge(
	t Float32x4,
	ranges []gradientRange,
) (r, g, b, a Float32x4) {
	rng := &ranges[0]
	r = BroadcastFloat32x4(rng.scale[0]).
		MulAdd(t, BroadcastFloat32x4(rng.bias[0]))
	g = BroadcastFloat32x4(rng.scale[1]).
		MulAdd(t, BroadcastFloat32x4(rng.bias[1]))
	b = BroadcastFloat32x4(rng.scale[2]).
		MulAdd(t, BroadcastFloat32x4(rng.bias[2]))
	a = BroadcastFloat32x4(rng.scale[3]).
		MulAdd(t, BroadcastFloat32x4(rng.bias[3]))

	for i := 1; i < len(ranges); i++ {
		rng = &ranges[i]
		mask := t.GreaterEqual(BroadcastFloat32x4(ranges[i-1].x1))
		cr := BroadcastFloat32x4(rng.scale[0]).
			MulAdd(t, BroadcastFloat32x4(rng.bias[0]))
		cg := BroadcastFloat32x4(rng.scale[1]).
			MulAdd(t, BroadcastFloat32x4(rng.bias[1]))
		cb := BroadcastFloat32x4(rng.scale[2]).
			MulAdd(t, BroadcastFloat32x4(rng.bias[2]))
		ca := BroadcastFloat32x4(rng.scale[3]).
			MulAdd(t, BroadcastFloat32x4(rng.bias[3]))
		// Merge(y, mask) = mask ? x : y
		// cr.Merge(r, mask) = mask ? cr : r
		// When t >= x1[i-1] (mask true), use new color (cr).
		r = cr.Merge(r, mask)
		g = cg.Merge(g, mask)
		b = cb.Merge(b, mask)
		a = ca.Merge(a, mask)
	}
	return r, g, b, a
}

// storeSIMD writes a column of 4 pixels (one SIMD vector per channel)
// to the planar Pixels buffer at column index x.
func storeSIMD(dst Pixels, x int, r, g, b, a Float32x4) {
	r.Store(&dst[0][x])
	g.Store(&dst[1][x])
	b.Store(&dst[2][x])
	a.Store(&dst[3][x])
}

// initPosVectors constructs the initial posX and posY vectors for a
// column, with 4 rows offset by yAdvX / yAdvY respectively.
func initPosVectors(
	startX, startY, yAdvX, yAdvY float32,
) (posX, posY Float32x4) {
	idx := LoadFloat32x4((*[4]float32)(_01230123[:]))
	posX = BroadcastFloat32x4(yAdvX).MulAdd(idx, BroadcastFloat32x4(startX))
	posY = BroadcastFloat32x4(yAdvY).MulAdd(idx, BroadcastFloat32x4(startY))
	return posX, posY
}

func (gf *gradientFiller) fillLinearSIMD(
	dst Pixels,
	_ encodedLinearGradient,
) {
	startX := float32(gf.curPos.X)
	yAdvX := float32(gf.gradient.yAdvance.X)
	xAdvX := float32(gf.gradient.xAdvance.X)

	var posXArr [4]float32
	posXArr[0] = startX
	posXArr[1] = startX + yAdvX
	posXArr[2] = startX + 2*yAdvX
	posXArr[3] = startX + 3*yAdvX
	posX := LoadFloat32x4(&posXArr)

	xAdvXVec := BroadcastFloat32x4(xAdvX)

	width := len(dst[0])
	for x := range width {
		t := applyExtendSIMD(posX, gf.gradient.extend)
		r, g, b, a := cascadeMerge(t, gf.gradient.ranges)
		storeSIMD(dst, x, r, g, b, a)
		posX = posX.Add(xAdvXVec)
	}
}

func (gf *gradientFiller) fillRadialSIMD(
	dst Pixels,
	kind encodedRadialGradient,
) {
	startX := float32(gf.curPos.X)
	startY := float32(gf.curPos.Y)
	yAdvX := float32(gf.gradient.yAdvance.X)
	yAdvY := float32(gf.gradient.yAdvance.Y)
	xAdvX := float32(gf.gradient.xAdvance.X)
	xAdvY := float32(gf.gradient.xAdvance.Y)

	posX, posY := initPosVectors(startX, startY, yAdvX, yAdvY)
	xAdvXVec := BroadcastFloat32x4(xAdvX)
	xAdvYVec := BroadcastFloat32x4(xAdvY)

	biasVec := BroadcastFloat32x4(kind.bias)
	scaleVec := BroadcastFloat32x4(kind.scale)

	width := len(dst[0])
	for x := range width {
		// dist = sqrt(posX^2 + posY^2)
		dist := posX.Mul(posX).Add(posY.Mul(posY)).Sqrt()
		// t = scale * dist + bias
		t := scaleVec.MulAdd(dist, biasVec)
		t = applyExtendSIMD(t, gf.gradient.extend)
		r, g, b, a := cascadeMerge(t, gf.gradient.ranges)
		storeSIMD(dst, x, r, g, b, a)
		posX = posX.Add(xAdvXVec)
		posY = posY.Add(xAdvYVec)
	}
}

func (gf *gradientFiller) fillStripSIMD(
	dst Pixels,
	kind encodedStripGradient,
) {
	startX := float32(gf.curPos.X)
	startY := float32(gf.curPos.Y)
	yAdvX := float32(gf.gradient.yAdvance.X)
	yAdvY := float32(gf.gradient.yAdvance.Y)
	xAdvX := float32(gf.gradient.xAdvance.X)
	xAdvY := float32(gf.gradient.xAdvance.Y)

	posX, posY := initPosVectors(startX, startY, yAdvX, yAdvY)
	xAdvXVec := BroadcastFloat32x4(xAdvX)
	xAdvYVec := BroadcastFloat32x4(xAdvY)

	r0sq := BroadcastFloat32x4(kind.r0ScaledSquared)
	zero := BroadcastFloat32x4(0)

	width := len(dst[0])
	for x := range width {
		// inner = r0sq - posY^2
		inner := r0sq.Sub(posY.Mul(posY))
		// definedMask is true where inner >= 0
		definedMask := inner.GreaterEqual(zero)
		// t = posX + sqrt(max(inner, 0))
		t := posX.Add(inner.Max(zero).Sqrt())
		t = applyExtendSIMD(t, gf.gradient.extend)
		r, g, b, a := cascadeMerge(t, gf.gradient.ranges)
		// Zero out undefined pixels
		r = r.Masked(definedMask)
		g = g.Masked(definedMask)
		b = b.Masked(definedMask)
		a = a.Masked(definedMask)
		storeSIMD(dst, x, r, g, b, a)
		posX = posX.Add(xAdvXVec)
		posY = posY.Add(xAdvYVec)
	}
}

func (gf *gradientFiller) fillFocalSIMD(
	dst Pixels,
	kind encodedFocalGradient,
) {
	startX := float32(gf.curPos.X)
	startY := float32(gf.curPos.Y)
	yAdvX := float32(gf.gradient.yAdvance.X)
	yAdvY := float32(gf.gradient.yAdvance.Y)
	xAdvX := float32(gf.gradient.xAdvance.X)
	xAdvY := float32(gf.gradient.xAdvance.Y)

	posX, posY := initPosVectors(startX, startY, yAdvX, yAdvY)
	xAdvXVec := BroadcastFloat32x4(xAdvX)
	xAdvYVec := BroadcastFloat32x4(xAdvY)

	fp0Vec := BroadcastFloat32x4(kind.fp0)
	fp1Vec := BroadcastFloat32x4(kind.fp1)
	zero := BroadcastFloat32x4(0)
	one := BroadcastFloat32x4(1)

	focalOnCircle := kind.focalData.focalOnCircle()
	wellBehaved := kind.focalData.wellBehaved()
	swapped := kind.focalData.swapped()
	negFocalX := 1.0-kind.focalData.fFocalX < 0.0
	nativelyFocal := kind.focalData.nativelyFocal()

	width := len(dst[0])
	for x := range width {
		var t Float32x4
		if focalOnCircle {
			// xy_to_2pt_conical_focal_on_circle: t = posX + posY*posY/posX
			t = posX.Add(posY.Mul(posY).Div(posX))
		} else if wellBehaved {
			// xy_to_2pt_conical_well_behaved:
			// t = sqrt(posX^2 + posY^2) - posX*fp0
			t = posX.Mul(posX).
				Add(posY.Mul(posY)).
				Sqrt().
				Sub(posX.Mul(fp0Vec))
		} else if swapped || negFocalX {
			// xy_to_2pt_conical_smaller:
			// t = -sqrt(posX^2 - posY^2) - posX*fp0
			inner := posX.Mul(posX).Sub(posY.Mul(posY)).Max(zero)
			t = zero.Sub(inner.Sqrt()).Sub(posX.Mul(fp0Vec))
		} else {
			// xy_to_2pt_conical_greater:
			// t = sqrt(posX^2 - posY^2) - posX*fp0
			inner := posX.Mul(posX).Sub(posY.Mul(posY)).Max(zero)
			t = inner.Sqrt().Sub(posX.Mul(fp0Vec))
		}

		var definedMask Mask32x4
		if !wellBehaved {
			// mask_2pt_conical_degenerates: defined where t > 0 && !isNaN(t)
			tGreaterZero := t.Greater(zero)
			// NaN != NaN, so t.Equal(t) is true for non-NaN values
			tNotNaN := t.Equal(t)
			definedMask = tGreaterZero.And(tNotNaN)
		}

		if negFocalX {
			// negate_x
			t = zero.Sub(t)
		}

		if !nativelyFocal {
			// alter_2pt_conical_compensate_focal
			t = t.Add(fp1Vec)
		}

		if swapped {
			// alter_2pt_conical_unswap
			t = one.Sub(t)
		}

		t = applyExtendSIMD(t, gf.gradient.extend)
		r, g, b, a := cascadeMerge(t, gf.gradient.ranges)

		if !wellBehaved {
			r = r.Masked(definedMask)
			g = g.Masked(definedMask)
			b = b.Masked(definedMask)
			a = a.Masked(definedMask)
		}

		storeSIMD(dst, x, r, g, b, a)
		posX = posX.Add(xAdvXVec)
		posY = posY.Add(xAdvYVec)
	}
}

// atan2SIMD computes atan2(y, x) for Float32x4 vectors using a
// 7th-order minimax polynomial approximation with |error| < 2e-5.
func atan2SIMD(y, x Float32x4) Float32x4 {
	signBitMask := BroadcastInt32x4(0x7FFFFFFF)
	absX := x.AsInt32x4().And(signBitMask).AsFloat32x4()
	absY := y.AsInt32x4().And(signBitMask).AsFloat32x4()

	zero := BroadcastFloat32x4(0)
	piOver2 := BroadcastFloat32x4(math.Pi / 2)
	pi := BroadcastFloat32x4(math.Pi)

	// Determine if we need to swap to keep ratio in [0, 1]
	swapMask := absY.Greater(absX)
	minV := absX.Min(absY)
	maxV := absX.Max(absY)

	// r = min/max, in [0, 1]
	r := minV.Div(maxV)
	r2 := r.Mul(r)

	// 7th-order minimax polynomial for atan(r)/r on [0, 1]:
	// atan(r) ≈ r * (c0 + r²*(c1 + r²*(c2 + r²*c3)))
	c0 := BroadcastFloat32x4(0.99997726)
	c1 := BroadcastFloat32x4(-0.33262347)
	c2 := BroadcastFloat32x4(0.19354346)
	c3 := BroadcastFloat32x4(-0.11643287)

	// Horner evaluation
	p := c3.MulAdd(r2, c2) // c3*r2 + c2
	p = p.MulAdd(r2, c1)   // p*r2 + c1
	p = p.MulAdd(r2, c0)   // p*r2 + c0
	result := r.Mul(p)     // r * polynomial

	// If swapped (absY > absX): result = pi/2 - result
	// swapMask true => use swapped form (receiver)
	// piOver2.Sub(result).Merge(result, swapMask)
	// = swapMask ? (pi/2 - result) : result
	result = piOver2.Sub(result).Merge(result, swapMask)

	// If x < 0: result = pi - result
	xNeg := x.Less(zero)
	result = pi.Sub(result).Merge(result, xNeg)

	// Apply sign of y: copy sign bit from y to result
	ySignBit := y.AsInt32x4().
		And(BroadcastInt32x4(math.MinInt32))
	result = result.AsInt32x4().Xor(ySignBit).AsFloat32x4()

	return result
}

func (gf *gradientFiller) fillSweepSIMD(
	dst Pixels,
	kind encodedSweepGradient,
) {
	startX := float32(gf.curPos.X)
	startY := float32(gf.curPos.Y)
	yAdvX := float32(gf.gradient.yAdvance.X)
	yAdvY := float32(gf.gradient.yAdvance.Y)
	xAdvX := float32(gf.gradient.xAdvance.X)
	xAdvY := float32(gf.gradient.xAdvance.Y)

	posX, posY := initPosVectors(startX, startY, yAdvX, yAdvY)
	xAdvXVec := BroadcastFloat32x4(xAdvX)
	xAdvYVec := BroadcastFloat32x4(xAdvY)

	zero := BroadcastFloat32x4(0)
	twoPi := BroadcastFloat32x4(float32(2 * math.Pi))
	startAngleVec := BroadcastFloat32x4(kind.startAngle)
	invAngleDeltaVec := BroadcastFloat32x4(kind.invAngleDelta)

	width := len(dst[0])
	for x := range width {
		// angle = atan2(-posY, posX)
		negPosY := zero.Sub(posY)
		angle := atan2SIMD(negPosY, posX)

		// adj = angle if angle >= 0, else angle + 2*pi
		nonNeg := angle.GreaterEqual(zero)
		// angle.Merge(angle.Add(twoPi), nonNeg)
		// = nonNeg ? angle : angle+2pi
		adj := angle.Merge(angle.Add(twoPi), nonNeg)

		// t = (adj - startAngle) * invAngleDelta
		t := adj.Sub(startAngleVec).Mul(invAngleDeltaVec)
		t = applyExtendSIMD(t, gf.gradient.extend)
		r, g, b, a := cascadeMerge(t, gf.gradient.ranges)
		storeSIMD(dst, x, r, g, b, a)
		posX = posX.Add(xAdvXVec)
		posY = posY.Add(xAdvYVec)
	}
}
