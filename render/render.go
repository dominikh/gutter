package render

import (
	"fmt"
	"math"
	"strings"

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

// TODO rename Layout to PerformLayout and Paint to PerformPaint

type Object interface {
	// Layout lays out the object.
	//
	// Don't call Object.Layout directly. Use [Layout] instead.
	Layout() (size f32.Point)
	// Paint paints the object at the specified offset.
	//
	// Don't call Object.Paint directly. Use [Renderer.Paint] instead.
	Paint(r *Renderer, ops *op.Ops)

	MarkNeedsLayout()
	MarkNeedsPaint()
	VisitChildren(yield func(Object) bool)
	Handle() *ObjectHandle
}

type Attacher interface {
	Attach(owner *PipelineOwner)
	Detach()
}

type ObjectWithChild interface {
	Object
	SetChild(child Object)
}

type ObjectWithChildren interface {
	Object
	InsertChild(child Object, after Object)
}

type SizedByParenter interface {
	// Marker method that indicates that the object is sized by the parent.
	SizedByParent()
}

type Disposable interface {
	Dispose()
}

type ObjectHandle struct {
	size                       f32.Point
	needsPaint                 bool
	needsLayout                bool
	needsCompositingBitsUpdate bool
	parent                     Object
	constraints                Constraints
	relayoutBoundary           Object
	depth                      int
	owner                      *PipelineOwner
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
	if false /* && isRepaintBoundary(obj) && h.wasRepaintBoundary */ {
		if h.owner != nil {
			h.owner.nodesNeedingPaint = append(h.owner.nodesNeedingPaint, obj)
			h.owner.RequestVisualUpdate()
		}
	} else if h.parent != nil {
		h.parent.MarkNeedsPaint()
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
		if h.parent != nil {
			// _relayoutBoundary is cleaned by an ancestor in RenderObject.layout.
			// Conservatively mark everything dirty until it reaches the closest
			// known relayout boundary.
			h.parent.MarkNeedsLayout()
		}
		return
	}
	if h.relayoutBoundary != obj {
		if h.parent == nil {
			panic(fmt.Sprintf("%[1]T(%[1]p) isn't a relayout boundary but also doesn't have a parent", obj))
		}
		h.parent.MarkNeedsLayout()
	} else {
		h.needsLayout = true
		h.owner.nodesNeedingLayout = append(h.owner.nodesNeedingLayout, obj)
		h.owner.RequestVisualUpdate()
	}
}

func (h *ObjectHandle) SetParent(parent Object) { h.parent = parent }

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

func (c *SingleChild) SetChild(child Object) {
	c.Child = child
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

func (c *ManyChildren) InsertChild(child Object, after Object) {
	panic("not implemented") // XXX
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
	obj.Paint(r, ops)
	call := m.Stop()
	r.ops[obj] = cachedOps{ops, call}
	return call
}

func isType[T any](obj any) bool {
	_, ok := obj.(T)
	return ok
}

func Layout(obj Object, cs Constraints, parentUsesSize bool) (OUT f32.Point) {
	if cs.Min.X > cs.Max.X || cs.Min.Y > cs.Max.Y || cs.Min.X < 0 || cs.Min.Y < 0 {
		panic(fmt.Sprintf("constraints %v are malformed", cs))
	}

	h := obj.Handle()
	var relayoutBoundary Object
	if !parentUsesSize || isType[SizedByParenter](obj) || cs.Tight() {
		// We're the relayout boundary
		relayoutBoundary = obj
	} else {
		relayoutBoundary = h.parent.Handle().relayoutBoundary
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
				parentRelayoutBoundary := childh.parent.Handle().relayoutBoundary
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
		var cleanRelayoutBoundary func(child Object) bool
		cleanRelayoutBoundary = func(child Object) bool {
			childh := child.Handle()
			if childh.relayoutBoundary != child {
				childh.relayoutBoundary = nil
				child.VisitChildren(cleanRelayoutBoundary)
			}
			return true
		}
		obj.VisitChildren(cleanRelayoutBoundary)
	}
	obj.Handle().size = obj.Layout()
	h.needsLayout = false
	obj.MarkNeedsPaint()

	sz := obj.Handle().Size()
	if sz.X < cs.Min.X || sz.X > cs.Max.X || sz.Y < cs.Min.Y || sz.Y > cs.Max.Y {
		panic(fmt.Sprintf("(%[1]T)(%[1]p).Layout violated constraints %v by computing size %v", obj, cs, sz))
	}

	return sz
}

func NewRenderer() *Renderer {
	return &Renderer{
		ops: make(map[Object]cachedOps),
	}
}

// TODO(dh): evaluate if we actually need Dispose, or if GC does all the work for us
func Dispose(obj Object) {
	if obj, ok := obj.(Disposable); ok {
		obj.Dispose()
	}
}

func ScheduleInitialLayout(obj Object) {
	h := obj.Handle()
	h.needsLayout = true
	h.relayoutBoundary = obj
	h.owner.nodesNeedingLayout = append(h.owner.nodesNeedingLayout, obj)
}

func ScheduleInitialPaint(obj Object) {
	h := obj.Handle()
	h.needsPaint = true
	h.owner.nodesNeedingPaint = append(h.owner.nodesNeedingPaint, obj)
}
