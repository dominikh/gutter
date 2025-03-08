// SPDX-FileCopyrightText: 2025 Dominik Honnef and contributors
//
// SPDX-License-Identifier: MIT

package main

import (
	. "github.com/mmcloughlin/avo/build"
	"github.com/mmcloughlin/avo/ir"
	. "github.com/mmcloughlin/avo/operand"
	"github.com/mmcloughlin/avo/reg"
)

var gOne = ConstData("one", F32(1))

func PCALIGN(im Op) {
	Instruction(&ir.Instruction{
		Opcode:   "PCALIGN",
		Operands: []Op{im},
	})
}

func main() {
	Package("honnef.co/go/gutter/internal/sparse")
	ConstraintExpr("!purego")

	memsetColumnsAVX()
	fillComplexAVX()

	memsetColumnsSSE()
	fillComplexSSE()

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
	VMOVSS(gOne, one)
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
	MOVSS(gOne, one)
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
