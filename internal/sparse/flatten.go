// SPDX-FileCopyrightText: 2024 the Piet Authors
// SPDX-FileCopyrightText: 2025 Dominik Honnef and contributors
//
// SPDX-License-Identifier: MIT

package sparse

import (
	"fmt"
	"iter"
	"math"

	"honnef.co/go/curve"
	"honnef.co/go/stuff/syncutil"
)

const flattenTolerance = 0.25

type flatLine struct {
	p0 vec2
	p1 vec2
}

type vec2 struct {
	x, y float32
}

func (v vec2) String() string {
	return fmt.Sprintf("(%g, %g)", v.x, v.y)
}

var lineBufPool = syncutil.NewPool(func() []flatLine { return nil })

func fill(path iter.Seq[curve.PathElement], affine curve.Affine) []flatLine {
	lineBuf := lineBufPool.Get()[:0]

	var start, p0 curve.Point
	iter := func(yield func(el curve.PathElement) bool) {
		for el := range path {
			if !yield(el.Transform(affine)) {
				return
			}
		}
	}
	closePath := func(start, p0 curve.Point) {
		pt0 := vec2{float32(p0.X), float32(p0.Y)}
		pt1 := vec2{float32(start.X), float32(start.Y)}
		if pt0 != pt1 {
			lineBuf = append(lineBuf, flatLine{pt0, pt1})
		}
	}

	closed := true
	for el := range curve.Flatten(iter, flattenTolerance) {
		switch el.Kind {
		case curve.MoveToKind:
			if !closed && p0 != start {
				closePath(start, p0)
			}
			closed = false
			start = el.P0
			p0 = el.P0
		case curve.LineToKind:
			p := el.P0
			pt0 := vec2{float32(p0.X), float32(p0.Y)}
			pt1 := vec2{float32(p.X), float32(p.Y)}
			lineBuf = append(lineBuf, flatLine{pt0, pt1})
			p0 = p
		case curve.ClosePathKind:
			closed = true
			closePath(start, p0)

		default:
			panic(fmt.Sprintf("unreachable: %v", el.Kind))
		}
	}

	if !closed {
		closePath(start, p0)
	}

	return lineBuf
}

func stroke(path iter.Seq[curve.PathElement], style curve.Stroke, affine curve.Affine) []flatLine {
	iter := func(yield func(el curve.PathElement) bool) {
		for el := range path {
			if !yield(el.Transform(affine)) {
				return
			}
		}
	}

	// Scale stroke width based on affine transformation. This transforms a
	// circle with radius style.Width, then computes the geometric mean of the
	// resulting ellipse's semi-major and semi-minor axes (its two "radii").
	//
	// This matches the behavior of Skia's SkMatrix::mapRadius.
	noTranslation := affine
	noTranslation.N4 = 0
	noTranslation.N5 = 0
	l0 := curve.Vec2(curve.Pt(0, style.Width).Transform(noTranslation)).Hypot()
	l1 := curve.Vec2(curve.Pt(style.Width, 0).Transform(noTranslation)).Hypot()
	style.Width = math.Sqrt(l0 * l1)

	stroked := curve.StrokePath(iter, style, curve.StrokeOpts{}, flattenTolerance)
	return fill(stroked, curve.Identity)
}
