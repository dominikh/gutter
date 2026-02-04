// SPDX-FileCopyrightText: 2026 Dominik Honnef and contributors
//
// SPDX-License-Identifier: MIT

package sparse

import (
	. "simd/archsimd"
	"unsafe"
)

func memsetUint8PixelsAVX(b [][4]byte, v [4]byte) {
	vf8 := Float32x4{}.SetElem(0, *(*float32)(unsafe.Pointer(&v))).Broadcast1To8()

	var i int
	if len(b) >= 32 {
		ptr := unsafe.Pointer(unsafe.SliceData(b))
		for {
			vf8.Store((*[8]float32)(ptr))
			vf8.Store((*[8]float32)(unsafe.Add(ptr, 1*8*4)))
			vf8.Store((*[8]float32)(unsafe.Add(ptr, 2*8*4)))
			vf8.Store((*[8]float32)(unsafe.Add(ptr, 3*8*4)))
			if i >= len(b)-31 {
				break
			}
			i += 32
			ptr = unsafe.Add(ptr, 4*8*4)
		}
	}
	if i >= 0 { // To prove that i isn't negative
		for ; i < len(b); i++ {
			b[i] = v
		}
	}
	ClearAVXUpperBits()
}

func memsetUint8Pixels(b [][4]byte, v [4]byte) {
	if X86.AVX() {
		memsetUint8PixelsAVX(b, v)
	} else {
		for i := range b {
			b[i] = v
		}
	}
}
