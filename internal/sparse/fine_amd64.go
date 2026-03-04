// SPDX-FileCopyrightText: 2025 Dominik Honnef and contributors
//
// SPDX-License-Identifier: MIT

//go:build goexperiment.simd

package sparse

import (
	. "simd/archsimd"
	"unsafe"

	"honnef.co/go/gutter/gfx"
)

func fineFillComplexAVX(buf Pixels, color gfx.PlainColor) {
	oneMinusAlpha := BroadcastFloat32x8(1 - color[3])

	for ch := range 4 {
		c := BroadcastFloat32x8(color[ch])
		plane := buf[ch]
		// Each column is [stripHeight]float32 = [4]float32 = 16 bytes.
		// Two adjacent columns = 32 bytes = one YMM register.
		for i := 0; i < len(plane)-1; i += 2 {
			ptr := (*[8]float32)(unsafe.Pointer(&plane[i]))
			bg := LoadFloat32x8(ptr)
			bg = bg.Mul(oneMinusAlpha)
			bg = bg.Add(c)
			bg.Store(ptr)
		}
		// Handle odd trailing column.
		if len(plane)%2 != 0 {
			col := &plane[len(plane)-1]
			bg := LoadFloat32x4((*[4]float32)(col))
			bg = bg.Mul(oneMinusAlpha.GetLo())
			bg = bg.Add(c.GetLo())
			bg.Store((*[4]float32)(col))
		}
	}

	ClearAVXUpperBits()
}

func memsetColumnsAVX(buf Pixels, c gfx.PlainColor) {
	for ch := range 4 {
		v := BroadcastFloat32x8(c[ch])
		plane := buf[ch]
		for i := 0; i < len(plane)-1; i += 2 {
			v.Store((*[8]float32)(unsafe.Pointer(&plane[i])))
		}
		if len(plane)%2 != 0 {
			v.GetLo().Store((*[4]float32)(&plane[len(plane)-1]))
		}
	}

	ClearAVXUpperBits()
}
