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
			Mount(element, nil, nil)
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

// GetChild implements SingleChildElement.
func (el *viewElement) GetChild() Element {
	return el.child
}

// SetChild implements SingleChildElement.
func (el *viewElement) SetChild(child Element) {
	el.child = child
}

// AttachRenderObject implements RenderTreeRootElement.
func (el *viewElement) AttachRenderObject(slot any) {
	RenderTreeRootElementAttachRenderObject(el, slot)
}

func newViewElement(view *View, po *render.PipelineOwner) *viewElement {
	var el viewElement
	el.widget = view
	el.pipelineOwner = po
	return &el
}

func (el *viewElement) SetConfiguration(cs render.ViewConfiguration) {
	el.renderObject.(*render.View).SetConfiguration(cs)
}

func (el *viewElement) ForgetChild(child Element) {
	el.child = nil
}

func (el *viewElement) updateChild() {
	child := el.widget.(*View).Child
	el.child = UpdateChild(el, el.child, child, nil)
}

func (el *viewElement) PerformRebuild() {
	RenderTreeRootElementPerformRebuild(el)
	el.updateChild()
}

func (el *viewElement) AfterActivate() {
	el.pipelineOwner.SetRootNode(el.renderObject)
}

func (el *viewElement) BeforeDeactivate() {
	el.pipelineOwner.SetRootNode(nil)
}

func (el *viewElement) AfterUpdate(newWidget Widget) {
	RenderTreeRootElementAfterUpdate(el, newWidget)
	el.updateChild()
}

func (el *viewElement) InsertRenderObjectChild(child render.Object, slot any) {
	el.renderObject.(render.ObjectWithChild).SetChild(child)
}

func (el *viewElement) MoveRenderObjectChild(child render.Object, oldSlot, newSlot any) {
	panic("unexpected call")
}

func (el *viewElement) RemoveRenderObjectChild(child render.Object, slot any) {
	el.renderObject.(render.ObjectWithChild).SetChild(nil)
}

func (el *viewElement) AfterMount(parent Element, newSlot any) {
	RenderTreeRootElementAfterMount(el, parent, newSlot)
	el.pipelineOwner.SetRootNode(el.renderObject)
	el.updateChild()
	el.renderObject.(*render.View).PrepareInitialFrame()
}

func (el *viewElement) AfterUnmount() {
	el.pipelineOwner.Dispose()
	RenderObjectElementAfterUnmount(el)
}

func (el *viewElement) AssignOwner(owner *BuildOwner) {
	el.owner = owner
}

func (el *viewElement) VisitChildren(yield func(Element) bool) {
	if el.child != nil {
		yield(el)
	}
}
