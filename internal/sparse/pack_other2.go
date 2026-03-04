// SPDX-FileCopyrightText: 2026 Dominik Honnef and contributors
//
// SPDX-License-Identifier: MIT

//go:build !amd64 || !goexperiment.simd

package sparse

func memsetUint8Pixels(b [][4]byte, v [4]byte) {
	for i := range b {
		b[i] = v
	}
}
