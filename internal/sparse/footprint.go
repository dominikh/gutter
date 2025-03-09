// SPDX-FileCopyrightText: 2024 the Piet Authors
// SPDX-FileCopyrightText: 2025 Dominik Honnef and contributors
//
// SPDX-License-Identifier: MIT

package sparse

import "math/bits"

// footprint is a bitset representing the pixels covered by a set of tiles. Any
// individual tile will cover a contiguous range of pixels, as each tile
// contains exactly one line segment. However, when multiple tiles are processed
// together, footprints can be ORed together, which may result in gaps.
type footprint uint32

func (t tile) footprint() footprint {
	x0 := t.p0.x
	x1 := t.p1.x
	xmin := floor32(min(x0, x1))
	xmax := ceil32(max(x0, x1))
	start := uint32(xmin)
	end := min(max((start+1), uint32(xmax)), tileWidth)
	return footprintFromRange(uint8(start), uint8(end))
}

func footprintFromIndex(idx uint8) footprint {
	return 1 << idx
}

func footprintFromRange(start, end uint8) footprint {
	return (1 << end) - (1 << start)
}

func (fp footprint) x0() uint32 {
	return uint32(bits.TrailingZeros32(uint32(fp)))
}

func (fp footprint) x1() uint32 {
	return uint32(32 - bits.LeadingZeros32(uint32(fp)))
}

func (fp footprint) extend(idx uint8) footprint {
	return fp | 1<<idx
}

func (fp footprint) merge(o footprint) footprint {
	return fp | o
}
