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

func SingleChildRenderObjectElementAfterUpdate(el SingleChildRenderObjectElement, oldWidget Widget) {
	RenderObjectElementAfterUpdate(el, oldWidget)
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
func SingleChildRenderObjectElementInsertRenderObjectChild(el SingleChildRenderObjectElement, child render.Object, slot int) {
	render.SetChild(el.RenderHandle().RenderObject.(render.ObjectWithChild), child)
}
func SingleChildRenderObjectElementMoveRenderObjectChild(el SingleChildRenderObjectElement, child render.Object, newSlot int) {
	panic("XXX")
}
func SingleChildRenderObjectElementRemoveRenderObjectChild(el SingleChildRenderObjectElement, child render.Object, slot int) {
	render.RemoveChild(el.RenderHandle().RenderObject.(render.ChildRemover), child)
}

func MultiChildRenderObjectElementInsertRenderObjectChild(el MultiChildRenderObjectElement, child render.Object, slot int) {
	if slot >= 0 {
		slot--
	}
	render.InsertChild(el.RenderHandle().RenderObject.(render.ObjectWithChildren), child, slot)
}
func MultiChildRenderObjectElementMoveRenderObjectChild(el MultiChildRenderObjectElement, child render.Object, newSlot int) {
	if newSlot >= 0 {
		newSlot--
	}
	render.MoveChild(el.RenderHandle().RenderObject.(render.ObjectWithChildren), child, newSlot)
}
func MultiChildRenderObjectElementRemoveRenderObjectChild(el MultiChildRenderObjectElement, child render.Object, slot int) {
	render.RemoveChild(el.RenderHandle().RenderObject.(render.ObjectWithChildren), child)
}
func MultiChildRenderObjectElementVisitChildren(el MultiChildRenderObjectElement, yield func(el Element) bool) {
	forgotten := el.ForgottenChildren()
	for _, child := range *el.Children() {
		if _, ok := forgotten[child]; !ok {
			if !yield(child) {
				break
			}
		}
	}
}
func MultiChildRenderObjectElementForgetChild(el MultiChildRenderObjectElement, child Element) {
	el.ForgottenChildren()[child] = struct{}{}
}
func MultiChildRenderObjectElementAfterMount(el MultiChildRenderObjectElement, parent Element, newSlot int) {
	RenderObjectElementAfterMount(el, parent, newSlot)
	w := el.Handle().widget.(MultiChildWidget)
	children := *el.Children()
	if cap(children) >= len(w.GetChildren()) {
		clear(children[:cap(children)])
		children = children[:len(w.GetChildren())]
	} else {
		children = make([]Element, len(w.GetChildren()))
	}
	for i, childWidget := range w.GetChildren() {
		children[i] = InflateWidget(el, childWidget, i)
	}
	*el.Children() = children
}
func MultiChildRenderObjectElementAfterUpdate(el MultiChildRenderObjectElement, oldWidget MultiChildRenderObjectWidget) {
	RenderObjectElementAfterUpdate(el, oldWidget)
	widget := el.Handle().widget.(MultiChildRenderObjectWidget)
	*el.Children() = UpdateChildren(el, *el.Children(), widget.GetChildren(), el.ForgottenChildren())
	clear(el.ForgottenChildren())
}
func MultiChildRenderObjectElementAfterUnmount(el MultiChildRenderObjectElement) {
	RenderObjectElementAfterUnmount(el)
}
func MultiChildRenderObjectElementAttachRenderObject(el MultiChildRenderObjectElement, slot int) {
	RenderObjectElementAttachRenderObject(el, slot)
}
func MultiChildRenderObjectElementPerformRebuild(el MultiChildRenderObjectElement) {
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
