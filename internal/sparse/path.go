// SPDX-FileCopyrightText: 2024 the Piet Authors
// SPDX-FileCopyrightText: 2025 Dominik Honnef and contributors
//
// SPDX-License-Identifier: MIT

package sparse

import (
	"iter"
	"math"

	"honnef.co/go/curve"
	"honnef.co/go/gutter/gfx"
	"honnef.co/go/stuff/container/maybe"
)

type Path struct {
	strips   []strip
	alphas   [][stripHeight]uint8
	fillRule gfx.FillRule
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
				strips:   strips,
				alphas:   alphas,
				fillRule: gfx.NonZero,
			}
		}
	}

	// TODO(dh): scale precision based on transformation
	lines := fill(shape.PathElements(0.1), affine)
	strips, alphas := renderPathCommon(lines, fillRule, width, height)
	return Path{strips, alphas, fillRule}
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
	return Path{strips, alphas, gfx.NonZero}
}

type region struct {
	kind   regionKind
	start  uint16               // fillRegion, stripRegion
	width  uint16               // fillRegion, stripRegion
	alphas [][stripHeight]uint8 // stripRegion
}

func (r region) end() uint16 {
	return r.start + r.width
}

type regionKind int

const (
	fillRegionKind regionKind = iota
	stripRegionKind
)

func (p Path) regions(stripY uint16) iter.Seq[region] {
	return func(yield func(region) bool) {
		// OPT(dh): users of regions will look at consecutive rows, which makes
		// it silly to always start looking for the first relevant strip from
		// zero. We could be O(nlogn) instead of O(n²) by using binary search,
		// or O(n) by simply remembering the last index.
		curIdx := 0
		for p.strips[curIdx].stripY() < stripY {
			// This loop depends on the presence of a sentinel tile. Without it,
			// the increment might wrap around and loop forever.
			curIdx++
		}

		onStrip := true

		for {
			if !onStrip {
				onStrip = true

				curStrip, nextStrip := p.strips[curIdx], p.strips[curIdx+1]
				curIdx++
				var shouldFill bool
				switch p.fillRule {
				case gfx.NonZero:
					shouldFill = nextStrip.winding != 0
				case gfx.EvenOdd:
					shouldFill = nextStrip.winding%2 != 0
				}
				if shouldFill {
					x := curStrip.x + uint16((nextStrip.col - curStrip.col))
					width := nextStrip.x - x
					if !yield(region{kind: fillRegionKind, start: x, width: width}) {
						return
					}
				}
			}

			onStrip = false

			curStrip := p.strips[curIdx]
			if curStrip.x == math.MaxUint16 || curStrip.stripY() != stripY {
				return
			}
			nextStrip := p.strips[curIdx+1]

			x := curStrip.x
			width := uint16(nextStrip.col - curStrip.col)
			alphas := p.alphas[curStrip.col:nextStrip.col]

			if !yield(region{kind: stripRegionKind, start: x, width: width, alphas: alphas}) {
				return
			}
		}
	}
}

func (p Path) Intersect(o Path) Path {
	if len(p.strips) == 0 || len(o.strips) == 0 {
		return Path{}
	}

	out := Path{
		// In theory any fill rule is fine since all filled regions are marked
		// with a winding number of 1.
		fillRule: gfx.NonZero,
	}

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

	// OPT(dh): do these closures cause variables to escape? would this be more
	// efficient with parameters?
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
				winding: state.winding,
			})
		}
	}

	startStrip := func(x uint16, winding int32) {
		flushStrip()
		unflushedStrip = maybe.Some(strip{
			x:       x,
			col:     uint32(len(out.alphas)),
			winding: winding,
		})
	}

	for ; curStripY <= lastStripY; curStripY++ {
		nextP, stopP := iter.Pull(p.regions(curStripY))
		nextO, stopO := iter.Pull(o.regions(curStripY))

		regionP, okP := nextP()
		regionO, okO := nextO()

		for okP && okO {
			switch {
			case regionP.end() <= regionO.start:
				regionP, okP = nextP()
			case regionP.start >= regionO.end():
				regionO, okO = nextO()
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
					startStrip(overlapEnd, 1)

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
						startStrip(overlapStart, 0)
					}

					stripAlphas := s.alphas[overlapStart-s.start:][:overlapWidth]
					out.alphas = append(out.alphas, stripAlphas...)

				case regionP.kind == stripRegionKind && regionO.kind == stripRegionKind:
					// Two strips, we need to multiply the opacitie masks from both paths.
					//
					// Once again, only create a new strip if we can't extend the current one.
					if shouldStartNewStrip(overlapStart) {
						startStrip(overlapStart, 0)
					}

					// Get the right alpha values for the specific position.
					stripAlphasP := regionP.alphas[overlapStart-regionP.start:][:overlapWidth]
					stripAlphasO := regionO.alphas[overlapStart-regionO.start:][:overlapWidth]

					for i := 0; i < min(len(stripAlphasP), len(stripAlphasO)); i++ {
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
					regionP, okP = nextP()
				} else {
					regionO, okO = nextO()
				}
			}
		}

		flushStrip()
		stopP()
		stopO()
	}

	// Push the sentinel strip.
	out.strips = append(out.strips, strip{
		x:       math.MaxUint16,
		y:       lastStripY * tileHeight,
		col:     uint32(len(out.alphas)),
		winding: 0,
	})

	return out
}
