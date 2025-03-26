// SPDX-FileCopyrightText: 2024 the Piet Authors
// SPDX-FileCopyrightText: 2025 Dominik Honnef and contributors
//
// SPDX-License-Identifier: MIT

package sparse

import (
	"fmt"
	"iter"

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

func fill(path iter.Seq[curve.PathElement], affine curve.Affine, lineBuf []flatLine) []flatLine {
	lineBuf = lineBuf[:0]

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

func stroke(path iter.Seq[curve.PathElement], style curve.Stroke, affine curve.Affine, lineBuf []flatLine) []flatLine {
	iter := func(yield func(el curve.PathElement) bool) {
		for el := range path {
			if !yield(el.Transform(affine)) {
				return
			}
		}
	}
	stroked := curve.StrokePath(iter, style, curve.StrokeOpts{}, flattenTolerance)
	return fill(stroked, curve.Identity, lineBuf)
}
