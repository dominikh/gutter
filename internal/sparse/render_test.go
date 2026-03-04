// SPDX-FileCopyrightText: 2026 Dominik Honnef and contributors
//
// SPDX-License-Identifier: MIT

package sparse

import (
	"testing"

	"honnef.co/go/color"
	"honnef.co/go/curve"
	"honnef.co/go/gutter/gfx"
)

// TestLazyLayerPanic tests that pushing a clipless, non-lazy layer on top of a
// lazy layer works correctly.
func TestLazyLayerPanic(t *testing.T) {
	ctx := NewRenderer(100, 100)

	// Lazy layer
	ctx.PushLayerCompiled(LayerCompiled{Opacity: 1})

	// Non-lazy layer due to its blend mode, but no clip.
	ctx.PushLayerCompiled(LayerCompiled{
		BlendMode: gfx.BlendMode{Mix: gfx.MixMultiply},
		Opacity:   1,
	})

	// Force materialization of all lazy layers
	rect := curve.NewRectFromOrigin(curve.Pt(10, 10), curve.Sz(50, 50))
	ctx.Fill(rect, curve.Identity, gfx.NonZero, gfx.Solid(color.Make(color.SRGB, 1, 0, 0, 1)))

	ctx.PopLayer()
	ctx.PopLayer()
}
