// SPDX-FileCopyrightText: 2024 the Piet Authors
// SPDX-FileCopyrightText: 2025 Dominik Honnef and contributors
//
// SPDX-License-Identifier: MIT

package sparse

import (
	"honnef.co/go/curve"
	"honnef.co/go/gutter/gfx"
)

type Path struct {
	strips   []strip
	alphas   [][stripHeight]uint8
	fillRule gfx.FillRule
}

func CompileFillPath(
	shape gfx.Shape,
	affine curve.Affine,
	fillRule gfx.FillRule,
	width uint16,
	height uint16,
) Path {
	// The transformation mustn't skew the shape for our optimizations to apply.
	if affine.N1 == 0 && affine.N2 == 0 {
		switch shape := shape.(type) {
		case curve.Rect:
			// OPT(dh): all rectangles of the same size that fall on integer
			// coordinates are the same, especially if their Y coordinates only
			// differ in multiples of the strip height.

			a, d, e, f := affine.N0, affine.N3, affine.N4, affine.N5
			shape = curve.Rect{
				X0: shape.X0*a + e,
				Y0: shape.Y0*d + f,
				X1: shape.X1*a + e,
				Y1: shape.Y1*d + f,
			}

			strips, alphas := renderRect(shape, width, height)
			return Path{
				strips:   strips,
				alphas:   alphas,
				fillRule: gfx.NonZero,
			}
		}
	}

	// TODO(dh): scale precision based on transformation
	lines := fill(shape.PathElements(0.1), affine)
	strips, alphas := renderPathCommon(lines, fillRule, width, height)
	return Path{strips, alphas, fillRule}
}

func CompileStrokedPath(
	shape gfx.Shape,
	affine curve.Affine,
	stroke_ curve.Stroke,
	width uint16,
	height uint16,
) Path {
	// TODO(dh): scale precision based on transformation
	path := shape.PathElements(0.1)
	lines := stroke(path, stroke_, affine)
	strips, alphas := renderPathCommon(lines, gfx.NonZero, width, height)
	return Path{strips, alphas, gfx.NonZero}
}
