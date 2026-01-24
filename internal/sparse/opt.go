// SPDX-FileCopyrightText: 2025 Dominik Honnef and contributors
//
// SPDX-License-Identifier: MIT

package sparse

import (
	"fmt"
	"iter"
	"log"
	"math"
	"math/bits"
	"slices"
	"strings"

	"honnef.co/go/gutter/gfx"
)

// Whether to check for bugs in the optimization passes.
const extraSafetyChecks = true

// TODO(dh): this
//
// 	Clear(x=0, width=256, paint=[1 0 0 1])
// 	PushLayer(blend={Normal SrcOver}, opacity=1)
// 	  Clear(x=0, width=256, paint=[0 1 0 1])
// 	  Blend(x=246, width=10, blend={Normal SrcOver}, opacity=1)
// 	  PopLayer()
//
// could optimize to
//
// 	Clear(x=0, width=256, paint=[1 0 0 1])
// 	Clear(x=246, width=10, paint=[0 1 0 1])
//
// or
//
// 	Clear(x=0, width=246, paint=[1 0 0 1])
// 	Clear(x=246, width=10, paint=[0 1 0 1])

// TODO(dh): this
//
// 	AlphaFill(x=80, width=4, paint=[0.68657786 0.049683332 0.015928429 1]) [[255 224 140 55] [55 1 0 0] [0 0 0 0] [0 0 0 0]]
// 	AlphaFill(x=60, width=8, paint=[0.3509753 0.017603893 0.015928429 1]) [[0 0 0 0] [0 0 0 0] [0 0 0 18] [1 63 164 247] [217 255 255 255] [255 255 255 255] [255 255 255 255] [255 255 255 255]]
// 	Clear(x=68, width=28, paint=[0.3509753 0.017603893 0.015928429 1])
//
// could optimize to
//
// 	Clear(x=68, width=28, paint=[0.3509753 0.017603893 0.015928429 1])

// TODO(dh): this
//
// 	PushLayer()
// 	  Fill(x=0, width=256, paint=[0 0.5 0 0.5])
//
// could optimize to PushLayer + Clear

// TODO(dh): some layers with Clear blend mode could just be Clear commands in
// the parent

// TODO(dh): when only part of the layer gets blended back into its parent then
// we don't have to fill the rest of the layer. this would especially help the
// performance of gradients. at the same time we should be careful that this
// doesn't prevent the "simple layer" optimization in the fine stage. that is,
// if we could have a solid fill for the whole tile then we shouldn't prevent
// that by cutting fills to size.
//
// On the other hand, if we only have blends, not alpha blends, in a SrcOver
// layer, if we can guarantee that the parts outside the clip never get drawn
// to, we can unwrap the layer.

// TODO(dh): implement optimizations for more blend modes. at a minimum we'll
// want dstIn and dstOut.

// TODO(dh): would it be beneficial to split AlphaFills and AlphaBlends into
// non-Alpha and Alpha parts, based on the concrete alpha values?

// TODO(dh): everything that gets blended with a SrcIn or SrcOver only needs its
// alpha channel drawn, as the colors don't matter (but only when using MixNormal).

type optBitset [4]uint64

func (bs *optBitset) Lsh(n uint) {
	const W = 64

	wordShift := uint64(n / W)
	bitShift := n % W
	invBitShift := (W - bitShift) & 63

	get := func(i uint64) uint64 {
		mask := ((i | (3 - i)) >> 63) ^ 1
		return bs[i&3] & -mask
	}

	bs[3] = (bs[(3-wordShift)&3] << bitShift) | (get(2-wordShift) >> invBitShift)
	bs[2] = (get(2-wordShift) << bitShift) | (get(1-wordShift) >> invBitShift)
	bs[1] = (get(1-wordShift) << bitShift) | (get(0-wordShift) >> invBitShift)
	bs[0] = (get(0-wordShift) << bitShift)
}

