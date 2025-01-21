// SPDX-FileCopyrightText: 2024 the Piet Authors
// SPDX-FileCopyrightText: 2025 Dominik Honnef and contributors
//
// SPDX-License-Identifier: MIT

package sparse

import (
	"cmp"
	"math"
	"math/bits"
	"structs"
)

type loc struct {
	x, y uint16
}

type footprint = uint32

type tile struct {
	x, y   uint16
	p0, p1 uint32
}

type strip struct {
	_ structs.HostLayout

	xy      uint32
	col     uint32
	winding int32
}

func (l loc) sameStrip(other loc) bool {
	return l.sameRow(other) && (other.x-l.x)/2 == 0
}

func (l loc) sameRow(other loc) bool {
	return l.y == other.y
}

func newTile(l loc, fp footprint, delta int32) tile {
	p0 := uint32(bits.TrailingZeros32(fp) * TILE_SCALE)
	if delta == -1 {
		p0 += 65536
	}
	p1 := uint32((32 - bits.LeadingZeros32(fp)) * TILE_SCALE)
	if delta == 1 {
		p1 += 65536
	}
	return tile{
		x:  l.x,
		y:  l.y,
		p0: p0,
		p1: p1,
	}
}

func (t tile) loc() loc {
	return loc{
		x: t.x,
		y: t.y,
	}
}

func (t tile) footprint() footprint {
	x0 := float64(t.p0&0xffff) * (1.0 / TILE_SCALE)
	x1 := float64(t.p1&0xffff) * (1.0 / TILE_SCALE)
	// TODO: On CPU, might be better to do this as fixed point
	xmin := uint32(math.Floor(min(x0, x1)))
	xmax := min(max(xmin+1, uint32(math.Ceil(max(x0, x1)))), TILE_WIDTH)
	return (1 << xmax) - (1 << xmin)
}

func (t tile) delta() int {
	var a, b int
	if t.p1>>16 == 0 {
		a = 1
	}
	if t.p0>>16 == 0 {
		b = 1
	}
	return a - b
}

func (t tile) cmp(b tile) int {
	xya := (uint32(t.y) << 16) + uint32(t.x)
	xyb := (uint32(b.y) << 16) + uint32(b.x)
	return cmp.Compare(xya, xyb)
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

func renderStripsScalar(tiles []tile, strip_buf []strip, alpha_buf []uint32) ([]strip, []uint32) {
	strip_buf = strip_buf[:0]

	strip_start := true
	cols := uint32(len(alpha_buf))
	prev_tile := &tiles[0]
	fp := prev_tile.footprint()
	seg_start := 0
	delta := 0
	// Note: the input should contain a sentinel tile, to avoid having
	// logic here to process the final strip.
	for i := 1; i < len(tiles); i++ {
		tile := &tiles[i]
		if prev_tile.loc() != tile.loc() {
			start_delta := delta
			same_strip := prev_tile.loc().sameStrip(tile.loc())
			if same_strip {
				fp |= 8
			}
			x0 := uint32(bits.TrailingZeros32(fp))
			x1 := uint32(32 - bits.LeadingZeros32(fp))
			area := [4]float32{
				float32(start_delta),
				float32(start_delta),
				float32(start_delta),
				float32(start_delta),
			}
			areas := [4][4]float32{area, area, area, area}

			for j := seg_start; j < i; j++ {
				tile := &tiles[j]
				delta += tile.delta()
				p0 := unpackVec2(tile.p0)
				p1 := unpackVec2(tile.p1)
				slope := (p1.x - p0.x) / (p1.y - p0.y)
				_ = areas[x0]
				_ = areas[x1-1]
				for x := x0; x < x1; x++ {
					startx := p0.x - float32(x)
					for y := range 4 {
						starty := p0.y - float32(y)
						y0 := clamp(starty, 0, 1)
						y1 := clamp(p1.y-float32(y), 0, 1)
						dy := y0 - y1
						if dy != 0.0 {
							xx0 := startx + (y0-starty)*slope
							xx1 := startx + (y1-starty)*slope
							xmin0 := min(xx0, xx1)
							xmax := max(xx0, xx1)
							xmin := min(xmin0, 1.0) - 1e-6
							b := min(xmax, 1.0)
							c := max(b, 0.0)
							d := max(xmin, 0.0)
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
			for x := x0; x < x1; x++ {
				alphas := uint32(0)
				for y := range 4 {
					area := areas[x][y]
					// nonzero winding number rule
					area_u8 := satConv[uint32](math.Round(min(math.Abs(float64(area)), 1.0) * 255.0))
					alphas += area_u8 << (y * 8)
				}
				alpha_buf = append(alpha_buf, alphas)
			}

			if strip_start {
				xy := (1<<18)*uint32(prev_tile.y) + 4*uint32(prev_tile.x) + x0
				strip := strip{
					xy:      xy,
					col:     cols,
					winding: int32(start_delta),
				}
				strip_buf = append(strip_buf, strip)
			}
			cols += x1 - x0
			if same_strip {
				fp = 1
			} else {
				fp = 0
			}
			strip_start = !same_strip
			seg_start = i
			if !prev_tile.loc().sameRow(tile.loc()) {
				delta = 0
			}
		}
		fp |= tile.footprint()
		prev_tile = tile
	}

	return strip_buf, alpha_buf
}

func (s *strip) x() uint32 {
	return s.xy & 0xFFFF
}

func (s *strip) y() uint32 {
	return s.xy / (1 << 16)
}

func (s *strip) strip_y() uint32 {
	return s.xy / ((1 << 16) * STRIP_HEIGHT)
}
