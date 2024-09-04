// SPDX-FileCopyrightText: 2014 The Flutter Authors. All rights reserved.
// SPDX-FileCopyrightText: 2024 Dominik Honnef and contributors
//
// SPDX-License-Identifier: MIT AND BSD-3-Clause

package render

import (
	"slices"

	"honnef.co/go/curve"
	"honnef.co/go/gutter/mem"
	"honnef.co/go/gutter/wsi"
	"honnef.co/go/jello"
)

type PipelineOwner struct {
	painter                           *Painter
	rootNode                          Object
	nodesNeedingLayout                mem.DoubleBufferedSlice[Object]
	nodesNeedingCompositingBitsUpdate []Object
	shouldMergeDirtyNodes             bool
	OnNeedVisualUpdate                func()
	EmitEvent                         func(ev wsi.Event)
}

func NewPipelineOwner() *PipelineOwner {
	return &PipelineOwner{
		painter: NewPainter(),
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
	if o.OnNeedVisualUpdate != nil {
		o.OnNeedVisualUpdate()
	}
}

func (o *PipelineOwner) enableMutationsToDirtySubtrees(fn func()) {
	fn()
	o.shouldMergeDirtyNodes = true
}

func (o *PipelineOwner) FlushLayout() {
	for len(o.nodesNeedingLayout.Front) != 0 {
		dirtyNodes := o.nodesNeedingLayout.Front
		o.nodesNeedingLayout.Swap()
		slices.SortFunc(dirtyNodes, func(a, b Object) int {
			return a.Handle().depth - b.Handle().depth
		})
		for i := range dirtyNodes {
			if o.shouldMergeDirtyNodes {
				o.shouldMergeDirtyNodes = false
				if len(o.nodesNeedingLayout.Front) != 0 {
					o.nodesNeedingLayout.Front = append(o.nodesNeedingLayout.Front, dirtyNodes[i:]...)
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

// XXX what's the meaning of this function name?
func layoutWithoutResize(obj Object) {
	obj.Handle().size = obj.PerformLayout()
	obj.Handle().needsLayout = false
	MarkNeedsPaint(obj)
}

func (o *PipelineOwner) FlushPaint(scene *jello.Scene) {
	if o.rootNode != nil {
		o.painter.PaintAt(o.rootNode, scene, curve.Point{})
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

func Attach(obj Object, owner *PipelineOwner) {
	h := obj.Handle()
	h.owner = owner
	// If the node was dirtied in some way while unattached, make sure to add
	// it to the appropriate dirty list now that an owner is available
	if h.needsLayout && h.relayoutBoundary != nil {
		// Don't enter this block if we've never laid out at all;
		// scheduleInitialLayout() will handle it
		h.needsLayout = false
		MarkNeedsLayout(obj)
	}
	if h.needsCompositingBitsUpdate {
		h.needsCompositingBitsUpdate = false
		// obj.MarkNeedsCompositingBitsUpdate()
	}
	if h.needsPaint {
		// Don't enter this block if we've never painted at all;
		// scheduleInitialPaint() will handle it
		h.needsPaint = false
		MarkNeedsPaint(obj)
	}

	if aobj, ok := obj.(Attacher); ok {
		aobj.PerformAttach(owner)
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
		obj.PerformDetach()
	}
}
