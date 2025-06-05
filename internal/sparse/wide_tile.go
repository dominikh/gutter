// SPDX-FileCopyrightText: 2024 the Piet Authors
// SPDX-FileCopyrightText: 2025 Dominik Honnef and contributors
//
// SPDX-License-Identifier: MIT

package sparse

import (
	"fmt"

	"honnef.co/go/gutter/gfx"
)

const wideTileWidth = 256

type wideTile struct {
	bg           gfx.PlainColor
	cmds         []cmd
	numZeroClips int
	numLayers    int
}

// TODO rename to cmdKind, same for cmd.typ field
type cmdType uint8

const (
	cmdFill cmdType = iota
	cmdAlphaFill
	cmdPushLayer
	cmdPopLayer
	cmdBlend
	cmdAlphaBlend
)

type cmd struct {
	paint   gfx.EncodedPaint     // fill, alphaFill
	opacity float32              // alphaBlend, blend
	alphas  [][stripHeight]uint8 // alphaFill, alphaBlend
	x       uint16               // fill, alphaFill, blend, alphaBlend
	width   uint16               // fill, alphaFill, blend, alphaBlend
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
	case cmdBlend:
		return fmt.Sprintf("Blend(x=%v, width=%v, blend=%s)", cmd.x, cmd.width, cmd.blend)
	case cmdAlphaBlend:
		return fmt.Sprintf("AlphaBlend(x=%v, width=%v, blend=%s)",
			cmd.x, cmd.width, cmd.blend)
	default:
		panic(fmt.Sprintf("invalid command type %v", cmd.typ))
	}
}

func (wt *wideTile) fill(x, width uint16, paint gfx.EncodedPaint) {
	if wt.isZeroClip() {
		return
	}
	if s, ok := paint.(gfx.PlainColor); ok {
		// Note that we could be more aggressive in optimizing a whole-tile opaque fill
		// even with a layer stack. It would be valid to elide all drawing commands from
		// the enclosing layer push up to the fill. Further, we could extend the layer
		// push command to include a background color, rather than always starting with
		// a transparent buffer. Lastly, a sequence of push(bg); alphaFill/fill; pop could
		// be replaced with alphaFill/fill with the color (the latter is true even with a
		// non-opaque color).
		//
		// However, the extra cost of tracking such optimizations may outweigh the
		// benefit, especially in hybrid mode with GPU painting.
		if x == 0 && width == wideTileWidth && s[3] == 1.0 && wt.numLayers == 0 {
			wt.cmds = wt.cmds[:0]
			wt.bg = s
			return
		}
	}
	wt.cmds = append(wt.cmds, cmd{typ: cmdFill, x: x, width: width, paint: paint})
}

func (wt *wideTile) pushLayer() {
	if wt.isZeroClip() {
		return
	}
	wt.cmds = append(wt.cmds, cmd{typ: cmdPushLayer})
	wt.numLayers++
}

func (wt *wideTile) popLayer() {
	if wt.isZeroClip() {
		return
	}
	if len(wt.cmds) > 0 && wt.cmds[len(wt.cmds)-1].typ == cmdPushLayer {
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
		panic("unbalanced zero clips")
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
	if len(wt.cmds) > 0 && wt.cmds[len(wt.cmds)-1].typ == cmdPushLayer && c.blend.Compose&gfx.ComposeAffectsDestRegion == 0 {
		return
	}
	wt.cmds = append(wt.cmds, c)
}

func (wt *wideTile) blend(x, width uint16, blend gfx.BlendMode, opacity float32) {
	if wt.isZeroClip() {
		return
	}
	if len(wt.cmds) > 0 && wt.cmds[len(wt.cmds)-1].typ == cmdPushLayer && blend.Compose&gfx.ComposeAffectsDestRegion == 0 {
		return
	}
	if len(wt.cmds) == 0 {
		panic("internal error: called blend without pushing a layer")
	}
	wt.cmds = append(wt.cmds, cmd{
		typ:     cmdBlend,
		x:       x,
		width:   width,
		blend:   blend,
		opacity: opacity,
	})
}
