package render

import (
	"slices"

	"gioui.org/op"
)

type PipelineOwner struct {
	renderer                          *Renderer
	rootNode                          Object
	nodesNeedingPaint                 []Object
	nodesNeedingLayout                []Object
	nodesNeedingCompositingBitsUpdate []Object
	shouldMergeDirtyNodes             bool
	onNeedVisualUpdate                func()
}

func NewPipelineOwner() *PipelineOwner {
	return &PipelineOwner{
		renderer: NewRenderer(),
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

	o.shouldMergeDirtyNodes = false
}

func layoutWithoutResize(obj Object) {
	obj.Layout()
	obj.Handle().needsLayout = false
	obj.MarkNeedsPaint()
}

func (o *PipelineOwner) FlushPaint(ops *op.Ops) {
	dirtyNodes := o.nodesNeedingPaint
	// OPT(dh): avoid this alloc, probably via double buffering
	o.nodesNeedingPaint = nil

	// Sort the dirty nodes in reverse order (deepest first).
	slices.SortFunc(dirtyNodes, func(a, b Object) int {
		return b.Handle().depth - a.Handle().depth
	})

	for _, node := range dirtyNodes {
		h := node.Handle()
		if h.needsPaint && h.owner == o {
			o.renderer.Paint(node)
		}
	}

	if o.rootNode != nil {
		o.renderer.Paint(o.rootNode).Add(ops)
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
	if h.needsPaint {
		// Don't enter this block if we've never painted at all;
		// scheduleInitialPaint() will handle it
		h.needsPaint = false
		obj.MarkNeedsPaint()
	}

	if aobj, ok := obj.(Attacher); ok {
		aobj.Attach(owner)
	} else {
		obj.VisitChildren(func(child Object) bool {
			Attach(child, owner)
			return true
		})
	}
}

func Detach(obj Object) {
	obj.Handle().owner = nil
	if obj, ok := obj.(Attacher); ok {
		obj.Detach()
	}
}
