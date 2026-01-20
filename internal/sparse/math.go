// SPDX-FileCopyrightText: 2025 Dominik Honnef and contributors
//
// SPDX-License-Identifier: MIT

package sparse

import (
	"math"

	"golang.org/x/exp/constraints"
	"honnef.co/go/stuff/math/math32"
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

func divCeil[T int | uint16 | uint32](a, b T) T {
	return T((uint64(a) + uint64(b) - 1) / uint64(b))
}

func pow32(x, y float32) float32 {
	// TODO move to math32
	return float32(math.Pow(float64(x), float64(y)))
}

// erf7 approximates the erf function.
//
// For details on how it works, see
// https://raphlinus.github.io/audio/2018/09/05/sigmoid.html and
// https://raphlinus.github.io/graphics/2020/04/21/blurred-rounded-rects.html.
func erf7(x float32) float32 {
	x = x * (2 / math.SqrtPi)
	xx := x * x
	x = x + (0.24295+(0.03395+0.0104*xx)*xx)*(x*xx)
	return x / math32.Sqrt(1.0+x*x)
}
