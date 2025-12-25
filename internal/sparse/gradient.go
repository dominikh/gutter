// SPDX-FileCopyrightText: 2012 Google Inc.
// SPDX-FileCopyrightText: 2025 the Piet Authors
// SPDX-FileCopyrightText: 2025 the Vello Authors
// SPDX-FileCopyrightText: 2025 Dominik Honnef and contributors
//
// SPDX-License-Identifier: MIT AND BSD-3-Clause

package sparse

import (
	"fmt"
	"math"

	"honnef.co/go/curve"
	"honnef.co/go/gutter/gfx"
	"honnef.co/go/stuff/math/math32"
)

type gradientFiller struct {
	curPos   curve.Point
	gradient *gfx.EncodedGradient
}

func newGradientFiller(
	e *gfx.EncodedGradient,
	startX uint16,
	startY uint16,
) *gradientFiller {
	return &gradientFiller{
		curPos:   curve.Pt(float64(startX), float64(startY)).Transform(e.Transform),
		gradient: e,
	}
}

func (gf *gradientFiller) run(dst [][stripHeight]gfx.PlainColor) {
	oldPos := gf.curPos

	for x := range dst {
		col := &dst[x]
		gf.runColumn(col, &gf.gradient.LUT)
		gf.curPos = gf.curPos.Translate(gf.gradient.XAdvance)
	}

	// Radial gradients can have positions that are undefined and thus shouldn't be
	// painted at all. Checking for this inside of the main filling logic would be
	// an unnecessary overhead for the general case, while this is really just an edge
	// case. Because of this, in the first run we will fill it using a dummy color, and
	// in case the gradient might have undefined locations, we do another run over
	// the buffer and override the positions with a transparent fill. This way, we need
	// 2x as long to handle such gradients, but for the common case we don't incur any
	// additional overhead.
	if gf.gradient.Kind.HasUndefined() {
		gf.curPos = oldPos
		gf.runUndefined(dst)
	}
}

func (gf *gradientFiller) runColumn(col *[stripHeight]gfx.PlainColor, lut *gfx.GradientLUT) {
	pos := gf.curPos
	for y := range col {
		px := &col[y]
		t := gf.gradient.Kind.CurPos(pos)
		t = applyExtend(t, gf.gradient.Extend)
		*px = lut.LUT[int(t*lut.Scale)]

		pos = pos.Translate(gf.gradient.YAdvance)
	}
}

func (gf *gradientFiller) runUndefined(dst [][stripHeight]gfx.PlainColor) {
	for i := range dst {
		col := &dst[i]
		pos := gf.curPos
		for i := range col {
			px := &col[i]
			if !gf.gradient.Kind.IsDefined(pos) {
				*px = gfx.PlainColor{}
			}
			pos = pos.Translate(gf.gradient.YAdvance)
		}
		gf.curPos = gf.curPos.Translate(gf.gradient.XAdvance)
	}
}

func applyExtend(val float32, extend gfx.GradientExtend) float32 {
	switch extend {
	case gfx.GradientExtendPad:
		return max(min(val, 1), 0)
	case gfx.GradientExtendRepeat:
		_, fract := math.Modf(float64(val - math32.Floor(val)))
		return float32(fract)
	case gfx.GradientExtendReflect:
		// See https://github.com/google/skia/blob/220738774f7a0ce4a6c7bd17519a336e5e5dea5b/src/opts/SkRasterPipeline_opts.h#L6472-L6475
		return min(max(math32.Abs((val-1)-2*math32.Floor((val-1)*0.5)-1), 0), 1)
	default:
		panic(fmt.Sprintf("unexpected gfx.GradientExtend: %#v", extend))
	}
}