func (bs *optBitset) Rsh(n uint) {
	const W = 64

	wordShift := uint64(n / W)
	bitShift := n % W
	invBitShift := (W - bitShift) & 63

	get := func(i uint64) uint64 {
		mask := ((i | (3 - i)) >> 63) ^ 1
		return bs[i&3] & -mask
	}

	bs[0] = (bs[(0+wordShift)&3] >> bitShift) | (get((1 + wordShift)) << invBitShift)
	bs[1] = (get((1 + wordShift)) >> bitShift) | (get((2 + wordShift)) << invBitShift)
	bs[2] = (get((2 + wordShift)) >> bitShift) | (get((3 + wordShift)) << invBitShift)
	bs[3] = (get((3 + wordShift)) >> bitShift)
}

func (bs *optBitset) Dec() {
	borrow := uint64(1)
	var x uint64

	x = bs[0]
	bs[0] -= 1
	borrow = (^x & borrow) >> 63

	x = bs[1]
	bs[1] -= borrow
	borrow = (^x & borrow) >> 63

	x = bs[2]
	bs[2] -= borrow
	borrow = (^x & borrow) >> 63

	bs[3] -= borrow
}

func (bs *optBitset) And(o optBitset) {
	bs[0] &= o[0]
	bs[1] &= o[1]
	bs[2] &= o[2]
	bs[3] &= o[3]
}

func (bs *optBitset) Or(o optBitset) {
	bs[0] |= o[0]
	bs[1] |= o[1]
	bs[2] |= o[2]
	bs[3] |= o[3]
}

func (bs *optBitset) Not() {
	bs[0] = ^bs[0]
	bs[1] = ^bs[1]
	bs[2] = ^bs[2]
	bs[3] = ^bs[3]
}

func (bs *optBitset) Set(i uint8, b bool) {
	if b {
		bs[i/64] |= 1 << (i % 64)
	} else {
		bs[i/64] &^= 1 << (i % 64)
	}
}

func (bs *optBitset) Get(i uint8) bool {
	return bs[i/64]&(1<<(i%64)) != 0
}

func (bs optBitset) String() string {
	return fmt.Sprintf("%064b%064b%064b%064b", bs[3], bs[2], bs[1], bs[0])
}

func (bs *optBitset) IsZero() bool {
	return bs[0] == 0 && bs[1] == 0 && bs[2] == 0 && bs[3] == 0
}

func (bs *optBitset) All() bool {
	return *bs == optBitset{math.MaxUint64, math.MaxUint64, math.MaxUint64, math.MaxUint64}
}

func (bs *optBitset) LeadingZeros() int {
	if n := bits.LeadingZeros64(bs[3]); n < 64 {
		return n
	} else if n := bits.LeadingZeros64(bs[2]); n < 64 {
		return 64 + n
	} else if n := bits.LeadingZeros64(bs[1]); n < 64 {
		return 128 + n
	} else if n := bits.LeadingZeros64(bs[0]); n < 64 {
		return 192 + n
	} else {
		return 256
	}
}

func (bs *optBitset) TrailingZeros() int {
	if n := bits.TrailingZeros64(bs[0]); n < 64 {
		return n
	} else if n := bits.TrailingZeros64(bs[1]); n < 64 {
		return 64 + n
	} else if n := bits.TrailingZeros64(bs[2]); n < 64 {
		return 128 + n
	} else if n := bits.TrailingZeros64(bs[3]); n < 64 {
		return 192 + n
	} else {
		return 256
	}
}

func makeMask(start, length uint16) optBitset {
	if length == wideTileWidth {
		if extraSafetyChecks && start != 0 {
			panic(fmt.Sprintf("internal error: called makeMask(%d, %d)", start, length))
		}
		return optBitset{math.MaxUint64, math.MaxUint64, math.MaxUint64, math.MaxUint64}
	}

	var bs optBitset

	if start < 1*64 {
		n := min(length, 64-start%64)
		bs[0] = (1<<n - 1) << (start % 64)
		start += n
		length -= n
	}
	if start < 128 {
		n := min(length, 64-start%64)
		bs[1] = (1<<n - 1) << (start % 64)
		start += n
		length -= n
	}
	if start < 192 {
		n := min(length, 64-start%64)
		bs[2] = (1<<n - 1) << (start % 64)
		start += n
		length -= n
	}
	if start < 256 {
		n := min(length, 64-start%64)
		bs[3] = (1<<n - 1) << (start % 64)
		start += n
		length -= n
	}

	return bs
}

