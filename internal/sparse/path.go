// SPDX-FileCopyrightText: 2024 the Piet Authors
// SPDX-FileCopyrightText: 2025 Dominik Honnef and contributors
//
// SPDX-License-Identifier: MIT

package sparse

import (
	"math"
	"slices"

	"honnef.co/go/curve"
	"honnef.co/go/gutter/gfx"
	"honnef.co/go/stuff/container/maybe"
)

type Path struct {
	strips []strip
	alphas [][stripHeight]uint8
}

func CompileFillPath(
	shape gfx.Shape,
	affine curve.Affine,
	fillRule gfx.FillRule,
	width uint16,
	height uint16,
) Path {
	// The transformation mustn't skew the shape for our optimizations to apply.
	if affine.N1 == 0 && affine.N2 == 0 {
		switch shape := shape.(type) {
		case curve.Rect:
			// OPT(dh): all rectangles of the same size that fall on integer
			// coordinates are the same, especially if their Y coordinates only
			// differ in multiples of the strip height.

			a, d, e, f := affine.N0, affine.N3, affine.N4, affine.N5
			shape = curve.Rect{
				X0: shape.X0*a + e,
				Y0: shape.Y0*d + f,
				X1: shape.X1*a + e,
				Y1: shape.Y1*d + f,
			}

			strips, alphas := renderRect(shape, width, height)
			return Path{
				strips: strips,
				alphas: alphas,
			}
		}
	}

	// TODO(dh): scale precision based on transformation
	lines := fill(shape.PathElements(0.1), affine)
	strips, alphas := renderPathCommon(lines, fillRule, width, height)
	return Path{strips, alphas}
}

func CompileStrokedPath(
	shape gfx.Shape,
	affine curve.Affine,
	stroke_ curve.Stroke,
	width uint16,
	height uint16,
) Path {
	// TODO(dh): scale precision based on transformation
	path := shape.PathElements(0.1)
	lines := stroke(path, stroke_, affine)
	strips, alphas := renderPathCommon(lines, gfx.NonZero, width, height)
	return Path{strips, alphas}
}

type region struct {
	kind   regionKind
	start  uint16               // fillRegion, stripRegion
	width  uint16               // fillRegion, stripRegion
	alphas [][stripHeight]uint8 // stripRegion
}

func (r *region) end() uint16 {
	return r.start + r.width
}

type regionKind int

const (
	fillRegionKind regionKind = iota
	stripRegionKind
)

// regionIter is an iterator over the regions in a path.
//
// We don't use iter.Seq to cut down on the amount of garbage being produced by
// the iterator and the coroutine we use to pull from it.
type regionIter struct {
	path    Path
	stripY  uint16
	curIdx  int
	onStrip bool
}

func (it *regionIter) next(out *region) bool {
	p := it.path
	stripY := it.stripY

	if !it.onStrip {
		it.onStrip = true

		curStrip, nextStrip := &p.strips[it.curIdx], &p.strips[it.curIdx+1]
		it.curIdx++
		if nextStrip.fillGap {
			x := curStrip.x + uint16((nextStrip.col - curStrip.col))
			width := nextStrip.x - x
			*out = region{kind: fillRegionKind, start: x, width: width}
			return true
		}
	}

	it.onStrip = false

	curStrip := &p.strips[it.curIdx]
	if curStrip.x == math.MaxUint16 || curStrip.stripY() != stripY {
		return false
	}
	nextStrip := &p.strips[it.curIdx+1]

	x := curStrip.x
	width := uint16(nextStrip.col - curStrip.col)
	alphas := p.alphas[curStrip.col:nextStrip.col]

	*out = region{kind: stripRegionKind, start: x, width: width, alphas: alphas}
	return true
}

// regions returns an iterator over the regions (i.e. strips and fills) in the
// path. The startStripIdx parameter is used to speed up finding the first
// relevant strip and can be set to the final index of a previous iterator for a
// smaller stripY.
func (p Path) regions(stripY uint16, startStripIdx int) regionIter {
	curIdx := max(startStripIdx, 0)
	for ; curIdx < len(p.strips) && p.strips[curIdx].stripY() < stripY; curIdx++ {
	}

	return regionIter{
		path:    p,
		stripY:  stripY,
		curIdx:  curIdx,
		onStrip: true,
	}
}

