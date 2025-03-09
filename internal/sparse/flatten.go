// SPDX-FileCopyrightText: 2024 the Piet Authors
// SPDX-FileCopyrightText: 2025 Dominik Honnef and contributors
//
// SPDX-License-Identifier: MIT

package sparse

import (
	"fmt"
	"iter"
	"slices"

	"honnef.co/go/curve"
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

func (v vec2) add(o vec2) vec2 {
	return vec2{
		x: v.x + o.x,
		y: v.y + o.y,
	}
}

func (v vec2) sub(o vec2) vec2 {
	return vec2{
		x: v.x - o.x,
		y: v.y - o.y,
	}
}

func (v vec2) mul(f float32) vec2 {
	return vec2{
		x: v.x * f,
		y: v.y * f,
	}
}

func fill(path iter.Seq[curve.PathElement], affine curve.Affine) iter.Seq[flatLine] {
	return func(yield func(flatLine) bool) {
		var start, p0 curve.Point
		iter := func(yield func(el curve.PathElement) bool) {
			for el := range path {
				if !yield(el.Transform(affine)) {
					return
				}
			}
		}
		yieldClosePath := func(start, p0 curve.Point) bool {
			pt0 := vec2{float32(p0.X), float32(p0.Y)}
			pt1 := vec2{float32(start.X), float32(start.Y)}
			if pt0 != pt1 {
				return yield(flatLine{pt0, pt1})
			}
			return true
		}

		closed := true
		for el := range curve.Flatten(iter, flattenTolerance) {
			switch el.Kind {
			case curve.MoveToKind:
				if !closed && p0 != start {
					if !yieldClosePath(start, p0) {
						return
					}
				}
				closed = false
				start = el.P0
				p0 = el.P0
			case curve.LineToKind:
				p := el.P0
				pt0 := vec2{float32(p0.X), float32(p0.Y)}
				pt1 := vec2{float32(p.X), float32(p.Y)}
				if !yield(flatLine{pt0, pt1}) {
					return
				}
				p0 = p
			case curve.ClosePathKind:
				closed = true
				if !yieldClosePath(start, p0) {
					return
				}

			default:
				panic(fmt.Sprintf("unreachable: %v", el.Kind))
			}
		}

		if !closed {
			if !yieldClosePath(start, p0) {
				return
			}
		}
	}
}

func stroke(path iter.Seq[curve.PathElement], style curve.Stroke, affine curve.Affine) iter.Seq[flatLine] {
	iter := func(yield func(el curve.PathElement) bool) {
		for el := range path {
			if !yield(el.Transform(affine)) {
				return
			}
		}
	}
	stroked := curve.StrokePath(iter, style, curve.StrokeOpts{}, flattenTolerance)
	return fill(slices.Values(stroked), curve.Identity)
}
