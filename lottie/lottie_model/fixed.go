// SPDX-FileCopyrightText: 2024 the Velato Authors
// SPDX-FileCopyrightText: 2024 Dominik Honnef and contributors
//
// SPDX-License-Identifier: MIT

package lottie_model

import (
	"fmt"
	"math"

	"honnef.co/go/curve"
	"honnef.co/go/jello/gfx"
)

type FixedRepeater struct {
	Copies       int
	Offset       float64
	AnchorPoint  curve.Point
	Position     curve.Point
	Rotation     float64
	Scale        curve.Vec2
	StartOpacity float64
	EndOpacity   float64
}

func toRadians(deg float64) float64 {
	return deg * (math.Pi / 180)
}

func (r *FixedRepeater) Transform(index int) curve.Affine {
	t := r.Offset + float64(index)
	return curve.Translate(curve.Vec(
		t*r.Position.X+r.AnchorPoint.X,
		t*r.Position.Y+r.AnchorPoint.Y,
	)).
		Mul(curve.Rotate(toRadians(t * r.Rotation))).
		Mul(curve.Scale(
			math.Pow(r.Scale.X/100.0, t),
			math.Pow(r.Scale.Y/100.0, t),
		)).
		Mul(curve.Translate(curve.Vec(-r.AnchorPoint.X, -r.AnchorPoint.Y)))
}

func BrushWithAlpha(brush gfx.Brush, alpha float64) gfx.Brush {
	if alpha == 1 {
		return brush
	}

	switch brush := brush.(type) {
	case gfx.SolidBrush:
		c := brush.Color
		c.Values[3] = alpha
		return gfx.SolidBrush{
			Color: c,
		}
	case gfx.GradientBrush:
		switch grad := brush.Gradient.(type) {
		case gfx.LinearGradient:
			stops := make([]gfx.ColorStop, len(grad.Stops))
			for i, stop := range grad.Stops {
				stops[i] = stop.WithAlphaFactor(float32(alpha))
			}
			grad.Stops = stops
			brush.Gradient = grad
			return brush
		case gfx.RadialGradient:
			stops := make([]gfx.ColorStop, len(grad.Stops))
			for i, stop := range grad.Stops {
				stops[i] = stop.WithAlphaFactor(float32(alpha))
			}
			grad.Stops = stops
			brush.Gradient = grad
			return brush
		case gfx.SweepGradient:
			stops := make([]gfx.ColorStop, len(grad.Stops))
			for i, stop := range grad.Stops {
				stops[i] = stop.WithAlphaFactor(float32(alpha))
			}
			grad.Stops = stops
			brush.Gradient = grad
			return brush
		default:
			panic(fmt.Sprintf("internal error: unhandled type %T", grad))
		}
	default:
		panic(fmt.Sprintf("internal error: unhandled type %T", brush))
	}
}
