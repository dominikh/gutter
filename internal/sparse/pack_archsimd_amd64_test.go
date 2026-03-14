// SPDX-FileCopyrightText: 2026 Dominik Honnef and contributors
//
// SPDX-License-Identifier: MIT

//go:build goexperiment.simd

package sparse

import (
	"testing"

	"honnef.co/go/gutter/internal/arch"
)

func Test_memsetUint8PixelsAVX(t *testing.T) {
	t.Parallel()

	if !arch.AVX() {
		t.Skip("no AVX support")
	}

	for _, sz := range []int{256, 192, 32, 63, 5} {
		b := make([][4]byte, sz)
		p := [4]byte{1, 2, 3, 4}
		memsetUint8PixelsAVX(b, p)
		for i, v := range b {
			if v != p {
				t.Fatalf("size %d: b[%d] = %v, want %v", sz, i, v, p)
			}
		}
	}
}
