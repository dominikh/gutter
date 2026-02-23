// SPDX-FileCopyrightText: 2025 Dominik Honnef and contributors
//
// SPDX-License-Identifier: MIT
// SPDX-FileAttributionText: https://github.com/thomcc/fast-srgb8
// SPDX-FileAttributionText: https://github.com/linebender/fearless_simd/blob/c3632abfdbe3357ddb68496f9c4dd001ff13e218/fearless_simd/examples/srgb.rs

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

var uint64Consts = map[uint64]Mem{}
var floatConsts = map[float32]Mem{}
var floatConsts4 = map[float32]Mem{}

func uint64Const(v uint64) Mem {
	if k, ok := uint64Consts[v]; ok {
		return k
	}
	k := ConstData(fmt.Sprintf("q_%08x", v), U64(v))
	uint64Consts[v] = k
	return k
}

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

func PCALIGN(im Op) {
	Instruction(&ir.Instruction{
		Opcode:   "PCALIGN",
		Operands: []Op{im},
	})
}

var zeroToFour, absMask Mem

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
}

func main() {
	Package("honnef.co/go/gutter/internal/sparse")
	ConstraintExpr("!purego")

	memsetColumnsAVX()
	processOutOfBoundsWindingSSE()
	computeAlphasNonZeroAVX()
	linearRgbaF32ToSrgbU8_Polynomial_AVX2()

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

	permMask := YMM()
	VPMOVSXBD(uint64Const(0x0703060205010400), permMask)
	VPERMD(areas[0], permMask, areas[0])

	// Store sixteen uint8 to memory
	d1, _ := tail.Index(0).Index(0).Resolve()
	VMOVUPS(areas[0].AsX(), d1.Addr)

	VZEROUPPER()
	RET()
}

