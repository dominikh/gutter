package widget

import (
	"honnef.co/go/gutter/render"
)

var _ SingleChildWidget = (*View)(nil)
var _ RenderObjectWidget = (*View)(nil)
var _ RenderTreeRootElement = (*viewElement)(nil)
var _ SingleChildElement = (*viewElement)(nil)

func NewView(root Widget, po *render.PipelineOwner) *View {
	return &View{
		PipelineOwner: po,
		Child:         root,
	}
}

type View struct {
	PipelineOwner *render.PipelineOwner
	Child         Widget
}

// GetChild implements SingleChildWidget.
func (w *View) GetChild() Widget {
	return w.Child
}

func (w *View) Attach(owner *BuildOwner, element *viewElement) *viewElement {
	if element == nil {
		element = w.CreateElement().(*viewElement)
		element.AssignOwner(owner)
		owner.BuildScope(element, func() {
			Mount(element, nil, 0)
		})
	} else {
		MarkNeedsBuild(element)
	}
	return element
}

// CreateElement implements RenderObjectWidget.
func (w *View) CreateElement() Element {
	return newViewElement(w, w.PipelineOwner)
}

// CreateRenderObject implements RenderObjectWidget.
func (w *View) CreateRenderObject(ctx BuildContext) render.Object {
	// XXX
	return render.NewView()
}

// Key implements RenderObjectWidget.
func (v *View) Key() any {
	// XXX implement this correctly

	// TODO use "the view" as the key. maybe the app.Window?
	// panic("unimplemented")
	return v
}

// UpdateRenderObject implements RenderObjectWidget.
func (*View) UpdateRenderObject(ctx BuildContext, obj render.Object) {}

type viewElement struct {
	RenderObjectElementHandle
	child Element

	pipelineOwner *render.PipelineOwner
}

func (el *viewElement) Transition(t ElementTransition) {
	switch t.Kind {
	case ElementActivated:
		el.pipelineOwner.SetRootNode(el.RenderObject)
	case ElementDeactivating:
		el.pipelineOwner.SetRootNode(nil)
	case ElementUpdated:
		RenderTreeRootElementAfterUpdate(el, t.NewWidget)
		el.updateChild()
	case ElementMounted:
		RenderTreeRootElementAfterMount(el, t.Parent, t.NewSlot)
		el.pipelineOwner.SetRootNode(el.RenderObject)
		el.updateChild()
		el.RenderObject.(*render.View).PrepareInitialFrame()
	case ElementUnmounted:
		RenderObjectElementAfterUnmount(el)
	}
}

// GetChild implements SingleChildElement.
func (el *viewElement) GetChild() Element {
	return el.child
}

// SetChild implements SingleChildElement.
func (el *viewElement) SetChild(child Element) {
	el.child = child
}

// AttachRenderObject implements RenderTreeRootElement.
func (el *viewElement) AttachRenderObject(slot int) {
	RenderTreeRootElementAttachRenderObject(el, slot)
}

func newViewElement(view *View, po *render.PipelineOwner) *viewElement {
	var el viewElement
	el.widget = view
	el.pipelineOwner = po
	return &el
}

func (el *viewElement) SetConfiguration(cs render.ViewConfiguration) {
	el.RenderObject.(*render.View).SetConfiguration(cs)
}

func (el *viewElement) ForgetChild(child Element) {
	el.child = nil
}

func (el *viewElement) updateChild() {
	child := el.widget.(*View).Child
	el.child = UpdateChild(el, el.child, child, 0)
}

func (el *viewElement) PerformRebuild() {
	RenderTreeRootElementPerformRebuild(el)
	el.updateChild()
}

func (el *viewElement) InsertRenderObjectChild(child render.Object, slot int) {
	render.SetChild(el.RenderObject.(render.ObjectWithChild), child)
}

func (el *viewElement) MoveRenderObjectChild(child render.Object, oldSlot, newSlot int) {
	panic("unexpected call")
}

func (el *viewElement) RemoveRenderObjectChild(child render.Object, slot int) {
	render.SetChild(el.RenderObject.(render.ObjectWithChild), nil)
}

func (el *viewElement) AssignOwner(owner *BuildOwner) {
	el.BuildOwner = owner
}
