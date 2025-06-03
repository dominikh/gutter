// SPDX-FileCopyrightText: 2025 the Piet Authors
// SPDX-FileCopyrightText: 2025 Dominik Honnef and contributors
//
// SPDX-License-Identifier: MIT

package sparse

import (
	"honnef.co/go/color"
	"honnef.co/go/curve"
	"honnef.co/go/gutter/gfx"
)

type gradientFiller struct {
	curPos   curve.Point
	rangeIdx int
	gradient *gfx.EncodedGradient

	prevC  color.Color
	prevPC gfx.PlainColor
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

func (gf *gradientFiller) colorToInternal(c color.Color) gfx.PlainColor {
	if gf.prevC == c {
		return gf.prevPC
	}
	pc := gfx.ColorToInternal(c)
	gf.prevC = c
	gf.prevPC = pc
	return pc
}

func (gf *gradientFiller) curRange() *gfx.GradientRange {
	// OPT(dh): cache the current range to avoid repeated bounds checks
	return &gf.gradient.Ranges[gf.rangeIdx]
}

func (gf *gradientFiller) advanceTo(targetPos float32) {
	for targetPos > gf.curRange().X1 || targetPos < gf.curRange().X0 {
		if gf.rangeIdx == 0 {
			gf.rangeIdx = len(gf.gradient.Ranges) - 1
		} else {
			gf.rangeIdx--
		}
	}
}

func (gf *gradientFiller) run(dst [][stripHeight]gfx.PlainColor) {
	oldPos := gf.curPos

	for x := range dst {
		col := &dst[x]
		gf.runColumn(col)
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

func (gf *gradientFiller) runColumn(col *[stripHeight]gfx.PlainColor) {
	pos := gf.curPos
	extend := func(val float32) float32 {
		return extend(val, gf.gradient.Pad, gf.gradient.ClampRange)
	}
	for y := range col {
		px := &col[y]
		extendedPos := extend(gf.gradient.Kind.CurPos(pos))
		gf.advanceTo(extendedPos)
		rng := gf.curRange()

		c := rng.C0

		for compIdx := range px {
			factor := (rng.Factors[compIdx] * (extendedPos - rng.X0))
			c.Values[compIdx] += float64(factor)
		}
		*px = gf.colorToInternal(c)
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

func extend(val float32, pad bool, clampRange [2]float32) float32 {
	if pad {
		return val
	}

	start := clampRange[0]
	end := clampRange[1]

	for val < start {
		val += end - start
	}
	for val > end {
		val -= end - start
	}
	return val
}
