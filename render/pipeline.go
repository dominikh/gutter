// SPDX-FileCopyrightText: 2014 The Flutter Authors. All rights reserved.
// SPDX-FileCopyrightText: 2024 Dominik Honnef and contributors
//
// SPDX-License-Identifier: MIT AND BSD-3-Clause

package render

import (
	"math"
	"slices"
	"time"

	"honnef.co/go/curve"
	"honnef.co/go/gutter/animation"
	"honnef.co/go/gutter/debug"
	"honnef.co/go/gutter/gfx"
	"honnef.co/go/gutter/mem"
)

type Renderer struct {
	painter                           *Painter
	rootNode                          Object
	nodesNeedingLayout                mem.DoubleBufferedSlice[Object]
	nodesNeedingCompositingBitsUpdate []Object
	shouldMergeDirtyNodes             bool
	OnNeedVisualUpdate                func()

	htr                hitTestResult
	nextFrameCallbacks mem.DoubleBufferedSlice[func(now time.Duration)]
}

func (r *Renderer) ScheduleFrameCallback(fn animation.FrameCallback) uint64 {
	n := uint64(len(r.nextFrameCallbacks.Front))
	if n >= math.MaxUint32 {
		panic("tried registering 2**32 frame callbacks in a single frame")
	}
	r.nextFrameCallbacks.Front = append(r.nextFrameCallbacks.Front, fn)
	r.RequestVisualUpdate()
	return uint64(r.nextFrameCallbacks.Generation)<<32 | (n & 0xFFFFFFFF)
}

func (r *Renderer) CancelFrameCallback(id uint64) {
	gen := uint32(id >> 32)
	idx := int(id & 0xFFFFFFFF)
	if gen != r.nextFrameCallbacks.Generation {
		// If gen > Generation, then the callback is from an old frame.
		//
		// If gen < Generation, then the callback is from an old frame and the
		// generation counter has overflown.
		return
	}
	// If gen == Generation, then the callback is either from this frame, or
	// from 2**32 frames ago, which was around 200-800 days ago (at 60-240 Hz of
	// non-stop rendering). Hopefully nobody keeps a frame ID around for that
	// long...
	if idx >= len(r.nextFrameCallbacks.Front) {
		// An invalid index... Did someone pass in a very old ID?
		return
	}
	r.nextFrameCallbacks.Front[idx] = nil
}

func (r *Renderer) RunFrameCallbacks(now time.Duration) {
	fns := r.nextFrameCallbacks.Front
	r.nextFrameCallbacks.Swap()

	for _, fn := range fns {
		if fn == nil {
			// Cancelled callback
			continue
		}
		fn(now)
	}
}

func NewRenderer() *Renderer {
	r := &Renderer{
		painter: NewPainter(),
	}
	v := NewView()
	r.SetRootNode(v)
	v.PrepareInitialFrame()
	return r
}

func (r *Renderer) DrawFrame(rec gfx.Recorder) {
	debug.Assert(r.View() != nil)
	r.FlushLayout()
	r.FlushCompositingBits()
	r.FlushPaint(rec)
}

func (r *Renderer) View() *View {
	return r.rootNode.(*View)
}

// func (b *Binding) HandlePointerEvent(e giopointer.Event) {
// 	b.htr.Reset()
// 	hitTest(&b.htr, b.PipelineOwner.rootNode, e.Position)
// 	hits := b.htr.hits
// 	n := 0
// 	for _, hit := range hits {
// 		if _, ok := hit.Object.(PointerEventHandler); ok {
// 			n++
// 			if n >= 2 {
// 				break
// 			}
// 		}
// 	}
// 	var kind pointer.Priority
// 	if n < 2 {
// 		kind = pointer.Exclusive
// 	} else {
// 		kind = pointer.Shared
// 	}
// 	first := true
// 	for _, hit := range hits {
// 		if obj, ok := hit.Object.(PointerEventHandler); ok {
// 			prio := kind
// 			if first && prio == pointer.Shared {
// 				prio = pointer.Foremost
// 			}
// 			first = false
// 			ev := pointer.FromRaw(e)
// 			ev.Priority = prio
// 			obj.HandlePointerEvent(hit, ev)
// 		}
// 	}
// }

func (r *Renderer) RootNode() Object { return r.rootNode }
func (r *Renderer) SetRootNode(root Object) {
	if r.rootNode == root {
		return
	}
	if r.rootNode != nil {
		Detach(r.rootNode)
	}
	r.rootNode = root
	if root != nil {
		Attach(root, r)
	}
}

func (r *Renderer) RequestVisualUpdate() {
	if r.OnNeedVisualUpdate != nil {
		r.OnNeedVisualUpdate()
	}
}

