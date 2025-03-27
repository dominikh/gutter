// SPDX-FileCopyrightText: 2025 Dominik Honnef and contributors
//
// SPDX-License-Identifier: MIT

package sparse

import (
	"iter"
	"runtime"

	"honnef.co/go/curve"
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
	path  chan CompiledPath
	kind  renderTaskKind
	color Color
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
				r.r.FillCompiled(p, t.color)
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

func (r *ConcurrentRenderer) Width() uint16              { return r.r.Width() }
func (r *ConcurrentRenderer) Height() uint16             { return r.r.Height() }
func (r *ConcurrentRenderer) Reset()                     { r.r.Reset() }
func (r *ConcurrentRenderer) SetAffine(aff curve.Affine) { r.r.transform = aff }

func (r *ConcurrentRenderer) Stop() {
	close(r.tasks)
	<-r.done
}

func (r *ConcurrentRenderer) RenderToPixmap(width, height uint16, pixmap []Color) {
	r.Stop()
	r.r.RenderToPixmap(width, height, pixmap)
}

func (r *ConcurrentRenderer) Fill(
	path iter.Seq[curve.PathElement],
	fillRule FillRule,
	color Color,
) {
	t := renderTask{
		path:  make(chan CompiledPath, 1),
		kind:  fillRenderTask,
		color: color,
	}
	r.tasks <- t
	go func(width, height uint16, affine curve.Affine) {
		t.path <- CompileFillPath(path, fillRule, affine, width, height)
	}(r.Width(), r.Height(), r.r.transform)
}

func (r *ConcurrentRenderer) Stroke(
	path iter.Seq[curve.PathElement],
	stroke_ curve.Stroke,
	color Color,
) {
	t := renderTask{
		path:  make(chan CompiledPath, 1),
		kind:  fillRenderTask,
		color: color,
	}
	r.tasks <- t
	go func(width, height uint16, affine curve.Affine) {
		t.path <- CompileStrokedPath(path, stroke_, affine, width, height)
	}(r.Width(), r.Height(), r.r.transform)
}

func (r *ConcurrentRenderer) PushClip(
	path iter.Seq[curve.PathElement],
	fill FillRule,
) {
	t := renderTask{
		path: make(chan CompiledPath, 1),
		kind: clipRenderTask,
	}
	r.tasks <- t
	go func(width, height uint16, affine curve.Affine) {
		t.path <- CompileFillPath(path, fill, affine, width, height)
	}(r.Width(), r.Height(), r.r.transform)
}

func (r *ConcurrentRenderer) PushLayer(l Layer) {
	panic("XXX ot implemented")
}

func (r *ConcurrentRenderer) Save() {
	t := renderTask{
		path: make(chan CompiledPath),
		kind: saveRenderTask,
	}
	close(t.path)
	r.tasks <- t
}

func (r *ConcurrentRenderer) Restore() {
	t := renderTask{
		path: make(chan CompiledPath),
		kind: restoreRenderTask,
	}
	close(t.path)
	r.tasks <- t
}
