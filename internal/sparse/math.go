// SPDX-FileCopyrightText: 2025 Dominik Honnef and contributors
//
// SPDX-License-Identifier: MIT

package sparse

import (
	"math"

	"golang.org/x/exp/constraints"
)

func floor32(f float32) float32 {
	return float32(math.Floor(float64(f)))
}

func ceil32(f float32) float32 {
	return float32(math.Ceil(float64(f)))
}

func abs32(f float32) float32 {
	return math.Float32frombits(math.Float32bits(f) &^ (1 << 31))
}

func sqrt32(f float32) float32 {
	return float32(math.Sqrt(float64(f)))
}

func pow32(f float32, exp float32) float32 {
	return float32(math.Pow(float64(f), float64(exp)))
}

func sign32(f float32) float32 {
	if math.Float32bits(f)&(1<<31) != 0 {
		// f is -0.0 or negative
		return -1
	} else {
		return 1
	}
}

func satConv[D constraints.Unsigned, S ~float32 | ~float64](x S) D {
	max := ^D(0)
	if x != x || x < 0 {
		return 0
	} else if x > S(max) {
		return max
	} else {
		return D(x)
	}
}

func divCeil[T int | uint16 | uint32](a, b T) T {
	return T((uint64(a) + uint64(b) - 1) / uint64(b))
}
