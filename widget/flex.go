// SPDX-FileCopyrightText: 2014 The Flutter Authors. All rights reserved.
// SPDX-FileCopyrightText: 2024 Dominik Honnef and contributors
//
// SPDX-License-Identifier: MIT AND BSD-3-Clause

package widget

import (
	"honnef.co/go/gutter/maybe"
	"honnef.co/go/gutter/render"
)

type Flex struct {
	Direction          render.Axis
	MainAxisAlignment  render.MainAxisAlignment
	MainAxisSize       render.MainAxisSize
	CrossAxisAlignment render.CrossAxisAlignment
	// XXX add clip behavior
	Children []Widget
}

// CreateRenderObject implements RenderObjectWidget.
func (f *Flex) CreateRenderObject(ctx BuildContext) render.Object {
	obj := &render.Flex{}
	f.UpdateRenderObject(ctx, obj)
	return obj
}

// UpdateRenderObject implements RenderObjectWidget.
func (f *Flex) UpdateRenderObject(ctx BuildContext, obj render.Object) {
	fobj := obj.(*render.Flex)
	fobj.SetDirection(f.Direction)
	fobj.SetMainAxisAlignment(f.MainAxisAlignment)
	fobj.SetMainAxisSize(f.MainAxisSize)
	fobj.SetCrossAxisAlignment(f.CrossAxisAlignment)
}

// CreateElement implements MultiChildWidget.
func (f *Flex) CreateElement() Element {
	return NewRenderObjectElement(f)
}

type Flexible struct {
	Flex  float64
	Fit   render.FlexFit
	Child Widget
}

// CreateElement implements SingleChildWidget.
func (f *Flexible) CreateElement() Element {
	return NewProxyElement(f)
}

func (f *Flexible) ApplyParentData(obj render.Object) {
	data := obj.Handle().ParentData.(*render.FlexParentData)
	var needsLayout bool
	if data.Flex != f.Flex {
		data.Flex = f.Flex
		needsLayout = true
	}
	if data.Fit != f.Fit {
		data.Fit = f.Fit
		needsLayout = true
	}

	if needsLayout {
		if p := obj.Handle().Parent; p != nil {
			render.MarkNeedsLayout(p)
		}
	}
}

type Row struct {
	MainAxisAlignment  render.MainAxisAlignment
	MainAxisSize       render.MainAxisSize
	CrossAxisAlignment render.CrossAxisAlignment
	Children           []Widget
}

func (r *Row) CreateElement() Element {
	return NewInteriorElement(r)
}

func (r *Row) Build(ctx BuildContext) Widget {
	return &Flex{
		Direction:          render.Horizontal,
		MainAxisAlignment:  r.MainAxisAlignment,
		MainAxisSize:       r.MainAxisSize,
		CrossAxisAlignment: r.CrossAxisAlignment,
		Children:           r.Children,
	}
}

type Column struct {
	MainAxisAlignment  render.MainAxisAlignment
	MainAxisSize       render.MainAxisSize
	CrossAxisAlignment render.CrossAxisAlignment
	Children           []Widget
}

func (r *Column) CreateElement() Element {
	return NewInteriorElement(r)
}

func (r *Column) Build(ctx BuildContext) Widget {
	return &Flex{
		Direction:          render.Vertical,
		MainAxisAlignment:  r.MainAxisAlignment,
		MainAxisSize:       r.MainAxisSize,
		CrossAxisAlignment: r.CrossAxisAlignment,
		Children:           r.Children,
	}
}

type Spacer struct {
	Flex maybe.Option[float64]
}

func (s *Spacer) CreateElement() Element {
	return NewInteriorElement(s)
}

func (s *Spacer) Build(ctx BuildContext) Widget {
	return &Flexible{
		Flex:  s.Flex.UnwrapOr(1.0),
		Fit:   render.FlexFitTight,
		Child: &SizedBox{},
	}
}