func (p Path) Intersect(o Path) Path {
	if len(p.strips) == 0 || len(o.strips) == 0 {
		return Path{}
	}

	var out Path

	curStripY := min(
		p.strips[0].stripY(),
		o.strips[0].stripY(),
	)
	lastStripY := min(
		p.strips[len(p.strips)-1].stripY(),
		o.strips[len(o.strips)-1].stripY(),
	)

	// The strip we're currently building, which hasn't been flushed yet.
	var unflushedStrip maybe.Option[strip]

	shouldStartNewStrip := func(overlapStart uint16) bool {
		state, ok := unflushedStrip.Get()
		if !ok {
			return true
		}

		width := uint16(uint32(len(out.alphas)) - state.col)
		stripEnd := state.x + width

		return stripEnd < overlapStart-1
	}

	flushStrip := func() {
		if state, ok := unflushedStrip.Take().Get(); ok {
			out.strips = append(out.strips, strip{
				x:       state.x,
				y:       curStripY * tileHeight,
				col:     state.col,
				fillGap: state.fillGap,
			})
		}
	}

	startStrip := func(x uint16, fillGap bool) {
		flushStrip()
		unflushedStrip = maybe.Some(strip{
			x:       x,
			col:     uint32(len(out.alphas)),
			fillGap: fillGap,
		})
	}

	var idxP, idxO int
	for ; curStripY <= lastStripY; curStripY++ {
		iterP := p.regions(curStripY, idxP)
		iterO := o.regions(curStripY, idxO)

		var regionP, regionO region
		okP := iterP.next(&regionP)
		okO := iterO.next(&regionO)

		for okP && okO {
			switch {
			case regionP.end() <= regionO.start:
				okP = iterP.next(&regionP)
			case regionP.start >= regionO.end():
				okO = iterO.next(&regionO)
			default:
				overlapStart := max(regionP.start, regionO.start)
				overlapEnd := min(regionP.end(), regionO.end())
				overlapWidth := overlapEnd - overlapStart

				switch {
				case regionP.kind == fillRegionKind && regionO.kind == fillRegionKind:
					// Both regions are a fill. Flush the current strip and start a new
					// one at the end of the overlap region setting the winding number to
					// one, so that the whole area before that will be filled with a sparse
					// fill.
					startStrip(overlapEnd, true)

				case (regionP.kind == stripRegionKind && regionO.kind == fillRegionKind) ||
					// One fill one strip, so we simply use the alpha mask from the strip region.
					(regionP.kind == fillRegionKind && regionO.kind == stripRegionKind):
					var s region
					if regionP.kind == stripRegionKind {
						s = regionP
					} else {
						s = regionO
					}
					// If possible, don't create a new strip but just extend the current one.
					if shouldStartNewStrip(overlapStart) {
						startStrip(overlapStart, false)
					}

					stripAlphas := s.alphas[overlapStart-s.start:][:overlapWidth]
					out.alphas = append(out.alphas, stripAlphas...)

				case regionP.kind == stripRegionKind && regionO.kind == stripRegionKind:
					// Two strips, we need to multiply the opacitie masks from both paths.
					//
					// Once again, only create a new strip if we can't extend the current one.
					if shouldStartNewStrip(overlapStart) {
						startStrip(overlapStart, false)
					}

					// Get the right alpha values for the specific position.
					stripAlphasP := regionP.alphas[overlapStart-regionP.start:][:overlapWidth]
					stripAlphasO := regionO.alphas[overlapStart-regionO.start:][:overlapWidth]

					numAlphas := min(len(stripAlphasP), len(stripAlphasO))
					out.alphas = slices.Grow(out.alphas, numAlphas)
					for i := range numAlphas {
						// OPT(dh): this loop could trivially use SIMD
						stripAlphaP := stripAlphasP[i]
						stripAlphaO := stripAlphasO[i]

						var outAlpha [stripHeight]uint8
						for j := range outAlpha {
							outAlpha[j] = uint8((uint16(stripAlphaP[j]) * uint16(stripAlphaO[j])) / 255)
						}

						out.alphas = append(out.alphas, outAlpha)
					}
				}

				// Advance the iterator of the path whose region's end is further behind.
				if regionP.end() <= regionO.end() {
					okP = iterP.next(&regionP)
				} else {
					okO = iterO.next(&regionO)
				}
			}
		}

		flushStrip()

		// Remember indices of where we stopped, to speed up creation of next
		// iterators.
		idxP = iterP.curIdx
		idxO = iterO.curIdx
	}

	// Push the sentinel strip.
	out.strips = append(out.strips, strip{
		x:   math.MaxUint16,
		y:   lastStripY * tileHeight,
		col: uint32(len(out.alphas)),
	})

	return out
}
