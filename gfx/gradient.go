// SPDX-FileCopyrightText: 2025 Dominik Honnef and contributors
//
// SPDX-License-Identifier: MIT

package gfx

import (
	"honnef.co/go/color"
	"honnef.co/go/curve"
)

// TODO(dh): Allow specifying whether gradients should be interpolated with
// straight or premultiplied alpha.

//go:generate go tool stringer -type=GradientExtend -trimprefix=GradientExtend
type GradientExtend int

const (
	GradientExtendPad GradientExtend = iota
	GradientExtendRepeat
	GradientExtendReflect
)

type GradientStop struct {
	Offset float32
	Color  color.Color
}

type Gradient interface {
	Paint
}

var _ Gradient = (*LinearGradient)(nil)
var _ Gradient = (*RadialGradient)(nil)
var _ Gradient = (*SweepGradient)(nil)

func (*LinearGradient) isPaint() {}
func (*RadialGradient) isPaint() {}
func (*SweepGradient) isPaint()  {}

type LinearGradient struct {
	Stops      []GradientStop
	Extend     GradientExtend
	Start      curve.Point
	End        curve.Point
	ColorSpace *color.Space
}

type RadialGradient struct {
	Stops       []GradientStop
	Extend      GradientExtend
	StartCenter curve.Point
	StartRadius float32
	EndCenter   curve.Point
	EndRadius   float32
	ColorSpace  *color.Space
}

type SweepGradient struct {
	Stops  []GradientStop
	Extend GradientExtend
	Center curve.Point

	// The start and end angles, in radian.
	StartAngle float32
	EndAngle   float32

	ColorSpace *color.Space
}
