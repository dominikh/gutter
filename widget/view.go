package widget

import (
	"honnef.co/go/gutter/debug"
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

// Children implements RenderObjectElement.
func (el *viewElement) Children() []Element {
	if el.child == nil {
		return nil
	} else {
		// OPT(dh): this isn't great
		return []Element{el.child}
	}
}

// ForgottenChildren implements RenderObjectElement.
func (el *viewElement) ForgottenChildren() map[Element]struct{} {
	// XXX remove this
	return nil
}

// SetChildren implements RenderObjectElement.
func (el *viewElement) SetChildren(children []Element) {
	debug.Assert(len(children) < 2)
	if len(children) == 0 {
		el.child = nil
	} else {
		el.child = children[0]
	}
}

// VisitChildren implements RenderObjectElement.
func (el *viewElement) VisitChildren(yield func(e Element) bool) {
	if el.child == nil {
		return
	}
	yield(el.child)
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
