// SPDX-FileCopyrightText: 2024 the Piet Authors
// SPDX-FileCopyrightText: 2025 Dominik Honnef and contributors
//
// SPDX-License-Identifier: MIT

package sparse

import (
	"iter"
	"math"
)

const TILE_WIDTH = 4
const TILE_HEIGHT = 4

const TILE_SCALE_X = 1.0 / TILE_WIDTH
const TILE_SCALE_Y = 1.0 / TILE_HEIGHT

// / This is just Line but f32
type flatLine struct {
	// should these be vec2?
	p0 [2]float32
	p1 [2]float32
}

type vec2 struct {
	x, y float32
}

const TILE_SCALE = 8192

func unpackVec2(packed uint32) vec2 {
	x := float32(packed&0xFFFF) * (1.0 / TILE_SCALE)
	y := float32(packed>>16) * (1.0 / TILE_SCALE)
	return vec2{x, y}
}

// scale factor relative to unit square in tile
const FRAC_TILE_SCALE = TILE_SCALE * 4

func scaleUp(z float32) uint32 {
	v := math.Round(float64(z * FRAC_TILE_SCALE))
	return satConv[uint32](v)
}

// Note: this assumes values in range.
func (v vec2) pack() uint32 {
	// TODO: scale should depend on tile size
	x := satConv[uint32](math.Round(float64(v.x * TILE_SCALE)))
	y := satConv[uint32](math.Round(float64(v.y * TILE_SCALE)))
	return (y << 16) + x
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

func makeTiles(lines iter.Seq[flatLine], tile_buf *[]tile) {
	*tile_buf = (*tile_buf)[:0]
	for line := range lines {
		p0 := vec2{
			x: line.p0[0],
			y: line.p0[1],
		}
		p1 := vec2{
			x: line.p1[0],
			y: line.p1[1],
		}
		s0 := p0.mul(TILE_SCALE_X)
		s1 := p1.mul(TILE_SCALE_Y)
		count_x := span(s0.x, s1.x)
		count_y := span(s0.y, s1.y)
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
		packed0 := (yfrac0 << 16) + xfrac0

		// These could be replaced with <2 and the max(1.0) in span removed
		if count_x == 1 {
			xfrac1 := scaleUp(s1.x - x)
			if count_y == 1 {
				yfrac1 := scaleUp(s1.y - y)
				packed1 := (yfrac1 << 16) + xfrac1
				// 1x1 tile
				*tile_buf = append(*tile_buf, tile{
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
				var yclip uint32
				if sign > 0.0 {
					xclip0 += slope
					yclip = scaleUp(1.0)
				}
				last_packed := packed0
				for i := range count_y - 1 {
					xclip := xclip0 + float32(i)*sign*slope
					xfrac := max(scaleUp(xclip), 1)
					packed := (yclip << 16) + xfrac
					*tile_buf = append(*tile_buf, tile{
						x:  satConv[uint16](x),
						y:  satConv[uint16](y + float32(i)*sign),
						p0: last_packed,
						p1: packed,
					})
					// flip y between top and bottom of tile
					last_packed = packed ^ (FRAC_TILE_SCALE << 16)
				}
				yfrac1 := scaleUp(s1.y - (y + float32(count_y-1)*sign))
				packed1 := (yfrac1 << 16) + xfrac1

				*tile_buf = append(*tile_buf, tile{
					x:  satConv[uint16](x),
					y:  satConv[uint16](y + float32(count_y-1)*sign),
					p0: last_packed,
					p1: packed1,
				})
			}
		} else if count_y == 1 {
			// horizontal row
			slope := (s1.y - s0.y) / (s1.x - s0.x)
			sign := sign32(s1.x - s0.x)
			yclip0 := (s0.y - y) + (x-s0.x)*slope
			var xclip uint32
			if sign > 0.0 {
				yclip0 += slope
				xclip = scaleUp(1.0)
			}
			last_packed := packed0
			for i := range count_x - 1 {
				yclip := yclip0 + float32(i)*sign*slope
				yfrac := max(scaleUp(yclip), 1)
				packed := (yfrac << 16) + xclip
				*tile_buf = append(*tile_buf, tile{
					x:  satConv[uint16](x + float32(i)*sign),
					y:  satConv[uint16](y),
					p0: last_packed,
					p1: packed,
				})
				// flip x between left and right of tile
				last_packed = packed ^ FRAC_TILE_SCALE
			}
			xfrac1 := scaleUp(s1.x - (x + float32(count_x-1)*sign))
			yfrac1 := scaleUp(s1.y - y)
			packed1 := (yfrac1 << 16) + xfrac1

			*tile_buf = append(*tile_buf, tile{
				x:  satConv[uint16](x + float32(count_x-1)*sign),
				y:  satConv[uint16](y),
				p0: last_packed,
				p1: packed1,
			})
		} else {
			// general case
			recip_dx := 1.0 / (s1.x - s0.x)
			signx := sign32(s1.x - s0.x)
			recip_dy := 1.0 / (s1.y - s0.y)
			signy := sign32(s1.y - s0.y)
			// t parameter for next intersection with a vertical grid line
			t_clipx := (x - s0.x) * recip_dx
			var xclip uint32
			if signx > 0.0 {
				t_clipx += recip_dx
				xclip = scaleUp(1.0)
			}
			// t parameter for next intersection with a horizontal grid line
			t_clipy := (y - s0.y) * recip_dy
			var yclip uint32
			if signy > 0.0 {
				t_clipy += recip_dy
				yclip = scaleUp(1.0)
			}
			x1 := x + float32(count_x-1)*signx
			y1 := y + float32(count_y-1)*signy
			xi := x
			yi := y
			last_packed := packed0
			count := 0
			for xi != x1 || yi != y1 {
				count++

				if t_clipy < t_clipx {
					// intersected with horizontal grid line
					x_intersect := s0.x + (s1.x-s0.x)*t_clipy - xi
					xfrac := max(scaleUp(x_intersect), 1) // maybe should clamp?
					packed := (yclip << 16) + xfrac
					*tile_buf = append(*tile_buf, tile{
						x:  satConv[uint16](xi),
						y:  satConv[uint16](yi),
						p0: last_packed,
						p1: packed,
					})
					t_clipy += abs32(recip_dy)
					yi += signy
					last_packed = packed ^ (FRAC_TILE_SCALE << 16)
				} else {
					// intersected with vertical grid line
					y_intersect := s0.y + (s1.y-s0.y)*t_clipx - yi
					yfrac := max(scaleUp(y_intersect), 1) // maybe should clamp?
					packed := (yfrac << 16) + xclip
					*tile_buf = append(*tile_buf, tile{
						x:  satConv[uint16](xi),
						y:  satConv[uint16](yi),
						p0: last_packed,
						p1: packed,
					})
					t_clipx += abs32(recip_dx)
					xi += signx
					last_packed = packed ^ FRAC_TILE_SCALE
				}
			}
			xfrac1 := scaleUp(s1.x - xi)
			yfrac1 := scaleUp(s1.y - yi)
			packed1 := (yfrac1 << 16) + xfrac1

			*tile_buf = append(*tile_buf, tile{
				x:  satConv[uint16](xi),
				y:  satConv[uint16](yi),
				p0: last_packed,
				p1: packed1,
			})
		}
	}
	// This particular choice of sentinel tiles generates a sentinel strip.
	*tile_buf = append(*tile_buf, tile{
		x:  0x3ffd,
		y:  0x3fff,
		p0: 0,
		p1: 0,
	})
	*tile_buf = append(*tile_buf, tile{
		x:  0x3fff,
		y:  0x3fff,
		p0: 0,
		p1: 0,
	})
}
