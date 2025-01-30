// SPDX-FileCopyrightText: 2024 the Piet Authors
// SPDX-FileCopyrightText: 2025 Dominik Honnef and contributors
//
// SPDX-License-Identifier: MIT

package sparse

import (
	"cmp"
	"fmt"
	"math"
	"math/bits"
	"structs"
)

const stripHeight = 4

type loc struct {
	x, y uint16
}

// footprint is a bitset representing the pixels covered by a set of tiles. Any
// individual tile will cover a contiguous range of pixels, as each tile
// contains exactly one line segment. However, when multiple tiles are processed
// together, footprints can be ORed together, which may result in gaps.
type footprint = uint32

type tile struct {
	x, y   uint16
	p0, p1 vec16
}

type vec16 struct {
	x, y uint16
}

func (v vec16) float32() vec2 {
	x := float32(v.x) * (1.0 / tileScale)
	y := float32(v.y) * (1.0 / tileScale)
	return vec2{x, y}
}

func (v vec16) String() string {
	return v.float32().String()
}

func (t tile) String() string {
	return fmt.Sprintf("(%d, %d) = %s--%s", t.x, t.y, t.p0, t.p1)
}

type strip struct {
	_ structs.HostLayout

	xy      uint32
	col     uint32
	winding int32
}

func (s strip) String() string {
	return fmt.Sprintf("strip(x=%v, y=%v, col=%v, winding=%v)",
		s.x(), s.y(), s.col, s.winding)
}

func (l loc) sameStrip(other loc) bool {
	return l.sameRow(other) && (other.x-l.x)/2 == 0
}

func (l loc) sameRow(other loc) bool {
	return l.y == other.y
}

func (t tile) loc() loc {
	return loc{
		x: t.x,
		y: t.y,
	}
}

func (t tile) footprint() footprint {
	x0 := float64(t.p0.x) * (1.0 / tileScale)
	x1 := float64(t.p1.x) * (1.0 / tileScale)
	// TODO: On CPU, might be better to do this as fixed point
	xmin := uint32(math.Floor(min(x0, x1)))
	xmax := min(max(xmin+1, uint32(math.Ceil(max(x0, x1)))), tileWidth)
	return (1 << xmax) - (1 << xmin)
}

func (t tile) delta() int {
	var a, b int
	if t.p1.y == 0 {
		a = 1
	}
	if t.p0.y == 0 {
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

func renderStripsScalar(
	tiles []tile,
	fillRule FillRule,
	stripBuf []strip,
	alphaBuf []uint32,
) ([]strip, []uint32) {
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
		tile := &tiles[i]
		if prevTile.loc() != tile.loc() {
			startDelta := delta
			sameStrip := prevTile.loc().sameStrip(tile.loc())
			if sameStrip {
				fp |= 8
			}
			x0 := uint32(bits.TrailingZeros32(fp))
			x1 := uint32(32 - bits.LeadingZeros32(fp))
			area := [4]float32{
				float32(startDelta),
				float32(startDelta),
				float32(startDelta),
				float32(startDelta),
			}
			areas := [4][4]float32{area, area, area, area}

			for j := segStart; j < i; j++ {
				tile := &tiles[j]
				delta += tile.delta()
				p0 := tile.p0.float32()
				p1 := tile.p1.float32()
				slope := (p1.x - p0.x) / (p1.y - p0.y)
				if x0 >= x1 {
					continue
				}
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
					var areaU8 uint32
					switch fillRule {
					case NonZero:
						areaU8 = satConv[uint32](math.Round(min(math.Abs(float64(area)), 1.0) * 255.0))
					case EvenOdd:
						even := int32(area) % 2
						// If we have for example 2.68, then opacity is 68%, while for
						// 1.68 it would be (1 - 0.68) = 32%
						addVal := float32(even)
						// 1 for even, -1 for odd
						sign := float32(-2*even + 1)
						_, areaFrac := math.Modf(float64(area))
						areaU8 = satConv[uint32]((addVal+sign*float32(areaFrac))*255.0 + 0.5)
					default:
						panic(fmt.Sprintf("invalid fill rule %v", fillRule))
					}
					alphas += areaU8 << (y * 8)
				}
				alphaBuf = append(alphaBuf, alphas)
			}

			if stripStart {
				xy := (1<<18)*uint32(prevTile.y) + 4*uint32(prevTile.x) + x0
				strip := strip{
					xy:      xy,
					col:     cols,
					winding: int32(startDelta),
				}
				stripBuf = append(stripBuf, strip)
			}
			cols += x1 - x0
			if sameStrip {
				fp = 1
			} else {
				fp = 0
			}
			stripStart = !sameStrip
			segStart = i
			if !prevTile.loc().sameRow(tile.loc()) {
				delta = 0
			}
		}
		fp |= tile.footprint()
		prevTile = tile
	}

	return stripBuf, alphaBuf
}

func (s *strip) x() uint32 {
	return s.xy & 0xFFFF
}

func (s *strip) y() uint32 {
	return s.xy / (1 << 16)
}

func (s *strip) stripY() uint32 {
	return s.xy / ((1 << 16) * stripHeight)
}
