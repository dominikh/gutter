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
var floatConsts8 = map[float32]Mem{}

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

func floatConst8(f float32) Mem {
	if k, ok := floatConsts8[f]; ok {
		return k
	}
	k := GLOBL(fmt.Sprintf("f8_%08x", math.Float32bits(f)), attr.RODATA|attr.NOPTR)
	for i := range 8 {
		DATA(i*4, F32(f))
	}
	floatConsts8[f] = k
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
	ConstraintExpr("!noasm")

	processOutOfBoundsWindingSSE()
	computeAlphasNonZeroAVX()
	packUint8SRGB_AVX2()
	gradientLUTGatherAVX2Impl()
	gradientCascadeMergeAVX2Impl()

	Generate()
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

	areas0Hi := XMM()
	VEXTRACTF128(Imm(1), areas[0], areas0Hi)
	VPUNPCKLDQ(areas0Hi, areas[0].AsX(), areas[0].AsX())

	// Store sixteen uint8 to memory
	d1, _ := tail.Index(0).Index(0).Resolve()
	VMOVUPS(areas[0].AsX(), d1.Addr)

	VZEROUPPER()
	RET()
}

func packUint8SRGB_AVX2() {
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

	Implement("packUint8SRGB_AVX2_Impl")

	inData := Load(Param("in"), GP64())
	outData := Load(Param("out"), GP64())
	stride := Load(Param("stride"), GP64())
	outWidth := Load(Param("outWidth"), GP64())
	unpremul := Load(Param("unpremul"), GP8())

	outWidthBytes := GP64()
	MOVQ(outWidth, outWidthBytes)
	SHLQ(Imm(2), outWidthBytes)

	// c0 and c1 cannot be turned into memory operands because they get used
	// together with c2-c5 in the same instructions, which only permit one
	// memory operand. threshold is kept in a register because that benchmarked
	// better.
	c0, c1, threshold := YMM(), YMM(), YMM()
	VBROADCASTSS(floatConst(-2.88143143e-02), c0)
	VBROADCASTSS(floatConst(1.40194533e+00), c1)

	// These coefficients of the polynomial are used as memory operands and not
	// stored in reused registers, to free up registers for other purposes. This
	// improves performance overall, probably because the latency of the memory
	// loads is hidden by all the math instructions.
	c2mem := floatConst8(-9.12795913e-01)
	c3mem := floatConst8(1.06133172e+00)
	c4mem := floatConst8(-7.29192910e-01)
	c5mem := floatConst8(2.07758287e-01)

	// These constants are also used as memory operands, because benchmarks
	// determined no change in performance compared to broadcasting on every
	// loop iteration, and we don't have the spare registers to reuse them.
	biasMem := floatConst8(-5.35862651e-04)
	linearScaleMem := floatConst8(12.92)

	VBROADCASTSS(floatConst(0.0031308), threshold)

	// Permutation mask for the final 8x4 transpose.
	permMask := YMM()
	VPMOVSXBD(uint64Const(0x0703060205010400), permMask)

	// Registers for holding packed batch results across batches, avoiding
	// stack spills.
	batchResults := [4]reg.VecVirtual{YMM(), YMM(), YMM(), YMM()}

	row0, row1, row2, row3 := outData, GP64(), GP64(), GP64()
	LEAQ(Mem{Base: row0, Index: stride, Scale: 4}, row1)
	LEAQ(Mem{Base: row1, Index: stride, Scale: 4}, row2)
	LEAQ(Mem{Base: row2, Index: stride, Scale: 4}, row3)

	// WideTileBuffer is [4][wideTileWidth][stripHeight]float32. This is a
	// column-major, planar pixel layout. Each plane is 256 * 4 * 4 = 4096 bytes.
	// Each column is 4 * 4 = 16 bytes.
	const planeSize = 256 * 4 * 4
	const colStride = 4 * 4

	colOffset := GP64()
	rowOffset := GP64()
	XORQ(colOffset, colOffset)
	XORQ(rowOffset, rowOffset)

	Label("loop")

	// Check output bounds (need space for 8 pixels = 32 bytes)
	remaining := GP64()
	MOVQ(outWidthBytes, remaining)
	SUBQ(rowOffset, remaining)
	CMPQ(remaining, Imm(32))
	JL(LabelRef("done"))

	// Process 4 batches of 2 columns/8 pixels each
	for batch := range 4 {
		Commentf("batch %d: columns %d-%d", batch, batch*2, batch*2+1)

		batchDisp := batch * 2 * colStride

		channels := [4]reg.VecVirtual{YMM(), YMM(), YMM(), YMM()}
		for ch, reg := range channels {
			VMOVUPS(Mem{Base: inData, Index: colOffset, Scale: 1, Disp: ch*planeSize + batchDisp}, reg)
		}

		// According to
		// https://web.archive.org/web/20250815165940/https://hacksoflife.blogspot.com/2022/06/srgb-pre-multiplied-alpha-and.html
		// whether the color needs to be premultiplied with alpha before or
		// after converting it to sRGB depends on the consumer of the data and
		// whether they will blend in linear or sRGB space. But we can't know
		// what our consumer (likely a display manager) will do… We'll assume
		// that they're modern and blend in linear space and premultiply our
		// colors before conversion to sRGB. Because our colors are already
		// stored premultiplied this saves us work, too.
		//
		// https://web.archive.org/web/20250829113330/https://ssp.impulsetrain.com/gamma-premult.html
		// covers the same topic and says that premultiplying before encoding in
		// sRGB is the right thing to do for GPU textures.
		TESTB(unpremul, unpremul)
		JZ(LabelRef(fmt.Sprintf("skipUnpremul%d", batch)))
		zero, one := YMM(), YMM()
		VXORPS(zero, zero, zero)
		VBROADCASTSS(floatConst(1), one)
		// Multiply with approximate reciprocal of alpha to undo
		// premultiplication. This has lower precision than proper division, but
		// we still stay within our target of 0.6 ULP. This is faster than
		// division, and reduces port pressure on at least all generations of
		// Zen.
		invAlpha := YMM()
		VRCPPS(channels[3], invAlpha)
		for _, reg := range channels[:3] {
			VMULPS(invAlpha, reg, reg)
			VMINPS(one, reg, reg)
			VMAXPS(zero, reg, reg)
		}
		VMINPS(one, channels[3], channels[3])
		VMAXPS(zero, channels[3], channels[3])

		Label(fmt.Sprintf("skipUnpremul%d", batch))

		// Convert RGB channels using polynomial approximation.
		for i, reg := range channels[:3] {
			Commentf("plane %d", i)

			x := YMM()
			VADDPS(biasMem, reg, x)

			even1, even2, odd1, sqrtX, x2 :=
				YMM(), YMM(), YMM(), YMM(), YMM()

			VMOVAPS(x, even1)
			VFMADD132PS(c2mem, c0, even1)

			VMULPS(x, x, x2)
			VMOVAPS(x2, even2)
			VFMADD132PS(c4mem, even1, even2)

			VMOVAPS(x, odd1)
			VFMADD132PS(c3mem, c1, odd1)

			VFMADD132PS(c5mem, odd1, x2)
			odd2, x2 := x2, nil

			VSQRTPS(x, sqrtX)

			VFMADD132PS(sqrtX, even2, odd2)
			poly, odd2 := odd2, nil

			lin := YMM()
			VMULPS(linearScaleMem, reg, lin)

			m := YMM()
			VCMPPS(Imm(0xE), threshold, reg, m)

			VPBLENDVB(m, poly, lin, reg)
			VMULPS(floatConst8(255), reg, reg)

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
			VCVTPS2DQ(reg, channels[i])
		}

		Comment("plane 3")
		VMULPS(floatConst8(255), channels[3], channels[3])
		VCVTPS2DQ(channels[3], channels[3])

		Comment("pack")
		planarRgbaU32ToPackedRgbaU8(channels, batchResults[batch])
	}

	Comment("transpose 4x8 matrix of pixels")

	u0, u1, u2, u3 := transpose8x4(
		batchResults[0], // c01
		batchResults[1], // c23
		batchResults[2], // c45
		batchResults[3], // c67
		permMask,
	)

	// Store to 4 output rows
	VMOVDQU(u0, Mem{Base: row0, Index: rowOffset, Scale: 1})
	VMOVDQU(u1, Mem{Base: row1, Index: rowOffset, Scale: 1})
	VMOVDQU(u2, Mem{Base: row2, Index: rowOffset, Scale: 1})
	VMOVDQU(u3, Mem{Base: row3, Index: rowOffset, Scale: 1})

	// Advance pointers
	ADDQ(U32(8*colStride), colOffset) // 8 columns × 16 bytes/column = 128 bytes
	ADDQ(Imm(32), rowOffset)          // 8 pixels × 4 bytes
	CMPQ(colOffset, U32(256*colStride))
	JL(LabelRef("loop"))

	Label("done")
	VZEROUPPER()
	RET()
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

var int32Consts = map[uint32]Mem{}

func int32Const(v uint32) Mem {
	if k, ok := int32Consts[v]; ok {
		return k
	}
	k := ConstData(fmt.Sprintf("d_%08x", v), U32(v))
	int32Consts[v] = k
	return k
}

// gradientLUTGatherAVX2Impl generates the gradient LUT gather function.
// It reads pre-computed t values (already extended) from a buffer,
// converts them to LUT indices, gathers 4 color channels via VGATHERDPS,
// and stores to 4 planar destination buffers.
func gradientLUTGatherAVX2Impl() {
	Implement("gradientLUTGatherAVX2")

	dst := [4]reg.Register{
		Load(Param("dst0"), GP64()),
		Load(Param("dst1"), GP64()),
		Load(Param("dst2"), GP64()),
		Load(Param("dst3"), GP64()),
	}
	lutBase := Load(Param("lut"), GP64())
	tBufBase := Load(Param("tBuf"), GP64())
	width := Load(Param("width"), GP64())
	maskBase := Load(Param("masks"), GP64())

	// Broadcast lutScale (2047.0) to YMM
	lutScaleParam, _ := Param("lutScale").Resolve()
	lutScale := YMM()
	VBROADCASTSS(lutScaleParam.Addr, lutScale)

	// Broadcast max LUT index (2047) as int32
	maxIdx := YMM()
	VPBROADCASTD(int32Const(2047), maxIdx)

	// Zero register for VPMAXSD clamping
	zeroInt := YMM()
	VPXOR(zeroInt, zeroInt, zeroInt)

	// Compute loop bounds: widthBytes = width * 16 (each column = [4]float32)
	widthBytes := GP64()
	MOVQ(width, widthBytes)
	SHLQ(Imm(4), widthBytes)

	// Threshold for 2-column (YMM) processing: need at least 32 bytes
	threshold := GP64()
	MOVQ(widthBytes, threshold)
	SUBQ(Imm(16), threshold)

	offset := GP64()
	XORQ(offset, offset)

	PCALIGN(Imm(32))
	Label("gather_loop")
	CMPQ(offset, threshold)
	JG(LabelRef("gather_tail"))

	// Load 8 t values (2 columns × 4 rows)
	t := YMM()
	VMOVUPS(Mem{Base: tBufBase, Index: offset, Scale: 1}, t)

	// Scale to LUT index range: t * lutScale
	VMULPS(lutScale, t, t)

	// Convert to int32 indices (truncate toward zero)
	indices := YMM()
	VCVTTPS2DQ(t, indices)

	// Clamp to [0, 2047]
	VPMAXSD(zeroInt, indices, indices)
	VPMINSD(maxIdx, indices, indices)

	// Convert to byte offsets: indices * 16 (each LUT entry = [4]float32 = 16 bytes)
	// OPT(dh): couldn't we set the appropriate scale in the VGATHERDPS instead?
	VPSLLD(Imm(4), indices, indices)

	// Gather 4 channels from LUT and store to planar buffers.
	// In Go plan9 asm, VGATHERDPS arg1=mask, arg2=mem, arg3=dst.
	storeMask := YMM()
	VMOVUPS(Mem{Base: maskBase, Index: offset, Scale: 1}, storeMask)
	for ch := range 4 {
		gatherMask := YMM()
		VPCMPEQD(gatherMask, gatherMask, gatherMask) // all-ones mask

		result := YMM()
		VGATHERDPS(gatherMask, Mem{Base: lutBase, Disp: ch * 4, Index: indices, Scale: 1}, result)

		VMASKMOVPS(result, storeMask, Mem{Base: dst[ch], Index: offset, Scale: 1})
	}

	ADDQ(Imm(32), offset) // advance 2 columns = 32 bytes
	JMP(LabelRef("gather_loop"))

	Label("gather_tail")
	// Handle remaining column (if width is odd)
	CMPQ(offset, widthBytes)
	JGE(LabelRef("gather_done"))

	// Load 4 t values (1 column)
	tX := XMM()
	VMOVUPS(Mem{Base: tBufBase, Index: offset, Scale: 1}, tX)

	// Scale
	lutScaleX := XMM()
	VBROADCASTSS(lutScaleParam.Addr, lutScaleX)
	VMULPS(lutScaleX, tX, tX)

	// Convert to int32
	idxX := XMM()
	VCVTTPS2DQ(tX, idxX)

	// Clamp
	zeroX := XMM()
	VPXOR(zeroX, zeroX, zeroX)
	maxIdxX := XMM()
	VPBROADCASTD(int32Const(2047), maxIdxX)
	VPMAXSD(zeroX, idxX, idxX)
	VPMINSD(maxIdxX, idxX, idxX)

	// Byte offsets
	VPSLLD(Imm(4), idxX, idxX)

	// Gather 4 channels (XMM = 4 elements)
	storeMask = XMM()
	VMOVUPS(Mem{Base: maskBase, Index: offset, Scale: 1}, storeMask)
	for ch := range 4 {
		maskX := XMM()
		VPCMPEQD(maskX, maskX, maskX)

		resultX := XMM()
		VGATHERDPS(maskX, Mem{Base: lutBase, Disp: ch * 4, Index: idxX, Scale: 1}, resultX)

		VMASKMOVPS(resultX, storeMask, Mem{Base: dst[ch], Index: offset, Scale: 1})
	}

	Label("gather_done")
	VZEROUPPER()
	RET()
}

// gradientCascadeMergeAVX2Impl generates the VPERMPS-based gradient cascade
// merge function. It processes pre-computed t values in a two-pass fashion:
// threshold scan to find range indices, then VPERMPS lookup for scale/bias
// per channel. Handles n <= 8 ranges only (caller must check).
func gradientCascadeMergeAVX2Impl() {
	Implement("gradientCascadeMergeAVX2")

	dst := [4]reg.Register{
		Load(Param("dst0"), GP64()),
		Load(Param("dst1"), GP64()),
		Load(Param("dst2"), GP64()),
		Load(Param("dst3"), GP64()),
	}
	tBufBase := Load(Param("tBuf"), GP64())
	srBase := Load(Param("sr"), GP64())
	width := Load(Param("width"), GP64())
	maskBase := Load(Param("masks"), GP64())

	// simdGradientRanges struct field offsets (amd64):
	//   n      int              @ 0   (8 bytes)
	//   x1     [4]float32      @ 8   (64 bytes)
	//   scaleR [4]float32      @ 24
	//   scaleG [4]float32      @ 40
	//   scaleB [4]float32      @ 56
	//   scaleA [4]float32      @ 72
	//   biasR  [4]float32      @ 88
	//   biasG  [4]float32      @ 104
	//   biasB  [4]float32      @ 120
	//   biasA  [4]float32      @ 136
	const (
		offN      = 0
		offX1     = 8
		offScaleR = 24
		offScaleG = 40
		offScaleB = 56
		offScaleA = 72
		offBiasR  = 88
		offBiasG  = 104
		offBiasB  = 120
		offBiasA  = 136
	)
	scaleOff := [4]int{offScaleR, offScaleG, offScaleB, offScaleA}
	biasOff := [4]int{offBiasR, offBiasG, offBiasB, offBiasA}

	// Load n-1 (number of threshold scan iterations)
	nMinus1 := GP64()
	MOVQ(Mem{Base: srBase, Disp: offN}, nMinus1)
	DECQ(nMinus1)

	// Compute loop bounds: widthBytes = width * 16
	widthBytes := GP64()
	MOVQ(width, widthBytes)
	SHLQ(Imm(4), widthBytes)

	// YMM threshold: process 2 columns (32 bytes) at a time.
	// We can always safely process 2 columns even if width is odd,
	// because the buffers are wideTileWidth (256) elements.
	ymmThreshold := GP64()
	MOVQ(widthBytes, ymmThreshold)
	SUBQ(Imm(16), ymmThreshold)

	oneConst := floatConst(1.0)

	offset := GP64()
	XORQ(offset, offset)

	// Pre-load the 8 scale/bias values for each channel into YMM registers.
	// With 4 channels × 2 (scale+bias) = 8 YMM registers, plus idx, t, one,
	// scratch = 12 total. Well within the 16 YMM budget.
	scaleRegs := [4]reg.VecVirtual{YMM(), YMM(), YMM(), YMM()}
	biasRegs := [4]reg.VecVirtual{YMM(), YMM(), YMM(), YMM()}
	for ch := range 4 {
		VMOVUPS(Mem{Base: srBase, Disp: scaleOff[ch]}, scaleRegs[ch])
		VMOVUPS(Mem{Base: srBase, Disp: biasOff[ch]}, biasRegs[ch])
	}

	PCALIGN(Imm(32))
	Label("cascade_loop")
	CMPQ(offset, ymmThreshold)
	JG(LabelRef("cascade_tail"))

	// Load 8 t values (2 columns)
	t := YMM()
	VMOVUPS(Mem{Base: tBufBase, Index: offset, Scale: 1}, t)

	// Threshold scan: count how many x1 thresholds each t value exceeds.
	one := YMM()
	VBROADCASTSS(oneConst, one)
	idx := YMM()
	VXORPS(idx, idx, idx)

	threshIdx := GP64()
	XORQ(threshIdx, threshIdx)

	Label("thresh_loop")
	CMPQ(threshIdx, nMinus1)
	JGE(LabelRef("thresh_done"))

	thresh := YMM()
	VBROADCASTSS(Mem{Base: srBase, Disp: offX1, Index: threshIdx, Scale: 4}, thresh)
	cmpResult := YMM()
	VCMPPS(Imm(0x0D), thresh, t, cmpResult) // cmpResult = (t >= thresh)
	masked := YMM()
	VANDPS(cmpResult, one, masked) // 1.0 where true
	VADDPS(masked, idx, idx)

	INCQ(threshIdx)
	JMP(LabelRef("thresh_loop"))

	Label("thresh_done")
	// Convert float indices to int32 for VPERMPS
	VCVTTPS2DQ(idx, idx)

	// VPERMPS lookup for all 4 channels: scale[idx] * t + bias[idx]
	storeMask := YMM()
	VMOVUPS(Mem{Base: maskBase, Index: offset, Scale: 1}, storeMask)
	for ch := range 4 {
		s := YMM()
		VPERMPS(scaleRegs[ch], idx, s)
		b := YMM()
		VPERMPS(biasRegs[ch], idx, b)
		VFMADD132PS(t, b, s) // s = s * t + b
		VMASKMOVPS(s, storeMask, Mem{Base: dst[ch], Index: offset, Scale: 1})
	}

	ADDQ(Imm(32), offset)
	JMP(LabelRef("cascade_loop"))

	// Tail: 1 remaining column (4 t values in low XMM, zero-extended to YMM)
	Label("cascade_tail")
	CMPQ(offset, widthBytes)
	JGE(LabelRef("cascade_done"))

	tTail := YMM()
	// Load 4 floats into XMM (zero-extends to YMM automatically)
	VMOVUPS(Mem{Base: tBufBase, Index: offset, Scale: 1}, tTail.AsX())

	// Threshold scan (same logic, using YMM with zeros in high half)
	oneTail := YMM()
	VBROADCASTSS(oneConst, oneTail)
	idxTail := YMM()
	VXORPS(idxTail, idxTail, idxTail)

	threshIdxTail := GP64()
	XORQ(threshIdxTail, threshIdxTail)

	Label("thresh_loop_tail")
	CMPQ(threshIdxTail, nMinus1)
	JGE(LabelRef("thresh_done_tail"))

	threshTail := YMM()
	VBROADCASTSS(Mem{Base: srBase, Disp: offX1, Index: threshIdxTail, Scale: 4}, threshTail)
	cmpTail := YMM()
	VCMPPS(Imm(0x0D), threshTail, tTail, cmpTail)
	maskedTail := YMM()
	VANDPS(cmpTail, oneTail, maskedTail)
	VADDPS(maskedTail, idxTail, idxTail)

	INCQ(threshIdxTail)
	JMP(LabelRef("thresh_loop_tail"))

	Label("thresh_done_tail")
	VCVTTPS2DQ(idxTail, idxTail)

	storeMask = XMM()
	VMOVUPS(Mem{Base: maskBase, Index: offset, Scale: 1}, storeMask)
	for ch := range 4 {
		s := YMM()
		VPERMPS(scaleRegs[ch], idxTail, s)
		b := YMM()
		VPERMPS(biasRegs[ch], idxTail, b)
		VFMADD132PS(tTail, b, s)
		// Store only the low XMM (4 values = 1 column)
		VMASKMOVPS(s.AsX(), storeMask, Mem{Base: dst[ch], Index: offset, Scale: 1})
	}

	Label("cascade_done")
	VZEROUPPER()
	RET()
}

func transpose8x4(c01, c23, c45, c67, permMask reg.VecVirtual) (r0, r1, r2, r3 reg.VecVirtual) {
	// Each element is a [4]byte, i.e. a double word.
	//
	// c01 = [c0r0, c0r1, c0r2, c0r3 | c1r0, c1r1, c1r2, c1r3]
	// c23 = [c2r0, c2r1, c2r2, c2r3 | c3r0, c3r1, c3r2, c3r3]
	// c45 = [c4r0, c4r1, c4r2, c4r3 | c5r0, c5r1, c5r2, c5r3]
	// c67 = [c6r0, c6r1, c6r2, c6r3 | c7r0, c7r1, c7r2, c7r3]

	// Step 1: 32-bit interleave
	t0, t1, t2, t3 := YMM(), YMM(), YMM(), YMM()
	VPUNPCKLDQ(c23, c01, t0) // t0 = [c0r0, c2r0, c0r1, c2r1 | c1r0, c3r0, c1r1, c3r1]
	VPUNPCKHDQ(c23, c01, t1) // t1 = [c0r2, c2r2, c0r3, c2r3 | c1r2, c3r2, c1r3, c3r3]
	VPUNPCKLDQ(c67, c45, t2) // t2 = [c4r0, c6r0, c4r1, c6r1 | c5r0, c7r0, c5r1, c7r1]
	VPUNPCKHDQ(c67, c45, t3) // t3 = [c4r2, c6r2, c4r3, c6r3 | c5r2, c7r2, c5r3, c7r3]

	// Step 2: 64-bit interleave
	u0, u1, u2, u3 := YMM(), YMM(), YMM(), YMM()
	VPUNPCKLQDQ(t2, t0, u0) // u0 = [c0r0, c2r0, c4r0, c6r0 | c1r0, c3r0, c5r0, c7r0]
	VPUNPCKHQDQ(t2, t0, u1) // u1 = [c0r1, c2r1, c4r1, c6r1 | c1r1, c3r1, c5r1, c7r1]
	VPUNPCKLQDQ(t3, t1, u2) // u2 = [c0r2, c2r2, c4r2, c6r2 | c1r2, c3r2, c5r2, c7r2]
	VPUNPCKHQDQ(t3, t1, u3) // u3 = [c0r3, c2r3, c4r3, c6r3 | c1r3, c3r3, c5r3, c7r3]

	// Step 3: Fix lane ordering with vpermd
	VPERMD(u0, permMask, u0) // u0 = [c0r0, c1r0, c2r0, c3r0 | c3r0, c5r0, c6r0, c7r0]
	VPERMD(u1, permMask, u1) // u1 = [c0r1, c1r1, c2r1, c3r1 | c3r1, c5r1, c6r1, c7r1]
	VPERMD(u2, permMask, u2) // u2 = [c0r2, c1r2, c2r2, c3r2 | c3r2, c5r2, c6r2, c7r2]
	VPERMD(u3, permMask, u3) // u3 = [c0r3, c1r3, c2r3, c3r3 | c3r3, c5r3, c6r3, c7r3]

	return u0, u1, u2, u3
}
