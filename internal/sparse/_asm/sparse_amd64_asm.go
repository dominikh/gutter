// SPDX-FileCopyrightText: 2025 Dominik Honnef and contributors
//
// SPDX-License-Identifier: MIT

package main

import (
	"fmt"
	"math"

	"github.com/mmcloughlin/avo/attr"
	. "github.com/mmcloughlin/avo/build"
	"github.com/mmcloughlin/avo/gotypes"
	"github.com/mmcloughlin/avo/ir"
	. "github.com/mmcloughlin/avo/operand"
	"github.com/mmcloughlin/avo/reg"
)

var floatConsts = map[float32]Mem{}
var floatConsts4 = map[float32]Mem{}
var floatConsts4_2 = map[[2]float32]Mem{}
var floatConsts8 = map[float32]Mem{}

func floatConst(f float32) Mem {
	if k, ok := floatConsts[f]; ok {
		return k
	}
	k := ConstData(fmt.Sprintf("f_%08x", math.Float32bits(f)), F32(f))
	floatConsts[f] = k
	return k
}

func floatConst4(f float32) Mem {
	if k, ok := floatConsts4[f]; ok {
		return k
	}
	k := GLOBL(fmt.Sprintf("f4_%08x", math.Float32bits(f)), attr.RODATA|attr.NOPTR)
	DATA(0, F32(f))
	DATA(4, F32(f))
	DATA(8, F32(f))
	DATA(12, F32(f))
	floatConsts4[f] = k
	return k
}

func floatConst4_2(f1 float32, f2 float32) Mem {
	key := [2]float32{f1, f2}
	if k, ok := floatConsts4_2[key]; ok {
		return k
	}
	k := GLOBL(fmt.Sprintf("f4_2_%08x%08x", math.Float32bits(f1), math.Float32bits(f2)), attr.RODATA|attr.NOPTR)
	DATA(0, F32(f1))
	DATA(4, F32(f1))
	DATA(8, F32(f1))
	DATA(12, F32(f1))
	DATA(16, F32(f2))
	DATA(20, F32(f2))
	DATA(24, F32(f2))
	DATA(28, F32(f2))
	floatConsts4_2[key] = k
	return k
}

func PCALIGN(im Op) {
	Instruction(&ir.Instruction{
		Opcode:   "PCALIGN",
		Operands: []Op{im},
	})
}

var zeroToFour, absMask Mem
var zeroToThree_x2, oneToFour_x2 Mem

func init() {
	zeroToFour = GLOBL("zeroToFour", RODATA|NOPTR)
	DATA(0, F32(0))
	DATA(4, F32(1))
	DATA(8, F32(2))
	DATA(12, F32(3))
	DATA(16, F32(4))

	absMask = GLOBL("absMask", RODATA|NOPTR)
	DATA(0, U32(1<<31))
	DATA(4, U32(1<<31))
	DATA(8, U32(1<<31))
	DATA(12, U32(1<<31))

	zeroToThree_x2 = GLOBL("zeroToThree_x2", RODATA|NOPTR)
	DATA(0, F32(0))
	DATA(4, F32(1))
	DATA(8, F32(2))
	DATA(12, F32(3))
	DATA(16, F32(0))
	DATA(20, F32(1))
	DATA(24, F32(2))
	DATA(28, F32(3))

	oneToFour_x2 = GLOBL("oneToFour_x2", RODATA|NOPTR)
	DATA(0, F32(1))
	DATA(4, F32(2))
	DATA(8, F32(3))
	DATA(12, F32(4))
	DATA(16, F32(1))
	DATA(20, F32(2))
	DATA(24, F32(3))
	DATA(28, F32(4))
}

func main() {
	Package("honnef.co/go/gutter/internal/sparse")
	ConstraintExpr("!purego")

	memsetColumnsAVX()
	fillComplexAVX()

	memsetColumnsSSE()
	fillComplexSSE()

	computeWindingSSE()
	computeWindingAVX(true)
	computeWindingAVX(false)

	processOutOfBoundsWindingSSE()

	computeAlphasNonZeroSSE()
	computeAlphasNonZeroAVX()

	Generate()
}

