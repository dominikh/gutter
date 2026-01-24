// SPDX-FileCopyrightText: 2026 Dominik Honnef and contributors
//
// SPDX-License-Identifier: MIT

package sparse

import (
	"honnef.co/go/color"
	"honnef.co/go/curve"
	"honnef.co/go/gutter/gfx"
)

func encodeColor(s color.Color, _ curve.Affine) encodedPaint {
	c := color.Color(s).Convert(gfx.ColorSpace)
	return colorToInternal(c)
}

var _ encodedPaint = encodedColor{}

type encodedColor gfx.PlainColor

// Opaque implements [encodedPaint].
func (e encodedColor) Opaque() bool {
	return e[3] == 1
}

// isEncodedPaint implements [encodedPaint].
func (e encodedColor) isEncodedPaint() {}

func colorToInternal(c color.Color) encodedColor {
	return encodedColor(gfx.ColorToInternal(c))
}
