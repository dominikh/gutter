package widget

// XXX delete this file

import "honnef.co/go/gutter/render"

var _ RenderObjectWidget = (*RenderObjectToWidgetAdapter)(nil)
var _ RenderObjectElement = (*RenderObjectToWidgetElement)(nil)

// / A bridge from a [RenderObject] to an [Element] tree.
// /
// / The given container is the [RenderObject] that the [Element] tree should be
// / inserted into. It must be a [RenderObject] that implements the
// / [RenderObjectWithChildMixin] protocol. The type argument `T` is the kind of
// / [RenderObject] that the container expects as its child.
// /
// / The [RenderObjectToWidgetAdapter] is an alternative to [RootWidget] for
// / bootstrapping an element tree. Unlike [RootWidget] it requires the
// / existence of a render tree (the [container]) to attach the element tree to.
type RenderObjectToWidgetAdapter struct {
	key any
	/// The widget below this widget in the tree.
	child Widget

	/// The [RenderObject] that is the parent of the [Element] created by this widget.
	container render.ObjectWithChild
}

// Key implements RenderObjectWidget.
func (w *RenderObjectToWidgetAdapter) Key() any {
	return w.key
}

func NewRenderObjectToWidgetAdapter(child Widget, container render.ObjectWithChild) *RenderObjectToWidgetAdapter {
	return &RenderObjectToWidgetAdapter{
		key:       GlobalObjectKey{container},
		child:     child,
		container: container,
	}
}

func (a *RenderObjectToWidgetAdapter) UpdateRenderObject(ctx BuildContext, obj render.Object) {}

func (a *RenderObjectToWidgetAdapter) CreateRenderObject(ctx BuildContext) render.Object {
	return a.container
}

func (a *RenderObjectToWidgetAdapter) CreateElement() Element {
	el := &RenderObjectToWidgetElement{}
	el.widget = a
	return el
}

func (a *RenderObjectToWidgetAdapter) AttachToRenderTree(
	owner *BuildOwner,
	element *RenderObjectToWidgetElement,
) *RenderObjectToWidgetElement {
	if element == nil {
		element = a.CreateElement().(*RenderObjectToWidgetElement)
		element.AssignOwner(owner)
		owner.BuildScope(element, func() {
			element.Mount(nil, nil)
		})
	} else {
		element.newWidget = a
		MarkNeedsBuild(element)
	}
	return element
}

// The root of an element tree that is hosted by a [RenderObject].
//
// This element class is the instantiation of a [RenderObjectToWidgetAdapter]
// widget. It can be used only as the root of an [Element] tree (it cannot be
// mounted into another [Element]; it's parent must be null).
//
// In typical usage, it will be instantiated for a [RenderObjectToWidgetAdapter]
// whose container is the [RenderView].
type RenderObjectToWidgetElement struct {
	RenderObjectElementHandle

	child     Element
	newWidget *RenderObjectToWidgetAdapter
}

// Activate implements RenderObjectElement.
func (el *RenderObjectToWidgetElement) Activate() {
	ElementActivate(el)
}

// RenderObjectAttachingChild implements RenderObjectElement.
func (el *RenderObjectToWidgetElement) RenderObjectAttachingChild() Element {
	return RenderObjectElementRenderObjectAttachingChild(el)
}

// Unmount implements RenderObjectElement.
func (el *RenderObjectToWidgetElement) Unmount() {
	RenderObjectElementUnmount(el)
}

// UpdateChild implements RenderObjectElement.
func (el *RenderObjectToWidgetElement) UpdateChild(child Element, newWidget Widget, newSlot any) Element {
	return ElementUpdateChild(el, child, newWidget, newSlot)
}

func (el *RenderObjectToWidgetElement) PerformRebuild() {
	if el.newWidget != nil {
		// _newWidget can be null if, for instance, we were rebuilt
		// due to a reassemble.
		newWidget := el.newWidget
		el.newWidget = nil
		el.Update(newWidget)
	}
	RenderObjectElementPerformRebuild(el)
}

func (el *RenderObjectToWidgetElement) Update(newWidget Widget) {
	RenderObjectElementUpdate(el, newWidget.(RenderObjectWidget))
	el.rebuild()
}

func (el *RenderObjectToWidgetElement) rebuild() {
	el.child = ElementUpdateChild(el, el.child, el.widget.(*RenderObjectToWidgetAdapter).child, el)
}

func (el *RenderObjectToWidgetElement) VisitChildren(yield func(Element) bool) {
	if el.child != nil {
		yield(el.child)
	}
}

func (el *RenderObjectToWidgetElement) ForgetChild(child Element) {
	el.child = nil
}

func (el *RenderObjectToWidgetElement) Mount(parent Element, newSlot any) {
	RenderObjectElementMount(el, parent, newSlot)
	el.rebuild()
}

func (el *RenderObjectToWidgetElement) InsertRenderObjectChild(child render.Object, slot any) {
	el.RenderObject().(render.ObjectWithChild).SetChild(child)
}

func (el *RenderObjectToWidgetElement) MoveRenderObjectChild(child render.Object, oldSlot, newSlot any) {
}

func (el *RenderObjectToWidgetElement) RemoveRenderObjectChild(child render.Object, slot any) {
	el.RenderObject().(render.ObjectWithChild).SetChild(nil)
}

func (el *RenderObjectToWidgetElement) AssignOwner(owner *BuildOwner) {
	el.owner = owner
}

func (el *RenderObjectToWidgetElement) AttachRenderObject(newSlot any) {
	el.slot = newSlot
}

func (el *RenderObjectToWidgetElement) DetachRenderObject() {
	el.slot = nil
}
