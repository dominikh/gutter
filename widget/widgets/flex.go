// SPDX-FileCopyrightText: 2014 The Flutter Authors. All rights reserved.
// SPDX-FileCopyrightText: 2024 Dominik Honnef and contributors
//
// SPDX-License-Identifier: MIT AND BSD-3-Clause

package widgets

import (
	"honnef.co/go/gutter/render"
	"honnef.co/go/gutter/widget"
	"honnef.co/go/stuff/container/maybe"
)

type Flex struct {
	Direction          render.Axis
	MainAxisAlignment  render.MainAxisAlignment
	MainAxisSize       render.MainAxisSize
	CrossAxisAlignment render.CrossAxisAlignment
	// XXX add clip behavior
	Children []widget.Widget
}

// CreateRenderObject implements RenderObjectWidget.
func (f *Flex) CreateRenderObject(ctx widget.BuildContext) render.Object {
	obj := &render.Flex{}
	f.UpdateRenderObject(ctx, obj)
	return obj
}

// UpdateRenderObject implements RenderObjectWidget.
func (f *Flex) UpdateRenderObject(ctx widget.BuildContext, obj render.Object) {
	fobj := obj.(*render.Flex)
	fobj.SetDirection(f.Direction)
	fobj.SetMainAxisAlignment(f.MainAxisAlignment)
	fobj.SetMainAxisSize(f.MainAxisSize)
	fobj.SetCrossAxisAlignment(f.CrossAxisAlignment)
}

// CreateElement implements MultiChildWidget.
func (f *Flex) CreateElement() widget.Element {
	return widget.NewRenderObjectElement(f)
}

var _ widget.StatelessWidget = (*Flexible)(nil)

type Flexible struct {
	Flex  float64
	Fit   render.FlexFit
	Child widget.Widget
}

// Build implements widget.StatelessWidget.
func (f *Flexible) Build(ctx widget.BuildContext) widget.Widget {
	return f.Child
}

// CreateElement implements SingleChildWidget.
func (f *Flexible) CreateElement() widget.Element {
	return widget.NewInteriorElement(f)
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
	Children           []widget.Widget
}

func (r *Row) CreateElement() widget.Element {
	return widget.NewInteriorElement(r)
}

func (r *Row) Build(ctx widget.BuildContext) widget.Widget {
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
	Children           []widget.Widget
}

func (r *Column) CreateElement() widget.Element {
	return widget.NewInteriorElement(r)
}

func (r *Column) Build(ctx widget.BuildContext) widget.Widget {
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

func (s *Spacer) CreateElement() widget.Element {
	return widget.NewInteriorElement(s)
}

func (s *Spacer) Build(ctx widget.BuildContext) widget.Widget {
	return &Flexible{
		Flex:  s.Flex.UnwrapOr(1.0),
		Fit:   render.FlexFitTight,
		Child: &SizedBox{},
	}
}
