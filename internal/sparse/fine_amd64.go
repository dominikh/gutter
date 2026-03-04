// SPDX-FileCopyrightText: 2025 Dominik Honnef and contributors
//
// SPDX-License-Identifier: MIT

//go:build goexperiment.simd

package sparse

import (
	. "simd/archsimd"

	"honnef.co/go/gutter/gfx"
	"honnef.co/go/safeish"
)

func fineFillComplexAVX(buf [][stripHeight]gfx.PlainColor, color gfx.PlainColor) {
	// OPT(dh): ideally, this would compile to a single VBROADCASTF128
	color4 := LoadFloat32x4((*[4]float32)(&color))
	var colorx2 Float32x8
	colorx2 = colorx2.SetLo(color4)
	colorx2 = colorx2.SetHi(color4)

	alpha := color[3]
	// TODO(dh): with VSHUFPS and VINSERTF128 we could implement the broadcast
	// without needing AVX2.
	oneMinusAlpha := BroadcastFloat32x8(1 - alpha)

	out := safeish.SliceCast[[]float32](buf)
	for i := 0; i < len(out)-15; i += 16 {
		bg := LoadFloat32x8Slice(out[i : i+8])
		bg = bg.Mul(oneMinusAlpha)
		bg = bg.Add(colorx2)
		bg.StoreSlice(out[i:])

		// OPT(dh): this introduces a bounds check
		bg = LoadFloat32x8Slice(out[i+8 : i+16])
		bg = bg.Mul(oneMinusAlpha)
		bg = bg.Add(colorx2)
		bg.StoreSlice(out[i+8 : i+16])
	}

	ClearAVXUpperBits()
}