func fillPrologue() (data reg.Register, offset reg.Register) {
	outLen := Load(Param("buf").Len(), GP64())
	// multiply by strip height
	SHLQ(Imm(2), outLen)
	TESTQ(outLen, outLen)
	JZ(LabelRef("exit"))

	outData := Load(Param("buf").Base(), GP64())
	// multiply by byte size of color
	SHLQ(Imm(4), outLen)
	ADDQ(outLen, outData)
	NEGQ(outLen)

	return outData, outLen
}

func fillPrologueAVX() (data reg.Register, offset reg.Register, colorx2 reg.VecVirtual) {
	data, offset = fillPrologue()
	b, _ := Param("color").Index(0).Resolve()
	colorx2 = YMM()
	VBROADCASTF128(b.Addr, colorx2)

	return data, offset, colorx2
}

func fillEpilogueAVX() {
	Label("exit")
	VZEROUPPER()
	RET()
}

func memsetColumnsAVX() {
	Implement("memsetColumnsAVX")

	outData, outLen, colorx2 := fillPrologueAVX()

	PCALIGN(Imm(16))
	Label("loop")
	const unroll = 2
	for i := range unroll {
		VMOVUPS(colorx2, Mem{Base: outData}.Idx(outLen, 1).Offset(i*2*4*4))
	}
	ADDQ(Imm(unroll*2*4*4), outLen)
	JL(LabelRef("loop"))

	fillEpilogueAVX()
}

func fillComplexAVX() {
	Implement("fineFillComplexAVX")

	outData, outLen, colorx2 := fillPrologueAVX()

	// Load() would emit a MOVSS instruction, which on our Ryzen 3950X results
	// in slower code than using VMOVSS, probably because of mixing SSE and AVX.
	alphaAddr, _ := Param("color").Index(3).Resolve()
	alpha := XMM()
	VMOVSS(alphaAddr.Addr, alpha)

	const unroll = 2

	// New color is translucent, blend with old pixels
	oneMinusAlpha := YMM()
	one := XMM()
	VMOVSS(floatConst(1), one)
	VSUBSS(alpha, one, oneMinusAlpha.AsX())

	// These two instructions achieve the same as
	// VBROADCASTSS(oneMinusAlpha.AsX(), oneMinusAlpha), are virtually identical
	// in speed on our Ryzen 3950X but don't need AVX2.
	VSHUFPS(Imm(0), oneMinusAlpha.AsX(), oneMinusAlpha.AsX(), oneMinusAlpha.AsX())
	VINSERTF128(Imm(1), oneMinusAlpha.AsX(), oneMinusAlpha, oneMinusAlpha)

	PCALIGN(Imm(16))
	Label("loop")
	for i := range unroll {
		bg := YMM()
		VMOVAPS(Mem{Base: outData}.Idx(outLen, 1).Offset(i*2*4*4), bg)
		VMULPS(oneMinusAlpha, bg, bg)
		VADDPS(colorx2, bg, bg)
		VMOVAPS(bg, Mem{Base: outData}.Idx(outLen, 1).Offset(i*2*4*4))
	}

	ADDQ(I32(unroll*2*4*4), outLen)
	JL(LabelRef("loop"))

	fillEpilogueAVX()
}

func fillPrologueSSE() (data reg.Register, offset reg.Register, color reg.VecVirtual) {
	data, offset = fillPrologue()
	b, _ := Param("color").Index(0).Resolve()
	color = XMM()
	MOVUPS(b.Addr, color)

	return data, offset, color
}

func fillEpilogueSSE() {
	Label("exit")
	RET()
}

func memsetColumnsSSE() {
	Implement("memsetColumnsSSE")

	outData, outLen, color := fillPrologueSSE()

	// New color is opaque, replace old pixels
	Label("loop")
	const unroll = 2
	for i := range unroll {
		MOVAPS(color, Mem{Base: outData}.Idx(outLen, 1).Offset(i*4*4))
	}
	ADDQ(Imm(unroll*4*4), outLen)
	JL(LabelRef("loop"))

	fillEpilogueSSE()
}

