package widget

import "honnef.co/go/gutter/render"

type ComponentElement interface {
	Element

	AfterMount(parent Element, newSlot any)
	PerformRebuild()
	GetChild() Element
	SetChild() Element
}

type StatelessElement interface {
	Element

	AfterUpdate(newWidget Widget)
	AfterMount(parent Element, newSlot any)
	PerformRebuild()
}

type StatefulElement interface {
	Element

	SingleChildElement
	WidgetBuilder
	GetStateHandle() *StateHandle
	GetState() State
	AfterUpdate(newWidget Widget)
	PerformRebuild()
	AfterActivate()
	BeforeDeactivate()
	AfterMount(parent Element, newSlot any)
	AfterUnmount()
	DidChangeDependencies()
}

type RenderObjectElement interface {
	Element

	RenderHandle() *RenderObjectElementHandle

	InsertRenderObjectChild(child render.Object, slot any)
	RemoveRenderObjectChild(child render.Object, slot any)
	MoveRenderObjectChild(child render.Object, oldSlot, newSlot any)

	AfterUpdate(newWidget Widget)
	AfterMount(parent Element, newSlot any)
	AfterUnmount()
	AttachRenderObject(slot any)
	PerformRebuild()
}

type SingleChildRenderObjectElement interface {
	Element
	SingleChildElement
	RenderObjectElement
}

type RenderTreeRootElement interface {
	Element
	RenderObjectElement
}

func ComponentElementAfterMount(el Element, parent Element, newSlot any) {
	rebuild(el)
}
func ComponentElementPerformRebuild(el Element) {
	built := el.(WidgetBuilder).Build()
	cel := el.(SingleChildElement)
	cel.SetChild(UpdateChild(el, cel.GetChild(), built, el.Handle().slot))
	el.Handle().dirty = false
}

func StatelessElementAfterUpdate(el Element, newWidget Widget) {
	forceRebuild(el)
}
func StatelessElementAfterMount(el Element, parent Element, newSlot any) {
	ComponentElementAfterMount(el, parent, newSlot)
}
func StatelessElementPerformRebuild(el Element) {
	ComponentElementPerformRebuild(el)
}

func RenderObjectElementAfterUpdate(el Element, newWidget Widget) {
	rebuild(el)
}
func RenderObjectElementAfterMount(el RenderObjectElement, parent Element, newSlot any) {
	h := el.RenderHandle()
	h.renderObject = h.widget.(RenderObjectWidget).CreateRenderObject(el)
	AttachRenderObject(el, newSlot)
	rebuild(el)
}
func RenderObjectElementAfterUnmount(el RenderObjectElement) {
	h := el.RenderHandle()
	oldWidget := h.widget.(RenderObjectWidget)
	if n, ok := oldWidget.(RenderObjectUnmountNotifyee); ok {
		n.DidUnmountRenderObject(h.renderObject)
	}
	render.Dispose(h.renderObject)
	h.renderObject = nil
}
func RenderObjectElementAttachRenderObject(el RenderObjectElement, slot any) {
	h := el.RenderHandle()
	h.slot = slot
	h.ancestorRenderObjectElement = findAncestorRenderObjectElement(el.(RenderObjectElement))
	if h.ancestorRenderObjectElement != nil {
		h.ancestorRenderObjectElement.InsertRenderObjectChild(h.renderObject, slot)
	}
}
func RenderObjectElementPerformRebuild(el RenderObjectElement) {
	h := el.RenderHandle()
	h.widget.(RenderObjectWidget).UpdateRenderObject(el, h.renderObject)
	el.Handle().dirty = false
}

func SingleChildRenderObjectElementAfterUpdate(el SingleChildElement, newWidget Widget) {
	RenderObjectElementAfterUpdate(el, newWidget)
	el.SetChild(UpdateChild(el, el.GetChild(), el.Handle().widget.(SingleChildWidget).GetChild(), nil))
}
func SingleChildRenderObjectElementAfterMount(el interface {
	SingleChildElement
	RenderObjectElement
}, parent Element, newSlot any) {
	RenderObjectElementAfterMount(el, parent, newSlot)
	h := el.Handle()
	el.SetChild(UpdateChild(el, el.GetChild(), h.widget.(SingleChildWidget).GetChild(), nil))
}
func SingleChildRenderObjectElementAfterUnmount(el RenderObjectElement) {
	RenderObjectElementAfterUnmount(el)
}
func SingleChildRenderObjectElementAttachRenderObject(el RenderObjectElement, slot any) {
	RenderObjectElementAttachRenderObject(el, slot)
}
func SingleChildRenderObjectElementPerformRebuild(el RenderObjectElement) {
	RenderObjectElementPerformRebuild(el)
}

func RenderTreeRootElementAfterUpdate(el RenderObjectElement, newWidget Widget) {
	RenderObjectElementAfterUpdate(el, newWidget)
}
func RenderTreeRootElementAfterMount(el RenderObjectElement, parent Element, newSlot any) {
	RenderObjectElementAfterMount(el, parent, newSlot)
}
func RenderTreeRootElementAfterUnmount(el RenderObjectElement) {
	RenderObjectElementAfterUnmount(el)
}
func RenderTreeRootElementAttachRenderObject(el RenderObjectElement, newSlot any) {
	el.Handle().slot = newSlot
}
func RenderTreeRootElementPerformRebuild(el RenderObjectElement) {
	RenderObjectElementPerformRebuild(el)
}

// Element -> ComponentElement -> StatefulElement

func StatefulElementAfterUpdate(el StatefulElement, newWidget Widget) {
	h := el.GetStateHandle()
	oldWidget := h.Widget
	h.Widget = el.Handle().widget.(StatefulWidget)
	if s, ok := el.GetState().(DidUpdateWidgeter); ok {
		s.DidUpdateWidget(oldWidget)
	}
	forceRebuild(el)
}
func StatefulElementPerformRebuild(el StatefulElement) {
	h := el.GetStateHandle()
	s := el.GetState()
	if h.didChangeDependencies {
		if s, ok := s.(StateDidChangeDependencieser); ok {
			s.DidChangeDependencies()
		}
		h.didChangeDependencies = false
	}
	ComponentElementPerformRebuild(el)
}
func StatefulElementAfterActivate(el StatefulElement) {
	if s, ok := el.GetState().(StateActivater); ok {
		s.Activate()
	}
	MarkNeedsBuild(el)
}
func StatefulElementBeforeDeactivate(el StatefulElement) {
	if s, ok := el.GetState().(StateDeactivater); ok {
		s.Deactivate()
	}
}
func StatefulElementAfterUnmount(el StatefulElement) {
	h := el.GetStateHandle()
	if s, ok := el.GetState().(Disposer); ok {
		s.Dispose()
	}
	h.Element = nil
}
func StatefulElementAfterDidChangeDependencies(el StatefulElement) {
	el.GetStateHandle().didChangeDependencies = true
}
func StatefulElementBuild(el StatefulElement) Widget {
	return el.GetState().Build()
}
func StatefulElementAfterMount(el StatefulElement, parent Element, newSlot any) {
	s := el.GetState()
	if s, ok := s.(InitStater); ok {
		s.InitState()
	}
	if s, ok := s.(StateDidChangeDependencieser); ok {
		s.DidChangeDependencies()
	}
	ComponentElementAfterMount(el, parent, newSlot)
}

/*
class StatefulElement extends ComponentElement {
  /// Creates an element that uses the given widget as its configuration.
  StatefulElement(StatefulWidget widget)
      : _state = widget.createState(),
        super(widget) {
    state._element = this;
    state._widget = widget;
  }
}
*/