func linearRgbaF32ToSrgbU8_Polynomial_AVX2() {
	// This function uses a degree 5 polynomial to approximate the non-linear
	// portion of the linear to sRGB transfer function.
	//
	// This is based on Raph Levien's math in
	// https://colab.research.google.com/drive/13HdyQAABQKVsJbTBCojzdBEeTibsPfVF#scrollTo=CCm2xKs5h3-G
	// and the two implementations at
	// https://gist.github.com/raphlinus/8a39ed43ecfd5eb28a9b3bb2c9ad6dc0 and
	// https://github.com/linebender/fearless_simd/blob/c3632abfdbe3357ddb68496f9c4dd001ff13e218/fearless_simd/examples/srgb.rs.
	//
	// For the conversion to 8-bit bytes, this function provides results of very
	// similar quality to the LUT-based approach, at over twice the speed. This
	// function could also be repurposed to produce sRGB values in float32 at
	// well over 8-bits of accuracy.

	Implement("linearRgbaF32ToSrgbU8_Polynomial_AVX2")
	const batchSizeInFloats = 32

	// inLen := Load(Param("in").Len(), GP64())
	inData := Load(Param("in"), GP64())
	inLen := GP64()
	// width * height * 4 floats per pixel
	MOVQ(U32(256*4*4), inLen)

	outData := Load(Param("out"), GP64())

	LEAQ(Mem{Base: inData, Index: inLen, Scale: 4}, inData)
	LEAQ(Mem{Base: outData, Index: inLen, Scale: 1}, outData)
	NEGQ(inLen)

	c5, c4, c2, c0, c3, c1, threshold :=
		YMM(), YMM(), YMM(), YMM(), YMM(), YMM(), YMM()
	VBROADCASTSS(floatConst(-2.88143143e-02), c0)
	VBROADCASTSS(floatConst(1.40194533e+00), c1)
	VBROADCASTSS(floatConst(-9.12795913e-01), c2)
	VBROADCASTSS(floatConst(1.06133172e+00), c3)
	VBROADCASTSS(floatConst(-7.29192910e-01), c4)
	VBROADCASTSS(floatConst(2.07758287e-01), c5)
	VBROADCASTSS(floatConst(0.0031308), threshold)

	Label("loop")

	Comment("load data")
	channels := [4]reg.VecVirtual{YMM(), YMM(), YMM(), YMM()}
	for i, reg := range channels {
		VMOVUPS(Mem{Base: inData, Index: inLen, Scale: 4, Disp: i * 8 * 4}, reg)
	}

	packedRgbaF32ToPlanaRgbaF32(channels, channels)

	unpremul := Load(Param("unpremul"), GP8())
	TESTB(unpremul, unpremul)
	JZ(LabelRef("skipUnpremul"))

	zero, one := YMM(), YMM()
	VXORPS(zero, zero, zero)
	VBROADCASTSS(floatConst(1), one)
	for _, reg := range channels[:3] {
		VDIVPS(channels[3], reg, reg)
		VMINPS(one, reg, reg)
		VMAXPS(zero, reg, reg)
	}
	VMINPS(one, channels[3], channels[3])
	VMAXPS(zero, channels[3], channels[3])

	Label("skipUnpremul")

	for i, reg := range channels[:3] {
		Commentf("plane %d", i)

		x := YMM()
		bias := YMM()
		VBROADCASTSS(floatConst(-5.35862651e-04), bias)
		VADDPS(bias, reg, x)

		even1, even2, odd1, sqrtX, x2 :=
			YMM(), YMM(), YMM(), YMM(), YMM()
		VMOVAPS(x, even1)
		VFMADD132PS(c2, c0, even1)

		VMOVAPS(x, odd1)
		VFMADD132PS(c3, c1, odd1)

		VMULPS(x, x, x2)
		VMOVAPS(x2, even2)
		VFMADD132PS(c4, even1, even2)

		VFMADD132PS(c5, odd1, x2)
		odd2, x2 := x2, nil

		VSQRTPS(x, sqrtX)
		VFMADD132PS(sqrtX, even2, odd2)
		poly, odd2 := odd2, nil

		lin := YMM()
		mult := YMM()
		VBROADCASTSS(floatConst(12.92), mult)
		VMULPS(mult, reg, lin)

		m := YMM()
		VCMPPS(Imm(0xE), threshold, reg, m)

		res := reg
		VPBLENDVB(m, poly, lin, res)
		maxByte := YMM()
		VBROADCASTSS(floatConst(255), maxByte)
		VMULPS(res, maxByte, res)

		// The DirectX spec requires rounding via floor(c + 0.5), which, for
		// positive values, is round to nearest, round half up (with some
		// inaccuracies caused by precision; for example
		// float32(0.49999997) + 0.5 = 1).
		//
		// VCVTPS2DQ will round to nearest, round half to even instead (with the
		// default value of MXCSR.) With our approximation, it doesn't matter.
		// Our worst error is >0.5 ULP either way, and all interesting metrics
		// (max error, cumulative error, number of wrongly rounded values) are
		// virtually identical between the two rounding modes. Plus, the spec's
		// behavior isn't consistent between 32-bit and 64-bit floats, anyway.
		VCVTPS2DQ(res, res)
	}

	Comment("plane 3")
	maxByte := YMM()
	VBROADCASTSS(floatConst(255), maxByte)
	VMULPS(channels[3], maxByte, channels[3])
	VCVTPS2DQ(channels[3], channels[3])

	Comment("footer")
	rgba := YMM()
	planarRgbaU32ToPackedRgbaU8(channels, rgba)
	VMOVUPS(rgba, Mem{Base: outData, Index: inLen, Scale: 1})

	ADDQ(Imm(batchSizeInFloats), inLen)
	JL(LabelRef("loop"))

	VZEROUPPER()
	RET()
}

func _MM_SHUFFLE(fp3, fp2, fp1, fp0 uint8) Constant {
	return Imm(uint64((fp3 << 6) | (fp2 << 4) | (fp1 << 2) | fp0))
}

// rgbaPlanarToPacked takes four input registers, one per R, G, B, and A plane,
// each containing eight float32 values and stores to four output registers
// packed RGBA pixels, each containing two pixels.
func rgbaPlanarToPacked(in [4]reg.VecVirtual, out [4]reg.VecVirtual) {
	// XXX verify that this function actually works
	Comment("planar to packed")
	r, g, b, a := in[0], in[1], in[2], in[3]

	rgLo, baLo, rgHi, baHi := YMM(), YMM(), YMM(), YMM()
	VUNPCKLPS(g, r, rgLo) // rgLo = [r0 g0 r1 g1 | r4 g4 r5 g5]
	VUNPCKHPS(g, r, rgHi) // rgHi = [r2 g2 r3 g3 | r6 g6 r7 g7]
	VUNPCKLPS(a, b, baLo) // baLo = [b0 a0 b1 a1 | b4 a4 b5 a5]
	VUNPCKHPS(a, b, baHi) // baHi = [b2 a2 b3 a3 | b6 a6 b7 a7]

	chunky0, chunky1, chunky2, chunky3 := YMM(), YMM(), YMM(), YMM()
	VSHUFPS(_MM_SHUFFLE(2, 0, 2, 0), baLo, rgLo, chunky0) // chunky0 = [r0 g0 b0 a0 | r1 g1 b1 a1]
	VSHUFPS(_MM_SHUFFLE(3, 1, 3, 1), baLo, rgLo, chunky1) // chunky1 = [r4 g4 b4 a4 | r5 g5 b5 a5]
	VSHUFPS(_MM_SHUFFLE(2, 0, 2, 0), baHi, rgHi, chunky2) // chunky2 = [r2 g2 b2 a2 | r3 g3 b3 a3]
	VSHUFPS(_MM_SHUFFLE(3, 1, 3, 1), baHi, rgHi, chunky3) // chunky3 = [r6 g6 b6 a6 | r7 g7 b7 a7]

	VPERM2F128(Imm(0x20), chunky2, chunky0, out[0])
	VPERM2F128(Imm(0x31), chunky2, chunky0, out[1])
	VPERM2F128(Imm(0x20), chunky3, chunky1, out[2])
	VPERM2F128(Imm(0x31), chunky3, chunky1, out[3])
}

