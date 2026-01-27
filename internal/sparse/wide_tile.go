// SPDX-FileCopyrightText: 2024 the Piet Authors
// SPDX-FileCopyrightText: 2025 Dominik Honnef and contributors
//
// SPDX-License-Identifier: MIT

package sparse

import (
	"fmt"

	"honnef.co/go/gutter/gfx"
)

const disableWideTileOpts = false
const wideTileWidth = 256

type wideTile struct {
	bg           gfx.PlainColor
	cmds         []cmd
	numZeroClips int
	numLayers    int
}

// TODO rename to cmdKind, same for cmd.typ field
//
//go:generate go tool stringer -type=cmdType
type cmdType uint8

const (
	cmdNop cmdType = iota
	cmdFill
	cmdAlphaFill
	cmdPushLayer
	cmdPopLayer
	cmdCopyBackdrop
	cmdBlend
	cmdAlphaBlend
	// cmdClear sets all pixels to the specified color
	//
	// TODO(dh): we won't need this once cmdFill supports blend modes
	cmdClear
)

type cmd struct {
	paint   encodedPaint         // fill, alphaFill, clear
	alphas  [][stripHeight]uint8 // alphaFill, alphaBlend
	opacity float32              // alphaBlend, blend
	x       uint16               // fill, alphaFill, blend, alphaBlend, clear
	width   uint16               // fill, alphaFill, blend, alphaBlend, clear
	blend   gfx.BlendMode        // alphaBlend, blend
	typ     cmdType
}

func (cmd cmd) String() string {
	switch cmd.typ {
	case cmdFill:
		return fmt.Sprintf("Fill(x=%v, width=%v, paint=%v)",
			cmd.x, cmd.width, cmd.paint)
	case cmdAlphaFill:
		return fmt.Sprintf("AlphaFill(x=%v, width=%v, paint=%v)",
			cmd.x, cmd.width, cmd.paint)
	case cmdPushLayer:
		return "PushLayer()"
	case cmdPopLayer:
		return "PopLayer()"
	case cmdCopyBackdrop:
		return "CopyBackdrop()"
	case cmdBlend:
		return fmt.Sprintf("Blend(x=%v, width=%v, blend=%s, opacity=%v)",
			cmd.x, cmd.width, cmd.blend, cmd.opacity)
	case cmdAlphaBlend:
		return fmt.Sprintf("AlphaBlend(x=%v, width=%v, blend=%s, opacity=%v)",
			cmd.x, cmd.width, cmd.blend, cmd.opacity)
	case cmdNop:
		return "Nop()"
	case cmdClear:
		return fmt.Sprintf("Clear(x=%v, width=%v, paint=%v)",
			cmd.x, cmd.width, cmd.paint)
	default:
		panic(fmt.Sprintf("invalid command type %v", cmd.typ))
	}
}

func (wt *wideTile) fill(x, width uint16, paint encodedPaint) {
	if wt.isZeroClip() {
		return
	}
	t := cmdFill
	if paint.Opaque() {
		t = cmdClear
	}
	wt.cmds = append(wt.cmds, cmd{typ: t, x: x, width: width, paint: paint})
}

func (wt *wideTile) alphaFill(c cmd) {
	if wt.isZeroClip() {
		return
	}
	wt.cmds = append(wt.cmds, c)
}

func (wt *wideTile) pushLayer() {
	if wt.isZeroClip() {
		return
	}

	wt.cmds = append(wt.cmds, cmd{typ: cmdPushLayer})
	wt.numLayers++
}

func (wt *wideTile) copyBackdrop() {
	if wt.isZeroClip() {
		return
	}

	wt.cmds = append(wt.cmds, cmd{typ: cmdCopyBackdrop})
}

func (wt *wideTile) popLayer() {
	if wt.isZeroClip() {
		return
	}
	if !disableWideTileOpts &&
		len(wt.cmds) > 0 &&
		wt.cmds[len(wt.cmds)-1].typ == cmdPushLayer {
		// Nothing was drawn inside the layer, elide it.
		wt.cmds = wt.cmds[:len(wt.cmds)-1]
	} else {
		wt.cmds = append(wt.cmds, cmd{typ: cmdPopLayer})
	}
	wt.numLayers--
}

func (wt *wideTile) pushZeroClip() {
	wt.numZeroClips++
}

func (wt *wideTile) popZeroClip() {
	if wt.numZeroClips == 0 {
		panic("internal error: unbalanced zero clips")
	}
	wt.numZeroClips--
}

func (wt *wideTile) isZeroClip() bool {
	return wt.numZeroClips > 0
}

func (wt *wideTile) alphaBlend(c cmd) {
	if wt.isZeroClip() {
		return
	}
	if !disableWideTileOpts &&
		len(wt.cmds) > 0 &&
		wt.cmds[len(wt.cmds)-1].typ == cmdPushLayer &&
		c.blend.Compose&gfx.ComposeAffectsDestRegion == 0 {
		return
	}
	wt.cmds = append(wt.cmds, c)
}

func (wt *wideTile) blend(
	x uint16,
	width uint16,
	blend gfx.BlendMode,
	opacity float32,
) {
	if wt.isZeroClip() {
		return
	}
	if len(wt.cmds) == 0 {
		panic("internal error: called blend without pushing a layer")
	}
	prevCmd := &wt.cmds[len(wt.cmds)-1]
	if !disableWideTileOpts &&
		blend.Compose&gfx.ComposeAffectsDestRegion == 0 &&
		prevCmd.typ == cmdPushLayer {
		// Blending when nothing has been drawn in the layer yet has no visible
		// effect for some compose operators, notably SrcOver.
		return
	}

	// We don't check that the blend mode and opacity match, because at command
	// generation time, an uninterrupted run of blends is only possible while
	// popping a layer.
	if !disableWideTileOpts && prevCmd.typ == cmdBlend && x == prevCmd.x+prevCmd.width {
		prevCmd.width += width
		return
	}
	wt.cmds = append(wt.cmds, cmd{
		typ:     cmdBlend,
		x:       x,
		width:   width,
		blend:   blend,
		opacity: opacity,
	})
}
