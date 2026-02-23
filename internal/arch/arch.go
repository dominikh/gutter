// SPDX-FileCopyrightText: 2026 Dominik Honnef and contributors
//
// SPDX-License-Identifier: MIT

package arch

import (
	"runtime"
	"simd/archsimd"
)

func AVX() bool {
	return GOAMD64 >= 3 || (runtime.GOARCH == "amd64" && archsimd.X86.AVX())
}

func AVX2() bool {
	return GOAMD64 >= 3 || (runtime.GOARCH == "amd64" && archsimd.X86.AVX2())
}

func FMA() bool {
	return GOAMD64 >= 3 || (runtime.GOARCH == "amd64" && archsimd.X86.FMA())
}
