package widget

import (
	"honnef.co/go/gutter/render"
)

var _ Widget = (*View)(nil)
var _ RenderObjectWidget = (*View)(nil)
var _ RenderObjectElement = (*viewElement)(nil)

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
	SingleChildElement

	pipelineOwner *render.PipelineOwner
}

func (el *viewElement) Transition(t ElementTransition) {
	switch t.Kind {
	case ElementActivated:
		el.pipelineOwner.SetRootNode(el.RenderObject)
	case ElementDeactivating:
		el.pipelineOwner.SetRootNode(nil)
	case ElementUpdated:
		RenderTreeRootElementAfterUpdate(el, t.OldWidget)
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
	el.SetChild(nil)
}

func (el *viewElement) updateChild() {
	child := el.widget.(*View).Child
	el.SetChild(UpdateChild(el, el.Child(), child, 0))
}

func (el *viewElement) PerformRebuild() {
	RenderTreeRootElementPerformRebuild(el)
	el.updateChild()
}

func (el *viewElement) InsertRenderObjectChild(child render.Object, slot int) {
	render.InsertChild(el.RenderObject.(render.ObjectWithChildren), child, -1)
}

func (el *viewElement) MoveRenderObjectChild(child render.Object, newSlot int) {
	panic("unexpected call")
}

func (el *viewElement) RemoveRenderObjectChild(child render.Object, slot int) {
	render.RemoveChild(el.RenderObject.(render.ObjectWithChildren), el.RenderObject)
}

func (el *viewElement) AssignOwner(owner *BuildOwner) {
	el.BuildOwner = owner
}
