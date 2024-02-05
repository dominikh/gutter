package widget

import (
	"honnef.co/go/gutter/f32"
	"honnef.co/go/gutter/render"
)

var _ RenderObjectWidget = (*View)(nil)
var _ RenderObjectElement = (*rawViewElement)(nil)

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

// CreateElement implements RenderObjectWidget.
func (w *View) CreateElement() Element {
	return newRawViewElement(w, w.PipelineOwner)
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

type rawViewElement struct {
	SingleChildRenderObjectElement
	Root render.Object

	pipelineOwner       *render.PipelineOwner
	parentPipelineOwner *render.PipelineOwner
}

func newRawViewElement(view *View, po *render.PipelineOwner) *rawViewElement {
	var el rawViewElement
	el.widget = view
	el.parentPipelineOwner = po
	el.pipelineOwner = render.NewPipelineOwner()
	return &el
}

func (el *rawViewElement) effectivePipelineOwner() *render.PipelineOwner {
	return el.pipelineOwner
}

func (el *rawViewElement) ForgetChild(child Element) {
	el.ChildElement = nil
}

func (el *rawViewElement) MoveRenderObjectChild(child render.Object, oldSlot, newSlot any) {
	panic("unexpected call")
}

func (el *rawViewElement) updateChild() {
	child := el.widget.(*View).Child
	el.ChildElement = el.UpdateChild(el.ChildElement, child, nil)
}

func (el *rawViewElement) attachView() {
	el.parentPipelineOwner.AdoptChild(el.effectivePipelineOwner())
	// XXX get the actual window size
	sz := f32.Pt(400, 400)
	el.renderObject.(*render.View).SetConfiguration(render.Constraints{sz, sz})
}

func (el *rawViewElement) detachView() {
	parentPipelineOwner := el.parentPipelineOwner
	if parentPipelineOwner != nil {
		// delete(AllRenderViews, el.renderObject.(*render.View))
		// RendererBinding.instance.removeRenderView(el.renderObject)
		parentPipelineOwner.DropChild(el.effectivePipelineOwner())
		el.parentPipelineOwner = nil
	}
}

// XXX lol, get rid of this global state
// var AllRenderViews map[*render.View]struct{}

func (el *rawViewElement) PerformRebuild() {
	RenderObjectElementPerformRebuild(el)
	el.updateChild()
}

func (el *rawViewElement) Activate() {
	ElementActivate(el)
	el.Root = el.renderObject
	el.effectivePipelineOwner().SetRootNode(el.renderObject)
	el.attachView()
}

func (el *rawViewElement) Deactivate() {
	el.detachView()
	el.Root = nil
	el.effectivePipelineOwner().SetRootNode(nil)
	ElementDeactivate(el)
}

func (el *rawViewElement) Update(newWidget Widget) {
	SingleChildRenderObjectElementUpdate(el, newWidget.(RenderObjectWidget))
	el.updateChild()
}

func (el *rawViewElement) InsertRenderObjectChild(child render.Object, slot any) {
	el.renderObject.(render.ObjectWithChild).SetChild(child)
}

func (el *rawViewElement) RemoveRenderObjectChild(child render.Object, slot any) {
	el.renderObject.(render.ObjectWithChild).SetChild(nil)
}

func (el *rawViewElement) Mount(parent Element, newSlot any) {
	RenderObjectElementMount(el, parent, newSlot)
	el.Root = el.renderObject
	el.effectivePipelineOwner().SetRootNode(el.renderObject)
	el.attachView()
	el.updateChild()
	el.renderObject.(*render.View).PrepareInitialFrame()
}

func (el *rawViewElement) Unmount() {
	if o := el.effectivePipelineOwner(); o != nil {
		o.Dispose()
	}
	RenderObjectElementUnmount(el)
}

func (el *rawViewElement) AttachRenderObject(newSlot any) {
	el.slot = newSlot
}

func (el *rawViewElement) DetachRenderObject() {
	el.slot = nil
}
