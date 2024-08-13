// SPDX-FileCopyrightText: 2024 the Velato Authors
// SPDX-FileCopyrightText: 2024 Dominik Honnef and contributors
//
// SPDX-License-Identifier: MIT

package lottie_model

import (
	"math"

	"honnef.co/go/color"
	"honnef.co/go/curve"
	"honnef.co/go/gutter/animation"
	"honnef.co/go/jello/gfx"
)

type BrushKind int

const (
	BrushKindSolid BrushKind = iota + 1
	BrushKindGradient
)

type Brush struct {
	Kind     BrushKind
	Solid    [3]animation.Keyframes[float64]
	Gradient animation.Gradient
}

func (b Brush) Evaluate(alpha, frame float64) gfx.Brush {
	switch b.Kind {
	case BrushKindSolid:
		return gfx.SolidBrush{
			Color: color.Make(
				color.SRGB,
				b.Solid[0].Evaluate(frame),
				b.Solid[1].Evaluate(frame),
				b.Solid[2].Evaluate(frame),
				alpha,
			),
		}
	case BrushKindGradient:
		return b.Gradient.Evaluate(frame)
	default:
		return nil
	}
}

type Star struct {
	IsPolygon      bool
	Direction      float64
	Position       animation.Point
	InnerRadius    animation.Keyframes[float64]
	InnerRoundness animation.Keyframes[float64]
	OuterRadius    animation.Keyframes[float64]
	OuterRoundness animation.Keyframes[float64]
	Rotation       animation.Keyframes[float64]
	Points         animation.Keyframes[float64]
}

type Spline struct {
	IsClosed bool
	animation.Keyframes[[]curve.Point]
}

func ToPath(points []curve.Point, isClosed bool, path curve.BezPath) (curve.BezPath, bool) {
	n := len(points)
	if n == 0 {
		return path, true
	}

	path.Push(curve.MoveTo(points[0]))
	n_vertices := n / 3
	add_element := func(from_vertex, to_vertex int) {
		from_index := 3 * from_vertex
		to_index := 3 * to_vertex
		p0 := points[from_index]
		p1 := points[to_index]
		c0 := points[from_index+2]
		c0.X += p0.X
		c0.Y += p0.Y
		c1 := points[to_index+1]
		c1.X += p1.X
		c1.Y += p1.Y
		if c0 == p0 && c1 == p1 {
			path.Push(curve.LineTo(p1))
		} else {
			path.Push(curve.CubicTo(c0, c1, p1))
		}
	}
	for i := 1; i < n_vertices; i++ {
		add_element(i-1, i)
	}
	if isClosed && n_vertices != 0 {
		add_element(n_vertices-1, 0)
		path.Push(curve.ClosePath())
	}

	return path, true
}

func ToPath3(from, to []curve.Point, t float64, isClosed bool, path curve.BezPath) (curve.BezPath, bool) {
	n := min(len(from), len(to))
	get := func(idx int) curve.Point {
		return from[idx].Lerp(to[idx], t)
	}
	if n == 0 {
		return path, true
	}

	path.Push(curve.MoveTo(get(0)))
	n_vertices := n / 3
	add_element := func(from_vertex, to_vertex int) {
		from_index := 3 * from_vertex
		to_index := 3 * to_vertex
		p0 := get(from_index)
		p1 := get(to_index)
		c0 := get(from_index + 2)
		c0.X += p0.X
		c0.Y += p0.Y
		c1 := get(to_index + 1)
		c1.X += p1.X
		c1.Y += p1.Y
		if c0 == p0 && c1 == p1 {
			path.Push(curve.LineTo(p1))
		} else {
			path.Push(curve.CubicTo(c0, c1, p1))
		}
	}
	for i := 1; i < n_vertices; i++ {
		add_element(i-1, i)
	}
	if isClosed && n_vertices != 0 {
		add_element(n_vertices-1, 0)
		path.Push(curve.ClosePath())
	}

	return path, true
}

func (s Spline) Evaluate(frame float64, path curve.BezPath) (curve.BezPath, bool) {
	from, to, t, ok := s.ComputeFramesAndWeight(frame)
	if !ok {
		return path, false
	}
	return ToPath3(from, to, t, s.IsClosed, path)
}

type Repeater struct {
	Copies       animation.Keyframes[float64]
	Offset       animation.Keyframes[float64]
	AnchorPoint  animation.Point
	Position     animation.Point
	Rotation     animation.Keyframes[float64]
	Scale        animation.Vec2
	StartOpacity animation.Keyframes[float64]
	EndOpacity   animation.Keyframes[float64]
}

func (r Repeater) Evaluate(frame float64) FixedRepeater {
	copies := r.Copies.Evaluate(frame)
	offset := r.Offset.Evaluate(frame)
	anchorPoint := r.AnchorPoint.Evaluate(frame)
	position := r.Position.Evaluate(frame)
	rotation := r.Rotation.Evaluate(frame)
	scale := r.Scale.Evaluate(frame)
	startOpacity := r.StartOpacity.Evaluate(frame)
	endOpacity := r.EndOpacity.Evaluate(frame)
	return FixedRepeater{
		Copies:       int(math.Round(copies)),
		Offset:       offset,
		AnchorPoint:  anchorPoint,
		Position:     position,
		Rotation:     rotation,
		Scale:        scale,
		StartOpacity: startOpacity,
		EndOpacity:   endOpacity,
	}
}
