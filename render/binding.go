package render

import (
	"gioui.org/f32"
	"gioui.org/op"
)

var _ PipelineManifold = (*RendererBinding)(nil)

// XXX Flutter uses a singleton… we really don't want to
var TheRendererBinding = NewRendererBinding()

type RendererBinding struct {
	rootPipelineOwner *PipelineOwner
	manifold          PipelineManifold
	renderer          *Renderer

	views map[*View]struct{}
}

func NewRendererBinding() *RendererBinding {
	rb := &RendererBinding{
		rootPipelineOwner: createRootPipelineOwner(),
		views:             make(map[*View]struct{}),
		renderer:          NewRenderer(),
	}
	rb.manifold = rb
	rb.rootPipelineOwner.Attach(rb.manifold)
	return rb
	// XXX
	// addPersistentFrameCallback(_handlePersistentFrameCallback);
}

func createRootPipelineOwner() *PipelineOwner {
	// return NewPipelineOwner()
	return TopPipelineOwner
}

// XXX lol, get rid of this global state
var TopPipelineOwner = NewPipelineOwner()

func (rb *RendererBinding) DrawFrame(ops *op.Ops) {
	rb.rootPipelineOwner.FlushLayout()
	rb.rootPipelineOwner.FlushCompositingBits()
	rb.rootPipelineOwner.FlushPaint(rb.renderer, ops)
	// for _, view := range rb.views {
	// 	// view.compositeFrame() // this sends the bits to the GPU
	// }
}

func (rb *RendererBinding) AddRenderView(view *View) {
	rb.views[view] = struct{}{}
	view.configuration = rb.createViewConfigurationFor(view)
}

func (rb *RendererBinding) RemoveRenderView(view *View) {
	delete(rb.views, view)
}

func (rb *RendererBinding) createViewConfigurationFor(view *View) ViewConfiguration {
	// XXX integrate with window size
	return Constraints{
		Min: f32.Pt(400, 400),
		Max: f32.Pt(400, 400),
	}
}

func (rb *RendererBinding) forceRepaint() {
	var fn func(child Object) bool
	fn = func(child Object) bool {
		child.MarkNeedsPaint()
		child.VisitChildren(fn)
		return true
	}
	for view := range rb.views {
		view.VisitChildren(fn)
	}
}

func (rb *RendererBinding) RequestVisualUpdate() {
	// XXX call Invalidate
}
