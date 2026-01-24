// SPDX-FileCopyrightText: 2025 Dominik Honnef and contributors
//
// SPDX-License-Identifier: MIT

package sparse

import (
	"math"
	. "simd/archsimd"
)

var memPxLeftX = [2][8]float32{
	{0, 0, 0, 0, 1, 1, 1, 1},
	{2, 2, 2, 2, 3, 3, 3, 3},
}

var memPxRightX = [2][8]float32{
	{1, 1, 1, 1, 2, 2, 2, 2},
	{3, 3, 3, 3, 4, 4, 4, 4},
}

var memArea = [2][8]float32{
	{2, 2, 2, 2, 4, 4, 4, 4},
	{6, 6, 6, 6, 8, 8, 8, 8},
}

var _01230123 = [8]float32{0, 1, 2, 3, 0, 1, 2, 3}
var _12341234 = [8]float32{1, 2, 3, 4, 1, 2, 3, 4}

func computeWindingAVX(
	lineTopY float32,
	lineTopX float32,
	lineBottomY float32,
	sign float32,
	xSlope float32,
	ySlope float32,
	locationWinding *[tileWidth][tileHeight]float32,
	accumulatedWinding *[tileHeight]float32,
) {
	lineTopY_8 := BroadcastFloat32x8(lineTopY)
	lineBottomY_8 := BroadcastFloat32x8(lineBottomY)
	lineTopX_8 := BroadcastFloat32x8(lineTopX)

	pxTopY_8 := LoadFloat32x8(&_01230123)
	pxBottomY_8 := LoadFloat32x8(&_12341234)

	ymin := pxTopY_8.Max(lineTopY_8)
	ymax := pxBottomY_8.Min(lineBottomY_8)

	xSlope_8 := BroadcastFloat32x8(xSlope)
	ySlope_8 := BroadcastFloat32x8(ySlope)
	sign_8 := BroadcastFloat32x8(sign)

	mask := BroadcastFloat32x8(math.Float32frombits(1 << 31))
	oneHalf := BroadcastFloat32x8(0.5)

	// FIXME(dh): this convoluted way of initializing acc to zero works around a
	// shortcoming in the compiler where it uses MOV instead of VMOV to copy an
	// XMM register while surrounded by AVX instructions, killing performance.
	//
	// var acc Float32x4
	acc := BroadcastFloat32x4(1)
	acc = acc.Sub(acc)

	fn := func(it int, muladd func(a, b, c Float32x8) Float32x8) {
		xIdx := 2 * it
		xIdx2 := 2*it + 1

		linePxLeftY := LoadFloat32x8(&memPxLeftX[it])
		linePxLeftY = linePxLeftY.Sub(lineTopX_8)
		linePxLeftY = muladd(linePxLeftY, ySlope_8, lineTopY_8)
		linePxLeftY = linePxLeftY.Max(ymin).Min(ymax)

		linePxRightY := LoadFloat32x8(&memPxRightX[it])
		linePxRightY = linePxRightY.Sub(lineTopX_8)
		linePxRightY = muladd(linePxRightY, ySlope_8, lineTopY_8)
		linePxRightY = linePxRightY.Max(ymin).Min(ymax)

		linePxLeftYX := linePxLeftY.Sub(lineTopY_8)
		linePxLeftYX = muladd(linePxLeftYX, xSlope_8, lineTopX_8)

		linePxRightYX := linePxRightY.Sub(lineTopY_8)
		linePxRightYX = muladd(linePxRightYX, xSlope_8, lineTopX_8)

		h_8 := linePxRightY.Sub(linePxLeftY)
		h_8 = h_8.AsInt32x8().AndNot(mask.AsInt32x8()).AsFloat32x8()

		area_8 := LoadFloat32x8(&memArea[it])
		area_8 = area_8.Sub(linePxRightYX).Sub(linePxLeftYX).Mul(h_8).Mul(oneHalf)

		signarea_8 := area_8.Mul(sign_8)
		signh_8 := h_8.Mul(sign_8)

		d := &locationWinding[xIdx]
		tmp := acc.Add(LoadFloat32x4(d))
		signarea_4_1 := signarea_8.GetLo()
		tmp = tmp.Add(signarea_4_1)
		tmp.Store(d)

		signh_4_1 := signh_8.GetLo()
		acc = acc.Add(signh_4_1)

		d = &locationWinding[xIdx2]
		tmp = acc.Add(LoadFloat32x4(d))
		signarea_4_2 := signarea_8.GetHi()
		tmp = tmp.Add(signarea_4_2)
		tmp.Store(d)

		signh_4_2 := signh_8.GetHi()
		acc = acc.Add(signh_4_2)
	}

	// Both fn and muladd get inlined.
	if X86.FMA() {
		muladd := func(a, b, c Float32x8) Float32x8 {
			return a.MulAdd(b, c)
		}
		fn(0, muladd)
		fn(1, muladd)
	} else {
		muladd := func(a, b, c Float32x8) Float32x8 {
			return a.Mul(b).Add(c)
		}
		fn(0, muladd)
		fn(1, muladd)
	}

	tmp := acc.Add(LoadFloat32x4(accumulatedWinding))
	tmp.Store(accumulatedWinding)

	ClearAVXUpperBits()
}