func fillComplexSSE() {
	Implement("fineFillComplexSSE")

	outData, outLen, color := fillPrologueSSE()

	alpha := Load(Param("color").Index(3), XMM())

	oneMinusAlpha := XMM()
	one := XMM()
	MOVSS(floatConst(1), one)
	MOVSS(one, oneMinusAlpha)
	SUBSS(alpha, oneMinusAlpha)
	SHUFPS(Imm(0), oneMinusAlpha, oneMinusAlpha)

	Label("loop")
	const unroll = 2
	for i := range unroll {
		bg := XMM()
		MOVAPS(Mem{Base: outData}.Idx(outLen, 1).Offset(i*4*4), bg)
		MULPS(oneMinusAlpha, bg)
		ADDPS(color, bg)
		MOVAPS(bg, Mem{Base: outData}.Idx(outLen, 1).Offset(i*4*4))
	}
	ADDQ(Imm(unroll*4*4), outLen)
	JL(LabelRef("loop"))

	fillEpilogueSSE()
}

func computeWindingSSE() {
	Implement("computeWindingSSE")

	acc := XMM()
	XORPS(acc, acc)

	lineTopY_4 := broadcastF32Param(Param("lineTopY"), XMM())
	lineBottomY_4 := broadcastF32Param(Param("lineBottomY"), XMM())
	lineTopX_4 := broadcastF32Param(Param("lineTopX"), XMM())
	pxTopY_4 := XMM()
	MOVUPS(zeroToFour, pxTopY_4)
	pxBottomY_4 := XMM()
	MOVUPS(zeroToFour.Offset(4), pxBottomY_4)

	ymin := XMM()
	MOVUPS(lineTopY_4, ymin)
	MAXPS(pxTopY_4, ymin)
	ymax := XMM()
	MOVUPS(lineBottomY_4, ymax)
	MINPS(pxBottomY_4, ymax)
	xSlope_4 := broadcastF32Param(Param("xSlope"), XMM())
	ySlope_4 := broadcastF32Param(Param("ySlope"), XMM())
	sign_4 := broadcastF32Param(Param("sign"), XMM())

	mask := XMM()
	MOVUPS(absMask, mask)

	for xIdx := range 4 {
		memPxLeftX_4 := floatConst4(float32(xIdx))
		memPxRightX_4 := floatConst4(float32(xIdx + 1))

		linePxLeftY := XMM()
		MOVUPS(memPxLeftX_4, linePxLeftY)
		SUBPS(lineTopX_4, linePxLeftY)
		MULPS(ySlope_4, linePxLeftY)
		ADDPS(lineTopY_4, linePxLeftY)
		MAXPS(ymin, linePxLeftY)
		MINPS(ymax, linePxLeftY)

		linePxRightY := XMM()
		MOVUPS(memPxRightX_4, linePxRightY)
		SUBPS(lineTopX_4, linePxRightY)
		MULPS(ySlope_4, linePxRightY)
		ADDPS(lineTopY_4, linePxRightY)
		MAXPS(ymin, linePxRightY)
		MINPS(ymax, linePxRightY)

		linePxLeftYX := XMM()
		MOVUPS(linePxLeftY, linePxLeftYX)
		SUBPS(lineTopY_4, linePxLeftYX)
		MULPS(xSlope_4, linePxLeftYX)
		ADDPS(lineTopX_4, linePxLeftYX)

		linePxRightYX := XMM()
		MOVUPS(linePxRightY, linePxRightYX)
		SUBPS(lineTopY_4, linePxRightYX)
		MULPS(xSlope_4, linePxRightYX)
		ADDPS(lineTopX_4, linePxRightYX)

		h_4 := XMM()
		MOVUPS(linePxRightY, h_4)
		SUBPS(linePxLeftY, h_4)
		tmp := XMM()
		MOVUPS(mask, tmp)
		ANDNPS(h_4, tmp)
		h_4 = tmp
		tmp = nil

		area_4 := XMM()
		MOVUPS(floatConst4(float32(2*(1+xIdx))), area_4)
		SUBPS(linePxRightYX, area_4)
		SUBPS(linePxLeftYX, area_4)
		MULPS(h_4, area_4)
		tmp = XMM()
		MOVUPS(floatConst4(0.5), tmp)
		MULPS(tmp, area_4)

		MULPS(sign_4, area_4)
		ADDPS(acc, area_4)
		d, _ := Dereference(Param("locationWinding")).Index(xIdx).Index(0).Resolve()
		tmp = XMM()
		MOVUPS(d.Addr, tmp)
		ADDPS(tmp, area_4)
		MOVUPS(area_4, d.Addr)

		MULPS(sign_4, h_4)
		ADDPS(h_4, acc)
	}

	d, _ := Dereference(Param("accumulatedWinding")).Index(0).Resolve()

	tmp := XMM()
	MOVUPS(d.Addr, tmp)
	ADDPS(acc, tmp)
	MOVUPS(tmp, d.Addr)
	RET()
}

