package widget

import (
	"honnef.co/go/gutter/render"
)

func RenderObjectElementAfterUpdate(el RenderObjectElement, newWidget Widget) {
	el.Handle().widget.(RenderObjectWidget).UpdateRenderObject(el, el.RenderHandle().RenderObject)
	forceRebuild(el)
}
func RenderObjectElementAfterMount(el RenderObjectElement, parent Element, newSlot int) {
	h := el.RenderHandle()
	h.RenderObject = h.widget.(RenderObjectWidget).CreateRenderObject(el)
	AttachRenderObject(el, newSlot)
	rebuild(el)
}
func RenderObjectElementAfterUnmount(el RenderObjectElement) {
	h := el.RenderHandle()
	oldWidget := h.widget.(RenderObjectWidget)
	if n, ok := oldWidget.(RenderObjectUnmountNotifyee); ok {
		n.DidUnmountRenderObject(h.RenderObject)
	}
	render.Dispose(h.RenderObject)
	h.RenderObject = nil
}
func RenderObjectElementAttachRenderObject(el RenderObjectElement, slot int) {
	h := el.RenderHandle()
	h.slot = slot
	h.ancestorRenderObjectElement = findAncestorRenderObjectElement(el)
	if h.ancestorRenderObjectElement != nil {
		h.ancestorRenderObjectElement.InsertRenderObjectChild(h.RenderObject, slot)
	}
}
func RenderObjectElementPerformRebuild(el RenderObjectElement) {
	h := el.RenderHandle()
	h.widget.(RenderObjectWidget).UpdateRenderObject(el, h.RenderObject)
	el.Handle().dirty = false
}

func SingleChildRenderObjectElementAfterUpdate(el SingleChildRenderObjectElement, newWidget Widget) {
	RenderObjectElementAfterUpdate(el, newWidget)
	el.SetChild(UpdateChild(el, el.GetChild(), el.Handle().widget.(SingleChildWidget).GetChild(), 0))
}
func SingleChildRenderObjectElementAfterMount(el SingleChildRenderObjectElement, parent Element, newSlot int) {
	RenderObjectElementAfterMount(el, parent, newSlot)
	h := el.Handle()
	el.SetChild(UpdateChild(el, el.GetChild(), h.widget.(SingleChildWidget).GetChild(), 0))
}
func SingleChildRenderObjectElementAfterUnmount(el RenderObjectElement) {
	RenderObjectElementAfterUnmount(el)
}
func SingleChildRenderObjectElementAttachRenderObject(el RenderObjectElement, slot int) {
	RenderObjectElementAttachRenderObject(el, slot)
}
func SingleChildRenderObjectElementPerformRebuild(el RenderObjectElement) {
	RenderObjectElementPerformRebuild(el)
}

func RenderTreeRootElementAfterUpdate(el RenderObjectElement, newWidget Widget) {
	RenderObjectElementAfterUpdate(el, newWidget)
}
func RenderTreeRootElementAfterMount(el RenderObjectElement, parent Element, newSlot int) {
	RenderObjectElementAfterMount(el, parent, newSlot)
}
func RenderTreeRootElementAfterUnmount(el RenderObjectElement) {
	RenderObjectElementAfterUnmount(el)
}
func RenderTreeRootElementAttachRenderObject(el RenderObjectElement, newSlot int) {
	el.Handle().slot = newSlot
}
func RenderTreeRootElementPerformRebuild(el RenderObjectElement) {
	RenderObjectElementPerformRebuild(el)
}
