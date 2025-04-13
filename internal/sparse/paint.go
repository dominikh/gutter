// SPDX-FileCopyrightText: 2025 the Piet Authors
// SPDX-FileCopyrightText: 2025 Dominik Honnef and contributors
//
// SPDX-License-Identifier: MIT

package sparse

import "honnef.co/go/color"

type Paint interface {
	encode() encodedPaint
}

type Solid color.Color

func (s Solid) encode() encodedPaint {
	c := (*color.Color)(&s).Convert(ColorSpace)
	cc := colorToInternal(c)
	return cc
}

type encodedPaint interface {
	isEncodedPaint()
}
