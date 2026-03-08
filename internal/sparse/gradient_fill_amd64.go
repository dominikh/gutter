// SPDX-FileCopyrightText: 2026 Dominik Honnef and contributors
//
// SPDX-License-Identifier: MIT

//go:build goexperiment.simd

package sparse

import (
	"math"
	. "simd/archsimd"
	"unsafe"

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
	zero := Float32x4{}
	// OPT(dh): this function gets called once per column, we really don't want
	// to broadcast this over and over.
	one := BroadcastFloat32x4(1)
	// OPT(dh): the extend is constant for all iterations of a gradient, it'd be
	// nice to avoid this branch.
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

// applyExtendSIMD8 applies the gradient extend mode to a wide vector of t values.
func applyExtendSIMD8(
	t Float32x8,
	extend gfx.GradientExtend,
) Float32x8 {
	zero := Float32x8{}
	one := BroadcastFloat32x8(1)
	switch extend {
	case gfx.GradientExtendPad:
		return t.Max(zero).Min(one)
	case gfx.GradientExtendRepeat:
		result := t.Sub(t.Floor())
		return result.Sub(result.Floor())
	case gfx.GradientExtendReflect:
		half := BroadcastFloat32x8(0.5)
		two := BroadcastFloat32x8(2)
		tMinus1 := t.Sub(one)
		floored := tMinus1.Mul(half).Floor()
		reflected := tMinus1.Sub(two.Mul(floored)).Sub(one)
		signMask := BroadcastInt32x8(0x7FFFFFFF)
		absReflected := reflected.AsInt32x8().
			And(signMask).
			AsFloat32x8()
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

// initPosVectors8 constructs initial posX and posY vectors for two
// columns at once, using YMM registers. The lower 4 elements hold
// column x (rows 0–3) and the upper 4 hold column x+1 (rows 0–3).
func initPosVectors8(
	startX, startY, yAdvX, yAdvY, xAdvX, xAdvY float32,
) (posX, posY Float32x8) {
	idx := LoadFloat32x8(&_01230123)
	var baseX Float32x8
	baseX = baseX.SetLo(BroadcastFloat32x4(startX))
	baseX = baseX.SetHi(BroadcastFloat32x4(startX + xAdvX))
	posX = BroadcastFloat32x8(yAdvX).MulAdd(idx, baseX)
	var baseY Float32x8
	baseY = baseY.SetLo(BroadcastFloat32x4(startY))
	baseY = baseY.SetHi(BroadcastFloat32x4(startY + xAdvY))
	posY = BroadcastFloat32x8(yAdvY).MulAdd(idx, baseY)
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

	curPos := gf.curPos.X
	yAdvX := float32(gf.gradient.yAdvance.X)
	xAdv := gf.gradient.xAdvance.X

	idx := LoadFloat32x8(&_01230123)
	yAdvXVec := BroadcastFloat32x8(yAdvX)

	for x := 0; x < width-1; x += 2 {
		var baseX Float32x8
		baseX = baseX.SetLo(BroadcastFloat32x4(float32(curPos)))
		baseX = baseX.SetHi(BroadcastFloat32x4(float32(curPos + xAdv)))
		posX8 := yAdvXVec.MulAdd(idx, baseX)
		t := applyExtendSIMD8(posX8, gf.gradient.extend)
		t.Store((*[8]float32)(unsafe.Pointer(&tBuf[x])))
		curPos += 2 * xAdv
	}

	if width%2 != 0 {
		idx4 := LoadFloat32x4((*[4]float32)(_01230123[:]))
		posX := BroadcastFloat32x4(yAdvX).MulAdd(idx4, BroadcastFloat32x4(float32(curPos)))
		t := applyExtendSIMD(posX, gf.gradient.extend)
		t.Store(&tBuf[width-1])
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

	posX8, posY8 := initPosVectors8(startX, startY, yAdvX, yAdvY, xAdvX, xAdvY)
	twoXAdvXVec := BroadcastFloat32x8(2 * xAdvX)
	twoXAdvYVec := BroadcastFloat32x8(2 * xAdvY)

	biasVec8 := BroadcastFloat32x8(kind.bias)
	scaleVec8 := BroadcastFloat32x8(kind.scale)

	width := len(dst[0])

	var tBuf [wideTileWidth][stripHeight]float32
	for x := 0; x < width-1; x += 2 {
		dist := posX8.Mul(posX8).Add(posY8.Mul(posY8)).Sqrt()
		t := scaleVec8.MulAdd(dist, biasVec8)
		t = applyExtendSIMD8(t, gf.gradient.extend)
		t.Store((*[8]float32)(unsafe.Pointer(&tBuf[x])))
		posX8 = posX8.Add(twoXAdvXVec)
		posY8 = posY8.Add(twoXAdvYVec)
	}

	if width%2 != 0 {
		posX := posX8.GetLo()
		posY := posY8.GetLo()
		biasVec := BroadcastFloat32x4(kind.bias)
		scaleVec := BroadcastFloat32x4(kind.scale)
		dist := posX.Mul(posX).Add(posY.Mul(posY)).Sqrt()
		t := scaleVec.MulAdd(dist, biasVec)
		t = applyExtendSIMD(t, gf.gradient.extend)
		t.Store(&tBuf[width-1])
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

	posX8, posY8 := initPosVectors8(startX, startY, yAdvX, yAdvY, xAdvX, xAdvY)
	twoXAdvXVec := BroadcastFloat32x8(2 * xAdvX)
	twoXAdvYVec := BroadcastFloat32x8(2 * xAdvY)

	r0sq8 := BroadcastFloat32x8(kind.r0ScaledSquared)
	zero8 := BroadcastFloat32x8(0)

	width := len(dst[0])

	// Pass 1: compute t values and defined masks.
	var tBuf [wideTileWidth][stripHeight]float32
	var maskBuf [wideTileWidth][stripHeight]int32
	for x := 0; x < width-1; x += 2 {
		inner := r0sq8.Sub(posY8.Mul(posY8))
		inner.GreaterEqual(zero8).ToInt32x8().Store((*[8]int32)(unsafe.Pointer(&maskBuf[x])))
		t := posX8.Add(inner.Max(zero8).Sqrt())
		t = applyExtendSIMD8(t, gf.gradient.extend)
		t.Store((*[8]float32)(unsafe.Pointer(&tBuf[x])))
		posX8 = posX8.Add(twoXAdvXVec)
		posY8 = posY8.Add(twoXAdvYVec)
	}

	if width%2 != 0 {
		posX := posX8.GetLo()
		posY := posY8.GetLo()
		r0sq := BroadcastFloat32x4(kind.r0ScaledSquared)
		zero := BroadcastFloat32x4(0)
		inner := r0sq.Sub(posY.Mul(posY))
		inner.GreaterEqual(zero).ToInt32x4().Store(&maskBuf[width-1])
		t := posX.Add(inner.Max(zero).Sqrt())
		t = applyExtendSIMD(t, gf.gradient.extend)
		t.Store(&tBuf[width-1])
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

	posX8, posY8 := initPosVectors8(startX, startY, yAdvX, yAdvY, xAdvX, xAdvY)
	twoXAdvXVec := BroadcastFloat32x8(2 * xAdvX)
	twoXAdvYVec := BroadcastFloat32x8(2 * xAdvY)

	fp0Vec8 := BroadcastFloat32x8(kind.fp0)
	fp1Vec8 := BroadcastFloat32x8(kind.fp1)
	zero8 := BroadcastFloat32x8(0)
	one8 := BroadcastFloat32x8(1)

	focalOnCircle := kind.focalData.focalOnCircle()
	wellBehaved := kind.focalData.wellBehaved()
	swapped := kind.focalData.swapped()
	negFocalX := 1.0-kind.focalData.fFocalX < 0.0
	nativelyFocal := kind.focalData.nativelyFocal()

	width := len(dst[0])

	// Pass 1: compute t values and defined masks.
	var tBuf [wideTileWidth][stripHeight]float32
	var maskBuf [wideTileWidth][stripHeight]int32

	// OPT(dh): it'd be great to be able to pull all conditions out of the loop,
	// but there are 18 unique combinations.
	for x := 0; x < width-1; x += 2 {
		var t Float32x8
		if focalOnCircle {
			t = posX8.Add(posY8.Mul(posY8).Div(posX8))
		} else if wellBehaved {
			t = posX8.Mul(posX8).
				Add(posY8.Mul(posY8)).
				Sqrt().
				Sub(posX8.Mul(fp0Vec8))
		} else if swapped || negFocalX {
			inner := posX8.Mul(posX8).Sub(posY8.Mul(posY8))
			t = zero8.Sub(inner.Sqrt()).Sub(posX8.Mul(fp0Vec8))
		} else {
			inner := posX8.Mul(posX8).Sub(posY8.Mul(posY8))
			t = inner.Sqrt().Sub(posX8.Mul(fp0Vec8))
		}

		if !wellBehaved {
			tGreaterZero := t.Greater(zero8)
			tNotNaN := t.Equal(t)
			tGreaterZero.And(tNotNaN).ToInt32x8().Store((*[8]int32)(unsafe.Pointer(&maskBuf[x])))
		}

		if negFocalX {
			t = zero8.Sub(t)
		}
		if !nativelyFocal {
			t = t.Add(fp1Vec8)
		}
		if swapped {
			t = one8.Sub(t)
		}

		t = applyExtendSIMD8(t, gf.gradient.extend)
		t.Store((*[8]float32)(unsafe.Pointer(&tBuf[x])))
		posX8 = posX8.Add(twoXAdvXVec)
		posY8 = posY8.Add(twoXAdvYVec)
	}

	if width%2 != 0 {
		posX := posX8.GetLo()
		posY := posY8.GetLo()
		fp0Vec := BroadcastFloat32x4(kind.fp0)
		fp1Vec := BroadcastFloat32x4(kind.fp1)
		zero := BroadcastFloat32x4(0)
		one := BroadcastFloat32x4(1)

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
			tGreaterZero.And(tNotNaN).ToInt32x4().Store(&maskBuf[width-1])
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
		t.Store(&tBuf[width-1])
	}

	if wellBehaved {
		runGradientSIMD(gf.gradient, dst, &tBuf, nil)
	} else {
		runGradientSIMD(gf.gradient, dst, &tBuf, &maskBuf)
	}
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

	posX8, posY8 := initPosVectors8(startX, startY, yAdvX, yAdvY, xAdvX, xAdvY)
	twoXAdvXVec := BroadcastFloat32x8(2 * xAdvX)
	twoXAdvYVec := BroadcastFloat32x8(2 * xAdvY)

	zero8 := Float32x8{}
	twoPi8 := BroadcastFloat32x8(float32(2 * math.Pi))
	startAngleVec8 := BroadcastFloat32x8(kind.startAngle)
	invAngleDeltaVec8 := BroadcastFloat32x8(kind.invAngleDelta)

	width := len(dst[0])

	signBitMask8 := BroadcastInt32x8(0x7FFFFFFF)
	piOver2_8 := BroadcastFloat32x8(math.Pi / 2)
	pi8 := BroadcastFloat32x8(math.Pi)
	minInt32_8 := BroadcastInt32x8(math.MinInt32)

	c0_8 := BroadcastFloat32x8(0.99997726)
	c1_8 := BroadcastFloat32x8(-0.33262347)
	c2_8 := BroadcastFloat32x8(0.19354346)
	c3_8 := BroadcastFloat32x8(-0.11643287)
	c4_8 := BroadcastFloat32x8(0.05265332)
	c5_8 := BroadcastFloat32x8(-0.01172120)

	// Pass 1: compute t values.
	var tBuf [wideTileWidth][stripHeight]float32
	for x := 0; x < width-1; x += 2 {
		atanY, atanX := zero8.Sub(posY8), posX8

		absX := atanX.AsInt32x8().And(signBitMask8).AsFloat32x8()
		absY := atanY.AsInt32x8().And(signBitMask8).AsFloat32x8()

		swapMask := absY.Greater(absX)
		minV := absX.Min(absY)
		maxV := absX.Max(absY)

		r := minV.Div(maxV)
		r2 := r.Mul(r)

		p := c5_8
		p = p.MulAdd(r2, c4_8)
		p = p.MulAdd(r2, c3_8)
		p = p.MulAdd(r2, c2_8)
		p = p.MulAdd(r2, c1_8)
		p = p.MulAdd(r2, c0_8)
		angle := p.Mul(r)

		angle = piOver2_8.Sub(angle).Merge(angle, swapMask)

		xNeg := atanX.Less(zero8)
		angle = pi8.Sub(angle).Merge(angle, xNeg)

		ySignBit := atanY.AsInt32x8().And(minInt32_8)
		angle = angle.AsInt32x8().Xor(ySignBit).AsFloat32x8()

		nonNeg := angle.GreaterEqual(zero8)
		adj := angle.Merge(angle.Add(twoPi8), nonNeg)

		t := adj.Sub(startAngleVec8).Mul(invAngleDeltaVec8)
		t = applyExtendSIMD8(t, gf.gradient.extend)
		t.Store((*[8]float32)(unsafe.Pointer(&tBuf[x])))
		posX8 = posX8.Add(twoXAdvXVec)
		posY8 = posY8.Add(twoXAdvYVec)
	}

	if width%2 != 0 {
		posX := posX8.GetLo()
		posY := posY8.GetLo()

		zero := Float32x4{}
		twoPi := BroadcastFloat32x4(float32(2 * math.Pi))
		startAngleVec := BroadcastFloat32x4(kind.startAngle)
		invAngleDeltaVec := BroadcastFloat32x4(kind.invAngleDelta)

		signBitMask := BroadcastInt32x4(0x7FFFFFFF)
		piOver2 := BroadcastFloat32x4(math.Pi / 2)
		pi := BroadcastFloat32x4(math.Pi)

		c0 := BroadcastFloat32x4(0.99997726)
		c1 := BroadcastFloat32x4(-0.33262347)
		c2 := BroadcastFloat32x4(0.19354346)
		c3 := BroadcastFloat32x4(-0.11643287)
		c4 := BroadcastFloat32x4(0.05265332)
		c5 := BroadcastFloat32x4(-0.01172120)

		atanY, atanX := zero.Sub(posY), posX

		absX := atanX.AsInt32x4().And(signBitMask).AsFloat32x4()
		absY := atanY.AsInt32x4().And(signBitMask).AsFloat32x4()

		swapMask := absY.Greater(absX)
		minV := absX.Min(absY)
		maxV := absX.Max(absY)

		r := minV.Div(maxV)
		r2 := r.Mul(r)

		p := c5
		p = p.MulAdd(r2, c4)
		p = p.MulAdd(r2, c3)
		p = p.MulAdd(r2, c2)
		p = p.MulAdd(r2, c1)
		p = p.MulAdd(r2, c0)
		angle := p.Mul(r)

		angle = piOver2.Sub(angle).Merge(angle, swapMask)

		xNeg := atanX.Less(zero)
		angle = pi.Sub(angle).Merge(angle, xNeg)

		ySignBit := atanY.AsInt32x4().And(BroadcastInt32x4(math.MinInt32))
		angle = angle.AsInt32x4().Xor(ySignBit).AsFloat32x4()

		nonNeg := angle.GreaterEqual(zero)
		adj := angle.Merge(angle.Add(twoPi), nonNeg)

		t := adj.Sub(startAngleVec).Mul(invAngleDeltaVec)
		t = applyExtendSIMD(t, gf.gradient.extend)
		t.Store(&tBuf[width-1])
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
