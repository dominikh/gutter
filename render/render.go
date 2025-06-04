// SPDX-FileCopyrightText: 2014 The Flutter Authors. All rights reserved.
// SPDX-FileCopyrightText: 2024 Dominik Honnef and contributors
//
// SPDX-License-Identifier: MIT AND BSD-3-Clause

package render

import (
	"fmt"
	"iter"
	"slices"
	"strings"

	"honnef.co/go/curve"
	"honnef.co/go/gutter/debug"
	"honnef.co/go/gutter/gfx"
	"honnef.co/go/gutter/gmath"
)

// TODO implement support for multiple layers
// TODO guard assertions behind debug flag
// TODO support baseline stuff
// TODO hit testing
// TODO accessibility
// TODO RTL support (see https://api.flutter.dev/flutter/dart-ui/TextDirection.html)
// TODO should we handle nil children?
// TODO dry layout/intrinsic dimensions/https://github.com/flutter/flutter/issues/48679

type Object interface {
	// PerformLayout lays out the object.
	PerformLayout() (size curve.Size)
	PerformPaint(p *Painter)

	Handle() *ObjectHandle
}

type Attacher interface {
	PerformAttach(r *Renderer)
	PerformDetach()
}

type ObjectWithChildren interface {
	Object
	Children() iter.Seq[Object]
	PerformInsertChild(child Object, after int)
	PerformMoveChild(child Object, after int)
	PerformRemoveChild(child Object)
}

type ChildRemover interface {
	Object
	PerformRemoveChild(child Object)
}

type SizedByParenter interface {
	// Marker method that indicates that the object is sized by the parent.
	SizedByParent()
}

type Disposable interface {
	PerformDispose()
}

type ParentDataSetuper interface {
	PerformSetupParentData(child Object)
}

type ObjectHandle struct {
	size curve.Size
	// The object's position as a relative Offset from the parent object's
	// origin. Having to configure a child's Offset is so common that we have a
	// dedicated field for it, instead of requiring the use of parentData.
	Offset                     curve.Point
	ParentData                 any
	needsPaint                 bool
	needsLayout                bool
	needsCompositingBitsUpdate bool
	Parent                     Object
	constraints                Constraints
	relayoutBoundary           Object
	depth                      int
	renderer                   *Renderer
	HitTestBehavior            HitTestBehavior
}

func (h *ObjectHandle) Handle() *ObjectHandle    { return h }
func (h *ObjectHandle) Size() curve.Size         { return h.size }
func (h *ObjectHandle) Constraints() Constraints { return h.constraints }
func (h *ObjectHandle) Attached() bool           { return h.renderer != nil }

func MarkNeedsPaint(obj Object) {
	h := obj.Handle()
	if h.needsPaint {
		return
	}
	h.needsPaint = true

	// We always have to walk the tree up to the parent because our composition
	// of objects is implemented by parents appending to a jello.Scene.
	if h.Parent != nil {
		MarkNeedsPaint(h.Parent)
	} else {
		if h.renderer != nil {
			h.renderer.RequestVisualUpdate()
		}
	}
}

func MarkNeedsLayout(obj Object) {
	h := obj.Handle()
	if h.needsLayout {
		return
	}

	// log.Println(string(rdebug.Stack()))

	if h.relayoutBoundary == nil {
		h.needsLayout = true
		if h.Parent != nil {
			MarkNeedsLayout(h.Parent)
		}
		return
	}
	if h.relayoutBoundary != obj {
		if h.Parent == nil {
			panic(fmt.Sprintf("%[1]T(%[1]p) isn't a relayout boundary but also doesn't have a parent", obj))
		}
		MarkNeedsLayout(h.Parent)
	} else {
		h.needsLayout = true
		h.renderer.nodesNeedingLayout.Front = append(h.renderer.nodesNeedingLayout.Front, obj)
		h.renderer.RequestVisualUpdate()
	}
}

func (h *ObjectHandle) SetParent(parent Object) { h.Parent = parent }

type Constraints struct {
	Min, Max curve.Size
}

func (c Constraints) Tight() bool {
	return c.Min == c.Max
}

func (c Constraints) Enforce(oc Constraints) Constraints {
	return Constraints{
		Min: curve.Size{
			Width:  gmath.Clamp(c.Min.Width, oc.Min.Width, oc.Max.Width),
			Height: gmath.Clamp(c.Min.Height, oc.Min.Height, oc.Max.Height),
		},
		Max: curve.Size{
			Width:  gmath.Clamp(c.Max.Width, oc.Min.Width, oc.Max.Width),
			Height: gmath.Clamp(c.Max.Height, oc.Min.Height, oc.Max.Height),
		},
	}
}

