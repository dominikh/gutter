// SPDX-FileCopyrightText: 2024 the Piet Authors
// SPDX-FileCopyrightText: 2025 Dominik Honnef and contributors
//
// SPDX-License-Identifier: MIT

package sparse

import (
	"fmt"
)

const wideTileWidth = 256

type wideTile struct {
	bg           [4]float32
	cmds         []cmd
	numZeroClips int
	numClips     int
}

// TODO rename to cmdKind, same for cmd.typ field
type cmdType uint8

const (
	cmdFill cmdType = iota
	cmdAlphaFill
	cmdPushClip
	cmdPopClip
	cmdClipFill
	cmdClipAlphaFill
)

type cmd struct {
	paint   encodedPaint         // fill, alphaFill
	opacity float32              // clipAlphaFill, clipFill
	alphas  [][stripHeight]uint8 // alphaFill, clipAlphaFill
	x       uint16               // fill, alphaFill, clipFill, clipAlphaFill
	width   uint16               // fill, alphaFill, clipFill, clipAlphaFill
	blend   BlendMode            // clipAlphaFill, clipFill
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
	case cmdPushClip:
		return "PushClip()"
	case cmdPopClip:
		return "PopClip()"
	case cmdClipFill:
		return fmt.Sprintf("ClipFill(x=%v, width=%v, blend=%s)", cmd.x, cmd.width, cmd.blend)
	case cmdClipAlphaFill:
		return fmt.Sprintf("ClipAlphaFill(x=%v, width=%v, blend=%s)",
			cmd.x, cmd.width, cmd.blend)
	default:
		panic(fmt.Sprintf("invalid command type %v", cmd.typ))
	}
}

func (wt *wideTile) fill(x, width uint16, paint encodedPaint) {
	if wt.isZeroClip() {
		return
	}
	if s, ok := paint.(plainColor); ok {
		// Note that we could be more aggressive in optimizing a whole-tile opaque fill
		// even with a clip stack. It would be valid to elide all drawing commands from
		// the enclosing clip push up to the fill. Further, we could extend the clip
		// push command to include a background color, rather than always starting with
		// a transparent buffer. Lastly, a sequence of push(bg); alphaFill/fill; pop could
		// be replaced with alphaFill/fill with the color (the latter is true even with a
		// non-opaque color).
		//
		// However, the extra cost of tracking such optimizations may outweigh the
		// benefit, especially in hybrid mode with GPU painting.
		if x == 0 && width == wideTileWidth && s[3] == 1.0 && wt.numClips == 0 {
			wt.cmds = wt.cmds[:0]
			wt.bg = s
			return
		}
	}
	wt.cmds = append(wt.cmds, cmd{typ: cmdFill, x: x, width: width, paint: paint})
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
	wt.cmds = append(wt.cmds, cmd{typ: cmdPushClip})
	wt.numClips++
}

func (wt *wideTile) popLayer() {
	if wt.isZeroClip() {
		return
	}
	if len(wt.cmds) > 0 && wt.cmds[len(wt.cmds)-1].typ == cmdPushClip {
		// Nothing was drawn inside the clip, elide it.
		wt.cmds = wt.cmds[:len(wt.cmds)-1]
	} else {
		wt.cmds = append(wt.cmds, cmd{typ: cmdPopClip})
	}
	wt.numClips--
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

func (wt *wideTile) clipAlphaFill(c cmd) {
	if wt.isZeroClip() {
		return
	}
	if len(wt.cmds) > 0 && wt.cmds[len(wt.cmds)-1].typ == cmdPushClip && c.blend.Compose&composeAffectsDestRegion == 0 {
		return
	}
	wt.cmds = append(wt.cmds, c)
}

func (wt *wideTile) clipFill(x, width uint16, blend BlendMode, opacity float32) {
	if wt.isZeroClip() {
		return
	}
	if len(wt.cmds) > 0 && wt.cmds[len(wt.cmds)-1].typ == cmdPushClip && blend.Compose&composeAffectsDestRegion == 0 {
		return
	}
	if len(wt.cmds) == 0 {
		panic("internal error: called clipFill without pushing a clip")
	}
	wt.cmds = append(wt.cmds, cmd{
		typ:     cmdClipFill,
		x:       x,
		width:   width,
		blend:   blend,
		opacity: opacity,
	})
}
