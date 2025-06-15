// SPDX-FileCopyrightText: 2024 the Color Authors
// SPDX-FileCopyrightText: 2025 Dominik Honnef and contributors
//
// SPDX-License-Identifier: MIT

// TODO(dh): consider moving gradient approximation into the color module. At
// that point, copy in linebender/color's documentation of the functions.

package gfx

import (
	"honnef.co/go/color"
)

type ColorInterpolator struct {
	apm [4]float64
	Δpm [4]float64
	cs  *color.Space
}

func Interpolate(c1, c2 color.Color, cs *color.Space) *ColorInterpolator {
	// TODO(dh): support cylindrical color spaces

	a := c1.Convert(cs)
	b := c2.Convert(cs)

	apm := [4]float64{
		a.Values[0] * a.Values[3],
		a.Values[1] * a.Values[3],
		a.Values[2] * a.Values[3],
		a.Values[3],
	}
	bpm := [4]float64{
		b.Values[0] * b.Values[3],
		b.Values[1] * b.Values[3],
		b.Values[2] * b.Values[3],
		b.Values[3],
	}

	Δpm := [4]float64{
		bpm[0] - apm[0],
		bpm[1] - apm[1],
		bpm[2] - apm[2],
		bpm[3] - apm[3],
	}

	return &ColorInterpolator{apm, Δpm, cs}
}

func (ci *ColorInterpolator) Evaluate(t float64) color.Color {
	// TODO(dh): support cylindrical color spaces

	pm := [4]float64{
		ci.apm[0] + t*ci.Δpm[0],
		ci.apm[1] + t*ci.Δpm[1],
		ci.apm[2] + t*ci.Δpm[2],
		ci.apm[3] + t*ci.Δpm[3],
	}
	if pm[3] == 0 || pm[3] == 1 {
		return color.Color{
			Values: pm,
			Space:  ci.cs,
		}
	} else {
		straight := [4]float64{
			pm[0] / pm[3],
			pm[1] / pm[3],
			pm[2] / pm[3],
			pm[3],
		}
		return color.Color{
			Values: straight,
			Space:  ci.cs,
		}
	}
}
