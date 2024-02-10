package render

import (
	"gioui.org/io/pointer"
	"gioui.org/op"
	"honnef.co/go/gutter/f32"
)

var _ Object = (*PointerRegion)(nil)
var _ PointerEventHandler = (*PointerRegion)(nil)

type HitTestEntry struct {
	Object Object
	Offset f32.Point
}

type HitTestResult struct {
	Hits           []HitTestEntry
	transform      f32.Affine2D
	transformStack []f32.Affine2D
}

func (ht *HitTestResult) PushTransform(trans f32.Affine2D) {
	ht.transformStack = append(ht.transformStack, ht.transform)
	ht.transform = ht.transform.Mul(trans)
}

func (ht *HitTestResult) PopTransform() {
	if len(ht.transformStack) > 0 {
		ht.transform = ht.transformStack[len(ht.transformStack)-1]
		ht.transformStack = ht.transformStack[:len(ht.transformStack)-1]
	}
}

func (ht *HitTestResult) Transform(p f32.Point) f32.Point {
	return ht.transform.Transform(p)
}

func (ht *HitTestResult) PushOffset(offset f32.Point) {
	ht.PushTransform(f32.Affine2D{}.Offset(offset).Invert())
}

func (ht *HitTestResult) Add(obj Object, pos f32.Point) {
	ht.Hits = append(ht.Hits, HitTestEntry{obj, pos})
}

type HitTester interface {
	HitTest(res *HitTestResult, pos f32.Point) bool
}

type ChildrenHitTester interface {
	HitTestChildren(res *HitTestResult, pos f32.Point) bool
}

func HitTest(res *HitTestResult, obj Object, pos f32.Point) bool {
	if ht, ok := obj.(HitTester); ok {
		return ht.HitTest(res, pos)
	} else {
		h := obj.Handle()
		tpos := res.Transform(pos)
		if !(f32.Rectangle{Min: f32.Pt(0, 0), Max: h.size}).Contains(tpos) {
			return false
		}
		// If we hit a child, or are opaque, then we've been hit
		hit := HitTestChildren(res, obj, pos) || h.HitTestBehavior == Opaque
		// If we're translucent then we're still part of the result, but don't prevent other objects from
		// being hit.
		if hit || h.HitTestBehavior == Translucent {
			res.Add(obj, tpos)
		}
		return hit
	}
}

func HitTestChildren(res *HitTestResult, obj Object, pos f32.Point) bool {
	if ht, ok := obj.(ChildrenHitTester); ok {
		return ht.HitTestChildren(res, pos)
	} else {
		hit := false
		obj.VisitChildren(func(o Object) bool {
			res.PushOffset(o.Handle().offset)
			defer res.PopTransform()
			if HitTest(res, o, pos) {
				hit = true
			}
			return true
		})
		return hit
	}
}

type HitTestBehavior uint8

const (
	DeferToChild HitTestBehavior = iota
	Opaque
	Translucent
)

type PointerEventHandler interface {
	HandlePointerEvent(hit HitTestEntry, ev pointer.Event)
}

type PointerRegion struct {
	Box
	SingleChild
	OnMove func(hit HitTestEntry, ev pointer.Event)
}

// Layout implements render.Object.
func (c *PointerRegion) Layout() (size f32.Point) {
	if c.Child != nil {
		return Layout(c.Child, c.Constraints(), true)
	} else {
		return c.Constraints().Max
	}
}

// Paint implements render.Object.
func (c *PointerRegion) Paint(r *Renderer, ops *op.Ops) {
	if c.Child != nil {
		r.Paint(c.Child).Add(ops)
	}
}

func (c *PointerRegion) HandlePointerEvent(hit HitTestEntry, ev pointer.Event) {
	switch ev.Kind {
	case pointer.Move, pointer.Drag:
		c.OnMove(hit, ev)
	}
}
