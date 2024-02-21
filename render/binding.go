// SPDX-FileCopyrightText: 2014 The Flutter Authors. All rights reserved.
// SPDX-FileCopyrightText: 2024 Dominik Honnef and contributors
//
// SPDX-License-Identifier: MIT AND BSD-3-Clause

package render

import (
	"time"

	"honnef.co/go/gutter/debug"
	"honnef.co/go/gutter/f32"
	"honnef.co/go/gutter/io/pointer"

	"gioui.org/app"
	giopointer "gioui.org/io/pointer"
	"gioui.org/op"
)

type Binding struct {
	pipelineOwner *PipelineOwner
	htr           hitTestResult
}

func NewBinding(win *app.Window) *Binding {
	b := &Binding{
		pipelineOwner: NewPipelineOwner(),
	}
	b.pipelineOwner.OnNeedVisualUpdate = win.Invalidate
	v := NewView()
	b.SetView(v)
	v.PrepareInitialFrame()
	return b
}

func (b *Binding) RunFrameCallbacks(now time.Time) {
	b.pipelineOwner.RunFrameCallbacks(now)
}

func (b *Binding) DrawFrame(e app.FrameEvent, ops *op.Ops) {
	debug.Assert(b.View() != nil)
	b.View().SetConfiguration(ViewConfiguration{Min: f32.FPt(e.Size), Max: f32.FPt(e.Size)})
	b.pipelineOwner.FlushLayout()
	b.pipelineOwner.FlushCompositingBits()
	b.pipelineOwner.FlushPaint(ops)
	e.Frame(ops)

}

func (b *Binding) View() *View {
	return b.pipelineOwner.rootNode.(*View)
}

func (b *Binding) SetView(v *View) {
	debug.Assert(v != nil)
	b.pipelineOwner.SetRootNode(v)
}

func (b *Binding) HandlePointerEvent(e giopointer.Event) {
	b.htr.Reset()
	hitTest(&b.htr, b.pipelineOwner.rootNode, e.Position)
	hits := b.htr.hits
	n := 0
	for _, hit := range hits {
		if _, ok := hit.Object.(PointerEventHandler); ok {
			n++
			if n >= 2 {
				break
			}
		}
	}
	var kind pointer.Priority
	if n < 2 {
		kind = pointer.Exclusive
	} else {
		kind = pointer.Shared
	}
	first := true
	for _, hit := range hits {
		if obj, ok := hit.Object.(PointerEventHandler); ok {
			prio := kind
			if first && prio == pointer.Shared {
				prio = pointer.Foremost
			}
			first = false
			ev := pointer.FromRaw(e)
			ev.Priority = prio
			obj.HandlePointerEvent(hit, ev)
		}
	}
}

var _ Object = (*View)(nil)

type View struct {
	ObjectHandle
	SingleChild

	r             *Renderer
	ops           op.Ops
	configuration ViewConfiguration
}

func NewView() *View {
	return &View{
		r: NewRenderer(),
	}
}

func (v *View) PerformPaint(r *Renderer, ops *op.Ops) {
	if v.Child != nil {
		r.Paint(v.Child).Add(ops)
	}
}

// XXX include pxperdp etc in the view configuration
type ViewConfiguration = Constraints

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

func (v *View) PerformLayout() f32.Point {
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
