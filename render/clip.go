package render

import (
	"gioui.org/f32"
	"gioui.org/op"
	"gioui.org/op/clip"
)

type FRect struct {
	Min f32.Point
	Max f32.Point
}

func (r FRect) Path(ops *op.Ops) clip.PathSpec {
	var p clip.Path
	p.Begin(ops)
	r.IntoPath(&p)
	return p.End()
}

func (r FRect) IntoPath(p *clip.Path) {
	p.MoveTo(r.Min)
	p.LineTo(f32.Pt(r.Max.X, r.Min.Y))
	p.LineTo(r.Max)
	p.LineTo(f32.Pt(r.Min.X, r.Max.Y))
	p.LineTo(r.Min)
}

func (r FRect) IntoPathR(p *clip.Path) {
	p.MoveTo(r.Min)
	p.LineTo(f32.Pt(r.Min.X, r.Max.Y))
	p.LineTo(r.Max)
	p.LineTo(f32.Pt(r.Max.X, r.Min.Y))
	p.LineTo(r.Min)
}

func (r FRect) Op(ops *op.Ops) clip.Op {
	return clip.Outline{Path: r.Path(ops)}.Op()
}

func (r FRect) Contains(pt f32.Point) bool {
	return pt.X >= r.Min.X && pt.X < r.Max.X &&
		pt.Y >= r.Min.Y && pt.Y < r.Max.Y
}

func (r FRect) Dx() float32 {
	return r.Max.X - r.Min.X
}

func (r FRect) Dy() float32 {
	return r.Max.Y - r.Min.Y
}