func processOutOfBoundsWindingSSE() {
	Implement("processOutOfBoundsWindingSSE")

	pxTopY := XMM()
	MOVUPS(zeroToFour, pxTopY)
	pxBottomY := XMM()
	MOVUPS(zeroToFour.Offset(4), pxBottomY)

	ymin := broadcastF32Param(Param("ymin"), XMM())
	MAXPS(pxTopY, ymin)

	ymax := broadcastF32Param(Param("ymax"), XMM())
	MINPS(pxBottomY, ymax)

	sign := broadcastF32Param(Param("sign"), XMM())

	zero := XMM()
	XORPS(zero, zero)
	SUBPS(ymin, ymax)
	MAXPS(zero, ymax)
	MULPS(sign, ymax)
	signh := ymax

	d, _ := Dereference(Param("accumulatedWinding")).Index(0).Resolve()
	tmp := XMM()
	MOVUPS(d.Addr, tmp)
	ADDPS(signh, tmp)
	MOVUPS(tmp, d.Addr)

	locationWinding := Dereference(Param("locationWinding"))
	for xIdx := range 4 {
		d, _ := locationWinding.Index(xIdx).Index(0).Resolve()
		tmp := XMM()
		MOVUPS(d.Addr, tmp)
		ADDPS(signh, tmp)
		MOVUPS(tmp, d.Addr)
	}

	RET()
}

func broadcastF32Param(param gotypes.Component, dst reg.VecVirtual) reg.VecVirtual {
	if dst.Size() < 32 {
		Load(param, dst)
		SHUFPS(Imm(0), dst, dst)
		return dst
	} else {
		r, err := param.Resolve()
		if err != nil {
			panic(err)
		}
		VBROADCASTSS(r.Addr, dst)
		return dst
	}
}

