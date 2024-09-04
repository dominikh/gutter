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
	renderViewElement *RenderObjectToWidgetElement
	buildOwner        *BuildOwner
	Renderer          *render.Renderer
	rootWidget        Widget

	mediaQuery *MediaQuery
}

func NewBinding(sys *wsi.System, win wsi.Window) *Binding {
	b := &Binding{
		buildOwner: NewBuildOwner(),
		Renderer:   render.NewRenderer(sys, win),
	}
	b.buildOwner.Renderer = b.Renderer
	b.buildOwner.OnBuildScheduled = func() {
		// XXX surely there's a better way to rebuild than to ask (and wait!)
		// for a frame from wayland
		//
		// this is not just a matter of performance, but also correctness. we
		// should rebuild the tree immediately after every event that
		// invalidates it, so that consecutive events can observe updates to the
		// tree.
		win.RequestFrame()
	}
	return b
}

func (b *Binding) DrawFrame(ev *wsi.RedrawRequested, scene *jello.Scene) {
	debug.Assert(!b.buildOwner.inDrawFrame)
	b.buildOwner.inDrawFrame = true
	b.AttachRootWidget(b.rootWidget)
	b.buildOwner.RunFrameCallbacks(ev.When)
	if b.renderViewElement != nil {
		b.buildOwner.BuildScope(nil)
	}
	b.Renderer.DrawFrame(scene)
	b.buildOwner.FinalizeTree()
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
	b.renderViewElement = (&RenderObjectToWidgetAdapter{
		Container: b.Renderer.View(),
		Child:     b.mediaQuery,
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
