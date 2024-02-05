package render

import (
	"slices"

	"gioui.org/op"
)

type PipelineOwner struct {
	rootNode                          Object
	children                          map[*PipelineOwner]struct{}
	nodesNeedingPaint                 []Object
	nodesNeedingLayout                []Object
	nodesNeedingCompositingBitsUpdate []Object
	shouldMergeDirtyNodes             bool
	onNeedVisualUpdate                func()
}

func NewPipelineOwner() *PipelineOwner {
	return &PipelineOwner{
		children: make(map[*PipelineOwner]struct{}),
	}
}

func (o *PipelineOwner) RootNode() Object { return o.rootNode }
func (o *PipelineOwner) SetRootNode(root Object) {
	if o.rootNode == root {
		return
	}
	if o.rootNode != nil {
		Detach(o.rootNode)
	}
	o.rootNode = root
	if root != nil {
		Attach(root, o)
	}
}

func (o *PipelineOwner) RequestVisualUpdate() {
	if o.onNeedVisualUpdate != nil {
		o.onNeedVisualUpdate()
	}
}

func (o *PipelineOwner) enableMutationsToDirtySubtrees(fn func()) {
	fn()
	o.shouldMergeDirtyNodes = true
}

// Update the layout information for all dirty render objects.
//
// This function is one of the core stages of the rendering pipeline. Layout
// information is cleaned prior to painting so that render objects will
// appear on screen in their up-to-date locations.
//
// See [RendererBinding] for an example of how this function is used.
func (o *PipelineOwner) FlushLayout() {
	for len(o.nodesNeedingLayout) != 0 {
		dirtyNodes := o.nodesNeedingLayout
		// OPT(dh): avoid this alloc, probably via double buffering
		o.nodesNeedingLayout = nil
		slices.SortFunc(dirtyNodes, func(a, b Object) int {
			return a.Handle().depth - b.Handle().depth
		})
		for i := range dirtyNodes {
			if o.shouldMergeDirtyNodes {
				o.shouldMergeDirtyNodes = false
				if len(o.nodesNeedingLayout) != 0 {
					o.nodesNeedingLayout = append(o.nodesNeedingLayout, dirtyNodes[i:]...)
					break
				}
			}
			node := dirtyNodes[i]
			if node.Handle().needsLayout && node.Handle().owner == o {
				layoutWithoutResize(node)
			}
		}
		// No need to merge dirty nodes generated from processing the last
		// relayout boundary back.
		o.shouldMergeDirtyNodes = false
	}

	for child := range o.children {
		child.FlushLayout()
	}
	o.shouldMergeDirtyNodes = false
}

func layoutWithoutResize(obj Object) {
	obj.Layout()
	obj.Handle().needsLayout = false
	obj.MarkNeedsPaint()
}

// / Update the display lists for all render objects.
// /
// / This function is one of the core stages of the rendering pipeline.
// / Painting occurs after layout and before the scene is recomposited so that
// / scene is composited with up-to-date display lists for every render object.
// /
// / See [RendererBinding] for an example of how this function is used.
func (o *PipelineOwner) FlushPaint(r *Renderer, ops *op.Ops) {
	dirtyNodes := o.nodesNeedingPaint
	// OPT(dh): avoid this alloc, probably via double buffering
	o.nodesNeedingPaint = nil

	// Sort the dirty nodes in reverse order (deepest first).
	slices.SortFunc(dirtyNodes, func(a, b Object) int {
		return b.Handle().depth - a.Handle().depth
	})

	for _, node := range dirtyNodes {
		h := node.Handle()
		if (h.needsPaint /* || h.needsCompositedLayerUpdate */) && h.owner == o {
			// if h.layerHandle.layer.attached {
			if h.needsPaint {
				r.Paint(node) // .Add(ops)
				// PaintingContext.repaintCompositedChild(node)
			} else {
				// PaintingContext.updateLayerProperties(node)
			}
			// } else {
			// 	// node.skippedPaintingOnLayer()
			// }
		}
	}
	for child := range o.children {
		child.FlushPaint(r, ops)
	}

	if o.rootNode != nil {
		r.Paint(o.rootNode).Add(ops)
	}
}

func (o *PipelineOwner) FlushCompositingBits() {
	nodes := o.nodesNeedingCompositingBitsUpdate
	slices.SortFunc(nodes, func(a, b Object) int {
		return a.Handle().depth - b.Handle().depth
	})
	for _, node := range nodes {
		h := node.Handle()
		if h.needsCompositingBitsUpdate && h.owner == o {
			// h.updateCompositingBits()
		}
	}
	clear(nodes)
	o.nodesNeedingCompositingBitsUpdate = nodes[:0]

	for child := range o.children {
		child.FlushCompositingBits()
	}
}

func (o *PipelineOwner) AdoptChild(child *PipelineOwner) {
	o.children[child] = struct{}{}
}

func (o *PipelineOwner) DropChild(child *PipelineOwner) {
	delete(o.children, child)
}

func (o *PipelineOwner) VisitChildren(yield func(*PipelineOwner) bool) {
	for child := range o.children {
		if !yield(child) {
			break
		}
	}
}

func (o *PipelineOwner) Dispose() {
	clear(o.nodesNeedingLayout)
	clear(o.nodesNeedingCompositingBitsUpdate)
	clear(o.nodesNeedingPaint)
}

func Attach(obj Object, owner *PipelineOwner) {
	h := obj.Handle()
	h.owner = owner
	// If the node was dirtied in some way while unattached, make sure to add
	// it to the appropriate dirty list now that an owner is available
	if h.needsLayout && h.relayoutBoundary != nil {
		// Don't enter this block if we've never laid out at all;
		// scheduleInitialLayout() will handle it
		h.needsLayout = false
		obj.MarkNeedsLayout()
	}
	if h.needsCompositingBitsUpdate {
		h.needsCompositingBitsUpdate = false
		// obj.MarkNeedsCompositingBitsUpdate()
	}
	if h.needsPaint /* && h.layerHandle.layer != nil */ {
		// Don't enter this block if we've never painted at all;
		// scheduleInitialPaint() will handle it
		h.needsPaint = false
		obj.MarkNeedsPaint()
	}

	if obj, ok := obj.(Attacher); ok {
		obj.Attach(owner)
	}
}

func Detach(obj Object) {
	obj.Handle().owner = nil
	if obj, ok := obj.(Attacher); ok {
		obj.Detach()
	}
}
