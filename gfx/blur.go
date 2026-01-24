// SPDX-FileCopyrightText: 2025 the Vello Authors
// SPDX-FileCopyrightText: 2026 Dominik Honnef and contributors
//
// SPDX-License-Identifier: MIT

package gfx

import (
	"honnef.co/go/color"
	"honnef.co/go/curve"
)

var _ Paint = (*BlurredRoundedRectangle)(nil)

type BlurredRoundedRectangle struct {
	// The base rectangle to use for the blur effect.
	Rect curve.Rect
	// The color of the blurred rectangle.
	Color color.Color
	// The radius of the blur effect.
	Radius float32
	// The standard deviation of the blur effect.
	StdDev float32
	// Whether to opt into further optimizations.
	LowPrecision bool
}

func (*BlurredRoundedRectangle) isPaint() {}
