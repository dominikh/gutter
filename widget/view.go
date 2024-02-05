package widget

import (
	"honnef.co/go/gutter/render"
)

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
			element.Mount(nil, nil)
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
	SingleChildRenderObjectElement
	Root render.Object

	pipelineOwner *render.PipelineOwner
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
	el.ChildElement = nil
}

func (el *viewElement) MoveRenderObjectChild(child render.Object, oldSlot, newSlot any) {
	panic("unexpected call")
}

func (el *viewElement) updateChild() {
	child := el.widget.(*View).Child
	el.ChildElement = el.UpdateChild(el.ChildElement, child, nil)
}

func (el *viewElement) PerformRebuild() {
	RenderObjectElementPerformRebuild(el)
	el.updateChild()
}

func (el *viewElement) Activate() {
	ElementActivate(el)
	el.Root = el.renderObject
	el.pipelineOwner.SetRootNode(el.renderObject)
}

func (el *viewElement) Deactivate() {
	el.Root = nil
	el.pipelineOwner.SetRootNode(nil)
	ElementDeactivate(el)
}

func (el *viewElement) Update(newWidget Widget) {
	SingleChildRenderObjectElementUpdate(el, newWidget.(RenderObjectWidget))
	el.updateChild()
}

func (el *viewElement) InsertRenderObjectChild(child render.Object, slot any) {
	el.renderObject.(render.ObjectWithChild).SetChild(child)
}

func (el *viewElement) RemoveRenderObjectChild(child render.Object, slot any) {
	el.renderObject.(render.ObjectWithChild).SetChild(nil)
}

func (el *viewElement) Mount(parent Element, newSlot any) {
	RenderObjectElementMount(el, parent, newSlot)
	el.Root = el.renderObject
	el.pipelineOwner.SetRootNode(el.renderObject)
	el.updateChild()
	el.renderObject.(*render.View).PrepareInitialFrame()
}

func (el *viewElement) Unmount() {
	el.pipelineOwner.Dispose()
	RenderObjectElementUnmount(el)
}

func (el *viewElement) AttachRenderObject(newSlot any) {
	el.slot = newSlot
}

func (el *viewElement) DetachRenderObject() {
	el.slot = nil
}

func (el *viewElement) AssignOwner(owner *BuildOwner) {
	el.owner = owner
}