// packedRgbaF32ToPlanaRgbaF32 takes four input registers, each containing eight packed
// float32 RGBA pixels and stores to four output registers separated R, G, B,
// and A planes, each containing eight float32 values.
func packedRgbaF32ToPlanaRgbaF32(in [4]reg.VecVirtual, out [4]reg.VecVirtual) {
	Comment("packed to planar")

	// Inputs:
	//
	// in[0] = [r0 g0 b0 a0 | r1 g1 b1 a1]
	// in[1] = [r2 g2 b2 a2 | r3 g3 b3 a3]
	// in[2] = [r4 g4 b4 a4 | r5 g5 b5 a5]
	// in[3] = [r6 g6 b6 a6 | r7 g7 b7 a7]

	t0, t1, t2, t3 := YMM(), YMM(), YMM(), YMM()
	VUNPCKLPS(in[1], in[0], t0) // t0 = [r0 r2 g0 g2 | r1 r3 g1 g3]
	VUNPCKHPS(in[1], in[0], t1) // t1 = [b0 b2 a0 a2 | b1 b3 a1 a3]
	VUNPCKLPS(in[3], in[2], t2) // t2 = [r4 r6 g4 g6 | r5 r7 g5 g7]
	VUNPCKHPS(in[3], in[2], t3) // t3 = [b4 b6 a4 a6 | b5 b7 a5 a7]

	VSHUFPS(Imm(0x44), t2, t0, out[0]) // out[0] = [r0 r2 r4 r6 | r1, r3, r5, r7]
	VSHUFPS(Imm(0xEE), t2, t0, out[1]) // out[1] = [g0 g2 g4 g6 | g1, g3, g5, g7]
	VSHUFPS(Imm(0x44), t3, t1, out[2]) // out[2] = [b0 b2 b4 b6 | b1, b3, b5, b7]
	VSHUFPS(Imm(0xEE), t3, t1, out[3]) // out[3] = [a0 a2 a4 a6 | a1, a3, a5, a7]

	for i, src := range out {
		dst := out[i]

		swapped := YMM()
		hi, lo := YMM(), YMM()
		VPERM2F128(Imm(1), src, src, swapped) // swapped = [r1 r3 r5 r7 | r0 r2 r4 r6]
		VUNPCKLPS(swapped, src, lo)           // lo      = [r0 r1 r2 r3 |  ?  ?  ?  ?]
		VUNPCKHPS(swapped, src, hi)           // hi      = [r4 r5 r6 r7 |  ?  ?  ?  ?]
		VPERM2F128(Imm(0x20), hi, lo, dst)    // dst     = [r0 r1 r2 r3 | r4 r5 r6 r7]
	}
}

// planarRgbaU32ToPackedRgbaU8 takes four input registers, one per R, G, B,
// and A plane, each containing eight uint32 values in the range [0, 255]. It
// stores to out eight packed uint8 RGBA pixels.
func planarRgbaU32ToPackedRgbaU8(in [4]reg.VecVirtual, out reg.VecVirtual) {
	// Each vector in 'in' contains eight 32-bit values in the range [0, 255],
	// which means only their lowest byte is nonzero. Each vector is one of the
	// RGBA planes. To pack them into 32 8-bit values in a single register, we
	// simply shift over the planes and OR them together. On most platforms this
	// should outperform, or be at parity with, using a sequence of VPACKUSDW,
	// VPACKUSWB, and VPSHUFB.
	r, g, b, a := in[0], in[1], in[2], in[3]

	gs, bs, as := YMM(), YMM(), YMM()
	VPSLLD(Imm(8), g, gs)
	VPSLLD(Imm(16), b, bs)
	VPSLLD(Imm(24), a, as)

	rg, ba := YMM(), YMM()
	VPOR(r, gs, rg)
	VPOR(bs, as, ba)
	VPOR(rg, ba, out)
}