func computeWindingAVX(fma bool) {
	if fma {
		Implement("computeWindingAVXFMA")
	} else {
		Implement("computeWindingAVX")
	}

	acc := XMM()
	VXORPS(acc, acc, acc)

	lineTopY_8 := broadcastF32Param(Param("lineTopY"), YMM())
	lineBottomY_8 := broadcastF32Param(Param("lineBottomY"), YMM())
	lineTopX_8 := broadcastF32Param(Param("lineTopX"), YMM())
	pxTopY_8 := YMM()
	VMOVUPS(zeroToThree_x2, pxTopY_8)
	pxBottomY_8 := YMM()
	VMOVUPS(oneToFour_x2, pxBottomY_8)

	ymin := YMM()
	VMAXPS(pxTopY_8, lineTopY_8, ymin)
	ymax := YMM()
	VMINPS(pxBottomY_8, lineBottomY_8, ymax)
	xSlope_8 := broadcastF32Param(Param("xSlope"), YMM())
	ySlope_8 := broadcastF32Param(Param("ySlope"), YMM())
	sign_8 := broadcastF32Param(Param("sign"), YMM())

	mask := YMM()
	VBROADCASTSS(floatConst(math.Float32frombits(1<<31)), mask)

	oneHalf := YMM()
	VBROADCASTSS(floatConst(0.5), oneHalf)

	locationWinding := Dereference(Param("locationWinding"))

	for it := range 2 {
		xIdx := 2 * it
		xIdx2 := 2*it + 1

		memPxLeftX_8 := floatConst4_2(float32(xIdx), float32(xIdx2))
		memPxRightX_8 := floatConst4_2(float32(xIdx+1), float32(xIdx2+1))

		linePxLeftY := YMM()
		VMOVUPS(memPxLeftX_8, linePxLeftY)
		VSUBPS(lineTopX_8, linePxLeftY, linePxLeftY)
		if fma {
			//           a =           a *        c +          b
			// linePxLeftY = linePxLeftY * ySlope_8 + lineTopY_8
			VFMADD132PS(ySlope_8, lineTopY_8, linePxLeftY)
		} else {
			VMULPS(ySlope_8, linePxLeftY, linePxLeftY)
			VADDPS(lineTopY_8, linePxLeftY, linePxLeftY)
		}
		VMAXPS(ymin, linePxLeftY, linePxLeftY)
		VMINPS(ymax, linePxLeftY, linePxLeftY)

		linePxRightY := YMM()
		VMOVUPS(memPxRightX_8, linePxRightY)
		VSUBPS(lineTopX_8, linePxRightY, linePxRightY)
		if fma {
			//            a =            a *        c +          b
			// linePxRightY = linePxRightY * ySlope_8 + lineTopY_8
			VFMADD132PS(ySlope_8, lineTopY_8, linePxRightY)
		} else {
			VMULPS(ySlope_8, linePxRightY, linePxRightY)
			VADDPS(lineTopY_8, linePxRightY, linePxRightY)
		}
		VMAXPS(ymin, linePxRightY, linePxRightY)
		VMINPS(ymax, linePxRightY, linePxRightY)

		linePxLeftYX := YMM()
		VSUBPS(lineTopY_8, linePxLeftY, linePxLeftYX)
		if fma {
			//            a =            a *        c + b
			// linePxLeftYX = linePxLeftYX * xSlope_8 + lineTopX_8
			VFMADD132PS(xSlope_8, lineTopX_8, linePxLeftYX)
		} else {
			VMULPS(xSlope_8, linePxLeftYX, linePxLeftYX)
			VADDPS(lineTopX_8, linePxLeftYX, linePxLeftYX)
		}

		linePxRightYX := YMM()
		VSUBPS(lineTopY_8, linePxRightY, linePxRightYX)
		if fma {
			//             a =             a *        c + b
			// linePxRightYX = linePxRightYX * xSlope_8 + lineTopX_8
			VFMADD132PS(xSlope_8, lineTopX_8, linePxRightYX)
		} else {
			VMULPS(xSlope_8, linePxRightYX, linePxRightYX)
			VADDPS(lineTopX_8, linePxRightYX, linePxRightYX)
		}

		h_8 := YMM()
		VSUBPS(linePxLeftY, linePxRightY, h_8)
		VANDNPS(h_8, mask, h_8)

		area_8 := YMM()
		VMOVUPS(floatConst4_2(float32(2*(1+xIdx)), float32(2*(1+xIdx2))), area_8)
		VSUBPS(linePxRightYX, area_8, area_8)
		VSUBPS(linePxLeftYX, area_8, area_8)
		VMULPS(h_8, area_8, area_8)
		VMULPS(oneHalf, area_8, area_8)

		signarea_8 := YMM()
		VMULPS(sign_8, area_8, signarea_8)
		signh_8 := YMM()
		VMULPS(sign_8, h_8, signh_8)

		tmp := XMM()
		d, _ := locationWinding.Index(xIdx).Index(0).Resolve()
		VADDPS(d.Addr, acc, tmp)
		signarea_4_1 := signarea_8.AsX()
		VADDPS(signarea_4_1, tmp, tmp)
		VMOVUPS(tmp, d.Addr)

		signh_4_1 := signh_8.AsX()
		VADDPS(signh_4_1, acc, acc)

		d, _ = locationWinding.Index(xIdx2).Index(0).Resolve()
		VADDPS(d.Addr, acc, tmp)
		signarea_4_2 := XMM()
		VEXTRACTF128(Imm(1), signarea_8, signarea_4_2)
		VADDPS(signarea_4_2, tmp, tmp)
		VMOVUPS(tmp, d.Addr)

		signh_4_2 := XMM()
		VEXTRACTF128(Imm(1), signh_8, signh_4_2)
		VADDPS(signh_4_2, acc, acc)
	}

	d, _ := Dereference(Param("accumulatedWinding")).Index(0).Resolve()

	tmp := XMM()
	VADDPS(d.Addr, acc, tmp)
	VMOVUPS(tmp, d.Addr)

	VZEROUPPER()
	RET()
}