// Constrain a size so each dimension is in the range [min, max].
func (c Constraints) Constrain(size curve.Size) curve.Size {
	size.Width = gmath.Clamp(size.Width, c.Min.Width, c.Max.Width)
	size.Height = gmath.Clamp(size.Height, c.Min.Height, c.Max.Height)
	return size
}

// Constrain a size so each dimension is in the range [min, max], while
// maintaining the aspect ratio of the input size.
func (c Constraints) ConstrainWithAspectRatio(size curve.Size) curve.Size {
	if c.Tight() {
		return c.Min
	}

	width := size.Width
	height := size.Height
	debug.Assert(width > 0.0)
	debug.Assert(height > 0.0)
	aspectRatio := width / height

	if width > c.Max.Width {
		width = c.Max.Width
		height = width / aspectRatio
	}

	if height > c.Max.Height {
		height = c.Max.Height
		width = height * aspectRatio
	}

	if width < c.Min.Width {
		width = c.Min.Width
		height = width / aspectRatio
	}

	if height < c.Min.Height {
		height = c.Min.Height
		width = height * aspectRatio
	}

	return curve.Sz(
		gmath.Clamp(width, c.Min.Width, c.Max.Height),
		gmath.Clamp(height, c.Min.Height, c.Max.Height),
	)
}

func (c Constraints) Loosen() Constraints {
	return Constraints{
		Max: c.Max,
	}
}

func FormatTree(root Object) string {
	var sb strings.Builder

	seen := map[Object]struct{}{}
	var formatTree func(root Object, depth int)
	formatTree = func(root Object, depth int) {
		if _, ok := seen[root]; ok {
			panic("render object tree is actually circular graph")
		}
		seen[root] = struct{}{}
		fmt.Fprintf(&sb, "%s(%[2]T)(%[2]p) (size: %s, relayout: %t)\n", strings.Repeat("\t", depth), root, root.Handle().Size(), root.Handle().relayoutBoundary)
		if root, ok := root.(ObjectWithChildren); ok {
			for child := range root.Children() {
				formatTree(child, depth+1)
			}
		}
	}
	formatTree(root, 0)

	return sb.String()
}

type SingleChild struct {
	Child Object
}

func (c *SingleChild) Children() iter.Seq[Object] {
	return func(yield func(Object) bool) {
		if c.Child != nil {
			yield(c.Child)
		}
	}
}

func (c *SingleChild) PerformInsertChild(child Object, after int) {
	debug.Assert(after == -1)
	c.Child = child
}

func (c *SingleChild) PerformRemoveChild(child Object) {
	debug.Assert(c.Child == child)
	c.Child = nil
}

func (c *SingleChild) PerformMoveChild(child Object, after int) {
	debug.Assert(c.Child == child)
	debug.Assert(after == -1)
	// Nothing to do
}

type ManyChildren struct {
	children []Object
}

func (c *ManyChildren) Children() iter.Seq[Object] {
	return func(yield func(Object) bool) {
		for _, child := range c.children {
			if !yield(child) {
				break
			}
		}
	}
}

func (c *ManyChildren) PerformInsertChild(child Object, after int) {
	if len(c.children) < after {
		c.children = slices.Grow(c.children, after-len(c.children))[:after]
	}
	c.children = slices.Insert(c.children, after+1, child)
}

func (c *ManyChildren) PerformMoveChild(child Object, after int) {
	idx := slices.Index(c.children, child)
	if after == idx {
		return
	}
	if after > idx {
		c.children = slices.Delete(c.children, idx, idx+1)
		c.children = slices.Insert(c.children, after, child)
	} else {
		c.children = slices.Delete(c.children, idx, idx+1)
		c.children = slices.Insert(c.children, after+1, child)
	}
}

func (c *ManyChildren) PerformRemoveChild(child Object) {
	idx := slices.Index(c.children, child)
	c.children = slices.Delete(c.children, idx, idx+1)
}

type Painter struct {
	Canvas gfx.Recorder
}

func (p *Painter) Paint(obj Object) {
	// XXX Paint should probably call Checkpoint

	debug.Assert(obj != nil)
	obj.Handle().needsPaint = false
	obj.PerformPaint(p)
}

func (p *Painter) PaintAt(obj Object, offset curve.Point) {
	cv := p.Canvas.Checkpoint()
	cv.SetFillRule(gfx.NonZero)
	if offset != (curve.Point{}) {
		cv.PushTransform(curve.Translate(curve.Vec2(offset)))
		cv = cv.Checkpoint()
	}

	pp := &Painter{
		Canvas: cv,
	}

	pp.Paint(obj)
}

func isType[T any](obj any) bool {
	_, ok := obj.(T)
	return ok
}

