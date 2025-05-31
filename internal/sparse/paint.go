// SPDX-FileCopyrightText: 2025 the Piet Authors
// SPDX-FileCopyrightText: 2025 Dominik Honnef and contributors
//
// SPDX-License-Identifier: MIT

package sparse

import (
	"honnef.co/go/color"
	"honnef.co/go/curve"
)

type Paint interface {
	encode(transform curve.Affine) encodedPaint
}

type Solid color.Color

func (s Solid) encode(_ curve.Affine) encodedPaint {
	c := color.Color(s).Convert(ColorSpace)
	cc := colorToInternal(c)
	return cc
}

type encodedPaint interface {
	isEncodedPaint()
}
