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
		for el := range curve.Flatten(iter, flattenTolerance) {
			switch el.Kind {
			case curve.MoveToKind:
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
				pt0 := vec2{float32(p0.X), float32(p0.Y)}
				pt1 := vec2{float32(start.X), float32(start.Y)}
				if pt0 != pt1 {
					if !yield(flatLine{pt0, pt1}) {
						return
					}
				}
			default:
				panic(fmt.Sprintf("unreachable: %v", el.Kind))
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