// How many entries may be present in alpha masks for all-ones/all-zeros
// optimizations to apply. Larger values have a higher cost, lower values
// allow more commands to be optimized.
const alphaValueCutoff = wideTileWidth

func orWithMask(bs *optBitset, x, width uint16) {
	if width == wideTileWidth {
		if extraSafetyChecks && x != 0 {
			panic(fmt.Sprintf("internal error: called orWithMask(_, %d, %d)", x, width))
		}
		*bs = optBitset{math.MaxUint64, math.MaxUint64, math.MaxUint64, math.MaxUint64}
	} else if width != 0 {
		bs.Or(makeMask(x, width))
	}
}

func andWithMask(bs *optBitset, x, width uint16) {
	if width == 0 {
		*bs = optBitset{}
	} else if width != wideTileWidth {
		bs.And(makeMask(x, width))
	}
}

type optLayer struct {
	parent         int
	children       []int
	push           int
	footer         int
	pop            int
	numAlphaBlends int

	// Pixels that were part of a Blend or AlphaBlend's range
	blended optBitset
	// Same as blended, minus pixels that were definitely transparent
	blendedNonZeroAlpha optBitset
	// Pixels in the layer that might not have been transparent at the time of
	// blending, not restricted to pixels that were actually blended.
	maybeNotTransparent optBitset
	blend               gfx.BlendMode
	opacity             float32
}

func childNeedsBackdrop(layers []optLayer, l *optLayer) bool {
	for _, child := range l.children {
		// TODO(dh): this might be overly restrictive
		if layers[child].blend != (gfx.BlendMode{}) {
			return true
		}
	}
	return false
}

func dfsInner(layers []optLayer, l *optLayer, f func(*optLayer) bool) bool {
	for _, child := range l.children {
		if !dfsInner(layers, &layers[child], f) {
			return false
		}
	}
	return f(l)
}

func dfs(layers []optLayer) iter.Seq[*optLayer] {
	return func(yield func(l *optLayer) bool) {
		dfsInner(layers, &layers[0], yield)
	}
}

