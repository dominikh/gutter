package widget

import (
	"honnef.co/go/gutter/render"
)

func ElementAfterUpdate(el Element, oldWidget Widget) {
	if pd, ok := el.Handle().widget.(ParentDataWidget); ok {
		ApplyParentData(pd, el)
	}
}

func RenderObjectElementAfterUpdate(el RenderObjectElement, oldWidget Widget) {
	ElementAfterUpdate(el, oldWidget)
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
	renderObject := el.RenderHandle().RenderObject
	ancestorParentDataElements(el)(func(pd ParentDataWidget) bool {
		pd.ApplyParentData(renderObject)
		return true
	})
}
func RenderObjectElementPerformRebuild(el RenderObjectElement) {
	h := el.RenderHandle()
	h.widget.(RenderObjectWidget).UpdateRenderObject(el, h.RenderObject)
	el.Handle().dirty = false
}

func MultiChildRenderObjectElementInsertRenderObjectChild(el RenderObjectElement, child render.Object, slot int) {
	if slot >= 0 {
		slot--
	}
	render.InsertChild(el.RenderHandle().RenderObject.(render.ObjectWithChildren), child, slot)
}
func MultiChildRenderObjectElementMoveRenderObjectChild(el RenderObjectElement, child render.Object, newSlot int) {
	if newSlot >= 0 {
		newSlot--
	}
	render.MoveChild(el.RenderHandle().RenderObject.(render.ObjectWithChildren), child, newSlot)
}
func MultiChildRenderObjectElementRemoveRenderObjectChild(el RenderObjectElement, child render.Object, slot int) {
	render.RemoveChild(el.RenderHandle().RenderObject.(render.ChildRemover), child)
}
func MultiChildRenderObjectElementVisitChildren(el RenderObjectElement, yield func(el Element) bool) {
	forgotten := el.ForgottenChildren()
	for _, child := range el.Children() {
		if _, ok := forgotten[child]; !ok {
			if !yield(child) {
				break
			}
		}
	}
}
func MultiChildRenderObjectElementForgetChild(el RenderObjectElement, child Element) {
	el.ForgottenChildren()[child] = struct{}{}
}
func MultiChildRenderObjectElementAfterMount(el RenderObjectElement, parent Element, newSlot int) {
	RenderObjectElementAfterMount(el, parent, newSlot)

	// OPT(dh): optimize for the single child case, which doesn't need iterators and slices.
	w := el.Handle().widget
	var children []Element
	WidgetChildrenIter(w)(func(i int, childWidget Widget) bool {
		children = append(children, InflateWidget(el, childWidget, i))
		return true
	})
	el.SetChildren(children)
}
func MultiChildRenderObjectElementAfterUpdate(el RenderObjectElement, oldWidget RenderObjectWidget) {
	RenderObjectElementAfterUpdate(el, oldWidget)

	// OPT(dh): optimize for the case where we had <2 children before and <2 children now. that doesn't need
	// to allocate slices or go through list reconciliation.
	widget := el.Handle().widget.(RenderObjectWidget)
	el.SetChildren(UpdateChildren(el, WidgetChildren(widget), el.ForgottenChildren()))
}
func MultiChildRenderObjectElementAfterUnmount(el RenderObjectElement) {
	RenderObjectElementAfterUnmount(el)
}
func MultiChildRenderObjectElementAttachRenderObject(el RenderObjectElement, slot int) {
	RenderObjectElementAttachRenderObject(el, slot)
}
func MultiChildRenderObjectElementPerformRebuild(el RenderObjectElement) {
	RenderObjectElementPerformRebuild(el)
}

func RenderTreeRootElementAfterUpdate(el RenderObjectElement, oldWidget Widget) {
	RenderObjectElementAfterUpdate(el, oldWidget)
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

func ancestorParentDataElements(el RenderObjectElement) func(yield func(pd ParentDataWidget) bool) {
	return func(yield func(pd ParentDataWidget) bool) {
		ancestor := el.Handle().parent
		for ancestor != nil && !isType[RenderObjectElement](ancestor) {
			if w, ok := ancestor.Handle().widget.(ParentDataWidget); ok {
				if !yield(w) {
					break
				}
			}
			ancestor = ancestor.Handle().parent
		}
	}
}

func isType[T any](x any) bool {
	_, ok := x.(T)
	return ok
}
