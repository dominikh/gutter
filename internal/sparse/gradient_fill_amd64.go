// SPDX-FileCopyrightText: 2026 Dominik Honnef and contributors
//
// SPDX-License-Identifier: MIT

//go:build goexperiment.simd

package sparse

import (
	"fmt"
	"math"
	. "simd/archsimd"
	"unsafe"

	"honnef.co/go/gutter/gfx"
	"honnef.co/go/gutter/internal/arch"
	"honnef.co/go/safeish"
)

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
		panic(fmt.Sprintf("internal error: unhandled type %T", kind))
	}
	ClearAVXUpperBits()
	return true
}

// applyExtendSIMD applies the gradient extend mode to a wide vector of t values.
func applyExtendSIMD(
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

// initPosVectors constructs initial posX and posY vectors for two
// columns at once, using YMM registers. The lower 4 elements hold
// column x (rows 0–3) and the upper 4 hold column x+1 (rows 0–3).
func initPosVectors(
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
	width := dst.width()
	var tBuf [wideTileWidth][stripHeight]float32

	startX := float32(gf.curPos.X)
	yAdvX := float32(gf.gradient.yAdvance.X)
	xAdvX := float32(gf.gradient.xAdvance.X)

	idx := LoadFloat32x8(&_01230123)
	var baseX Float32x8
	baseX = baseX.SetLo(BroadcastFloat32x4(startX))
	baseX = baseX.SetHi(BroadcastFloat32x4(startX + xAdvX))
	posX8 := BroadcastFloat32x8(yAdvX).MulAdd(idx, baseX)
	twoXAdvXVec := BroadcastFloat32x8(2 * xAdvX)

	// We check for x < width, not x < width - 1, because width is always <=
	// wideTileWidth, so if width % 2 != 0, width + 1 is still <= wideTileWidth
	// and storing one value too many is safe.
	for x := 0; x < width; x += 2 {
		t := applyExtendSIMD(posX8, gf.gradient.extend)
		t.Store((*[8]float32)(unsafe.Pointer(&tBuf[x])))
		posX8 = posX8.Add(twoXAdvXVec)
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

	posX8, posY8 := initPosVectors(startX, startY, yAdvX, yAdvY, xAdvX, xAdvY)
	twoXAdvXVec := BroadcastFloat32x8(2 * xAdvX)
	twoXAdvYVec := BroadcastFloat32x8(2 * xAdvY)

	biasVec8 := BroadcastFloat32x8(kind.bias)
	scaleVec8 := BroadcastFloat32x8(kind.scale)

	width := dst.width()

	var tBuf [wideTileWidth][stripHeight]float32
	// We check for x < width, not x < width - 1, because width is always <=
	// wideTileWidth, so if width % 2 != 0, width + 1 is still <= wideTileWidth
	// and storing one value too many is safe.
	for x := 0; x < width; x += 2 {
		dist := posX8.Mul(posX8).Add(posY8.Mul(posY8)).Sqrt()
		t := scaleVec8.MulAdd(dist, biasVec8)
		t = applyExtendSIMD(t, gf.gradient.extend)
		t.Store((*[8]float32)(unsafe.Pointer(&tBuf[x])))
		posX8 = posX8.Add(twoXAdvXVec)
		posY8 = posY8.Add(twoXAdvYVec)
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

	posX8, posY8 := initPosVectors(startX, startY, yAdvX, yAdvY, xAdvX, xAdvY)
	twoXAdvXVec := BroadcastFloat32x8(2 * xAdvX)
	twoXAdvYVec := BroadcastFloat32x8(2 * xAdvY)

	r0sq8 := BroadcastFloat32x8(kind.r0ScaledSquared)

	width := dst.width()

	// Pass 1: compute t values and defined masks.
	var tBuf [wideTileWidth][stripHeight]float32
	var maskBuf [wideTileWidth / 2]uint8
	// We check for x < width, not x < width - 1, because width is always <=
	// wideTileWidth, so if width % 2 != 0, width + 1 is still <= wideTileWidth
	// and storing one value too many is safe.
	for x := 0; x < width; x += 2 {
		inner := r0sq8.Sub(posY8.Mul(posY8))
		maskBuf[x/2] = inner.GreaterEqual(Float32x8{}).ToBits()
		t := posX8.Add(inner.Max(Float32x8{}).Sqrt())
		t = applyExtendSIMD(t, gf.gradient.extend)
		t.Store((*[8]float32)(unsafe.Pointer(&tBuf[x])))
		posX8 = posX8.Add(twoXAdvXVec)
		posY8 = posY8.Add(twoXAdvYVec)
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

	posX8, posY8 := initPosVectors(startX, startY, yAdvX, yAdvY, xAdvX, xAdvY)
	twoXAdvXVec := BroadcastFloat32x8(2 * xAdvX)
	twoXAdvYVec := BroadcastFloat32x8(2 * xAdvY)

	fp0Vec8 := BroadcastFloat32x8(kind.fp0)
	fp1Vec8 := BroadcastFloat32x8(kind.fp1)
	one8 := BroadcastFloat32x8(1)

	focalOnCircle := kind.focalData.focalOnCircle()
	wellBehaved := kind.focalData.wellBehaved()
	swapped := kind.focalData.swapped()
	negFocalX := 1.0-kind.focalData.fFocalX < 0.0
	nativelyFocal := kind.focalData.nativelyFocal()

	width := dst.width()

	// Pass 1: compute t values and defined masks.
	var tBuf [wideTileWidth][stripHeight]float32
	var maskBuf [wideTileWidth / 2]uint8
	anyMasked := false

	// OPT(dh): it'd be great to be able to pull all conditions out of the loop,
	// but there are 18 unique combinations.

	// We check for x < width, not x < width - 1, because width is always <=
	// wideTileWidth, so if width % 2 != 0, width + 1 is still <= wideTileWidth
	// and storing one value too many is safe.
	for x := 0; x < width; x += 2 {
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
			t = Float32x8{}.Sub(inner.Sqrt()).Sub(posX8.Mul(fp0Vec8))
		} else {
			inner := posX8.Mul(posX8).Sub(posY8.Mul(posY8))
			t = inner.Sqrt().Sub(posX8.Mul(fp0Vec8))
		}

		if !wellBehaved {
			tGreaterZero := t.Greater(Float32x8{})
			tNotNaN := t.Equal(t)
			m := tGreaterZero.And(tNotNaN).ToBits()
			anyMasked = anyMasked || m != 255
			maskBuf[x/2] = m
		}

		if negFocalX {
			t = Float32x8{}.Sub(t)
		}
		if !nativelyFocal {
			t = t.Add(fp1Vec8)
		}
		if swapped {
			t = one8.Sub(t)
		}

		t = applyExtendSIMD(t, gf.gradient.extend)
		t.Store((*[8]float32)(unsafe.Pointer(&tBuf[x])))
		posX8 = posX8.Add(twoXAdvXVec)
		posY8 = posY8.Add(twoXAdvYVec)
	}

	if anyMasked {
		runGradientSIMD(gf.gradient, dst, &tBuf, &maskBuf)
	} else {
		runGradientSIMD(gf.gradient, dst, &tBuf, nil)
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

	posX8, posY8 := initPosVectors(startX, startY, yAdvX, yAdvY, xAdvX, xAdvY)
	twoXAdvXVec := BroadcastFloat32x8(2 * xAdvX)
	twoXAdvYVec := BroadcastFloat32x8(2 * xAdvY)

	zero8 := Float32x8{}
	twoPi8 := BroadcastFloat32x8(float32(2 * math.Pi))
	startAngleVec8 := BroadcastFloat32x8(kind.startAngle)
	invAngleDeltaVec8 := BroadcastFloat32x8(kind.invAngleDelta)

	width := dst.width()

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
	// We check for x < width, not x < width - 1, because width is always <=
	// wideTileWidth, so if width % 2 != 0, width + 1 is still <= wideTileWidth
	// and storing one value too many is safe.
	for x := 0; x < width; x += 2 {
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
		t = applyExtendSIMD(t, gf.gradient.extend)
		t.Store((*[8]float32)(unsafe.Pointer(&tBuf[x])))
		posX8 = posX8.Add(twoXAdvXVec)
		posY8 = posY8.Add(twoXAdvYVec)
	}

	runGradientSIMD(gf.gradient, dst, &tBuf, nil)
}

func runGradientSIMD(g *encodedGradient, dst Pixels, tBuf *[wideTileWidth][stripHeight]float32, masks *[wideTileWidth / 2]uint8) {
	width := dst.width()
	if masks != nil {
		masksSlice := masks[:]
		for len(masksSlice) > 0 && masksSlice[len(masksSlice)-1] == 0 {
			masksSlice = masksSlice[:len(masksSlice)-1]
		}

		any := false
		// OPT(dh): we could check 8 bytes at a time, but then we have to handle
		// the remainder
		for _, b := range masksSlice {
			if b != 255 {
				any = true
				break
			}
		}
		if !any {
			masks = nil
		}

		width = min(width, len(masksSlice)*2)
	}
	if len(g.ranges) <= maxCascadeMergeRanges {
		gradientCascadeMergeAVX2(
			&dst.plane(0)[0],
			&dst.plane(1)[0],
			&dst.plane(2)[0],
			&dst.plane(3)[0],
			&tBuf[0],
			&g.simdRanges,
			width,
		)
	} else {
		lut := &g.lut
		gradientLUTGatherAVX2(
			&dst.plane(0)[0],
			&dst.plane(1)[0],
			&dst.plane(2)[0],
			&dst.plane(3)[0],
			(*[4]float32)(&lut.lut[0]),
			lut.scale,
			&tBuf[0],
			width,
		)
	}

	if masks != nil {
		// On average, it is faster to unconditionally draw the gradient, then
		// delete the bits that were masked out. Most columns are either not
		// masked at all or fully masked, which we can handle more efficiently
		// than by doing masked stores for all columns.

		bitIsolateMask := LoadInt32x8(&[8]int32{1, 2, 4, 8, 16, 32, 64, 128})
		for i := range width / 2 {
			// XXX make sure there is no bounds check here
			b := masks[i]
			switch b {
			case 0:
				for ch := range 4 {
					dst.plane(ch)[i*2] = [4]float32{}
					dst.plane(ch)[i*2+1] = [4]float32{}
				}
			case 255:
				// Nothing to do
			default:
				expandedMask := Int32x4{}.SetElem(0, int32(b)).Broadcast1To8().And(bitIsolateMask).NotEqual(bitIsolateMask)
				for ch := range 4 {
					Float32x8{}.StoreMasked(safeish.Cast[*[8]float32](&dst.plane(ch)[i*2]), expandedMask)
				}
			}
		}
		if width%2 != 0 {
			b := masks[width/2+1]
			b &= 0b1111
			switch b {
			case 0:
				dst.plane(0)[width-1] = [4]float32{}
				dst.plane(1)[width-1] = [4]float32{}
				dst.plane(2)[width-1] = [4]float32{}
				dst.plane(3)[width-1] = [4]float32{}
			case 15:
				// Nothing to do
			default:
				expandedMask := Int32x4{}.SetElem(0, int32(b)).Broadcast1To4().And(bitIsolateMask.GetLo()).NotEqual(bitIsolateMask.GetLo())
				for ch := range 4 {
					Float32x4{}.StoreMasked(&dst.plane(ch)[width-1], expandedMask)
				}
			}
		}
	}
}

func gradientCascadeMergeAVX2(
	dst0 *[stripHeight]float32,
	dst1 *[stripHeight]float32,
	dst2 *[stripHeight]float32,
	dst3 *[stripHeight]float32,
	tBuf *[stripHeight]float32,
	sr *simdGradientRanges,
	width int,
) {
	nMinus1 := uint(sr.n - 1)
	widthBytes := width << 4
	ymmThreshold := widthBytes - 16
	offset := 0

	scaleR := LoadFloat32x8(&sr.scaleR)
	scaleG := LoadFloat32x8(&sr.scaleG)
	scaleB := LoadFloat32x8(&sr.scaleB)
	scaleA := LoadFloat32x8(&sr.scaleA)

	biasR := LoadFloat32x8(&sr.biasR)
	biasG := LoadFloat32x8(&sr.biasG)
	biasB := LoadFloat32x8(&sr.biasB)
	biasA := LoadFloat32x8(&sr.biasA)

	one := BroadcastFloat32x8(1.0)

	for offset <= ymmThreshold {
		t := LoadFloat32x8((*[8]float32)(unsafe.Add(unsafe.Pointer(tBuf), offset)))

		idx := Float32x8{}

		threshIdx := uint(0)

		for threshIdx < nMinus1 {
			// OPT(dh): how do we eliminate the bounds check without using
			// unsafe? we can't do sr.x1[nMinus1-1] before the loop, because
			// nMinus1 might be zero. And we can't do it conditioned on nMinus1
			// being > 0 because the prover doesn't pick up on that.
			thresh := BroadcastFloat32x8(*safeish.Index(sr.x1[:], threshIdx))
			cmpResult := t.GreaterEqual(thresh)
			mask := cmpResult.And(Mask32x8(one.AsInt32x8())).ToInt32x8().AsFloat32x8()
			idx = idx.Add(mask)

			threshIdx++
		}

		idxi := idx.ConvertToInt32().AsUint32x8()

		{
			s := scaleR.Permute(idxi)
			b := biasR.Permute(idxi)
			s = s.MulAdd(t, b)
			s.Store((*[8]float32)(unsafe.Add(unsafe.Pointer(dst0), offset)))
		}
		{
			s := scaleG.Permute(idxi)
			b := biasG.Permute(idxi)
			s = s.MulAdd(t, b)
			s.Store((*[8]float32)(unsafe.Add(unsafe.Pointer(dst1), offset)))
		}
		{
			s := scaleB.Permute(idxi)
			b := biasB.Permute(idxi)
			s = s.MulAdd(t, b)
			s.Store((*[8]float32)(unsafe.Add(unsafe.Pointer(dst2), offset)))
		}
		{
			s := scaleA.Permute(idxi)
			b := biasA.Permute(idxi)
			s = s.MulAdd(t, b)
			s.Store((*[8]float32)(unsafe.Add(unsafe.Pointer(dst3), offset)))
		}

		offset += 32
	}
	if offset < widthBytes {
		var tTail Float32x8
		tTail.SetLo(
			LoadFloat32x4((*[4]float32)(unsafe.Add(unsafe.Pointer(tBuf), offset))),
		)

		idxTail := Float32x8{}

		threshIdxTail := uint(0)

		for threshIdxTail < nMinus1 {
			// OPT(dh): same bounds check issue as above
			threshTail := BroadcastFloat32x8(*safeish.Index(sr.x1[:], threshIdxTail))
			cmpTail := tTail.GreaterEqual(threshTail)
			maskedTail := cmpTail.And(Mask32x8(one.AsInt32x8())).ToInt32x8().AsFloat32x8()
			idxTail = idxTail.Add(maskedTail)

			threshIdxTail++
		}

		idxTaili := idxTail.ConvertToInt32().AsUint32x8()

		{
			s := scaleR.Permute(idxTaili)
			b := biasR.Permute(idxTaili)
			s = s.MulAdd(tTail, b)
			s.GetLo().Store((*[4]float32)(unsafe.Add(unsafe.Pointer(dst0), offset)))
		}
		{
			s := scaleG.Permute(idxTaili)
			b := biasG.Permute(idxTaili)
			s = s.MulAdd(tTail, b)
			s.GetLo().Store((*[4]float32)(unsafe.Add(unsafe.Pointer(dst1), offset)))
		}
		{
			s := scaleB.Permute(idxTaili)
			b := biasB.Permute(idxTaili)
			s = s.MulAdd(tTail, b)
			s.GetLo().Store((*[4]float32)(unsafe.Add(unsafe.Pointer(dst2), offset)))
		}
		{
			s := scaleA.Permute(idxTaili)
			b := biasA.Permute(idxTaili)
			s = s.MulAdd(tTail, b)
			s.GetLo().Store((*[4]float32)(unsafe.Add(unsafe.Pointer(dst3), offset)))
		}
	}

	ClearAVXUpperBits()
}
