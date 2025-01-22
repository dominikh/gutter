// SPDX-FileCopyrightText: 2024 the Piet Authors
// SPDX-FileCopyrightText: 2025 Dominik Honnef and contributors
//
// SPDX-License-Identifier: MIT

package sparse

import (
	"fmt"
	"iter"
	"math"
)

const (
	tileWidth  = 4
	tileHeight = 4
	tileScaleX = 1.0 / tileWidth
	tileScaleY = 1.0 / tileHeight
	tileScale  = 8192

	// scale factor relative to unit square in tile
	fracTileScale = tileScale * 4
)

type flatLine struct {
	p0 vec2
	p1 vec2
}

type vec2 struct {
	x, y float32
}

func (v vec2) String() string {
	return fmt.Sprintf("(%g, %g)", v.x, v.y)
}

func scaleUp(z float32) uint16 {
	v := math.Round(float64(z * fracTileScale))
	return satConv[uint16](v)
}

func (v vec2) add(o vec2) vec2 {
	return vec2{
		x: v.x + o.x,
		y: v.y + o.y,
	}
}

func (v vec2) sub(o vec2) vec2 {
	return vec2{
		x: v.x - o.x,
		y: v.y - o.y,
	}
}

func (v vec2) mul(f float32) vec2 {
	return vec2{
		x: v.x * f,
		y: v.y * f,
	}
}

func span(a, b float32) uint32 {
	return satConv[uint32](max(ceil32(max(a, b))-floor32(min(a, b)), 1))
}

func floor32(f float32) float32 {
	return float32(math.Floor(float64(f)))
}

func ceil32(f float32) float32 {
	return float32(math.Ceil(float64(f)))
}

func abs32(f float32) float32 {
	return float32(math.Abs(float64(f)))
}

func sign32(f float32) float32 {
	if math.Signbit(float64(f)) {
		// f is -0.0 or negative
		return -1
	} else {
		return 1
	}
}

