package render

import (
	"fmt"
	"math"
	"slices"
	"strings"

	"honnef.co/go/gutter/debug"
	"honnef.co/go/gutter/f32"

	"gioui.org/op"
)

// TODO implement support for multiple layers
// TODO guard assertions behind debug flag
// TODO support baseline stuff
// TODO hit testing
// TODO accessibility
// TODO RTL support (see https://api.flutter.dev/flutter/dart-ui/TextDirection.html)
// TODO should we handle nil children?
// TODO dry layout/intrinsic dimensions/https://github.com/flutter/flutter/issues/48679

// OPT if we could call op.Ops directly, then we wouldn't have to repaint parents, because their cached ops
//   would still be calling the repainted ops of the child. However, Gio makes us go through macros, and
//   macros record both the start and end PC, and we can't expect those to remain the same.

type Object interface {
	// PerformLayout lays out the object.
	PerformLayout() (size f32.Point)
	// PerformPaint paints the object at the specified offset.
	PerformPaint(r *Renderer, ops *op.Ops)

	VisitChildren(yield func(Object) bool)
	Handle() *ObjectHandle
}

type Attacher interface {
	PerformAttach(owner *PipelineOwner)
	PerformDetach()
}

type ObjectWithChildren interface {
	Object
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
	size f32.Point
	// The object's position as a relative offset from the parent object's origin. Having to configure a
	// child's offset is so common that we have a dedicated field for it, instead of requiring the use of
	// parentData.
	offset                     f32.Point
	ParentData                 any
	needsPaint                 bool
	needsLayout                bool
	needsCompositingBitsUpdate bool
	Parent                     Object
	constraints                Constraints
	relayoutBoundary           Object
	depth                      int
	owner                      *PipelineOwner
	HitTestBehavior            HitTestBehavior
}

func (h *ObjectHandle) Handle() *ObjectHandle    { return h }
func (h *ObjectHandle) Size() f32.Point          { return h.size }
func (h *ObjectHandle) Constraints() Constraints { return h.constraints }

func MarkNeedsPaint(obj Object) {
	h := obj.Handle()
	if h.needsPaint {
		return
	}
	h.needsPaint = true

	// We always have to walk the tree up to the parent because our composition of objects is implemented by
	// parents calling op.CallOp.
	if h.Parent != nil {
		MarkNeedsPaint(h.Parent)
	} else {
		if h.owner != nil {
			h.owner.RequestVisualUpdate()
		}
	}
}
func MarkNeedsLayout(obj Object) {
	h := obj.Handle()
	if h.needsLayout {
		return
	}

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
		h.owner.nodesNeedingLayout.Front = append(h.owner.nodesNeedingLayout.Front, obj)
		h.owner.RequestVisualUpdate()
	}
}

func (h *ObjectHandle) SetParent(parent Object) { h.Parent = parent }

type Constraints struct {
	Min, Max f32.Point
}

func (c Constraints) Tight() bool {
	return c.Min == c.Max && float64(c.Max.X) != math.Inf(1) && float64(c.Max.Y) != math.Inf(1)
}

func (c Constraints) Enforce(oc Constraints) Constraints {
	return Constraints{
		Min: f32.Point{
			X: f32.Clamp(c.Min.X, oc.Min.X, oc.Max.X),
			Y: f32.Clamp(c.Min.Y, oc.Min.Y, oc.Max.Y),
		},
		Max: f32.Point{
			X: f32.Clamp(c.Max.X, oc.Min.X, oc.Max.X),
			Y: f32.Clamp(c.Max.Y, oc.Min.Y, oc.Max.Y),
		},
	}
}

// Constrain a size so each dimension is in the range [min;max].
func (c Constraints) Constrain(size f32.Point) f32.Point {
	if min := c.Min.X; size.X < min {
		size.X = min
	}
	if min := c.Min.Y; size.Y < min {
		size.Y = min
	}
	if max := c.Max.X; size.X > max {
		size.X = max
	}
	if max := c.Max.Y; size.Y > max {
		size.Y = max
	}
	return size
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
		root.VisitChildren(func(o Object) bool {
			formatTree(o, depth+1)
			return true
		})
	}
	formatTree(root, 0)

	return sb.String()
}

