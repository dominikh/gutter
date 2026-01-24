// SPDX-FileCopyrightText: 2025 Dominik Honnef and contributors
//
// SPDX-License-Identifier: MIT

package sparse

import (
	"math"

	"golang.org/x/exp/constraints"
	"honnef.co/go/curve"
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

func hypot32(p, q float32) float32 {
	// TODO(dh): move this function to the math32 package in 'stuff'
	return float32(math.Hypot(float64(p), float64(q)))
}

func exp32(x float32) float32 {
	// TODO(dh): move this function to the math32 package in 'stuff'
	return float32(math.Exp(float64(x)))
}

func pow2(d float32) float32 { return d * d }

func xyAdvances(transform curve.Affine) (curve.Vec2, curve.Vec2) {
	c := transform.Coefficients()
	scaleSkewTransform := curve.NewAffine([6]float64{c[0], c[1], c[2], c[3], 0, 0})
	xAdvance := curve.Pt(1.0, 0.0).Transform(scaleSkewTransform)
	yAdvance := curve.Pt(0.0, 1.0).Transform(scaleSkewTransform)
	return curve.Vec2(xAdvance), curve.Vec2(yAdvance)
}
