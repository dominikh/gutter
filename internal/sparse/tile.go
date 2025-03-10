// SPDX-FileCopyrightText: 2024 the Piet Authors
// SPDX-FileCopyrightText: 2025 Dominik Honnef and contributors
//
// SPDX-License-Identifier: MIT

package sparse

import (
	"cmp"
	"iter"
	"math"
)

const (
	tileWidth  = 4
	tileHeight = 4

	tileWidthScale     = tileWidth
	tileHeightScale    = tileHeight
	invTileWidthScale  = 1.0 / tileWidthScale
	invTileHeightScale = 1.0 / tileHeightScale
	// The value of 8192.0 is mainly chosen for compatibility with the old cpu-sparse
	// implementation, where we scaled to u16.
	nudgeFactor        = 1.0 / 8192.0
	scaledXNudgeFactor = 1.0 / (8192.0 * tileWidthScale)
)

type tile struct {
	x      int32
	y      uint16
	p0, p1 vec2
}

func makeTiles(lines iter.Seq[flatLine], tileBuf []tile) []tile {
	tileBuf = tileBuf[:0]

	spannedTiles := func(a, b float32) uint32 {
		// Calculate how many tiles are covered between two positions. p0 and p1 are scaled
		// to the tile unit square.
		return satConv[uint32](max(ceil32(max(a, b))-floor32(min(a, b)), 1))
	}

	nudgePoint := func(p vec2) vec2 {
		// Lines that cross vertical tile boundaries need special treatment during
		// anti aliasing. This case is detected via tile-relative x == 0. However,
		// lines can naturally start or end at a multiple of the 4x4 grid, too, but
		// these don't constitute crossings. We nudge these points ever so slightly,
		// by ensuring that xfrac0 and xfrac1 are always at least 1, which
		// corresponds to 1/8192 of a pixel.
		if math.Mod(float64(p.x), 1) == 0 {
			p.x += scaledXNudgeFactor
		}
		return p
	}

	pushTile := func(x, y float32, p0, p1 vec2) {
		if y >= 0 {
			tileBuf = append(tileBuf, tile{
				x:  satConvI32(x),
				y:  satConv[uint16](y),
				p0: p0,
				p1: p1,
			})
		}
	}

	for line := range lines {
		// Points scaled to the tile unit square.
		s0 := nudgePoint(scaleDown(line.p0))
		s1 := nudgePoint(scaleDown(line.p1))

		// Count how many tiles are covered on each axis.
		tileCountX := spannedTiles(s0.x, s1.x)
		tileCountY := spannedTiles(s0.y, s1.y)

		// Note: This code is technically unreachable now, because we always nudge x points at tile-relative 0
		// position. But we might need it again in the future if we change the logic.
		x := floor32(s0.x)
		if s0.x == x && s1.x < x {
			// s0.x is on right side of first tile
			x -= 1.0
		}

		y := floor32(s0.y)
		if s0.y == y && s1.y < y {
			// Since the end point of the line is above the start point,
			// s0.y is conceptually on bottom of the previous tile instead of at the top
			// of the current tile, so we need to adjust the y location.
			y -= 1.0
		}

		xfrac0 := scaleUp(s0.x - x)
		yfrac0 := scaleUp(s0.y - y)
		packed0 := vec2{xfrac0, yfrac0}

		if tileCountX == 1 {
			xfrac1 := scaleUp(s1.x - x)

			if tileCountY == 1 {
				yfrac1 := scaleUp(s1.y - y)
				// 1x1 tile
				pushTile(x, y, packed0, vec2{xfrac1, yfrac1})
			} else {
				// vertical column
				invSlope := (s1.x - s0.x) / (s1.y - s0.y)
				sign := sign32(s1.y - s0.y)

				// For downward lines, xclip0 and yclip store the x and y intersection points
				// at the bottom side of the current tile. For upward lines, they store the in
				// intersection points at the top side of the current tile.
				xclip0 := (s0.x - x) + (y-s0.y)*invSlope
				// We handled the case of a 1x1 tile before, so in this case the line will
				// definitely cross the tile either at the top or bottom, and thus yclip is
				// either 0 or 1.
				var yclip, flip float32
				if sign > 0.0 {
					// If the line goes downward, instead store where the line would intersect
					// the first tile at the bottom
					xclip0 += invSlope
					yclip = scaleUp(1.0)
					flip = scaleUp(-1.0)
				} else {
					// Otherwise, the line goes up, and thus will intersect the top side of the
					// tile.
					yclip = scaleUp(0)
					flip = scaleUp(1)
				}

				lastPacked := packed0
				// For the first tile, as well as all subsequent tiles that are intersected
				// at the top and bottom, calculate the x intersection points and push the
				// corresponding tiles.
				//
				// Note: This could perhaps be SIMD-optimized, but initial experiments suggest
				// that in the vast majority of cases the number of tiles is between 0-5, so
				// it's probably not really worth it.
				for i := range tileCountY - 1 {
					// Calculate the next x intersection point.
					xclip := xclip0 + float32(i)*sign*invSlope
					// The max(1) is necessary to indicate that the point actually crosses the
					// edge instead of ending at it. Perhaps we can figure out a different way
					// to represent this.
					xfrac := max(scaleUp(xclip), nudgeFactor)
					packed := vec2{xfrac, yclip}
					pushTile(x, y, lastPacked, packed)

					// Flip y between top and bottom of tile (i.e. from TILE_HEIGHT
					// to 0 or 0 to TILE_HEIGHT).
					lastPacked = packed
					lastPacked = vec2{packed.x, packed.y + flip}
					y += sign
				}

				// Push the last tile, which might be at a fractional y offset.
				yfrac1 := scaleUp(s1.y - y)
				packed1 := vec2{xfrac1, yfrac1}

				pushTile(x, y, lastPacked, packed1)
			}
		} else if tileCountY == 1 {
			// A horizontal row.
			// Same explanations apply as above, but instead in the horizontal direction.

			slope := (s1.y - s0.y) / (s1.x - s0.x)
			sign := sign32(s1.x - s0.x)

			yclip0 := (s0.y - y) + (x-s0.x)*slope
			var xclip, flip float32
			if sign > 0.0 {
				yclip0 += slope
				xclip = scaleUp(1.0)
				flip = scaleUp(-1.0)
			} else {
				xclip = scaleUp(0.0)
				flip = scaleUp(1.0)
			}

			lastPacked := packed0

			for i := range tileCountX - 1 {
				yclip := yclip0 + float32(i)*sign*slope
				yfrac := max(scaleUp(yclip), nudgeFactor)
				packed := vec2{xclip, yfrac}
				pushTile(x, y, lastPacked, packed)
				lastPacked = vec2{packed.x + flip, packed.y}
				x += sign
			}

			xfrac1 := scaleUp(s1.x - x)
			yfrac1 := scaleUp(s1.y - y)
			packed1 := vec2{xfrac1, yfrac1}
			pushTile(x, y, lastPacked, packed1)
		} else {
			// General case (i.e. more than one tile covered in both directions). We perform a DDA
			// to "walk" along the path and find out which tiles are intersected by the line
			// and at which positions.

			recipDx := 1.0 / (s1.x - s0.x)
			signx := sign32(s1.x - s0.x)
			recipDy := 1.0 / (s1.y - s0.y)
			signy := sign32(s1.y - s0.y)

			// How much we advance at each intersection with a vertical grid line.
			tClipX := (x - s0.x) * recipDx

			// Similarly to the case "horizontal column", if the line goes to the right,
			// we will always intersect the tiles on the right side (except for perhaps the last
			// tile, but this case is handled separately in the end). Otherwise, we always intersect
			// on the left side.
			var xclip, flipX float32
			if signx > 0.0 {
				tClipX += recipDx
				xclip = scaleUp(1.0)
				flipX = scaleUp(-1.0)
			} else {
				xclip = scaleUp(0.0)
				flipX = scaleUp(1.0)
			}

			// How much we advance at each intersection with a horizontal grid line.
			tClipY := (y - s0.y) * recipDy

			// Same as xclip, but for the vertical direction, analogously to the
			// "vertical column" case.
			var yclip, flipY float32
			if signy > 0.0 {
				tClipY += recipDy
				yclip = scaleUp(1.0)
				flipY = scaleUp(-1.0)
			} else {
				yclip = scaleUp(0.0)
				flipY = scaleUp(1.0)
			}

			// x and y coordinates of the target tile.
			x1 := x + float32(tileCountX-1)*signx
			y1 := y + float32(tileCountY-1)*signy
			xi := x
			yi := y
			lastPacked := packed0

			for {
				// See
				// https://github.com/LaurenzV/cpu-sparse-experiments/issues/46
				// for why we don't just use an inequality check.
				var xcond, ycond bool
				if signx > 0.0 {
					xcond = xi >= x1
				} else {
					xcond = xi <= x1
				}
				if signy > 0.0 {
					ycond = yi >= y1
				} else {
					ycond = yi <= y1
				}
				if xcond && ycond {
					break
				}

				if tClipY < tClipX {
					// intersected with horizontal grid line
					xIntersect := s0.x + (s1.x-s0.x)*tClipY - xi
					xfrac := max(scaleUp(xIntersect), nudgeFactor)
					packed := vec2{xfrac, yclip}
					pushTile(xi, yi, lastPacked, packed)
					tClipY += abs32(recipDy)
					yi += signy
					lastPacked = vec2{packed.x, packed.y + flipY}
				} else {
					// intersected with vertical grid line
					yIntersect := s0.y + (s1.y-s0.y)*tClipX - yi
					yfrac := max(scaleUp(yIntersect), nudgeFactor)
					packed := vec2{xclip, yfrac}
					pushTile(xi, yi, lastPacked, packed)
					tClipX += abs32(recipDx)
					xi += signx
					lastPacked = vec2{packed.x + flipX, packed.y}
				}
			}

			// The last tile, where the end point is possibly not at an integer coordinate.
			xfrac1 := scaleUp(s1.x - xi)
			yfrac1 := scaleUp(s1.y - yi)
			packed1 := vec2{xfrac1, yfrac1}

			pushTile(xi, yi, lastPacked, packed1)
		}
	}
	return tileBuf
}

func (t *tile) delta() int {
	var a, b int
	if t.p1.y == 0 {
		a = 1
	}
	if t.p0.y == 0 {
		b = 1
	}
	return a - b
}

func (t *tile) cmp(b *tile) int {
	if n := cmp.Compare(t.y, b.y); n != 0 {
		return n
	}
	return cmp.Compare(t.x, b.x)
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

func scaleUp(z float32) float32 {
	return z * tileWidthScale
}

func scaleDown(z vec2) vec2 {
	return vec2{
		z.x * invTileWidthScale,
		z.y * invTileHeightScale,
	}
}