func (r *Renderer) enableMutationsToDirtySubtrees(fn func()) {
	fn()
	r.shouldMergeDirtyNodes = true
}

func (r *Renderer) FlushLayout() {
	for len(r.nodesNeedingLayout.Front) != 0 {
		dirtyNodes := r.nodesNeedingLayout.Front
		r.nodesNeedingLayout.Swap()
		slices.SortFunc(dirtyNodes, func(a, b Object) int {
			return a.Handle().depth - b.Handle().depth
		})
		for i := range dirtyNodes {
			if r.shouldMergeDirtyNodes {
				r.shouldMergeDirtyNodes = false
				if len(r.nodesNeedingLayout.Front) != 0 {
					r.nodesNeedingLayout.Front = append(r.nodesNeedingLayout.Front, dirtyNodes[i:]...)
					break
				}
			}
			node := dirtyNodes[i]
			if node.Handle().needsLayout && node.Handle().renderer == r {
				layoutWithoutResize(node)
			}
		}
		// No need to merge dirty nodes generated from processing the last
		// relayout boundary back.
		r.shouldMergeDirtyNodes = false
	}

	r.shouldMergeDirtyNodes = false
}

// XXX what's the meaning of this function name?
func layoutWithoutResize(obj Object) {
	obj.Handle().size = obj.PerformLayout()
	obj.Handle().needsLayout = false
	MarkNeedsPaint(obj)
}

func (r *Renderer) FlushPaint(rec gfx.Recorder) {
	r.painter.Canvas = rec
	if r.rootNode != nil {
		r.painter.Paint(r.rootNode)
	}
}

func (r *Renderer) FlushCompositingBits() {
	nodes := r.nodesNeedingCompositingBitsUpdate
	slices.SortFunc(nodes, func(a, b Object) int {
		return a.Handle().depth - b.Handle().depth
	})
	for _, node := range nodes {
		h := node.Handle()
		if h.needsCompositingBitsUpdate && h.renderer == r {
			// h.updateCompositingBits()
		}
	}
	clear(nodes)
	r.nodesNeedingCompositingBitsUpdate = nodes[:0]
}

func Attach(obj Object, r *Renderer) {
	h := obj.Handle()
	h.renderer = r
	// If the node was dirtied in some way while unattached, make sure to add
	// it to the appropriate dirty list now that an owner is available
	if h.needsLayout && h.relayoutBoundary != nil {
		// Don't enter this block if we've never laid out at all;
		// scheduleInitialLayout() will handle it
		h.needsLayout = false
		MarkNeedsLayout(obj)
	}
	if h.needsCompositingBitsUpdate {
		h.needsCompositingBitsUpdate = false
		// obj.MarkNeedsCompositingBitsUpdate()
	}
	if h.needsPaint {
		// Don't enter this block if we've never painted at all;
		// scheduleInitialPaint() will handle it
		h.needsPaint = false
		MarkNeedsPaint(obj)
	}

	if aobj, ok := obj.(Attacher); ok {
		aobj.PerformAttach(r)
	} else if obj, ok := obj.(ObjectWithChildren); ok {
		for child := range obj.Children() {
			Attach(child, r)
		}
	}
}

func Detach(obj Object) {
	obj.Handle().renderer = nil
	if obj, ok := obj.(Attacher); ok {
		obj.PerformDetach()
	} else if obj, ok := obj.(ObjectWithChildren); ok {
		for child := range obj.Children() {
			Detach(child)
		}
	}
}

var _ Object = (*View)(nil)

type View struct {
	ObjectHandle
	SingleChild

	configuration ViewConfiguration
}

func NewView() *View {
	return &View{}
}

func (v *View) PerformPaint(p *Painter) {
	if v.Child != nil {
		p.PaintAt(v.Child, curve.Point{})
	}
}

// XXX include pxperdp etc in the view configuration
type ViewConfiguration = Constraints

func (v *View) Configuration() ViewConfiguration {
	return v.configuration
}

func (v *View) SetConfiguration(value ViewConfiguration) {
	if v.configuration == value {
		return
	}
	v.configuration = value
	MarkNeedsLayout(v)
}

func (v *View) PrepareInitialFrame() {
	ScheduleInitialLayout(v)
	ScheduleInitialPaint(v)
}

func (v *View) constraints() Constraints {
	return v.configuration
}

func (v *View) PerformLayout() curve.Size {
	sizedByChild := !v.constraints().Tight()
	if v.Child != nil {
		Layout(v.Child, v.constraints(), sizedByChild)
	}
	if sizedByChild && v.Child != nil {
		return v.Child.Handle().size
	} else {
		return v.constraints().Min
	}
}
