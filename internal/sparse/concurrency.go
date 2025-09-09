// SPDX-FileCopyrightText: 2025 Dominik Honnef and contributors
//
// SPDX-License-Identifier: MIT

package sparse

import (
	"runtime"

	"honnef.co/go/curve"
	"honnef.co/go/gutter/gfx"
)

type ConcurrentRenderer struct {
	r     *Renderer
	tasks chan renderTask
	done  chan struct{}
}

type renderTaskKind int

const (
	fillRenderTask renderTaskKind = iota
	clipRenderTask
	saveRenderTask
	restoreRenderTask
)

type renderTask struct {
	path      chan Path
	kind      renderTaskKind
	transform curve.Affine
	paint     gfx.Paint
}

func NewConcurrentRenderer(width, height uint16, parallelism int) *ConcurrentRenderer {
	if parallelism == 0 {
		parallelism = runtime.GOMAXPROCS(0)
	}
	r := &ConcurrentRenderer{
		r:     NewRenderer(width, height),
		tasks: make(chan renderTask, parallelism),
		done:  make(chan struct{}),
	}

	go func() {
		for t := range r.tasks {
			p := <-t.path
			switch t.kind {
			case clipRenderTask:
				r.r.PushClipCompiled(p)
			case fillRenderTask:
				r.r.FillCompiled(p, t.transform, t.paint)
			case saveRenderTask:
				r.r.Save()
			case restoreRenderTask:
				r.r.Restore()
			}
		}
		close(r.done)
	}()

	return r
}

func (r *ConcurrentRenderer) Width() uint16  { return r.r.Width() }
func (r *ConcurrentRenderer) Height() uint16 { return r.r.Height() }
func (r *ConcurrentRenderer) Reset()         { r.r.Reset() }

func (r *ConcurrentRenderer) Stop() {
	close(r.tasks)
	<-r.done
}

func (r *ConcurrentRenderer) RenderToPixmap(width, height uint16, packer Packer) {
	r.Stop()
	r.r.Render(width, height, packer)
}

func (r *ConcurrentRenderer) Fill(
	shape gfx.Shape,
	transform curve.Affine,
	fillRule gfx.FillRule,
	paint gfx.Paint,
) {
	t := renderTask{
		path:      make(chan Path, 1),
		kind:      fillRenderTask,
		transform: transform,
		paint:     paint,
	}
	r.tasks <- t
	go func(width, height uint16) {
		t.path <- CompileFillPath(shape, transform, fillRule, width, height)
	}(r.Width(), r.Height())
}

func (r *ConcurrentRenderer) Stroke(
	shape gfx.Shape,
	transform curve.Affine,
	stroke_ curve.Stroke,
	paint gfx.Paint,
) {
	t := renderTask{
		path:      make(chan Path, 1),
		kind:      fillRenderTask,
		transform: transform,
		paint:     paint,
	}
	r.tasks <- t
	go func(width, height uint16) {
		t.path <- CompileStrokedPath(shape, transform, stroke_, width, height)
	}(r.Width(), r.Height())
}

func (r *ConcurrentRenderer) PushClip(
	shape gfx.Shape,
	transform curve.Affine,
	fill gfx.FillRule,
) {
	t := renderTask{
		path: make(chan Path, 1),
		kind: clipRenderTask,
	}
	r.tasks <- t
	go func(width, height uint16) {
		t.path <- CompileFillPath(shape, transform, fill, width, height)
	}(r.Width(), r.Height())
}

func (r *ConcurrentRenderer) PushLayer(l gfx.Layer) {
	panic("XXX ot implemented")
}

func (r *ConcurrentRenderer) Save() {
	t := renderTask{
		path: make(chan Path),
		kind: saveRenderTask,
	}
	close(t.path)
	r.tasks <- t
}

func (r *ConcurrentRenderer) Restore() {
	t := renderTask{
		path: make(chan Path),
		kind: restoreRenderTask,
	}
	close(t.path)
	r.tasks <- t
}
