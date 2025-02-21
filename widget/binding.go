// SPDX-FileCopyrightText: 2014 The Flutter Authors. All rights reserved.
// SPDX-FileCopyrightText: 2024 Dominik Honnef and contributors
//
// SPDX-License-Identifier: MIT AND BSD-3-Clause

package widget

import (
	"honnef.co/go/gutter/debug"
	"honnef.co/go/gutter/render"
	"honnef.co/go/gutter/wsi"
	"honnef.co/go/jello"
)

func RunApp(sys *wsi.System, win wsi.Window, app Widget) *Binding {
	b := NewBinding(sys, win)
	b.rootWidget = app
	// b.scheduleWarmUpFrame()
	return b
}

type Binding struct {
	renderViewElement *renderObjectToWidgetElement
	buildOwner        *BuildOwner
	Renderer          *render.Renderer
	rootWidget        Widget

	mediaQuery *MediaQuery
}

func NewBinding(sys *wsi.System, win wsi.Window) *Binding {
	b := &Binding{
		buildOwner: NewBuildOwner(),
		Renderer:   render.NewRenderer(),
	}
	b.buildOwner.EmitEvent = func(ev wsi.Event) {
		// TODO(dh): add a wsi.Window.EmitEvent method
		sys.EmitEvent(win, ev)
	}
	b.Renderer.OnNeedVisualUpdate = win.RequestFrame
	b.buildOwner.Renderer = b.Renderer
	b.buildOwner.OnBuildScheduled = win.RequestFrame
	return b
}

func (b *Binding) DrawFrame(ev *wsi.RedrawRequested, scene *jello.Scene) {
	debug.Assert(!b.buildOwner.inDrawFrame)
	b.buildOwner.inDrawFrame = true
	b.AttachRootWidget(b.rootWidget)
	b.Renderer.RunFrameCallbacks(ev.When)
	if b.renderViewElement != nil {
		b.buildOwner.BuildScope(nil)
	}
	b.Renderer.DrawFrame(scene)
	b.buildOwner.finalizeTree()
	b.buildOwner.inDrawFrame = false
}

func (b *Binding) AttachRootWidget(rootWidget Widget) {
	cs := b.Renderer.View().Configuration()
	data := MediaQueryData{
		Scale: 1.0, // XXX scale
		Size:  cs.Max,
	}
	if b.mediaQuery == nil || b.mediaQuery.Data != data {
		b.mediaQuery = &MediaQuery{
			Data:  data,
			Child: rootWidget,
		}
	}
	b.renderViewElement = (&renderObjectToWidgetAdapter{
		container: b.Renderer.View(),
		child:     b.mediaQuery,
	}).AttachToRenderTree(b.buildOwner, b.renderViewElement)
}

var _ RenderObjectWidget = (*renderObjectToWidgetAdapter)(nil)

type renderObjectToWidgetAdapter struct {
	child     Widget
	container render.Object
}

func (w *renderObjectToWidgetAdapter) CreateElement() Element {
	return newRenderObjectToWidgetElement(w)
}

func (w *renderObjectToWidgetAdapter) CreateRenderObject(ctx BuildContext) render.Object {
	return w.container
}

func (w *renderObjectToWidgetAdapter) UpdateRenderObject(ctx BuildContext, obj render.Object) {}

func (w *renderObjectToWidgetAdapter) AttachToRenderTree(bo *BuildOwner, el *renderObjectToWidgetElement) *renderObjectToWidgetElement {
	if el == nil {
		el = w.CreateElement().(*renderObjectToWidgetElement)
		el.AssignOwner(bo)
		bo.BuildScope(func() {
			mount(el, nil, 0)
		})
	} else {
		el.newWidget = w
		MarkNeedsBuild(el)
	}
	return el
}

var _ renderObjectElement = (*renderObjectToWidgetElement)(nil)

type renderObjectToWidgetElement struct {
	renderObjectElementHandle
	singleChildElement

	newWidget Widget
}

func newRenderObjectToWidgetElement(w Widget) *renderObjectToWidgetElement {
	var el renderObjectToWidgetElement
	el.widget = w
	return &el
}

func (el *renderObjectToWidgetElement) AssignOwner(owner *BuildOwner) {
	el.BuildOwner = owner
}

func (el *renderObjectToWidgetElement) transition(t elementTransition) {
	switch t.kind {
	case elementMounted:
		renderObjectElementAfterMount(el, t.parent, t.newSlot)
		el.rebuild()
	case elementUpdated:
		renderObjectElementAfterUpdate(el, t.oldWidget)
		el.rebuild()
	}
}
func (el *renderObjectToWidgetElement) performRebuild() {
	if el.newWidget != nil {
		w := el.newWidget
		el.newWidget = nil
		update(el, w)
	}
}

func (el *renderObjectToWidgetElement) rebuild() {
	el.SetChild(updateChild(el, el.Child(), el.widget.(*renderObjectToWidgetAdapter).child, 0))
}

func (el *renderObjectToWidgetElement) insertRenderObjectChild(child render.Object, slot int) {
	renderObjectElementInsertRenderObjectChild(el, child, slot)
}

func (el *renderObjectToWidgetElement) removeRenderObjectChild(child render.Object, slot int) {
	renderObjectElementRemoveRenderObjectChild(el, child, slot)
}

func (el *renderObjectToWidgetElement) moveRenderObjectChild(child render.Object, newSlot int) {
	panic("unexpected call")
}

func (el *renderObjectToWidgetElement) attachRenderObject(slot int) {
	renderObjectElementAttachRenderObject(el, slot)
}
