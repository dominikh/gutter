// SPDX-FileCopyrightText: 2024 the Piet Authors
// SPDX-FileCopyrightText: 2025 Dominik Honnef and contributors
//
// SPDX-License-Identifier: MIT

package sparse

import (
	"fmt"
	"math"
	"structs"
)

// The height of a strip.
// Requirement: stripHeight * 16 % 32 == 0
// Requirement: stripHeight >= widest vectorized load we use
const stripHeight = 4

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

func min32(a, b float32) float32 {
	// Unlike Go's min, this doesn't try to preserve NaN.

	if a != a {
		return b
	}
	if b != b {
		return a
	}

	if a <= b {
		return a
	} else {
		return b
	}
}

func max32(a, b float32) float32 {
	// Unlike Go's max, this doesn't try to preserve NaN.

	if a != a {
		return b
	}
	if b != b {
		return a
	}
	if a >= b {
		return a
	} else {
		return b
	}
}

func renderStripsScalar(
	tiles []tile,
	fillRule FillRule,
	lines []flatLine,
	stripBuf []strip,
	alphaBuf [][stripHeight]uint8,
) ([]strip, [][stripHeight]uint8) {
	stripBuf = stripBuf[:0]

	if len(tiles) == 0 {
		return stripBuf, alphaBuf
	}

	// The accumulated tile winding delta. A line that crosses the top edge of a tile
	// increments the delta if the line is directed upwards, and decrements it if goes
	// downwards. Horizontal lines leave it unchanged.
	var windingDelta int32

	// The previous tile visited.
	prevTile := tiles[0]
	// The accumulated (fractional) winding of the tile-sized location we're currently at.
	// Note multiple tiles can be at the same location.
	var locationWinding [tileWidth][tileHeight]float32
	// The accumulated (fractional) windings at this location's right edge. When we move to the
	// next location, this is splatted to that location's starting winding.
	var accumulatedWinding [tileHeight]float32

	strip_ := strip{
		x:       prevTile.x * tileWidth,
		y:       prevTile.y * tileHeight,
		col:     uint32(len(alphaBuf)),
		winding: 0,
	}

	for i := range len(tiles) + 1 {
		var tile_ tile
		if i < len(tiles) {
			tile_ = tiles[i]
		} else {
			tile_ = tile{
				x: math.MaxInt32,
				y: math.MaxUint16,
			}
		}

		line := lines[tile_.lineIdx]
		tileLeftX := float32(tile_.x) * tileWidth
		tileTopY := float32(tile_.y) * tileHeight
		p0_x := line.p0.x - tileLeftX
		p0_y := line.p0.y - tileTopY
		p1_x := line.p1.x - tileLeftX
		p1_y := line.p1.y - tileTopY

		// Push out the winding as an alpha mask when we move to the next location (i.e., a tile
		// without the same location).
		if !prevTile.sameLoc(&tile_) {
			switch fillRule {
			case NonZero:
				for x := range tileWidth {
					var alphas [stripHeight]uint8
					for y := range tileHeight {
						area := locationWinding[x][y]
						coverage := min32(abs32(area), 1.0)
						areaU8 := satConv[uint8](coverage*255.0 + 0.5)
						alphas[y] = areaU8
					}
					alphaBuf = append(alphaBuf, alphas)
				}
			case EvenOdd:
				for x := range tileWidth {
					var alphas [stripHeight]uint8
					for y := range tileHeight {
						area := locationWinding[x][y]
						coverage := abs32(area - 2.0*floor32((0.5*area)+0.5))
						areaU8 := satConv[uint8](coverage*255.0 + 0.5)
						alphas[y] = areaU8
					}
					alphaBuf = append(alphaBuf, alphas)
				}
			}

			for x := range tileWidth {
				locationWinding[x] = accumulatedWinding
			}
		}

		// Push out the strip if we're moving to a next strip.
		if !prevTile.sameLoc(&tile_) && !prevTile.prevLoc(&tile_) {
			if !prevTile.sameRow(&tile_) {
				windingDelta = 0
			}

			if a, b := (prevTile.x+1)*tileWidth-strip_.x, int32(len(alphaBuf))-int32(strip_.col); a != b {
				panic(fmt.Sprintf("%d != %d", a, b))
			}
			stripBuf = append(stripBuf, strip_)

			isSentinel := i == len(tiles)
			if !prevTile.sameRow(&tile_) {
				// Emit a final strip in the row if there is non-zero winding
				// for the sparse fill, or unconditionally if we've reached the
				// sentinel tile to end the path (the col field is used for
				// width calculations).
				if windingDelta != 0 || isSentinel {
					stripBuf = append(stripBuf, strip{
						x:       math.MaxInt32,
						y:       prevTile.y * tileHeight,
						col:     uint32(len(alphaBuf)),
						winding: windingDelta,
					})
				}

				windingDelta = 0
				clear(accumulatedWinding[:])
				clear(locationWinding[:])
			}
			if isSentinel {
				break
			}

			strip_ = strip{
				x:       tile_.x * tileWidth,
				y:       tile_.y * tileHeight,
				col:     uint32(len(alphaBuf)),
				winding: windingDelta,
			}

			// Note: this fill is mathematically not necessary. It provides a way to reduce
			// accumulation of float round errors.
			for i := range accumulatedWinding {
				accumulatedWinding[i] = float32(windingDelta)
			}
		}
		prevTile = tile_

		// TODO: horizontal geometry has no impact on winding. This branch will be removed when
		// horizontal geometry is culled at the tile-generation stage.
		if p0_y == p1_y {
			continue
		}

		// Lines moving upwards (in a y-down coordinate system) add to winding; lines moving
		// downwards subtract from winding.
		sign := sign32(p0_y - p1_y)

		// Calculate winding / pixel area coverage.
		//
		// Conceptually, horizontal rays are shot from left to right. Every time the ray crosses a
		// line that is directed upwards (decreasing `y`), the winding is incremented. Every time
		// the ray crosses a line moving downwards (increasing `y`), the winding is decremented.
		// The fractional area coverage of a pixel is the integral of the winding within it.
		//
		// Practically, to calculate this, each pixel is considered individually, and we determine
		// whether the line moves through this pixel. The line's y-delta within this pixel is
		// accumulated and added to the area coverage of pixels to the right. Within the pixel
		// itself, the area to the right of the line segment forms a trapezoid (or a triangle in
		// the degenerate case). The area of this trapezoid is added to the pixel's area coverage.
		//
		// For example, consider the following pixel square, with a line indicated by asterisks
		// starting inside the pixel and crossing its bottom edge. The area covered is the
		// trapezoid on the bottom-right enclosed by the line and the pixel square. The area is
		// positive if the line moves down, and negative otherwise.
		//
		//  __________________
		//  |                |
		//  |         *------|
		//  |        *       |
		//  |       *        |
		//  |      *         |
		//  |     *          |
		//  |    *           |
		//  |___*____________|
		//     *
		//    *

		var line_top_y, line_top_x, line_bottom_y, line_bottom_x float32
		if p0_y < p1_y {
			line_top_y, line_top_x, line_bottom_y, line_bottom_x = p0_y, p0_x, p1_y, p1_x
		} else {
			line_top_y, line_top_x, line_bottom_y, line_bottom_x = p1_y, p1_x, p0_y, p0_x
		}

		var lineLeftX, lineLeftY, lineRightX float32
		if p0_x < p1_x {
			lineLeftX = p0_x
			lineLeftY = p0_y
			lineRightX = p1_x
		} else {
			lineLeftX = p1_x
			lineLeftY = p1_y
			lineRightX = p0_x
		}

		y_slope := (line_bottom_y - line_top_y) / (line_bottom_x - line_top_x)
		x_slope := 1.0 / y_slope

		if tile_.winding {
			windingDelta += int32(sign)
		}

		// TODO: this should be removed when out-of-viewport tiles are culled at the
		// tile-generation stage. That requires calculating and forwarding winding to strip
		// generation.
		if tile_.x == 0 && lineLeftX < 0.0 {
			var ymin, ymax float32
			if line.p0.x == line.p1.x {
				ymin = line_top_y
				ymax = line_bottom_y
			} else {
				lineViewportLeftY := min32(max32(line_top_y-line_top_x*y_slope, line_top_y), line_bottom_y)

				ymin = min32(lineLeftY, lineViewportLeftY)
				ymax = max32(lineLeftY, lineViewportLeftY)
			}

			for yIdx := range tileHeight {
				px_top_y := float32(yIdx)
				px_bottom_y := 1.0 + float32(yIdx)

				ymin := max32(ymin, px_top_y)
				ymax := min32(ymax, px_bottom_y)

				h := max32(ymax-ymin, 0)
				accumulatedWinding[yIdx] += sign * h

				for xIdx := range tileWidth { // XXX
					locationWinding[xIdx][yIdx] += sign * h
				}
			}

			if lineRightX < 0. {
				// Early exit, as no part of the line is inside the tile.
				continue
			}
		}

		for y_idx := range tileHeight {
			px_top_y := float32(y_idx)
			px_bottom_y := 1.0 + float32(y_idx)

			ymin := max32(line_top_y, px_top_y)
			ymax := min32(line_bottom_y, px_bottom_y)

			acc := float32(0)
			for x_idx := range tileWidth {
				px_left_x := float32(x_idx)
				px_right_x := 1.0 + float32(x_idx)

				// The y-coordinate of the intersections between line and the pixel's left and
				// right edges respectively.
				//
				// There is some subtlety going on here: `y_slope` will usually be finite, but will
				// be `inf` for purely vertical lines (`p0_x == p1_x`).
				//
				// In the case of `inf`, the resulting slope calculation will be `-inf` or `inf`
				// depending on whether the pixel edge is left or right of the line, respectively
				// (from the viewport's coordinate system perspective). The `min` and `max`
				// y-clamping logic generalizes nicely, as a pixel edge to the left of the line is
				// clamped to `ymin`, and a pixel edge to the right is clamped to `ymax`.
				//
				// In the special case where a vertical line and pixel edge are at the exact same
				// x-position (collinear), the line belongs to the pixel on whose _left_ edge it is
				// situated. The resulting slope calculation for the edge the line is situated on
				// will be NaN, as `0 * inf` results in NaN. This is true for both the left and
				// right edge. In both cases, the call to `f32::max` will set this to `ymin`.
				line_px_left_y := min32(max32(line_top_y+(px_left_x-line_top_x)*y_slope, ymin), ymax)
				line_px_right_y := min32(max32(line_top_y+(px_right_x-line_top_x)*y_slope, ymin), ymax)

				// `x_slope` is always finite, as horizontal geometry is elided.
				line_px_left_yx := line_top_x + (line_px_left_y-line_top_y)*x_slope
				line_px_right_yx := line_top_x + (line_px_right_y-line_top_y)*x_slope
				h := abs32(line_px_right_y - line_px_left_y)

				// The trapezoidal area enclosed between the line and the right edge of the pixel
				// square.
				area := 0.5 * h * (2.*px_right_x - line_px_right_yx - line_px_left_yx)
				locationWinding[x_idx][y_idx] += acc + sign*area
				acc += sign * h
			}
			accumulatedWinding[y_idx] += acc
		}
	}

	return stripBuf, alphaBuf
}
