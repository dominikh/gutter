// SPDX-FileCopyrightText: 2024 the Piet Authors
// SPDX-FileCopyrightText: 2025 Dominik Honnef and contributors
//
// SPDX-License-Identifier: MIT

package sparse

import (
	"fmt"

	"honnef.co/go/curve"
)

// //! Utilities for flattening
// use flatten::stroke::LoweredPath;
// use piet_next::peniko::kurbo::{self, Affine, BezPath, Line, Point, Stroke};

// use crate::tiling::FlatLine;

// / The flattening tolerance
const TOL = 0.25

func fill(path curve.BezPath, affine curve.Affine, line_buf *[]flatLine) {
	*line_buf = (*line_buf)[:0]
	var start, p0 curve.Point
	iter := func(yield func(el curve.PathElement) bool) {
		for _, el := range path {
			if !yield(el.Transform(affine)) {
				return
			}
		}
	}
	for el := range curve.Flatten(iter, TOL) {
		switch el.Kind {
		case curve.MoveToKind:
			start = el.P0
			p0 = el.P0
		case curve.LineToKind:
			p := el.P0
			pt0 := [2]float32{float32(p0.X), float32(p0.Y)}
			pt1 := [2]float32{float32(p.X), float32(p.Y)}
			*line_buf = append(*line_buf, flatLine{pt0, pt1})
			p0 = p
		case curve.ClosePathKind:
			pt0 := [2]float32{float32(p0.X), float32(p0.Y)}
			pt1 := [2]float32{float32(start.X), float32(start.Y)}
			if pt0 != pt1 {
				*line_buf = append(*line_buf, flatLine{pt0, pt1})
			}
		default:
			panic(fmt.Sprintf("unreachable: %v", el.Kind))
		}
	}
}

func stroke(path curve.BezPath, style curve.Stroke, affine curve.Affine, line_buf *[]flatLine) {
	iter := func(yield func(el curve.PathElement) bool) {
		for _, el := range path {
			if !yield(el.Transform(affine)) {
				return
			}
		}
	}
	path = curve.StrokePath(iter, style, curve.StrokeOpts{}, TOL)
	fill(path, curve.Identity, line_buf)
}