func optimizeCommands(allCmds []cmd, cmds []int32, stackScratch []optLayer) (newCmdIdxs []int32, _ []optLayer) {
	const debug = false

	if len(cmds) == 0 {
		return cmds, stackScratch
	}

	lastCmd := func(l *optLayer) int {
		if l.push == -1 {
			return len(cmds) - 1
		} else {
			return l.pop
		}
	}

	indexLayers := func() []optLayer {
		layers := append(stackScratch[:0], optLayer{
			// The first layer on the stack is virtual and doesn't have a
			// corresponding PushLayer command. When we want to iterate over all
			// commands in a layer, we start at layer.push+1, to skip over the
			// PushLayer command. -1 makes this work.
			push: -1,
			pop:  len(cmds) - 1,
		})
		top := 0
		for i, cID := range cmds {
			c := &allCmds[cID]
			switch c.typ {
			case cmdAlphaBlend, cmdBlend:
				if layers[top].footer == 0 {
					layers[top].footer = i
					layers[top].blend = c.blend
					layers[top].opacity = c.opacity
				}
				blended := makeMask(c.x, c.width)
				blendedNonZeroAlpha := blended
				if c.typ == cmdAlphaBlend {
					layers[top].numAlphaBlends++
					for x, a := range c.alphas[:c.width] {
						if a == [4]uint8{} {
							blendedNonZeroAlpha.Set(uint8(x)+uint8(c.x), false)
						}
					}
				}
				layers[top].blendedNonZeroAlpha.Or(blendedNonZeroAlpha)
				layers[top].blended.Or(blended)
			case cmdAlphaFill:
			case cmdClear:
			case cmdFill:
			case cmdNop:
			case cmdCopyBackdrop:
			case cmdPopLayer:
				layers[top].pop = i
				top = layers[top].parent
			case cmdPushLayer:
				layers = append(layers, optLayer{parent: top, push: i})
				l := len(layers) - 1
				layers[top].children = append(layers[top].children, l)
				top = l
			default:
				panic(fmt.Sprintf("unexpected sparse.cmdType: %s", c.typ))
			}
		}
		return layers
	}

	debugPrintCmds := func() {
		depth := 0
		for _, cmdIdx := range cmds {
			prefix := strings.Repeat("  ", depth)
			cmd := allCmds[cmdIdx]
			log.Println(prefix, cmd)

			switch cmd.typ {
			case cmdPushLayer:
				depth++
			case cmdPopLayer:
				depth--
			}
		}
	}

	if debug {
		log.Println("before:")
		debugPrintCmds()
		log.Println()
	}

	// clearColor exists to avoid repeated calls to runtime.convTnoptr
	clearColor := encodedColor{}
	changed := true
	for changed {
		changed = false
		if len(cmds) == 0 {
			break
		}
		layers := indexLayers()
		stackScratch = layers

		// Rewrite all commands to stay within the blend bounds
		for _, lID := range layers[0].children {
			l := &layers[lID]
			if l.blended.IsZero() {
				continue
			}

			firstOpaque := uint16(l.blended.TrailingZeros())
			lastOpaque := uint16(255 - l.blended.LeadingZeros())

			for i := l.push + 1; i < l.footer; i++ {
				c := &allCmds[cmds[i]]
				switch c.typ {
				case cmdAlphaBlend, cmdAlphaFill, cmdBlend, cmdClear, cmdFill:
					switch {
					case c.x > lastOpaque:
						cmds[i] = 0
						changed = true
					case c.x+c.width-1 < firstOpaque:
						cmds[i] = 0
						changed = true
					case c.x < firstOpaque:
						d := firstOpaque - c.x
						c.x = firstOpaque
						c.width -= d
						if c.typ == cmdAlphaFill || c.typ == cmdAlphaBlend {
							c.alphas = c.alphas[d:]
						}
						changed = true
					case c.x+c.width-1 > lastOpaque:
						c.width = lastOpaque - c.x + 1
						changed = true
					}
					if c.width == 0 {
						cmds[i] = 0
						changed = true
					}
				case cmdNop:
				case cmdPopLayer:
					// TODO maintain a stack of tighter bounds as we descend the layer tree
				case cmdPushLayer:
				case cmdCopyBackdrop:
				default:
					panic(fmt.Sprintf("unexpected sparse.cmd: %s", c))
				}
			}
		}

		// Optimize layers, depth-first.
		for l := range dfs(layers) {
			numPushes := 0

			if l.blend.Compose == gfx.ComposeClear {
				// The only commands of significance in a clearing layer are the
				// blends.
				if l.footer != l.push+1 {
					clear(cmds[l.push+1 : l.footer])
					changed = true
				}
			}

			for i, lastCmd := l.push+1, lastCmd(l); i <= lastCmd; i++ {
				c := &allCmds[cmds[i]]
				switch c.typ {
				case cmdAlphaFill:
					// TODO(dh): if all alpha values are 0.0, we don't need the
					// fill at all.

					// Merge touching alpha fills
					for j := i + 1; j < len(cmds); j++ {
						c2 := &allCmds[cmds[j]]
						if c2.typ != cmdAlphaFill || c.x+c.width != c2.x || c.paint != c2.paint {
							break
						}
						c.alphas = slices.Concat(c.alphas[:c.width], c2.alphas[:c2.width])
						c.width += c2.width
						cmds[j] = 0
						changed = true
					}

					// TODO(dh): if the fill is a plain color with no opacity,
					// this doesn't change the transparency.

					// Update maybeNotTransparent
					orWithMask(&l.maybeNotTransparent, c.x, c.width)
				case cmdBlend, cmdAlphaBlend:
					if l.blend.Compose == gfx.ComposeSrcOver {
						bs := l.maybeNotTransparent
						andWithMask(&bs, c.x, c.width)

						// There's no point blending pixels we know are transparent.
						firstOpaque := bs.TrailingZeros()
						lastOpaque := 255 - bs.LeadingZeros()
						if lastOpaque < firstOpaque {
							// No pixels are opaque
							cmds[i] = 0
							changed = true
						} else {
							nx := uint16(firstOpaque)
							nw := uint16(lastOpaque - firstOpaque + 1)
							if c.x != nx || c.width != nw {
								if c.typ == cmdAlphaBlend {
									d := nx - c.x
									c.alphas = c.alphas[d:]
								}
								c.x = nx
								c.width = nw
								changed = true
							}
						}
					}
				case cmdClear:
					// Merge adjacent clears
					for j := i + 1; j <= lastCmd; j++ {
						c2 := &allCmds[cmds[j]]
						if c2.typ != cmdClear || c2.paint != c.paint || c2.x != c.x+c.width {
							break
						}
						c.width += c2.width
						cmds[j] = 0
						changed = true
					}

					// Update maybeNotTransparent
					if p, ok := c.paint.(encodedColor); ok && p[3] == 0 {
						if c.width == wideTileWidth {
							l.maybeNotTransparent = optBitset{}
						} else {
							m := makeMask(c.x, c.width)
							m.Not()
							l.maybeNotTransparent.And(m)
						}
					} else {
						orWithMask(&l.maybeNotTransparent, c.x, c.width)
					}

					// A full tile clear makes all previous commands in this
					// layer irrelevant.
					//
					// TODO(dh): extend this to non- full tile clears and
					// find the subset of draw commands that are irrelevant.
					if c.x == 0 && c.width == wideTileWidth {
						for j := l.push + 1; j < i; j++ {
							if cmds[j] != 0 {
								changed = true
								cmds[j] = 0
							}
						}
					}
				case cmdFill:
					// TODO(dh): if the fill is a plain color with no opacity,
					// this doesn't change the transparency.

					// Merge adjacent fills
					for j := i + 1; j <= lastCmd; j++ {
						c2 := &allCmds[cmds[j]]
						if c2.typ != cmdFill || c2.paint != c.paint || c2.x != c.x+c.width {
							break
						}
						c.width += c2.width
						cmds[j] = 0
						changed = true
					}

					// Update maybeNotTransparent
					orWithMask(&l.maybeNotTransparent, c.x, c.width)
				case cmdNop:
				case cmdPopLayer:
					if i != l.pop {
						panic(fmt.Sprintf("internal error: encountered PopLayer at %d which doesn't correspond to our layer's expected pop at %d",
							i, l.pop))
					}

					l.blendedNonZeroAlpha.And(l.maybeNotTransparent)
				case cmdCopyBackdrop:
					// We're processing layers inside out and have no idea what
					// the parent looks like yet. We must assume that all pixels
					// may be set.
					//
					// TODO(dh): should we switch from DFS to top-down processing?
					l.maybeNotTransparent = optBitset{math.MaxUint64, math.MaxUint64, math.MaxUint64, math.MaxUint64}
				case cmdPushLayer:
					child := &layers[l.children[numPushes]]
					numPushes++

					handleClearingLayer := func() bool {
						// A SrcIn or SrcOut layer that is transparent clears
						// the parent layer when blending. Replace it with a
						// sequence of clears.
						if !child.blendedNonZeroAlpha.IsZero() || child.numAlphaBlends != 0 {
							return false
						}
						clear(cmds[child.push:child.footer])
						for j := child.footer; j < child.pop; j++ {
							c2 := &allCmds[cmds[j]]
							switch c2.typ {
							case cmdBlend:
								c2.typ = cmdClear
								// TODO(dh): do the values of the rgb channels matter?
								c2.paint = clearColor
							default:
								panic(fmt.Sprintf("internal error: unexpected sparse.cmdType: %s", c2.typ))
							}
						}
						cmds[child.pop] = 0
						changed = true
						return true
					}

					switch child.blend.Compose {
					case gfx.ComposeClear:
						if child.numAlphaBlends == 0 {
							// Replace the layer with a series of Clears.
							clear(cmds[child.push:child.footer])
							for j := child.footer; j < child.pop; j++ {
								c2 := &allCmds[cmds[j]]
								c2.typ = cmdClear
								c2.paint = clearColor
							}
							cmds[child.pop] = 0
							changed = true
						}

					case gfx.ComposeSrcIn:
						if handleClearingLayer() {
							break
						}

						bs := child.blended
						bs.And(l.maybeNotTransparent)
						if bs.IsZero() {
							// All pixels that the child SrcIn layer blends to
							// are transparent. Thus, the child layer has no
							// visible effect on this layer.
							clear(cmds[child.push : child.pop+1])
							changed = true
						}
					case gfx.ComposeSrcOut:
						if handleClearingLayer() {
							break
						}

						bs := child.blended
						bs.And(l.maybeNotTransparent)
						if bs.IsZero() {
							// All pixels that the child SrcOut layer blends to
							// are transparent. Thus, the visible effect is
							// identical to blending with SrcOver instead.
							for j := child.footer; j < child.pop; j++ {
								c2 := &allCmds[cmds[j]]
								switch c2.typ {
								case cmdBlend, cmdAlphaBlend:
									c2.blend.Compose = gfx.ComposeSrcOver
								default:
									panic(fmt.Sprintf("internal error: unexpected %s", c2))
								}
							}
						}
					case gfx.ComposeSrcOver:
						if child.blendedNonZeroAlpha.IsZero() {
							// The child layer is empty. Delete it.
							clear(cmds[child.push : child.pop+1])
							changed = true
						} else if child.opacity == 1 &&
							child.blend.Mix == gfx.MixNormal &&
							child.numAlphaBlends == 0 &&
							!childNeedsBackdrop(layers, child) &&
							// TODO(dh): future-proof code would look for
							// CopyBackdrop anywhere in the layer, not just at
							// the beginning.
							allCmds[cmds[child.push+1]].typ != cmdCopyBackdrop {

							// If the layer has opaque pixels that aren't being
							// blended into the parent, then we can't unwrap the
							// layer.
							blended := child.blended
							// Pixels that are guaranteed to be transparent don't
							// need to be blended.
							mb := child.maybeNotTransparent
							mb.Not()
							blended.Or(mb)
							if blended.All() {
								// All non-transparent pixels of the child layer
								// get blended into this layer, with normal
								// blending, and opacity == 1. Furthermore, the
								// child layer doesn't itself have any child
								// layers that are sensitive to the backdrop.
								// That means the child layer doesn't have to be
								// a layer at all and its commands can be part
								// of this layer instead.
								cmds[child.push] = 0
								clear(cmds[child.footer : child.pop+1])
								changed = true

								// XXX this is probably wrong. imagine the child
								// fills the whole layer with a color, then sets
								// part of the layer to transparent, then blends
								// the non-transparent bits. If we unwrap the
								// layer. we'll set parts of the parent layer to
								// transparent. I don't think this is currently
								// possible, however. The only way to cut out a
								// shape is by using compositing; we don't
								// expose cmdClear to the user.
							}
						}

					case gfx.ComposeCopy:
						if child.opacity == 1 && allCmds[cmds[child.push+1]].typ == cmdCopyBackdrop && child.numAlphaBlends == 0 {
							unwrappable := true
							for j := child.footer; j < child.pop; j++ {
								c2 := allCmds[cmds[j]]
								if c2.typ != cmdBlend || c2.x != 0 || c2.width != wideTileWidth {
									unwrappable = false
									break
								}
							}
							if unwrappable {
								// This is a pure clip layer covering the whole
								// wide tile. The layer is unnecessary.
								//
								// OPT(dh): it'd be smarter to never generate
								// this layer in the first place, by adjusting
								// the logic in render.go
								cmds[child.push] = 0
								cmds[child.push+1] = 0
								clear(cmds[child.footer : child.pop+1])
								changed = true
							}
						}
					}

					// TODO(dh): model effect on pixels more correctly based on
					// porter-duff operator that's being used.
					l.maybeNotTransparent.Or(child.blendedNonZeroAlpha)

					// Skip child layer's commands, we've already processed them.
					i = child.pop
				default:
					panic(fmt.Sprintf("unexpected sparse.cmd: %s", c))
				}
			}
		}

		if changed {
			out := cmds[:0]
			for _, c := range cmds {
				if c != 0 {
					out = append(out, c)
				}
			}
			cmds = out

			if debug {
				log.Println("after:")
				debugPrintCmds()
				log.Println()
			}
		}
	}
	return cmds, stackScratch
}
