// SPDX-FileCopyrightText: 2014 The Flutter Authors. All rights reserved.
// SPDX-FileCopyrightText: 2024 Dominik Honnef and contributors
//
// SPDX-License-Identifier: MIT AND BSD-3-Clause

package render

import (
	"slices"

	"honnef.co/go/curve"
	"honnef.co/go/gutter/debug"
	"honnef.co/go/gutter/mem"
	"honnef.co/go/gutter/wsi"
	"honnef.co/go/jello"
)

type Renderer struct {
	painter                           *Painter
	rootNode                          Object
	nodesNeedingLayout                mem.DoubleBufferedSlice[Object]
	nodesNeedingCompositingBitsUpdate []Object
	shouldMergeDirtyNodes             bool
	OnNeedVisualUpdate                func()
	EmitEvent                         func(ev wsi.Event)

	htr hitTestResult
}

func NewRenderer(sys *wsi.System, win wsi.Window) *Renderer {
	r := &Renderer{
		painter:            NewPainter(),
		OnNeedVisualUpdate: win.RequestFrame,
		EmitEvent: func(ev wsi.Event) {
			// TODO(dh): add a wsi.Window.EmitEvent method
			sys.EmitEvent(win, ev)
		},
	}
	v := NewView()
	r.SetRootNode(v)
	v.PrepareInitialFrame()
	return r
}

func (r *Renderer) DrawFrame(scene *jello.Scene) {
	debug.Assert(r.View() != nil)
	r.FlushLayout()
	r.FlushCompositingBits()
	r.FlushPaint(scene)
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

func (r *Renderer) FlushPaint(scene *jello.Scene) {
	if r.rootNode != nil {
		r.painter.PaintAt(r.rootNode, scene, curve.Point{})
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
	} else {
		obj.VisitChildren(func(child Object) bool {
			Attach(child, r)
			return true
		})
	}
}

func Detach(obj Object) {
	obj.Handle().renderer = nil
	if obj, ok := obj.(Attacher); ok {
		obj.PerformDetach()
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

func (v *View) PerformPaint(p *Painter, scene *jello.Scene) {
	if v.Child != nil {
		scene.Append(p.Paint(v.Child), curve.Identity)
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
