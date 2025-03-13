// SPDX-FileCopyrightText: 2024 the Piet Authors
// SPDX-FileCopyrightText: 2025 Dominik Honnef and contributors
//
// SPDX-License-Identifier: MIT

package sparse

import (
	"cmp"
)

const (
	tileWidth  = 4
	tileHeight = 4
)

type tile struct {
	x uint16
	y uint16

	// The index is the unsigned number in the 31 least significant bits of this value.
	//
	// The last bit is 1 if and only if the line crosses the tile's top edge. Lines making this
	// crossing increment or decrement the coarse tile winding, depending on the line direction.
	packed_winding_line_idx uint32
}

func newTile(x, y uint16, lineIdx uint32, winding bool) tile {
	packed := lineIdx
	if winding {
		packed |= 1 << 31
	}
	return tile{x, y, packed}
}

// Populate the tiles' container with a buffer of lines.
//
// Tiles exceeding the top, right or bottom of the viewport (given by `width`
// and `height` in pixels) are culled.
func makeTiles(lineBuf []flatLine, tileBuf []tile, width, height uint16) []tile {
	// TODO: Tiles are clamped to the left edge of the viewport, but lines fully to the left of the
	// viewport are not culled yet. These lines impact winding, and would need forwarding of
	// winding to the strip generation stage.
	tileBuf = tileBuf[:0]

	if width == 0 || height == 0 {
		return nil
	}

	tileColumns := divCeil(width, tileWidth)
	tileRows := divCeil(height, tileHeight)

	for lineIdx, line := range lineBuf {
		p0x := line.p0.x / tileWidth
		p0y := line.p0.y / tileHeight
		p1x := line.p1.x / tileWidth
		p1y := line.p1.y / tileHeight

		var lineLeftX, lineRightX float32
		if p0x < p1x {
			lineLeftX = p0x
			lineRightX = p1x
		} else {
			lineLeftX = p1x
			lineRightX = p0x
		}
		var lineTopY, lineTopX, lineBottomY, lineBottomX float32
		if p0y < p1y {
			lineTopY = p0y
			lineTopX = p0x
			lineBottomY = p1y
			lineBottomX = p1x
		} else {
			lineTopY = p1y
			lineTopX = p1x
			lineBottomY = p0y
			lineBottomX = p0x
		}

		// For ease of logic, special-case purely vertical tiles.
		if lineLeftX == lineRightX {
			yTopTiles := min(satConv[uint16](lineTopY), tileRows)
			yBottomTiles := min(satConv[uint16](ceil32(lineBottomY)), tileRows)

			x := satConv[uint16](lineLeftX)
			for yIdx := yTopTiles; yIdx < yBottomTiles; yIdx++ {
				tileBuf = append(tileBuf, newTile(x, yIdx, uint32(lineIdx), float32(yIdx) >= lineTopY))
			}
		} else {
			xSlope := (p1x - p0x) / (p1y - p0y)
			yTopTiles := min(satConv[uint16](lineTopY), tileRows)
			yBottomTiles := min(satConv[uint16](ceil32(lineBottomY)), tileRows)

			for yIdx := yTopTiles; yIdx < yBottomTiles; yIdx++ {
				y := float32(yIdx)

				// The line's y-coordinates at the line's top- and bottom-most
				// points within the tile row.
				lineRowTopY := min32(max32(lineTopY, y), y+1)
				lineRowBottomY := min32(max32(lineBottomY, y), y+1)

				// The line's x-coordinates at the line's top- and bottom-most
				// points within the tile row.
				lineRowTopX := p0x + (lineRowTopY-p0y)*xSlope
				lineRowBottomX := p0x + (lineRowBottomY-p0y)*xSlope

				// The line's x-coordinates at the line's left- and right-most
				// points within the tile row.
				lineRowLeftX := max32(min32(lineRowTopX, lineRowBottomX), lineLeftX)
				lineRowRightX := min32(max32(lineRowTopX, lineRowBottomX), lineRightX)

				var windingX uint16
				if lineTopX < lineBottomX {
					windingX = satConv[uint16](lineRowLeftX)
				} else {
					windingX = satConv[uint16](lineRowRightX)
				}

				for xIdx := satConv[uint16](lineRowLeftX); xIdx <= min(satConv[uint16](lineRowRightX), tileColumns-1); xIdx++ {
					tileBuf = append(tileBuf, newTile(xIdx, yIdx, uint32(lineIdx), y >= lineTopY && xIdx == windingX))
				}
			}
		}
	}

	return tileBuf
}

func (t *tile) cmp(b *tile) int {
	return cmp.Compare(
		uint32(t.y)<<16|uint32(t.x),
		uint32(b.y)<<16|uint32(b.x),
	)
}

func (t *tile) winding() bool {
	return t.packed_winding_line_idx&(1<<31) != 0
}

func (t *tile) lineIdx() uint32 {
	return t.packed_winding_line_idx & (1<<31 - 1)
}

func (t *tile) sameLoc(o *tile) bool {
	return t.sameRow(o) && t.x == o.x
}

func (t *tile) prevLoc(o *tile) bool {
	return t.sameRow(o) && t.x+1 > t.x && t.x+1 == o.x
}

func (t *tile) sameRow(o *tile) bool {
	return t.y == o.y
}
