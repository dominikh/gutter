// SPDX-FileCopyrightText: 2025 Dominik Honnef and contributors
//
// SPDX-License-Identifier: MIT

package widget

import (
	"honnef.co/go/gutter/maybe"
	"honnef.co/go/gutter/paint"
	"honnef.co/go/gutter/render"
	"honnef.co/go/gutter/text"
	"honnef.co/go/gutter/text/bidi"
)

type WidgetSpan interface{}

var _ RenderObjectWidget = (*RichText)(nil)

type RichText struct {
	Text          paint.InlineSpan
	TextAlign     text.Alignment
	TextDirection maybe.Option[bidi.Direction]
	SingleLine    bool
	Overflow      text.Overflow
	// XXX textScaler
	MaxLines maybe.Option[int]
	// Language maybe.Option[paint.Language]
	// XXX StrutStyle
	// XXX TextWidthBasis
	// XXX textHeightBehavior (why is there no height field?)
	// XXX selection registrar
	// XXX selection color
}

// CreateElement implements widget.RenderObjectWidget.
func (r *RichText) CreateElement() Element {
	return NewRenderObjectElement(r)
}

// CreateRenderObject implements widget.RenderObjectWidget.
func (r *RichText) CreateRenderObject(ctx BuildContext) render.Object {
	// XXX
	return &render.Paragraph{}
}

// UpdateRenderObject implements widget.RenderObjectWidget.
func (r *RichText) UpdateRenderObject(ctx BuildContext, obj render.Object) {
	panic("unimplemented")
	// XXX update all the fields
}
