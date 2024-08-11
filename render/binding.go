// SPDX-FileCopyrightText: 2014 The Flutter Authors. All rights reserved.
// SPDX-FileCopyrightText: 2024 Dominik Honnef and contributors
//
// SPDX-License-Identifier: MIT AND BSD-3-Clause

package render

import (
	"time"

	"honnef.co/go/curve"
	"honnef.co/go/gutter/debug"
	"honnef.co/go/gutter/wsi"
	"honnef.co/go/jello"
)

type Binding struct {
	PipelineOwner *PipelineOwner
	htr           hitTestResult
}

func NewBinding(sys *wsi.System, win wsi.Window) *Binding {
	b := &Binding{
		PipelineOwner: NewPipelineOwner(),
	}
	b.PipelineOwner.OnNeedVisualUpdate = win.RequestFrame
	// TODO(dh): add a wsi.Window.EmitEvent method
	b.PipelineOwner.EmitEvent = func(ev wsi.Event) {
		sys.EmitEvent(win, ev)
	}
	v := NewView()
	b.SetView(v)
	v.PrepareInitialFrame()
	return b
}

func (b *Binding) RunFrameCallbacks(now time.Time) {
	b.PipelineOwner.RunFrameCallbacks(now)
}

func (b *Binding) DrawFrame(scene *jello.Scene) {
	debug.Assert(b.View() != nil)
	b.PipelineOwner.FlushLayout()
	b.PipelineOwner.FlushCompositingBits()
	b.PipelineOwner.FlushPaint(scene)
}

func (b *Binding) View() *View {
	return b.PipelineOwner.rootNode.(*View)
}

func (b *Binding) SetView(v *View) {
	debug.Assert(v != nil)
	b.PipelineOwner.SetRootNode(v)
}

// func (b *Binding) HandlePointerEvent(e giopointer.Event) {
// 	b.htr.Reset()
// 	hitTest(&b.htr, b.PipelineOwner.rootNode, e.Position)
// 	hits := b.htr.hits
// 	n := 0
// 	for _, hit := range hits {
// 		if _, ok := hit.Object.(PointerEventHandler); ok {
// 			n++
// 			if n >= 2 {
// 				break
// 			}
// 		}
// 	}
// 	var kind pointer.Priority
// 	if n < 2 {
// 		kind = pointer.Exclusive
// 	} else {
// 		kind = pointer.Shared
// 	}
// 	first := true
// 	for _, hit := range hits {
// 		if obj, ok := hit.Object.(PointerEventHandler); ok {
// 			prio := kind
// 			if first && prio == pointer.Shared {
// 				prio = pointer.Foremost
// 			}
// 			first = false
// 			ev := pointer.FromRaw(e)
// 			ev.Priority = prio
// 			obj.HandlePointerEvent(hit, ev)
// 		}
// 	}
// }

var _ Object = (*View)(nil)

type View struct {
	ObjectHandle
	SingleChild

	r             *Renderer
	configuration ViewConfiguration
}

func NewView() *View {
	return &View{
		r: NewRenderer(),
	}
}

func (v *View) PerformPaint(r *Renderer, scene *jello.Scene) {
	if v.Child != nil {
		scene.Append(r.Paint(v.Child), curve.Identity)
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

func (v *View) PerformLayout() curve.Size {
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
