// SPDX-FileCopyrightText: 2025 Dominik Honnef and contributors
//
// SPDX-License-Identifier: MIT

package main

import (
	. "github.com/mmcloughlin/avo/build"
	"github.com/mmcloughlin/avo/ir"
	. "github.com/mmcloughlin/avo/operand"
)

//go:generate go run . -out ../sparse_amd64.s -pkg sparse

func main() {
	Package("honnef.co/go/gutter/internal/sparse")
	ConstraintExpr("!purego")
	fillAVX()
	fillSSE()
	Generate()
}

var gOne = ConstData("one", F32(1))

func PCALIGN(im Op) {
	Instruction(&ir.Instruction{
		Opcode:   "PCALIGN",
		Operands: []Op{im},
	})
}

func fillAVX() {
	Implement("fineFillAVX")
	outLen := Load(Param("out").Len(), GP64())
	// multiply by strip height
	SHLQ(Imm(2), outLen)
	TESTQ(outLen, outLen)
	JZ(LabelRef("exit"))

	one := XMM()
	VMOVSS(gOne, one)

	b, _ := Param("color").Index(0).Resolve()
	colorx2 := YMM()
	VBROADCASTF128(b.Addr, colorx2)

	outData := Load(Param("out").Base(), GP64())
	SHLQ(Imm(4), outLen)
	ADDQ(outLen, outData)
	NEGQ(outLen)

	// Load() would emit a MOVSS instruction, which on our Ryzen 3950X results
	// in slower code than using VMOVSS, probably because of mixing SSE and AVX.
	alphaAddr, _ := Param("color").Index(3).Resolve()
	alpha := XMM()
	VMOVSS(alphaAddr.Addr, alpha)
	VUCOMISS(one, alpha)
	JNE(LabelRef("blend"))

	const unroll = 2

	// New color is opaque, replace old pixels
	PCALIGN(Imm(16))
	Label("loopOpaque")
	for i := range unroll {
		VMOVAPS(colorx2, Mem{Base: outData}.Idx(outLen, 1).Offset(i*2*4*4))
	}
	ADDQ(Imm(unroll*2*4*4), outLen)
	JL(LabelRef("loopOpaque"))
	VZEROUPPER()
	RET()

	// New color is translucent, blend with old pixels
	Label("blend")
	oneMinusAlpha := YMM()
	VSUBSS(alpha, one, oneMinusAlpha.AsX())

	// These two instructions achieve the same as
	// VBROADCASTSS(oneMinusAlpha.AsX(), oneMinusAlpha), are virtually identical
	// in speed on our Ryzen 3950X but don't need AVX2.
	VSHUFPS(Imm(0), oneMinusAlpha.AsX(), oneMinusAlpha.AsX(), oneMinusAlpha.AsX())
	VINSERTF128(Imm(1), oneMinusAlpha.AsX(), oneMinusAlpha, oneMinusAlpha)

	state := Load(Param("complex"), GP64())
	TESTQ(state, state)
	JNZ(LabelRef("loopTranslucent"))

	// Old tile contents are a single color
	bg := YMM()
	b, _ = Param("singleColor").Index(0).Resolve()
	VBROADCASTF128(b.Addr, bg)
	VMULPS(oneMinusAlpha, bg, bg)
	VADDPS(colorx2, bg, bg)
	PCALIGN(Imm(16))
	Label("loopTranslucentSingle")
	for i := range unroll {
		VMOVAPS(bg, Mem{Base: outData}.Idx(outLen, 1).Offset(i*2*4*4))
	}
	ADDQ(I32(unroll*2*4*4), outLen)
	JL(LabelRef("loopTranslucentSingle"))
	VZEROUPPER()
	RET()

	// Old tile contents are multiple colors
	PCALIGN(Imm(16))
	Label("loopTranslucent")
	for i := range unroll {
		bg := YMM()
		VMOVAPS(Mem{Base: outData}.Idx(outLen, 1).Offset(i*2*4*4), bg)
		VMULPS(oneMinusAlpha, bg, bg)
		VADDPS(colorx2, bg, bg)
		VMOVAPS(bg, Mem{Base: outData}.Idx(outLen, 1).Offset(i*2*4*4))
	}

	ADDQ(I32(unroll*2*4*4), outLen)
	JL(LabelRef("loopTranslucent"))

	Label("exit")
	VZEROUPPER()
	RET()
}

func fillSSE() {
	Implement("fineFillSSE")
	outLen := Load(Param("out").Len(), GP64())
	// multiply by strip height
	SHLQ(Imm(2), outLen)
	TESTQ(outLen, outLen)
	JZ(LabelRef("exit"))

	one := XMM()
	MOVSS(gOne, one)

	b, _ := Param("color").Index(0).Resolve()
	color := XMM()
	MOVUPS(b.Addr, color)

	outData := Load(Param("out").Base(), GP64())
	SHLQ(Imm(4), outLen)
	ADDQ(outLen, outData)
	NEGQ(outLen)

	alpha := Load(Param("color").Index(3), XMM())
	UCOMISS(one, alpha)
	JNE(LabelRef("blend"))

	const unroll = 2

	// New color is opaque, replace old pixels
	Label("loopOpaque")
	for i := range unroll {
		MOVAPS(color, Mem{Base: outData}.Idx(outLen, 1).Offset(i*4*4))
	}
	ADDQ(Imm(unroll*4*4), outLen)
	JL(LabelRef("loopOpaque"))
	RET()

	// New color is translucent, blend with old pixels
	Label("blend")
	oneMinusAlpha := XMM()
	MOVSS(one, oneMinusAlpha)
	SUBSS(alpha, oneMinusAlpha)
	SHUFPS(Imm(0), oneMinusAlpha, oneMinusAlpha)

	state := Load(Param("complex"), GP8())
	TESTB(state, state)
	JNZ(LabelRef("loopTranslucent"))

	// Old tile contents are a single color
	bg := XMM()
	b, _ = Param("singleColor").Index(0).Resolve()
	MOVUPS(b.Addr, bg)
	MULPS(oneMinusAlpha, bg)
	ADDPS(color, bg)

	Label("loopTranslucentSingle")
	for i := range unroll {
		MOVAPS(bg, Mem{Base: outData}.Idx(outLen, 1).Offset(i*4*4))
	}
	ADDQ(Imm(unroll*4*4), outLen)
	JL(LabelRef("loopTranslucentSingle"))
	RET()

	// Old tile contents are multiple colors
	Label("loopTranslucent")
	for i := range unroll {
		bg := XMM()
		MOVAPS(Mem{Base: outData}.Idx(outLen, 1).Offset(i*4*4), bg)
		MULPS(oneMinusAlpha, bg)
		ADDPS(color, bg)
		MOVAPS(bg, Mem{Base: outData}.Idx(outLen, 1).Offset(i*4*4))
	}
	ADDQ(Imm(unroll*4*4), outLen)
	JL(LabelRef("loopTranslucent"))

	Label("exit")
	RET()
}
