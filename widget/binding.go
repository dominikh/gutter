// SPDX-FileCopyrightText: 2014 The Flutter Authors. All rights reserved.
// SPDX-FileCopyrightText: 2024 Dominik Honnef and contributors
//
// SPDX-License-Identifier: MIT AND BSD-3-Clause

package widget

import (
	"gioui.org/app"
	"gioui.org/op"
	"honnef.co/go/gutter/render"
)

func RunApp(win *app.Window, app Widget) *Binding {
	b := NewBinding(win)
	b.AttachRootWidget(app)
	// b.scheduleWarmUpFrame()
	return b
}

type Binding struct {
	renderViewElement *RenderObjectToWidgetElement
	buildOwner        *BuildOwner
	RenderBinding     *render.Binding
}

func NewBinding(win *app.Window) *Binding {
	b := &Binding{
		buildOwner:    NewBuildOwner(),
		RenderBinding: render.NewBinding(win),
	}
	b.buildOwner.PipelineOwner = b.RenderBinding.PipelineOwner
	b.buildOwner.OnBuildScheduled = win.Invalidate
	return b
}

func (b *Binding) DrawFrame(e app.FrameEvent, ops *op.Ops) {
	ops.Reset()
	b.RenderBinding.RunFrameCallbacks(e.Now)
	if b.renderViewElement != nil {
		b.buildOwner.BuildScope(nil)
	}
	b.RenderBinding.DrawFrame(e, ops)
	b.buildOwner.FinalizeTree()
}

func (b *Binding) AttachRootWidget(rootWidget Widget) {
	b.renderViewElement = (&RenderObjectToWidgetAdapter{
		Container: b.RenderBinding.View(),
		Child:     rootWidget,
	}).AttachToRenderTree(b.buildOwner, b.renderViewElement)
}

var _ RenderObjectWidget = (*RenderObjectToWidgetAdapter)(nil)

type RenderObjectToWidgetAdapter struct {
	Child     Widget
	Container render.Object
}

func (w *RenderObjectToWidgetAdapter) CreateElement() Element {
	return newRenderObjectToWidgetElement(w)
}

func (w *RenderObjectToWidgetAdapter) CreateRenderObject(ctx BuildContext) render.Object {
	return w.Container
}

func (w *RenderObjectToWidgetAdapter) UpdateRenderObject(ctx BuildContext, obj render.Object) {}

func (w *RenderObjectToWidgetAdapter) AttachToRenderTree(bo *BuildOwner, el *RenderObjectToWidgetElement) *RenderObjectToWidgetElement {
	if el == nil {
		el = w.CreateElement().(*RenderObjectToWidgetElement)
		el.AssignOwner(bo)
		bo.BuildScope(func() {
			Mount(el, nil, 0)
		})
	} else {
		el.newWidget = w
		MarkNeedsBuild(el)
	}
	return el
}

var _ RenderObjectElement = (*RenderObjectToWidgetElement)(nil)

type RenderObjectToWidgetElement struct {
	RenderObjectElementHandle
	SingleChildElement

	newWidget Widget
}

func newRenderObjectToWidgetElement(w Widget) *RenderObjectToWidgetElement {
	var el RenderObjectToWidgetElement
	el.widget = w
	return &el
}

func (el *RenderObjectToWidgetElement) AttachRenderObject(slot int) {
	RenderObjectElementAttachRenderObject(el, slot)
}

func (el *RenderObjectToWidgetElement) AssignOwner(owner *BuildOwner) {
	el.BuildOwner = owner
}

func (el *RenderObjectToWidgetElement) Transition(t ElementTransition) {
	switch t.Kind {
	case ElementMounted:
		RenderObjectElementAfterMount(el, t.Parent, t.NewSlot)
		el.rebuild()
	case ElementUpdated:
		RenderObjectElementAfterUpdate(el, t.OldWidget)
		el.rebuild()
	}
}
func (el *RenderObjectToWidgetElement) PerformRebuild() {
	if el.newWidget != nil {
		w := el.newWidget
		el.newWidget = nil
		Update(el, w)
	}
}

func (el *RenderObjectToWidgetElement) rebuild() {
	el.SetChild(UpdateChild(el, el.Child(), el.widget.(*RenderObjectToWidgetAdapter).Child, 0))
}

func (el *RenderObjectToWidgetElement) InsertRenderObjectChild(child render.Object, slot int) {
	RenderObjectElementInsertRenderObjectChild(el, child, slot)
}

func (el *RenderObjectToWidgetElement) RemoveRenderObjectChild(child render.Object, slot int) {
	RenderObjectElementRemoveRenderObjectChild(el, child, slot)
}

func (el *RenderObjectToWidgetElement) MoveRenderObjectChild(child render.Object, newSlot int) {
	panic("unexpected call")
}
