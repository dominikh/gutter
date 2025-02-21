// SPDX-FileCopyrightText: 2014 The Flutter Authors. All rights reserved.
// SPDX-FileCopyrightText: 2024 Dominik Honnef and contributors
//
// SPDX-License-Identifier: MIT AND BSD-3-Clause

package widget

import (
	"honnef.co/go/gutter/render"
)

func renderObjectElementAfterUpdate(el renderObjectElement, oldWidget Widget) {
	forceRebuild(el)

	// OPT(dh): optimize for the case where we had <2 children before and <2 children now. that doesn't need
	// to allocate slices or go through list reconciliation.
	widget := el.handle().widget.(RenderObjectWidget)
	el.setChildren(updateChildren(el, widgetChildren(widget)))
}
func renderObjectElementAfterMount(el renderObjectElement, parent Element, newSlot int) {
	h := el.renderHandle()
	h.RenderObject = h.widget.(RenderObjectWidget).CreateRenderObject(el)
	attachRenderObject(el, newSlot)
	el.handle().dirty = false

	// OPT(dh): optimize for the single child case, which doesn't need iterators and slices.
	w := el.handle().widget
	var children []Element
	for i, childWidget := range widgetChildrenIter(w) {
		children = append(children, inflateWidget(el, childWidget, i))
	}
	el.setChildren(children)
}
func renderObjectElementAttachRenderObject(el renderObjectElement, slot int) {
	h := el.renderHandle()
	h.slot = slot
	h.ancestorRenderObjectElement = findAncestorRenderObjectElement(el)
	if h.ancestorRenderObjectElement != nil {
		h.ancestorRenderObjectElement.insertRenderObjectChild(h.RenderObject, slot)
	}
	renderObject := el.renderHandle().RenderObject
	ancestorParentDataElements(el)(func(pd ParentDataWidget) bool {
		pd.ApplyParentData(renderObject)
		return true
	})
}
func renderObjectElementInsertRenderObjectChild(el renderObjectElement, child render.Object, slot int) {
	if slot >= 0 {
		slot--
	}
	render.InsertChild(el.renderHandle().RenderObject.(render.ObjectWithChildren), child, slot)
}
func renderObjectElementRemoveRenderObjectChild(el renderObjectElement, child render.Object, slot int) {
	render.RemoveChild(el.renderHandle().RenderObject.(render.ChildRemover), child)
}

func ancestorParentDataElements(el renderObjectElement) func(yield func(pd ParentDataWidget) bool) {
	return func(yield func(pd ParentDataWidget) bool) {
		ancestor := el.handle().parent
		for ancestor != nil && !isType[renderObjectElement](ancestor) {
			if w, ok := ancestor.handle().widget.(ParentDataWidget); ok {
				if !yield(w) {
					break
				}
			}
			ancestor = ancestor.handle().parent
		}
	}
}

func isType[T any](x any) bool {
	_, ok := x.(T)
	return ok
}
