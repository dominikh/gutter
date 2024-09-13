// SPDX-FileCopyrightText: 2014 The Flutter Authors. All rights reserved.
// SPDX-FileCopyrightText: 2024 Dominik Honnef and contributors
//
// SPDX-License-Identifier: MIT AND BSD-3-Clause

package render

import (
	"honnef.co/go/curve"
	"honnef.co/go/gutter/io/pointer"
	"honnef.co/go/jello"
)

var _ Object = (*PointerRegion)(nil)
var _ PointerEventHandler = (*PointerRegion)(nil)

type HitTestEntry struct {
	Object Object
	Offset curve.Point
}

type hitTestResult struct {
	hits           []HitTestEntry
	transform      curve.Affine
	transformStack []curve.Affine
}

func (ht *hitTestResult) Reset() {
	clear(ht.hits[:cap(ht.hits)])
	ht.hits = ht.hits[:0]
}

func (ht *hitTestResult) PushTransform(trans curve.Affine) {
	ht.transformStack = append(ht.transformStack, ht.transform)
	ht.transform = ht.transform.Mul(trans)
}

func (ht *hitTestResult) PopTransform() {
	if len(ht.transformStack) > 0 {
		ht.transform = ht.transformStack[len(ht.transformStack)-1]
		ht.transformStack = ht.transformStack[:len(ht.transformStack)-1]
	}
}

func (ht *hitTestResult) Transform(p curve.Point) curve.Point {
	return p.Transform(ht.transform)
}

func (ht *hitTestResult) PushOffset(offset curve.Point) {
	ht.PushTransform(curve.Translate(curve.Vec2(offset)).Invert())
}

func (ht *hitTestResult) Add(obj Object, pos curve.Point) {
	ht.hits = append(ht.hits, HitTestEntry{obj, pos})
}

type HitTester interface {
	PerformHitTest(res *hitTestResult, pos curve.Point) bool
}

func hitTest(res *hitTestResult, obj Object, pos curve.Point) bool {
	if ht, ok := obj.(HitTester); ok {
		return ht.PerformHitTest(res, pos)
	} else {
		h := obj.Handle()
		tpos := res.Transform(pos)
		if !curve.NewRectFromPoints(curve.Pt(0, 0), curve.Point(h.size.AsVec2())).Contains(tpos) {
			return false
		}
		// If we hit a child, or are opaque, then we've been hit
		hit := hitTestChildren(res, obj, pos) || h.HitTestBehavior == Opaque
		// If we're translucent then we're still part of the result, but don't prevent other objects from
		// being hit.
		if hit || h.HitTestBehavior == Translucent {
			res.Add(obj, tpos)
		}
		return hit
	}
}

func hitTestChildren(res *hitTestResult, obj Object, pos curve.Point) bool {
	hit := false
	for o := range obj.Children() {
		res.PushOffset(o.Handle().Offset)
		defer res.PopTransform()
		if hitTest(res, o, pos) {
			hit = true
		}
	}
	return hit
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
	OnPress   func(hit HitTestEntry, ev pointer.Event)
	OnRelease func(hit HitTestEntry, ev pointer.Event)
	OnMove    func(hit HitTestEntry, ev pointer.Event)
	OnScroll  func(hit HitTestEntry, ev pointer.Event)
	OnAll     func(hit HitTestEntry, ev pointer.Event)
}

// PerformLayout implements render.Object.
func (c *PointerRegion) PerformLayout() curve.Size {
	if c.Child != nil {
		return Layout(c.Child, c.Constraints(), true)
	} else {
		return c.Constraints().Max
	}
}

// PerformPaint implements render.Object.
func (c *PointerRegion) PerformPaint(p *Painter, scene *jello.Scene) {
	if c.Child != nil {
		p.PaintAt(c.Child, scene, curve.Point{})
	}
}

func (c *PointerRegion) HandlePointerEvent(hit HitTestEntry, ev pointer.Event) {
	call := func(fn func(hit HitTestEntry, ev pointer.Event)) {
		if fn == nil {
			return
		}
		fn(hit, ev)
	}
	switch ev.Kind {
	case pointer.Move:
		call(c.OnMove)
	case pointer.Press:
		call(c.OnPress)
	case pointer.Release:
		call(c.OnRelease)
	case pointer.Scroll:
		call(c.OnScroll)
	}
	call(c.OnAll)
}
