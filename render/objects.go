// SPDX-FileCopyrightText: 2014 The Flutter Authors. All rights reserved.
// SPDX-FileCopyrightText: 2024 Dominik Honnef and contributors
//
// SPDX-License-Identifier: MIT AND BSD-3-Clause

package render

import (
	"honnef.co/go/color"
	"honnef.co/go/curve"
	"honnef.co/go/gutter/animation"
	"honnef.co/go/jello"
	"honnef.co/go/jello/gfx"
)

var _ Object = (*FillColor)(nil)

var _ ObjectWithChildren = (*Clip)(nil)
var _ ObjectWithChildren = (*Constrained)(nil)
var _ ObjectWithChildren = (*Opacity)(nil)
var _ ObjectWithChildren = (*Padding)(nil)

type Box struct {
	ObjectHandle
}

// Clip prevents its child from painting outside its bounds.
type Clip struct {
	Box
	SingleChild
}

// PerformLayout implements Object.
func (w *Clip) PerformLayout() curve.Size {
	Layout(w.Child, w.Handle().Constraints(), true)
	return w.Child.Handle().Size()
}

// PerformPaint implements Object.
func (w *Clip) PerformPaint(p *Painter, scene *jello.Scene) {
	scene.PushLayer(
		gfx.BlendMode{},
		1,
		curve.Identity,
		curve.NewRectFromPoints(curve.Pt(0, 0), curve.Point(w.Handle().Size().AsVec2())).Path(0.1),
	)
	defer scene.PopLayer()
	p.PaintAt(w.Child, scene, curve.Point{})
}

// FillColor fills an infinite plane with the provided color.
//
// In layout, it takes up the least amount of space possible.
type FillColor struct {
	Box
	color color.Color
}

// VisitChildren implements Object.
func (*FillColor) VisitChildren(yield func(Object) bool) {}

func (fc *FillColor) SetColor(c color.Color) {
	if fc.color != c {
		fc.color = c
		MarkNeedsPaint(fc)
	}
}

func (fc *FillColor) Color() color.Color {
	return fc.color
}

// PerformLayout implements Object.
func (c *FillColor) PerformLayout() curve.Size {
	return c.Handle().Constraints().Min
}

func (c *FillColor) SizedByParent() {}

// PerformPaint implements Object.
func (c *FillColor) PerformPaint(_ *Painter, scene *jello.Scene) {
	scene.Fill(
		gfx.NonZero,
		curve.Identity,
		gfx.SolidBrush{Color: c.color},
		curve.Identity,
		curve.NewRectFromPoints(curve.Pt(-1e9, -1e9), curve.Pt(1e9, 1e9)).Path(0.1),
	)
}

type Inset struct {
	Left, Top, Right, Bottom float64
}

func LerpInset(start, end Inset, t float64) Inset {
	return Inset{
		Left:   animation.Lerp(start.Left, end.Left, t),
		Top:    animation.Lerp(start.Top, end.Top, t),
		Right:  animation.Lerp(start.Right, end.Right, t),
		Bottom: animation.Lerp(start.Bottom, end.Bottom, t),
	}
}

type Padding struct {
	Box
	SingleChild
	inset Inset
}

func NewPadding(padding Inset) *Padding {
	return &Padding{inset: padding}
}

func (pad *Padding) SetInset(ins Inset) {
	if pad.inset != ins {
		pad.inset = ins
		MarkNeedsLayout(pad)
	}
}

func (pad *Padding) Inset() Inset {
	return pad.inset
}

// PerformLayout implements Object.
func (pad *Padding) PerformLayout() curve.Size {
	cs := pad.Handle().Constraints()
	if pad.Child == nil {
		return cs.Constrain(curve.Sz(pad.inset.Left+pad.inset.Right, pad.inset.Top+pad.inset.Bottom))
	}
	horiz := pad.inset.Left + pad.inset.Right
	vert := pad.inset.Top + pad.inset.Bottom
	newMin := curve.Sz(max(0, cs.Min.Width-horiz), max(0, cs.Min.Height-vert))
	innerCs := Constraints{
		Min: newMin,
		Max: curve.Sz(max(newMin.Width, cs.Max.Width-horiz), max(newMin.Height, cs.Max.Height-vert)),
	}
	childSz := Layout(pad.Child, innerCs, true)
	pad.Child.Handle().offset = curve.Pt(pad.inset.Left, pad.inset.Top)
	return cs.Constrain(childSz.Add(curve.Vec(horiz, vert)))
}

// PerformPaint implements Object.
func (pad *Padding) PerformPaint(p *Painter, scene *jello.Scene) {
	p.PaintAt(pad.Child, scene, pad.Child.Handle().offset)
}

type Constrained struct {
	Box
	SingleChild
	extraConstraints Constraints
}

func (c *Constrained) SetExtraConstraints(cs Constraints) {
	if c.extraConstraints != cs {
		c.extraConstraints = cs
		MarkNeedsLayout(c)
	}
}

func (c *Constrained) ExtraConstraints() Constraints {
	return c.extraConstraints
}

// PerformLayout implements Object.
func (c *Constrained) PerformLayout() curve.Size {
	cs := c.extraConstraints.Enforce(c.Handle().Constraints())
	Layout(c.Child, cs, true)
	return c.Child.Handle().Size()
}

// PerformPaint implements Object.
func (c *Constrained) PerformPaint(p *Painter, scene *jello.Scene) {
	p.PaintAt(c.Child, scene, curve.Point{})
}

type Opacity struct {
	Box
	SingleChild
	opacity float32
}

// PerformLayout implements Object.
func (o *Opacity) PerformLayout() curve.Size {
	if o.Child != nil {
		return Layout(o.Child, o.constraints, true)
	} else {
		return o.constraints.Constrain(curve.Sz(0, 0))
	}
}

// PerformPaint implements Object.
func (o *Opacity) PerformPaint(p *Painter, scene *jello.Scene) {
	switch o.opacity {
	case 0:
		return
	case 1:
		p.PaintAt(o.Child, scene, curve.Point{})
	default:
		scene.PushLayer(gfx.BlendMode{}, o.opacity, curve.Identity, nil)
		defer scene.PopLayer()
		p.PaintAt(o.Child, scene, curve.Point{})
	}
}

func (o *Opacity) SetOpacity(f float32) {
	if o.opacity != f {
		o.opacity = f
		MarkNeedsPaint(o)
	}
}
