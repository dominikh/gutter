// SPDX-FileCopyrightText: 2025 Dominik Honnef and contributors
//
// SPDX-License-Identifier: MIT

package sparse

import (
	"math"

	"golang.org/x/exp/constraints"
)

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

func satConvI32(x float32) int32 {
	if x != x {
		return 0
	} else if x < math.MinInt32 {
		return math.MinInt32
	} else if x > math.MaxInt32 {
		return math.MaxInt32
	} else {
		return int32(x)
	}
}

func divCeil[T int | uint16 | uint32](a, b T) T {
	return (a + b - 1) / b
}