func Layout(obj Object, cs Constraints, parentUsesSize bool) curve.Size {
	debug.Assert(obj != nil)
	if cs.Min.Width > cs.Max.Width || cs.Min.Height > cs.Max.Height || cs.Min.Width < 0 || cs.Min.Height < 0 {
		panic(fmt.Sprintf("constraints %v are malformed", cs))
	}

	h := obj.Handle()
	var relayoutBoundary Object
	if !parentUsesSize || isType[SizedByParenter](obj) || cs.Tight() {
		// We're the relayout boundary
		relayoutBoundary = obj
	} else {
		relayoutBoundary = h.Parent.Handle().relayoutBoundary
	}

	if !h.needsLayout && cs == h.constraints {
		if relayoutBoundary != h.relayoutBoundary {
			h.relayoutBoundary = relayoutBoundary
			var propagateRelayoutBoundary func(child Object) bool
			propagateRelayoutBoundary = func(child Object) bool {
				childh := child.Handle()
				if childh.relayoutBoundary == child {
					return true
				}
				parentRelayoutBoundary := childh.Parent.Handle().relayoutBoundary
				if parentRelayoutBoundary != childh.relayoutBoundary {
					childh.relayoutBoundary = parentRelayoutBoundary
					if child, ok := child.(ObjectWithChildren); ok {
						for child2 := range child.Children() {
							propagateRelayoutBoundary(child2)
						}
					}
				}
				return true
			}
			if obj, ok := obj.(ObjectWithChildren); ok {
				for child := range obj.Children() {
					propagateRelayoutBoundary(child)
				}
			}
		}
		return obj.Handle().size
	}
	h.constraints = cs
	if h.relayoutBoundary != nil && relayoutBoundary != h.relayoutBoundary {
		// The local relayout boundary has changed, must notify children in case
		// they also need updating. Otherwise, they will be confused about what
		// their actual relayout boundary is later.
		if obj, ok := obj.(ObjectWithChildren); ok {
			for child := range obj.Children() {
				cleanRelayoutBoundary(child)
			}
		}
	}
	obj.Handle().size = obj.PerformLayout()
	h.needsLayout = false
	MarkNeedsPaint(obj)

	sz := obj.Handle().Size()
	if sz.Width < cs.Min.Width || sz.Width > cs.Max.Width || sz.Height < cs.Min.Height || sz.Height > cs.Max.Height {
		panic(fmt.Sprintf("(%[1]T)(%[1]p).Layout violated constraints %v by computing size %v", obj, cs, sz))
	}

	return sz
}

func cleanRelayoutBoundary(child Object) bool {
	childh := child.Handle()
	if childh.relayoutBoundary != child {
		childh.relayoutBoundary = nil
		if child, ok := child.(ObjectWithChildren); ok {
			for child2 := range child.Children() {
				cleanRelayoutBoundary(child2)
			}
		}
	}
	return true
}

func NewPainter() *Painter {
	return &Painter{}
}

// TODO(dh): evaluate if we actually need Dispose, or if GC does all the work for us
func Dispose(obj Object) {
	if obj, ok := obj.(Disposable); ok {
		obj.PerformDispose()
	}
}

func ScheduleInitialLayout(obj Object) {
	h := obj.Handle()
	h.needsLayout = true
	h.relayoutBoundary = obj
	h.renderer.nodesNeedingLayout.Front = append(h.renderer.nodesNeedingLayout.Front, obj)
}

func ScheduleInitialPaint(obj Object) {
	h := obj.Handle()
	h.needsPaint = true
}

func InsertChild(parent ObjectWithChildren, child Object, after int) {
	parent.PerformInsertChild(child, after)
	adoptChild(parent, child)
}

func MoveChild(parent ObjectWithChildren, child Object, after int) {
	parent.PerformMoveChild(child, after)
	MarkNeedsLayout(parent)
}

func RemoveChild(parent ChildRemover, child Object) {
	parent.PerformRemoveChild(child)
	dropChild(parent, child)
}

func adoptChild(parent, child Object) {
	debug.Assert(child.Handle().Parent == nil)
	if parent, ok := parent.(ParentDataSetuper); ok {
		parent.PerformSetupParentData(child)
	}
	MarkNeedsLayout(parent)
	child.Handle().Parent = parent
	if parent.Handle().Attached() {
		Attach(child, parent.Handle().renderer)
	}
}

func dropChild(parent, child Object) {
	debug.Assert(child.Handle().Parent == parent)
	debug.Assert(child.Handle().Attached() == parent.Handle().Attached())
	// child._cleanRelayoutBoundary();
	// child.parentData!.detach();
	child.Handle().ParentData = nil
	child.Handle().Parent = nil
	if parent.Handle().Attached() {
		Detach(child)
	}
	MarkNeedsLayout(parent)
}
