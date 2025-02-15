// SPDX-FileCopyrightText: 2024 the Piet Authors
// SPDX-FileCopyrightText: 2025 Dominik Honnef and contributors
//
// SPDX-License-Identifier: MIT

package sparse

import "fmt"

const wideTileWidth = 256

type wideTile struct {
	bg   [4]float32
	cmds []cmd
}

type cmdType uint8

const (
	cmdFill cmdType = iota
	cmdStrip
)

type cmd struct {
	alphaIdx int        // strip
	x        uint32     // fill, strip
	width    uint32     // fill, strip
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
	default:
		panic(fmt.Sprintf("invalid command type %v", cmd.typ))
	}
}

func (wt *wideTile) fill(x, width uint32, c [4]float32) {
	if x == 0 && width == wideTileWidth && c[3] == 1.0 {
		wt.cmds = wt.cmds[:0]
		wt.bg = c
	} else {
		wt.cmds = append(wt.cmds, cmd{typ: cmdFill, x: x, width: width, color: c})
	}
}

func (wt *wideTile) push(cmd cmd) {
	wt.cmds = append(wt.cmds, cmd)
}
