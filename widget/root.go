package widget

var _ Widget = (*RootWidget)(nil)
var _ Element = (*RootElement)(nil)

type RootWidget struct {
	Child Widget
}

// CreateElement implements Widget.
func (w *RootWidget) CreateElement() Element {
	return NewRootElement(w)
}

func (w *RootWidget) Attach(owner *BuildOwner, element *RootElement) *RootElement {
	if element == nil {
		element = w.CreateElement().(*RootElement)
		element.AssignOwner(owner)
		owner.BuildScope(element, func() {
			element.Mount(nil, nil)
		})
	} else {
		element.newWidget = w
		MarkNeedsBuild(element)
	}
	return element
}

// Key implements Widget.
func (*RootWidget) Key() any {
	return nil
}

type RootElement struct {
	ElementHandle
	child     Element
	newWidget *RootWidget
}

func NewRootElement(w *RootWidget) *RootElement {
	var el RootElement
	el.widget = w
	return &el
}

// Activate implements Element.
func (el *RootElement) Activate() {
	ElementActivate(el)
}

// AttachRenderObject implements Element.
func (el *RootElement) AttachRenderObject(slot any) {
	ElementAttachRenderObject(el, slot)
}

// DetachRenderObject implements Element.
func (el *RootElement) DetachRenderObject() {
	ElementDetachRenderObject(el)
}

// RenderObjectAttachingChild implements Element.
func (el *RootElement) RenderObjectAttachingChild() Element {
	return ElementRenderObjectAttachingChild(el)
}

// Unmount implements Element.
func (el *RootElement) Unmount() {
	ElementUnmount(el)
}

// UpdateChild implements Element.
func (el *RootElement) UpdateChild(child Element, newWidget Widget, newSlot any) Element {
	return ElementUpdateChild(el, child, newWidget, newSlot)
}

func (el *RootElement) VisitChildren(yield func(Element) bool) {
	if el.child != nil {
		yield(el.child)
	}
}

func (el *RootElement) ForgetChild(child Element) {
	el.child = nil
}

func (el *RootElement) Mount(parent Element, newSlot any) {
	ElementMount(el, parent, newSlot)
	el._rebuild()
	el.Handle().PerformRebuild()
}

func (el *RootElement) Update(newWidget Widget) {
	ElementUpdate(el, newWidget)
	el._rebuild()
}

func (el *RootElement) performRebuild() {
	if el.newWidget != nil {
		// _newWidget can be null if, for instance, we were rebuilt
		// due to a reassemble.
		newWidget := el.newWidget
		el.newWidget = nil
		el.Update(newWidget)
	}
	el.PerformRebuild()
}

func (el *RootElement) _rebuild() {
	el.child = el.UpdateChild(el.child, el.Handle().widget.(*RootWidget).Child, nil)
}

func (el *RootElement) AssignOwner(owner *BuildOwner) {
	el.owner = owner
}
