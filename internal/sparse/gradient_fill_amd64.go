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

var allOnesMasks [wideTileWidth][stripHeight]int32

func init() {
	for x := range wideTileWidth {
		for y := range stripHeight {
			allOnesMasks[x][y] = -1
		}
	}
}

func (gf *gradientFiller) fillSIMD(dst Pixels) bool {
	if !arch.AVX2() || !arch.FMA() {
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
		// t - floor(t), with correction for the edge case where the
		// subtraction produces exactly 1.0 (must map to 0.0 to match
		// the scalar path's math.Modf behavior).
		result := t.Sub(t.Floor())
		return result.Sub(result.Floor())
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

// fillLinearSIMD uses the Avo-generated LUT gather for linear gradients.
// Position and extend are computed in Go, then VGATHERDPS indexes the LUT.
func (gf *gradientFiller) fillLinearSIMD(
	dst Pixels,
	_ encodedLinearGradient,
) {
	width := len(dst[0])
	var tBuf [wideTileWidth][stripHeight]float32

	curPos := gf.curPos
	for x := range width {
		yAdv := gf.gradient.yAdvance
		var arrX [stripHeight]float32
		p := curPos
		for i := range stripHeight {
			arrX[i] = float32(p.X)
			p = p.Translate(yAdv)
		}
		posX := LoadFloat32x4(&arrX)

		t := applyExtendSIMD(posX, gf.gradient.extend)
		t.Store(&tBuf[x])
		curPos = curPos.Translate(gf.gradient.xAdvance)
	}

	runGradientSIMD(gf.gradient, dst, &tBuf, nil)
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

	var tBuf [wideTileWidth][stripHeight]float32
	for x := range width {
		dist := posX.Mul(posX).Add(posY.Mul(posY)).Sqrt()
		t := scaleVec.MulAdd(dist, biasVec)
		t = applyExtendSIMD(t, gf.gradient.extend)
		t.Store(&tBuf[x])
		posX = posX.Add(xAdvXVec)
		posY = posY.Add(xAdvYVec)
	}

	runGradientSIMD(gf.gradient, dst, &tBuf, nil)
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

	// Pass 1: compute t values and defined masks.
	var tBuf [wideTileWidth][stripHeight]float32
	var maskBuf [wideTileWidth][stripHeight]int32
	for x := range width {
		inner := r0sq.Sub(posY.Mul(posY))
		// OPT(dh): should we convert the mask to bits, to save on storage? but
		// then we need to spend instructions on turning bits back into a mask.
		inner.GreaterEqual(zero).ToInt32x4().Store(&maskBuf[x])
		t := posX.Add(inner.Max(zero).Sqrt())
		t = applyExtendSIMD(t, gf.gradient.extend)
		t.Store(&tBuf[x])
		// OPT(dh): can we combine the mask and t? maybe store a negative t, and
		// then use that to mask in runGradientSIMD?
		posX = posX.Add(xAdvXVec)
		posY = posY.Add(xAdvYVec)
	}

	runGradientSIMD(gf.gradient, dst, &tBuf, &maskBuf)
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

	// Pass 1: compute t values and defined masks.
	var tBuf [wideTileWidth][stripHeight]float32
	var maskBuf [wideTileWidth][stripHeight]int32
	for x := range width {
		var t Float32x4
		if focalOnCircle {
			t = posX.Add(posY.Mul(posY).Div(posX))
		} else if wellBehaved {
			t = posX.Mul(posX).
				Add(posY.Mul(posY)).
				Sqrt().
				Sub(posX.Mul(fp0Vec))
		} else if swapped || negFocalX {
			inner := posX.Mul(posX).Sub(posY.Mul(posY))
			t = zero.Sub(inner.Sqrt()).Sub(posX.Mul(fp0Vec))
		} else {
			inner := posX.Mul(posX).Sub(posY.Mul(posY))
			t = inner.Sqrt().Sub(posX.Mul(fp0Vec))
		}

		if !wellBehaved {
			tGreaterZero := t.Greater(zero)
			tNotNaN := t.Equal(t)
			tGreaterZero.And(tNotNaN).ToInt32x4().Store(&maskBuf[x])
		}

		if negFocalX {
			t = zero.Sub(t)
		}
		if !nativelyFocal {
			t = t.Add(fp1Vec)
		}
		if swapped {
			t = one.Sub(t)
		}

		t = applyExtendSIMD(t, gf.gradient.extend)
		t.Store(&tBuf[x])
		posX = posX.Add(xAdvXVec)
		posY = posY.Add(xAdvYVec)
	}

	if wellBehaved {
		runGradientSIMD(gf.gradient, dst, &tBuf, nil)
	} else {
		runGradientSIMD(gf.gradient, dst, &tBuf, &maskBuf)
	}
}

// atan2SIMD computes atan2(y, x) for Float32x4 vectors using a
// 5th-order polynomial approximation with |error| < 1e-5.
func atan2SIMD(y, x Float32x4) Float32x4 {
	signBitMask := BroadcastInt32x4(0x7FFFFFFF)
	absX := x.AsInt32x4().And(signBitMask).AsFloat32x4()
	absY := y.AsInt32x4().And(signBitMask).AsFloat32x4()

	zero := BroadcastFloat32x4(0)
	piOver2 := BroadcastFloat32x4(math.Pi / 2)
	pi := BroadcastFloat32x4(math.Pi)

	swapMask := absY.Greater(absX)
	minV := absX.Min(absY)
	maxV := absX.Max(absY)

	r := minV.Div(maxV)
	r2 := r.Mul(r)

	c0 := BroadcastFloat32x4(0.99997726)
	c1 := BroadcastFloat32x4(-0.33262347)
	c2 := BroadcastFloat32x4(0.19354346)
	c3 := BroadcastFloat32x4(-0.11643287)
	c4 := BroadcastFloat32x4(0.05265332)
	c5 := BroadcastFloat32x4(-0.01172120)

	p := c5
	p = p.MulAdd(r2, c4)
	p = p.MulAdd(r2, c3)
	p = p.MulAdd(r2, c2)
	p = p.MulAdd(r2, c1)
	p = p.MulAdd(r2, c0)
	result := p.Mul(r)

	result = piOver2.Sub(result).Merge(result, swapMask)

	xNeg := x.Less(zero)
	result = pi.Sub(result).Merge(result, xNeg)

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

	// Pass 1: compute t values.
	var tBuf [wideTileWidth][stripHeight]float32
	for x := range width {
		angle := atan2SIMD(zero.Sub(posY), posX)

		nonNeg := angle.GreaterEqual(zero)
		adj := angle.Merge(angle.Add(twoPi), nonNeg)

		t := adj.Sub(startAngleVec).Mul(invAngleDeltaVec)
		t = applyExtendSIMD(t, gf.gradient.extend)
		t.Store(&tBuf[x])
		posX = posX.Add(xAdvXVec)
		posY = posY.Add(xAdvYVec)
	}

	runGradientSIMD(gf.gradient, dst, &tBuf, nil)
}

func runGradientSIMD(g *encodedGradient, dst Pixels, tBuf *[wideTileWidth][stripHeight]float32, masks *[wideTileWidth][stripHeight]int32) {
	width := len(dst[0])
	if len(g.ranges) <= maxCascadeMergeRanges {
		if masks != nil {
			gradientCascadeMergeMaskedAVX2(
				&dst[0][0],
				&dst[1][0],
				&dst[2][0],
				&dst[3][0],
				&tBuf[0],
				&g.simdRanges,
				&masks[0],
				width,
			)
		} else {
			gradientCascadeMergeAVX2(
				&dst[0][0],
				&dst[1][0],
				&dst[2][0],
				&dst[3][0],
				&tBuf[0],
				&g.simdRanges,
				width,
			)
		}
	} else {
		lut := &g.lut
		if masks != nil {
			gradientLUTGatherMaskedAVX2(
				&dst[0][0],
				&dst[1][0],
				&dst[2][0],
				&dst[3][0],
				(*[4]float32)(&lut.lut[0]),
				lut.scale,
				&tBuf[0],
				&masks[0],
				width,
			)
		} else {
			gradientLUTGatherAVX2(
				&dst[0][0],
				&dst[1][0],
				&dst[2][0],
				&dst[3][0],
				(*[4]float32)(&lut.lut[0]),
				lut.scale,
				&tBuf[0],
				width,
			)
		}
	}
}