func computeAlphasNonZeroSSE() {
	Implement("computeAlphasNonZeroSSE")

	locationWinding := Dereference(Param("locationWinding"))
	tail := Dereference(Param("tail"))
	maxUint8 := XMM()
	MOVUPS(floatConst4(255), maxUint8)
	one := XMM()
	MOVUPS(floatConst4(1), one)
	oneHalf := XMM()
	MOVUPS(floatConst4(0.5), oneHalf)
	mask := XMM()
	MOVUPS(absMask, mask)

	var areas [4]reg.VecVirtual
	for i := range 4 {
		d, _ := locationWinding.Index(i).Index(0).Resolve()
		areas[i] = XMM()
		MOVUPS(d.Addr, areas[i])
		tmp := XMM()
		MOVAPS(mask, tmp)
		ANDNPS(areas[i], tmp)
		areas[i] = tmp
		MINPS(one, areas[i])
		MULPS(maxUint8, areas[i])
		ADDPS(oneHalf, areas[i])
	}

	// Convert four float32 to four int32, four times
	CVTTPS2PL(areas[0], areas[0]) // CVTTPS2DQ
	CVTTPS2PL(areas[1], areas[1]) // CVTTPS2DQ
	CVTTPS2PL(areas[2], areas[2]) // CVTTPS2DQ
	CVTTPS2PL(areas[3], areas[3]) // CVTTPS2DQ

	// Convert eight int32 (four from each argument) to eight int16, two times
	PACKSSLW(areas[1], areas[0]) // PACKSSDW
	PACKSSLW(areas[3], areas[2]) // PACKSSDW

	// Convert sixteen int16 to sixteen uint8 (eight from each argument)
	PACKUSWB(areas[2], areas[0])

	// Store sixteen uint8 to memory
	d1, _ := tail.Index(0).Index(0).Resolve()
	MOVUPS(areas[0], d1.Addr)
	RET()
}

func computeAlphasNonZeroAVX() {
	Implement("computeAlphasNonZeroAVX")

	locationWinding := Dereference(Param("locationWinding"))
	tail := Dereference(Param("tail"))
	maxUint8 := YMM()
	VBROADCASTSS(floatConst(255), maxUint8)
	one := YMM()
	VBROADCASTSS(floatConst(1), one)
	oneHalf := YMM()
	VBROADCASTSS(floatConst(0.5), oneHalf)
	mask := YMM()
	VBROADCASTSS(floatConst(math.Float32frombits(1<<31)), mask)

	var areas [2]reg.VecVirtual
	for i := range 2 {
		d, _ := locationWinding.Index(2 * i).Index(0).Resolve()
		areas[i] = YMM()
		VANDNPS(d.Addr, mask, areas[i])
		VMINPS(one, areas[i], areas[i])
		VMULPS(maxUint8, areas[i], areas[i])
		VADDPS(oneHalf, areas[i], areas[i])
	}

	// Convert eight float32 to eight int32, two times
	VCVTTPS2DQ(areas[0], areas[0])
	VCVTTPS2DQ(areas[1], areas[1])

	// Convert sixteen int32 (eight from each argument) to sixteen int16
	VPACKSSDW(areas[1], areas[0], areas[0])

	// Convert sixteen int16 to sixteen uint8 (really 32 to 32, but we only have
	// 16 useful values)
	VPACKUSWB(areas[0], areas[0], areas[0])

	memBlendMask := GLOBL("permMask", RODATA|NOPTR)
	DATA(0, U32(0))
	DATA(4, U32(4))
	DATA(8, U32(1))
	DATA(12, U32(5))
	DATA(16, U32(2))
	DATA(20, U32(6))
	DATA(24, U32(3))
	DATA(28, U32(7))

	blendMask := YMM()
	VMOVUPS(memBlendMask, blendMask)
	VPERMD(areas[0], blendMask, areas[0])

	// Store sixteen uint8 to memory
	d1, _ := tail.Index(0).Index(0).Resolve()
	VMOVUPS(areas[0].AsX(), d1.Addr)

	VZEROUPPER()
	RET()
}
