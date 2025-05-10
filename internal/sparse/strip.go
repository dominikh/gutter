// SPDX-FileCopyrightText: 2024 the Piet Authors
// SPDX-FileCopyrightText: 2025 Dominik Honnef and contributors
//
// SPDX-License-Identifier: MIT

package sparse

import (
	"fmt"
	"math"
	"slices"
	"structs"
)

// The height of a strip.
// Requirement: stripHeight * 16 % 32 == 0
// Requirement: stripHeight >= widest vectorized load we use
const stripHeight = tileHeight

type strip struct {
	_ structs.HostLayout

	x       uint16
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
	if a < b {
		return a
	} else {
		return b
	}
}

func max32(a, b float32) float32 {
	// Unlike Go's max, this doesn't try to preserve NaN.
	if a > b {
		return a
	} else {
		return b
	}
}

func renderStripsScalar(
	tiles []tile,
	fillRule FillRule,
	lines []flatLine,
) ([]strip, [][stripHeight]uint8) {
	if len(tiles) == 0 {
		return nil, nil
	}

	// The accumulated tile winding delta. A line that crosses the top edge of a tile
	// increments the delta if the line is directed upwards, and decrements it if goes
	// downwards. Horizontal lines leave it unchanged.
	var windingDelta int32
	var stripBuf []strip
	var alphaBuf [][stripHeight]uint8

	// The previous tile visited.
	prevTile := tiles[0]
	// The accumulated (fractional) winding of the tile-sized location we're currently at.
	// Note multiple tiles can be at the same location.
	var locationWinding [tileWidth][tileHeight]float32
	// The accumulated (fractional) windings at this location's right edge. When we move to the
	// next location, this is splatted to that location's starting winding.
	var accumulatedWinding [tileHeight]float32

	strip_ := strip{
		x:       prevTile.x() * tileWidth,
		y:       prevTile.y() * tileHeight,
		col:     0,
		winding: 0,
	}

	for i := range len(tiles) + 1 {
		var tile_ tile
		if i < len(tiles) {
			tile_ = tiles[i]
		} else {
			tile_ = newTile(math.MaxUint16, math.MaxUint16, 0, false)
		}

		line := lines[tile_.lineIdx()]
		tileLeftX := float32(tile_.x()) * tileWidth
		tileTopY := float32(tile_.y()) * tileHeight
		p0x := line.p0.x - tileLeftX
		p0y := line.p0.y - tileTopY
		p1x := line.p1.x - tileLeftX
		p1y := line.p1.y - tileTopY

		// Push out the winding as an alpha mask when we move to the next location (i.e., a tile
		// without the same location).
		if !prevTile.sameLoc(tile_) {
			switch fillRule {
			case NonZero:
				// OPT(dh): slicing alphaBuf and tail introduces unnecessary bounds checks
				alphaBuf = slices.Grow(alphaBuf, tileWidth)[:len(alphaBuf)+tileWidth]
				tail := alphaBuf[len(alphaBuf)-tileWidth:][:tileWidth]
				computeAlphasNonZeroFp((*[tileWidth][tileHeight]uint8)(tail), &locationWinding)
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
		if !prevTile.sameLoc(tile_) && !prevTile.prevLoc(tile_) {
			if a, b := int((prevTile.x()+1)*tileWidth-strip_.x), len(alphaBuf)-int(strip_.col); a != b {
				panic(fmt.Sprintf("%d != %d", a, b))
			}
			stripBuf = append(stripBuf, strip_)

			isSentinel := i == len(tiles)
			if !prevTile.sameRow(tile_) {
				// Emit a final strip in the row if there is non-zero winding
				// for the sparse fill, or unconditionally if we've reached the
				// sentinel tile to end the path (the col field is used for
				// width calculations).
				if windingDelta != 0 || isSentinel {
					stripBuf = append(stripBuf, strip{
						x:       math.MaxUint16,
						y:       prevTile.y() * tileHeight,
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
				x:       tile_.x() * tileWidth,
				y:       tile_.y() * tileHeight,
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
		if p0y == p1y {
			continue
		}

		// Lines moving upwards (in a y-down coordinate system) add to winding; lines moving
		// downwards subtract from winding.
		sign := sign32(p0y - p1y)

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

		var lineTopY, lineTopX, lineBottomY, lineBottomX float32
		if p0y < p1y {
			lineTopY, lineTopX, lineBottomY, lineBottomX = p0y, p0x, p1y, p1x
		} else {
			lineTopY, lineTopX, lineBottomY, lineBottomX = p1y, p1x, p0y, p0x
		}

		var lineLeftX, lineLeftY, lineRightX float32
		if p0x < p1x {
			lineLeftX = p0x
			lineLeftY = p0y
			lineRightX = p1x
		} else {
			lineLeftX = p1x
			lineLeftY = p1y
			lineRightX = p0x
		}

		ySlope := (lineBottomY - lineTopY) / (lineBottomX - lineTopX)
		xSlope := 1.0 / ySlope

		if tile_.winding() {
			windingDelta += int32(sign)
		}

		// TODO: this should be removed when out-of-viewport tiles are culled at the
		// tile-generation stage. That requires calculating and forwarding winding to strip
		// generation.
		if tile_.x() == 0 && lineLeftX < 0.0 {
			var ymin, ymax float32
			if line.p0.x == line.p1.x {
				ymin = lineTopY
				ymax = lineBottomY
			} else {
				lineViewportLeftY := min32(max32(lineTopY-lineTopX*ySlope, lineTopY), lineBottomY)

				ymin = min32(lineLeftY, lineViewportLeftY)
				ymax = max32(lineLeftY, lineViewportLeftY)
			}

			processOutOfBoundsWindingFp(ymin, ymax, sign, &locationWinding, &accumulatedWinding)

			if lineRightX < 0.0 {
				// Early exit, as no part of the line is inside the tile.
				continue
			}
		}

		computeWindingFp(
			lineTopY,
			lineTopX,
			lineBottomY,
			sign,
			xSlope,
			ySlope,
			&locationWinding,
			&accumulatedWinding,
		)
	}

	return stripBuf, alphaBuf
}

func computeAlphasNonZeroNative(tail *[tileWidth][tileHeight]uint8, locationWinding *[tileWidth][tileHeight]float32) {
	for x := range tileWidth {
		for y := range tileHeight {
			area := locationWinding[x][y]
			coverage := min32(abs32(area), 1.0)
			// We don't need to use satConv here. coverage ∈ [0, 1]
			// and uint8(255.5) == uint8(255).
			tail[x][y] = uint8(coverage*255.0 + 0.5)
		}
	}
}

func processOutOfBoundsWindingNative(
	ymin float32,
	ymax float32,
	sign float32,
	locationWinding *[tileWidth][tileHeight]float32,
	accumulatedWinding *[tileHeight]float32,
) {
	for yIdx := range tileHeight {
		pxTopY := float32(yIdx)
		pxBottomY := 1.0 + float32(yIdx)

		ymin := max32(ymin, pxTopY)
		ymax := min32(ymax, pxBottomY)

		h := max32(ymax-ymin, 0)
		accumulatedWinding[yIdx] += sign * h

		for xIdx := range tileWidth {
			locationWinding[xIdx][yIdx] += sign * h
		}
	}
}

func computeWindingNative(
	lineTopY float32,
	lineTopX float32,
	lineBottomY float32,
	sign float32,
	xSlope float32,
	ySlope float32,
	locationWinding *[tileWidth][tileHeight]float32,
	accumulatedWinding *[tileHeight]float32,
) {
	for yIdx := range tileHeight {
		pxTopY := float32(yIdx)
		pxBottomY := 1.0 + float32(yIdx)

		ymin := max32(lineTopY, pxTopY)
		ymax := min32(lineBottomY, pxBottomY)

		acc := float32(0)
		for xIdx := range tileWidth {
			pxLeftX := float32(xIdx)
			pxRightX := 1.0 + float32(xIdx)

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
			linePxLeftY := min32(max32(lineTopY+(pxLeftX-lineTopX)*ySlope, ymin), ymax)
			linePxRightY := min32(max32(lineTopY+(pxRightX-lineTopX)*ySlope, ymin), ymax)

			// `x_slope` is always finite, as horizontal geometry is elided.
			linePxLeftYX := lineTopX + (linePxLeftY-lineTopY)*xSlope
			linePxRightYX := lineTopX + (linePxRightY-lineTopY)*xSlope
			h := abs32(linePxRightY - linePxLeftY)

			// The trapezoidal area enclosed between the line and the right edge of the pixel
			// square.
			area := 0.5 * h * (2.0*pxRightX - linePxRightYX - linePxLeftYX)
			locationWinding[xIdx][yIdx] += acc + sign*area
			acc += sign * h
		}
		accumulatedWinding[yIdx] += acc
	}
}
