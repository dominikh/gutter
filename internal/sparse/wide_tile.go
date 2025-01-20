// SPDX-FileCopyrightText: 2024 the Piet Authors
// SPDX-FileCopyrightText: 2025 Dominik Honnef and contributors
//
// SPDX-License-Identifier: MIT

package sparse

// use piet_next::peniko::color::{AlphaColor, Srgb};

const WIDE_TILE_WIDTH = 256
const STRIP_HEIGHT = 4

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
	alphaIdx int
	x        uint32
	width    uint32
	color    [4]float32
	typ      cmdType
}

func (wt *wideTile) fill(x, width uint32, c [4]float32) {
	if x == 0 && width == WIDE_TILE_WIDTH && c[3] == 1.0 {
		wt.cmds = wt.cmds[:0]
		wt.bg = c
	} else {
		wt.cmds = append(wt.cmds, cmd{typ: cmdFill, x: x, width: width, color: c})
	}
}

func (wt *wideTile) push(cmd cmd) {
	wt.cmds = append(wt.cmds, cmd)
}
