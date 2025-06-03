// SPDX-FileCopyrightText: 2024 the Velato Authors
// SPDX-FileCopyrightText: 2024 Dominik Honnef and contributors
//
// SPDX-License-Identifier: MIT

package lottie_model

import (
	"fmt"
	"math"
	"slices"

	"honnef.co/go/curve"
	"honnef.co/go/gutter/gfx"
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

func BrushWithAlpha(brush gfx.Paint, alpha float64) gfx.Paint {
	if alpha == 1 {
		return brush
	}

	switch brush := brush.(type) {
	case gfx.Solid:
		brush.Values[3] = alpha
		return brush
	case *gfx.LinearGradient:
		grad := *brush
		grad.Stops = slices.Clone(grad.Stops)
		for i := range grad.Stops {
			grad.Stops[i].Color.Values[3] = alpha
		}
		return &grad
	case *gfx.RadialGradient:
		grad := *brush
		grad.Stops = slices.Clone(grad.Stops)
		for i := range brush.Stops {
			grad.Stops[i].Color.Values[3] = alpha
		}
		return &grad
	case *gfx.SweepGradient:
		grad := *brush
		grad.Stops = slices.Clone(grad.Stops)
		for i := range brush.Stops {
			grad.Stops[i].Color.Values[3] = alpha
		}
		return &grad
	default:
		panic(fmt.Sprintf("internal error: unhandled type %T", brush))
	}
}