func makeTiles(lines iter.Seq[flatLine], tileBuf []tile) []tile {
	tileBuf = tileBuf[:0]
	for line := range lines {
		s0 := line.p0.mul(tileScaleX)
		s1 := line.p1.mul(tileScaleY)
		countX := span(s0.x, s1.x)
		countY := span(s0.y, s1.y)
		x := floor32(s0.x)
		if s0.x == x && s1.x < x {
			// s0.x is on right side of first tile
			x -= 1.0
		}
		y := floor32(s0.y)
		if s0.y == y && s1.y < y {
			// s0.y is on bottom of first tile
			y -= 1.0
		}
		xfrac0 := scaleUp(s0.x - x)
		yfrac0 := scaleUp(s0.y - y)
		packed0 := vec16{xfrac0, yfrac0}

		// These could be replaced with <2 and the max(1.0) in span removed
		if countX == 1 {
			xfrac1 := scaleUp(s1.x - x)
			if countY == 1 {
				yfrac1 := scaleUp(s1.y - y)
				packed1 := vec16{xfrac1, yfrac1}
				// 1x1 tile
				tileBuf = append(tileBuf, tile{
					x:  satConv[uint16](x),
					y:  satConv[uint16](y),
					p0: packed0,
					p1: packed1,
				})
			} else {
				// vertical column
				slope := (s1.x - s0.x) / (s1.y - s0.y)
				sign := sign32(s1.y - s0.y)
				xclip0 := (s0.x - x) + (y-s0.y)*slope
				var yclip uint16
				if sign > 0.0 {
					xclip0 += slope
					yclip = scaleUp(1.0)
				}
				lastPacked := packed0
				for i := range countY - 1 {
					xclip := xclip0 + float32(i)*sign*slope
					xfrac := max(scaleUp(xclip), 1)
					packed := vec16{xfrac, yclip}
					tileBuf = append(tileBuf, tile{
						x:  satConv[uint16](x),
						y:  satConv[uint16](y + float32(i)*sign),
						p0: lastPacked,
						p1: packed,
					})
					// flip y between top and bottom of tile
					lastPacked = packed
					lastPacked.y ^= fracTileScale
				}
				yfrac1 := scaleUp(s1.y - (y + float32(countY-1)*sign))
				packed1 := vec16{xfrac1, yfrac1}

				tileBuf = append(tileBuf, tile{
					x:  satConv[uint16](x),
					y:  satConv[uint16](y + float32(countY-1)*sign),
					p0: lastPacked,
					p1: packed1,
				})
			}
		} else if countY == 1 {
			// horizontal row
			slope := (s1.y - s0.y) / (s1.x - s0.x)
			sign := sign32(s1.x - s0.x)
			yclip0 := (s0.y - y) + (x-s0.x)*slope
			var xclip uint16
			if sign > 0.0 {
				yclip0 += slope
				xclip = scaleUp(1.0)
			}
			lastPacked := packed0
			for i := range countX - 1 {
				yclip := yclip0 + float32(i)*sign*slope
				yfrac := max(scaleUp(yclip), 1)
				packed := vec16{xclip, yfrac}
				tileBuf = append(tileBuf, tile{
					x:  satConv[uint16](x + float32(i)*sign),
					y:  satConv[uint16](y),
					p0: lastPacked,
					p1: packed,
				})
				// flip x between left and right of tile
				lastPacked = packed
				lastPacked.x ^= fracTileScale
			}
			xfrac1 := scaleUp(s1.x - (x + float32(countX-1)*sign))
			yfrac1 := scaleUp(s1.y - y)
			packed1 := vec16{xfrac1, yfrac1}

			tileBuf = append(tileBuf, tile{
				x:  satConv[uint16](x + float32(countX-1)*sign),
				y:  satConv[uint16](y),
				p0: lastPacked,
				p1: packed1,
			})
		} else {
			// general case
			recipDx := 1.0 / (s1.x - s0.x)
			signx := sign32(s1.x - s0.x)
			recipDy := 1.0 / (s1.y - s0.y)
			signy := sign32(s1.y - s0.y)
			// t parameter for next intersection with a vertical grid line
			tClipX := (x - s0.x) * recipDx
			var xclip uint16
			if signx > 0.0 {
				tClipX += recipDx
				xclip = scaleUp(1.0)
			}
			// t parameter for next intersection with a horizontal grid line
			tClipY := (y - s0.y) * recipDy
			var yclip uint16
			if signy > 0.0 {
				tClipY += recipDy
				yclip = scaleUp(1.0)
			}
			x1 := x + float32(countX-1)*signx
			y1 := y + float32(countY-1)*signy
			xi := x
			yi := y
			lastPacked := packed0
			count := 0
			for xi != x1 || yi != y1 {
				count++

				if tClipY < tClipX {
					// intersected with horizontal grid line
					xIntersect := s0.x + (s1.x-s0.x)*tClipY - xi
					xfrac := max(scaleUp(xIntersect), 1) // maybe should clamp?
					packed := vec16{xfrac, yclip}
					tileBuf = append(tileBuf, tile{
						x:  satConv[uint16](xi),
						y:  satConv[uint16](yi),
						p0: lastPacked,
						p1: packed,
					})
					tClipY += abs32(recipDy)
					yi += signy
					lastPacked = packed
					lastPacked.y ^= fracTileScale
				} else {
					// intersected with vertical grid line
					yIntersect := s0.y + (s1.y-s0.y)*tClipX - yi
					yfrac := max(scaleUp(yIntersect), 1) // maybe should clamp?
					packed := vec16{xclip, yfrac}
					tileBuf = append(tileBuf, tile{
						x:  satConv[uint16](xi),
						y:  satConv[uint16](yi),
						p0: lastPacked,
						p1: packed,
					})
					tClipX += abs32(recipDx)
					xi += signx
					lastPacked = packed
					lastPacked.x ^= fracTileScale
				}
			}
			xfrac1 := scaleUp(s1.x - xi)
			yfrac1 := scaleUp(s1.y - yi)
			packed1 := vec16{xfrac1, yfrac1}

			tileBuf = append(tileBuf, tile{
				x:  satConv[uint16](xi),
				y:  satConv[uint16](yi),
				p0: lastPacked,
				p1: packed1,
			})
		}
	}
	// This particular choice of sentinel tiles generates a sentinel strip.
	tileBuf = append(tileBuf, tile{
		x: 0x3ffd,
		y: 0x3fff,
	})
	tileBuf = append(tileBuf, tile{
		x: 0x3fff,
		y: 0x3fff,
	})
	return tileBuf
}