type SingleChild struct {
	Child Object
}

func (c *SingleChild) VisitChildren(yield func(Object) bool) {
	if c.Child != nil {
		yield(c.Child)
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

func (c *ManyChildren) VisitChildren(yield func(Object) bool) {
	for _, child := range c.children {
		if !yield(child) {
			break
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

type Renderer struct {
	// XXX delete from map when objects disappear
	ops map[Object]cachedOps
	// needsLayout []Object
	// needsPaint  []Object
}

type cachedOps struct {
	ops  *op.Ops
	call op.CallOp
}

func (r *Renderer) Paint(obj Object) op.CallOp {
	var ops *op.Ops
	if obj.Handle().needsPaint {
		obj.Handle().needsPaint = false
		if cached, ok := r.ops[obj]; ok {
			ops = cached.ops
			ops.Reset()
		} else {
			ops = new(op.Ops)
		}
	} else if cached, ok := r.ops[obj]; ok {
		return cached.call
	} else {
		ops = new(op.Ops)
	}

	m := op.Record(ops)
	obj.PerformPaint(r, ops)
	call := m.Stop()
	r.ops[obj] = cachedOps{ops, call}
	return call
}

func isType[T any](obj any) bool {
	_, ok := obj.(T)
	return ok
}

func Layout(obj Object, cs Constraints, parentUsesSize bool) f32.Point {
	if cs.Min.X > cs.Max.X || cs.Min.Y > cs.Max.Y || cs.Min.X < 0 || cs.Min.Y < 0 {
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
					child.VisitChildren(propagateRelayoutBoundary)
				}
				return true
			}
			obj.VisitChildren(propagateRelayoutBoundary)
		}
		return obj.Handle().size
	}
	h.constraints = cs
	if h.relayoutBoundary != nil && relayoutBoundary != h.relayoutBoundary {
		// The local relayout boundary has changed, must notify children in case
		// they also need updating. Otherwise, they will be confused about what
		// their actual relayout boundary is later.
		obj.VisitChildren(cleanRelayoutBoundary)
	}
	obj.Handle().size = obj.PerformLayout()
	h.needsLayout = false
	MarkNeedsPaint(obj)

	sz := obj.Handle().Size()
	if sz.X < cs.Min.X || sz.X > cs.Max.X || sz.Y < cs.Min.Y || sz.Y > cs.Max.Y {
		panic(fmt.Sprintf("(%[1]T)(%[1]p).Layout violated constraints %v by computing size %v", obj, cs, sz))
	}

	return sz
}

func cleanRelayoutBoundary(child Object) bool {
	childh := child.Handle()
	if childh.relayoutBoundary != child {
		childh.relayoutBoundary = nil
		child.VisitChildren(cleanRelayoutBoundary)
	}
	return true
}

func NewRenderer() *Renderer {
	return &Renderer{
		ops: make(map[Object]cachedOps),
	}
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
	h.owner.nodesNeedingLayout.Front = append(h.owner.nodesNeedingLayout.Front, obj)
}

func ScheduleInitialPaint(obj Object) {
	h := obj.Handle()
	h.needsPaint = true
}

func InsertChild(parent ObjectWithChildren, child Object, after int) {
	parent.PerformInsertChild(child, after)
	child.Handle().Parent = parent
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
	if parent, ok := parent.(ParentDataSetuper); ok {
		parent.PerformSetupParentData(child)
	}
	MarkNeedsLayout(parent)
}

func dropChild(parent, child Object) {
	// child._cleanRelayoutBoundary();
	// child.parentData!.detach();
	child.Handle().ParentData = nil
	child.Handle().Parent = nil
	// if attached {
	Detach(child)
	// }
	MarkNeedsLayout(parent)
}
