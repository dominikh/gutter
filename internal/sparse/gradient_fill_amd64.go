// SPDX-FileCopyrightText: 2026 Dominik Honnef and contributors
//
// SPDX-License-Identifier: MIT

//go:build goexperiment.simd

package sparse

import (
	"math"
	. "simd/archsimd"
	"unsafe"

	"honnef.co/go/curve"
	"honnef.co/go/gutter/gfx"
	"honnef.co/go/gutter/internal/arch"
)

const maxSIMDRanges = 16

func (gf *gradientFiller) fillSIMD(dst Pixels) bool {
	if !arch.AVX2() || !arch.FMA() {
		return false
	}

	switch kind := gf.gradient.kind.(type) {
	case encodedLinearGradient:
		gf.fillLinearSIMD(dst, kind)
	case encodedRadialGradient:
		if len(gf.gradient.ranges) > maxSIMDRanges {
			return false
		}
		gf.fillRadialSIMD(dst, kind)
	case encodedStripGradient:
		if len(gf.gradient.ranges) > maxSIMDRanges {
			return false
		}
		gf.fillStripSIMD(dst, kind)
	case encodedFocalGradient:
		if len(gf.gradient.ranges) > maxSIMDRanges {
			return false
		}
		gf.fillFocalSIMD(dst, kind)
	case encodedSweepGradient:
		if len(gf.gradient.ranges) > maxSIMDRanges {
			return false
		}
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

// simdGradientRanges is a SoA (Structure of Arrays) representation of
// gradient ranges, optimized for VPERMPS-based lookup. Instead of
// iterating all ranges with cascade merge, we find the range index via
// threshold scan, then use VPERMPS to gather the right scale/bias per pixel.
type simdGradientRanges struct {
	n      int
	x1     [maxSIMDRanges]float32
	scaleR [maxSIMDRanges]float32
	scaleG [maxSIMDRanges]float32
	scaleB [maxSIMDRanges]float32
	scaleA [maxSIMDRanges]float32
	biasR  [maxSIMDRanges]float32
	biasG  [maxSIMDRanges]float32
	biasB  [maxSIMDRanges]float32
	biasA  [maxSIMDRanges]float32
}

func initSIMDRanges(sr *simdGradientRanges, ranges []gradientRange) {
	sr.n = len(ranges)
	for i, rng := range ranges {
		sr.x1[i] = rng.x1
		sr.scaleR[i] = rng.scale[0]
		sr.scaleG[i] = rng.scale[1]
		sr.scaleB[i] = rng.scale[2]
		sr.scaleA[i] = rng.scale[3]
		sr.biasR[i] = rng.bias[0]
		sr.biasG[i] = rng.bias[1]
		sr.biasB[i] = rng.bias[2]
		sr.biasA[i] = rng.bias[3]
	}
}

// cascadeMergeAll dispatches to the Avo VPERMPS implementation for n <= 8,
// falling back to the Go cascade merge for larger n.
func cascadeMergeAll(
	dst Pixels,
	tBuf *[wideTileWidth]Float32x4,
	sr *simdGradientRanges,
	width int,
) {
	if sr.n <= 8 {
		gradientCascadeMergeAVX2(
			&dst[0][0],
			&dst[1][0],
			&dst[2][0],
			&dst[3][0],
			(*[stripHeight]float32)(unsafe.Pointer(&tBuf[0])),
			uintptr(unsafe.Pointer(sr)),
			width,
		)
	} else {
		cascadeMergeChannel(dst[0], tBuf, &sr.scaleR, &sr.biasR, &sr.x1, width, sr.n)
		cascadeMergeChannel(dst[1], tBuf, &sr.scaleG, &sr.biasG, &sr.x1, width, sr.n)
		cascadeMergeChannel(dst[2], tBuf, &sr.scaleB, &sr.biasB, &sr.x1, width, sr.n)
		cascadeMergeChannel(dst[3], tBuf, &sr.scaleA, &sr.biasA, &sr.x1, width, sr.n)
	}
}

// cascadeMergeChannel performs cascade merge for a single color channel,
// writing results directly to dst. Processing one channel at a time
// keeps register pressure low (avoids the Go compiler's legacy SSE
// spill issue with 8+ live YMM registers).
func cascadeMergeChannel(
	dst [][stripHeight]float32,
	tBuf *[wideTileWidth]Float32x4,
	scale *[maxSIMDRanges]float32,
	bias *[maxSIMDRanges]float32,
	x1 *[maxSIMDRanges]float32,
	width int,
	n int,
) {
	// Pre-broadcast all range constants to avoid repeated broadcasts
	// in the inner loop (saves n*3 broadcasts per column).
	var scaleVec, biasVec, threshVec [maxSIMDRanges]Float32x4
	for i := range n {
		scaleVec[i] = BroadcastFloat32x4(scale[i])
		biasVec[i] = BroadcastFloat32x4(bias[i])
	}
	for i := range n - 1 {
		threshVec[i] = BroadcastFloat32x4(x1[i])
	}

	for x := range width {
		t := tBuf[x]
		result := scaleVec[0].MulAdd(t, biasVec[0])
		for i := 1; i < n; i++ {
			candidate := scaleVec[i].MulAdd(t, biasVec[i])
			mask := t.GreaterEqual(threshVec[i-1])
			result = candidate.Merge(result, mask)
		}
		result.Store((*[4]float32)(&dst[x]))
	}
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

// columnPosVectors builds position vectors for a column by computing each
// row's position in float64 (matching scalar precision) and converting to
// float32 for SIMD use.
func columnPosVectors(pos curve.Point, yAdv curve.Vec2) (posX, posY Float32x4) {
	var arrX, arrY [4]float32
	p := pos
	for i := range 4 {
		arrX[i] = float32(p.X)
		arrY[i] = float32(p.Y)
		p = p.Translate(yAdv)
	}
	return LoadFloat32x4(&arrX), LoadFloat32x4(&arrY)
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
		posX, _ := columnPosVectors(curPos, gf.gradient.yAdvance)
		t := applyExtendSIMD(posX, gf.gradient.extend)
		t.Store(&tBuf[x])
		curPos = curPos.Translate(gf.gradient.xAdvance)
	}

	lut := &gf.gradient.lut
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

	var sr simdGradientRanges
	initSIMDRanges(&sr, gf.gradient.ranges)
	width := len(dst[0])

	// Pass 1: compute t values.
	var tBuf [wideTileWidth]Float32x4
	for x := range width {
		dist := posX.Mul(posX).Add(posY.Mul(posY)).Sqrt()
		t := scaleVec.MulAdd(dist, biasVec)
		t = applyExtendSIMD(t, gf.gradient.extend)
		tBuf[x] = t
		posX = posX.Add(xAdvXVec)
		posY = posY.Add(xAdvYVec)
	}

	// Pass 2: cascade merge (Avo VPERMPS for n<=8, Go fallback otherwise).
	cascadeMergeAll(dst, &tBuf, &sr, width)
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

	var sr simdGradientRanges
	initSIMDRanges(&sr, gf.gradient.ranges)
	width := len(dst[0])

	// Pass 1: compute t values and defined masks.
	var tBuf [wideTileWidth]Float32x4
	var maskBuf [wideTileWidth]Mask32x4
	for x := range width {
		inner := r0sq.Sub(posY.Mul(posY))
		maskBuf[x] = inner.GreaterEqual(zero)
		t := posX.Add(inner.Max(zero).Sqrt())
		t = applyExtendSIMD(t, gf.gradient.extend)
		tBuf[x] = t
		posX = posX.Add(xAdvXVec)
		posY = posY.Add(xAdvYVec)
	}

	// Pass 2: cascade merge (Avo VPERMPS for n<=8, Go fallback otherwise).
	cascadeMergeAll(dst, &tBuf, &sr, width)

	// Apply defined mask.
	for x := range width {
		m := maskBuf[x]
		LoadFloat32x4(&dst[0][x]).Masked(m).Store(&dst[0][x])
		LoadFloat32x4(&dst[1][x]).Masked(m).Store(&dst[1][x])
		LoadFloat32x4(&dst[2][x]).Masked(m).Store(&dst[2][x])
		LoadFloat32x4(&dst[3][x]).Masked(m).Store(&dst[3][x])
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

	var sr simdGradientRanges
	initSIMDRanges(&sr, gf.gradient.ranges)
	width := len(dst[0])

	// Pass 1: compute t values and defined masks.
	var tBuf [wideTileWidth]Float32x4
	var maskBuf [wideTileWidth]Mask32x4
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
			inner := posX.Mul(posX).Sub(posY.Mul(posY)).Max(zero)
			t = zero.Sub(inner.Sqrt()).Sub(posX.Mul(fp0Vec))
		} else {
			inner := posX.Mul(posX).Sub(posY.Mul(posY)).Max(zero)
			t = inner.Sqrt().Sub(posX.Mul(fp0Vec))
		}

		if !wellBehaved {
			tGreaterZero := t.Greater(zero)
			tNotNaN := t.Equal(t)
			maskBuf[x] = tGreaterZero.And(tNotNaN)
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
		tBuf[x] = t
		posX = posX.Add(xAdvXVec)
		posY = posY.Add(xAdvYVec)
	}

	// Pass 2: cascade merge (Avo VPERMPS for n<=8, Go fallback otherwise).
	cascadeMergeAll(dst, &tBuf, &sr, width)

	// Apply defined mask if needed.
	if !wellBehaved {
		for x := range width {
			m := maskBuf[x]
			LoadFloat32x4(&dst[0][x]).Masked(m).Store(&dst[0][x])
			LoadFloat32x4(&dst[1][x]).Masked(m).Store(&dst[1][x])
			LoadFloat32x4(&dst[2][x]).Masked(m).Store(&dst[2][x])
			LoadFloat32x4(&dst[3][x]).Masked(m).Store(&dst[3][x])
		}
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

	swapMask := absY.Greater(absX)
	minV := absX.Min(absY)
	maxV := absX.Max(absY)

	r := minV.Div(maxV)
	r2 := r.Mul(r)

	c0 := BroadcastFloat32x4(0.99997726)
	c1 := BroadcastFloat32x4(-0.33262347)
	c2 := BroadcastFloat32x4(0.19354346)
	c3 := BroadcastFloat32x4(-0.11643287)

	p := c3.MulAdd(r2, c2)
	p = p.MulAdd(r2, c1)
	p = p.MulAdd(r2, c0)
	result := r.Mul(p)

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

	var sr simdGradientRanges
	initSIMDRanges(&sr, gf.gradient.ranges)
	width := len(dst[0])

	// Pass 1: compute t values.
	var tBuf [wideTileWidth]Float32x4
	for x := range width {
		negPosY := zero.Sub(posY)
		angle := atan2SIMD(negPosY, posX)

		nonNeg := angle.GreaterEqual(zero)
		adj := angle.Merge(angle.Add(twoPi), nonNeg)

		t := adj.Sub(startAngleVec).Mul(invAngleDeltaVec)
		t = applyExtendSIMD(t, gf.gradient.extend)
		tBuf[x] = t
		posX = posX.Add(xAdvXVec)
		posY = posY.Add(xAdvYVec)
	}

	// Pass 2: cascade merge (Avo VPERMPS for n<=8, Go fallback otherwise).
	cascadeMergeAll(dst, &tBuf, &sr, width)
}
