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

// OPT don't use an interface
type cmd any

type cmdFill struct {
	x     uint32
	width uint32
	color [4]float32
}

type cmdStrip struct {
	x        uint32
	width    uint32
	alphaIdx int
	color    [4]float32
}

func (wt *wideTile) fill(x, width uint32, c [4]float32) {
	if x == 0 && width == WIDE_TILE_WIDTH && c[3] == 1.0 {
		wt.cmds = wt.cmds[:0]
		wt.bg = c
	} else {
		wt.cmds = append(wt.cmds, cmdFill{x, width, c})
	}
}

func (wt *wideTile) push(cmd cmd) {
	wt.cmds = append(wt.cmds, cmd)
}
