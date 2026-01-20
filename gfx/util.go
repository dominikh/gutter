package gfx

import (
	"math"

	"honnef.co/go/curve"
)

func xyAdvances(transform curve.Affine) (curve.Vec2, curve.Vec2) {
	c := transform.Coefficients()
	scaleSkewTransform := curve.NewAffine([6]float64{c[0], c[1], c[2], c[3], 0, 0})
	xAdvance := curve.Pt(1.0, 0.0).Transform(scaleSkewTransform)
	yAdvance := curve.Pt(0.0, 1.0).Transform(scaleSkewTransform)
	return curve.Vec2(xAdvance), curve.Vec2(yAdvance)
}

func hypot32(p, q float32) float32 {
	// TODO(dh): move this function to the math32 package in 'stuff'
	return float32(math.Hypot(float64(p), float64(q)))
}

func pow2(d float32) float32 { return d * d }

func exp32(x float32) float32 {
	// TODO(dh): move this function to the math32 package in 'stuff'
	return float32(math.Exp(float64(x)))
}
