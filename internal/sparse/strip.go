// SPDX-FileCopyrightText: 2024 the Piet Authors
// SPDX-FileCopyrightText: 2025 Dominik Honnef and contributors
//
// SPDX-License-Identifier: MIT

package sparse

import (
	"fmt"
	"structs"
)

// The height of a strip.
// Requirement: stripHeight * 16 % 32 == 0
// Requirement: stripHeight >= widest vectorized load we use
const stripHeight = 4

type loc struct {
	x int32
	y uint16
}

func (l loc) sameStrip(other loc) bool {
	abs := func(x int32) int32 {
		if x < 0 {
			x = -x
		}
		return x
	}
	return l.sameRow(other) && abs(other.x-l.x) <= 1
}

func (l loc) sameRow(other loc) bool {
	return l.y == other.y
}

func (t tile) String() string {
	return fmt.Sprintf("(%d, %d) = %s--%s", t.x, t.y, t.p0, t.p1)
}

type strip struct {
	_ structs.HostLayout

	x       int32
	y       uint16
	col     uint32
	winding int32
}

func (s *strip) stripY() uint16 {
	return s.y / stripHeight
}

func (s strip) String() string {
	return fmt.Sprintf("strip(x=%v, y=%v, col=%v, winding=%v)",
		s.x, s.y, s.col, s.winding)
}

func (t tile) loc() loc {
	return loc{
		x: t.x,
		y: t.y,
	}
}

func clamp[T float32 | int32](v, low, high T) T {
	// The if/elses are cheaper than min(max(v, low), high) because min/max are
	// stricter about NaNs.
	if v < low {
		return low
	} else if v > high {
		return high
	} else {
		return v
	}
}

func renderStripsScalar(
	tiles []tile,
	fillRule FillRule,
	stripBuf []strip,
	alphaBuf [][stripHeight]uint8,
) ([]strip, [][stripHeight]uint8) {
	stripBuf = stripBuf[:0]

	stripStart := true
	cols := uint32(len(alphaBuf))
	prevTile := &tiles[0]
	fp := prevTile.footprint()
	segStart := 0
	delta := 0

	// Note: the input should contain a sentinel tile, to avoid having
	// logic here to process the final strip.
	for i := 1; i < len(tiles); i++ {
		curTile := &tiles[i]

		if !prevTile.sameLoc(curTile) {
			startDelta := delta
			sameStrip := prevTile.prevLoc(curTile)

			if sameStrip {
				fp = fp.extend(3)
			}

			x0 := fp.x0()
			x1 := fp.x1()
			var area [tileWidth]float32
			for i := range area {
				area[i] = float32(startDelta)
			}
			var areas [tileHeight][tileWidth]float32
			for i := range areas {
				areas[i] = area
			}

			for j := segStart; j < i; j++ {
				tile := &tiles[j]

				delta += tile.delta()

				p0 := tile.p0
				p1 := tile.p1
				invSlope := (p1.x - p0.x) / (p1.y - p0.y)

				if x0 >= x1 {
					continue
				}
				_ = areas[x0]
				_ = areas[x1-1]
				for x := x0; x < x1; x++ {
					// Relative x offset of the start point from the current
					// column.
					relX := p0.x - float32(x)
					for y := range stripHeight {
						// Relative y offset of the start point from the current
						// row.
						relY := p0.y - float32(y)
						// y values will be 1 if the point is below the current
						// row, 0 if the point is above the current row, and
						// between 0-1 if it is on the same row.
						y0 := clamp(relY, 0, 1)
						y1 := clamp(p1.y-float32(y), 0, 1)
						// If != 0, the line intersects the current row in the
						// current tile.
						dy := y0 - y1

						if dy != 0.0 {
							// x intersection points in the current tile.
							xx0 := relX + (y0-relY)*invSlope
							xx1 := relX + (y1-relY)*invSlope
							xmin0 := min(xx0, xx1)
							xmax := max(xx0, xx1)
							// Subtract a small delta to prevent a division by zero
							// below.
							xmin := min(xmin0, 1.0) - 1e-6
							// Clip xmax to the right side of the pixel.
							b := min(xmax, 1.0)
							// Clip xmax to the left side of the pixel.
							c := max(b, 0.0)
							// Clip xmin to the left side of the pixel.
							d := max(xmin, 0.0)
							// Calculate the covered area.
							a := (b + 0.5*(d*d-c*c) - xmin) / (xmax - xmin)

							areas[x][y] += a * dy
						}

						if p0.x == 0.0 {
							areas[x][y] += clamp(float32(y)-p0.y+1.0, 0.0, 1.0)
						} else if p1.x == 0.0 {
							areas[x][y] -= clamp(float32(y)-p1.y+1.0, 0.0, 1.0)
						}
					}
				}
			}

			switch fillRule {
			case NonZero:
				for x := x0; x < x1; x++ {
					var alphas [stripHeight]uint8
					for y := range stripHeight {
						area := areas[x][y]
						coverage := min(abs32(area), 1.0)
						areaU8 := satConv[uint8](coverage*255.0 + 0.5)
						alphas[y] = areaU8
					}
					alphaBuf = append(alphaBuf, alphas)
				}
			case EvenOdd:
				for x := x0; x < x1; x++ {
					var alphas [stripHeight]uint8
					for y := range stripHeight {
						area := areas[x][y]
						coverage := abs32(area - 2.0*floor32((0.5*area)+0.5))
						areaU8 := satConv[uint8](coverage*255.0 + 0.5)
						alphas[y] = areaU8
					}
					alphaBuf = append(alphaBuf, alphas)
				}
			}

			if stripStart {
				strip := strip{
					x:       4*prevTile.x + int32(x0),
					y:       4 * prevTile.y,
					col:     cols,
					winding: int32(startDelta),
				}
				stripBuf = append(stripBuf, strip)
			}

			cols += x1 - x0
			if sameStrip {
				fp = footprintFromIndex(0)
			} else {
				fp = 0
			}
			stripStart = !sameStrip
			segStart = i
			if !prevTile.sameRow(curTile) {
				delta = 0
			}
		}
		fp = fp.merge(curTile.footprint())
		prevTile = curTile
	}

	return stripBuf, alphaBuf
}
