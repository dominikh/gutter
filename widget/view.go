package widget

import (
	"honnef.co/go/gutter/render"
)

var _ RenderObjectWidget = (*RawView)(nil)
var _ RenderObjectElement = (*rawViewElement)(nil)

func NewView(root Widget) *RawView {
	return &RawView{
		Child: root,
	}
}

type RawView struct {
	Child Widget
}

// CreateElement implements RenderObjectWidget.
func (w *RawView) CreateElement() Element {
	return newRawViewElement(w)
}

// CreateRenderObject implements RenderObjectWidget.
func (w *RawView) CreateRenderObject(ctx BuildContext) render.Object {
	// XXX
	return render.NewView()
}

// Key implements RenderObjectWidget.
func (v *RawView) Key() any {
	// XXX implement this correctly

	// TODO use "the view" as the key. maybe the app.Window?
	// panic("unimplemented")
	return v
}

// UpdateRenderObject implements RenderObjectWidget.
func (*RawView) UpdateRenderObject(ctx BuildContext, obj render.Object) {}

type rawViewElement struct {
	SingleChildRenderObjectElement
	Root render.Object

	pipelineOwner       *render.PipelineOwner
	parentPipelineOwner *render.PipelineOwner
}

func newRawViewElement(view *RawView) *rawViewElement {
	var el rawViewElement
	el.widget = view
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
	child := el.widget.(*RawView).Child
	el.ChildElement = el.UpdateChild(el.ChildElement, child, nil)
}

func (el *rawViewElement) attachView(parentPipelineOwner *render.PipelineOwner) {
	if parentPipelineOwner == nil {
		parentPipelineOwner = render.TopPipelineOwner
		// parentPipelineOwner = View.pipelineOwnerOf(el)
	}
	parentPipelineOwner.AdoptChild(el.effectivePipelineOwner())
	// AllRenderViews[el.renderObject.(*render.View)] = struct{}{}
	render.TheRendererBinding.AddRenderView(el.renderObject.(*render.View))
	el.parentPipelineOwner = parentPipelineOwner
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
	el.attachView(nil)
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
	el.attachView(nil)
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
