// SPDX-FileCopyrightText: 2024 the Piet Authors
// SPDX-FileCopyrightText: 2025 Dominik Honnef and contributors
//
// SPDX-License-Identifier: MIT

package sparse

import "fmt"

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
	cmdStrip
	cmdPushClip
	cmdPopClip
	cmdClipFill
	cmdClipStrip
)

type cmd struct {
	alphaIdx int        // strip, clipStrip
	x        uint32     // fill, strip, clipFill, clipStrip
	width    uint32     // fill, strip, clipFill, clipStrip
	color    [4]float32 // fill, strip
	typ      cmdType
}

func (cmd *cmd) String() string {
	switch cmd.typ {
	case cmdFill:
		return fmt.Sprintf("Fill(x=%v, width=%v, color=%v",
			cmd.x, cmd.width, cmd.color)
	case cmdStrip:
		return fmt.Sprintf("Strip(x=%v, width=%v, color=%v, alphaIdx=%v)",
			cmd.x, cmd.width, cmd.color, cmd.alphaIdx)
	case cmdPushClip:
		return "PushClip()"
	case cmdPopClip:
		return "PopClip()"
	case cmdClipFill:
		return fmt.Sprintf("ClipFill(x=%v, width=%v)", cmd.x, cmd.width)
	case cmdClipStrip:
		return fmt.Sprintf("ClipStrip(x=%v, width=%v, alphaIdx=%v)",
			cmd.x, cmd.width, cmd.alphaIdx)
	default:
		panic(fmt.Sprintf("invalid command type %v", cmd.typ))
	}
}

func (wt *wideTile) fill(x, width uint32, c [4]float32) {
	if wt.isZeroClip() {
		return
	}
	// Note that we could be more aggressive in optimizing a whole-tile opaque fill
	// even with a clip stack. It would be valid to elide all drawing commands from
	// the enclosing clip push up to the fill. Further, we could extend the clip
	// push command to include a background color, rather than always starting with
	// a transparent buffer. Lastly, a sequence of push(bg); strip/fill; pop could
	// be replaced with strip/fill with the color (the latter is true even with a
	// non-opaque color).
	//
	// However, the extra cost of tracking such optimizations may outweigh the
	// benefit, especially in hybrid mode with GPU painting.
	if x == 0 && width == wideTileWidth && c[3] == 1.0 && wt.numClips == 0 {
		wt.cmds = wt.cmds[:0]
		wt.bg = c
	} else {
		wt.cmds = append(wt.cmds, cmd{typ: cmdFill, x: x, width: width, color: c})
	}
}

func (wt *wideTile) strip(c cmd) {
	if wt.isZeroClip() {
		return
	}
	wt.cmds = append(wt.cmds, c)
}

func (wt *wideTile) pushClip() {
	if wt.isZeroClip() {
		return
	}
	wt.cmds = append(wt.cmds, cmd{typ: cmdPushClip})
}

func (wt *wideTile) popClip() {
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
	wt.numZeroClips--
}

func (wt *wideTile) isZeroClip() bool {
	return wt.numZeroClips > 0
}

func (wt *wideTile) clipStrip(c cmd) {
	if wt.isZeroClip() {
		return
	}
	if len(wt.cmds) > 0 && wt.cmds[len(wt.cmds)-1].typ == cmdPushClip {
		return
	}
	wt.cmds = append(wt.cmds, c)
}

func (wt *wideTile) clipFill(x, width uint32) {
	if wt.isZeroClip() {
		return
	}
	if len(wt.cmds) > 0 && wt.cmds[len(wt.cmds)-1].typ == cmdPushClip {
		return
	}
	if len(wt.cmds) == 0 {
		panic("internal error: called clipFill without pushing a clip")
	}
	wt.cmds = append(wt.cmds, cmd{
		typ:   cmdClipFill,
		x:     x,
		width: width,
	})
}
