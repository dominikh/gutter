package render

import (
	"gioui.org/f32"
	"gioui.org/op"
)

var _ Object = (*Row)(nil)

// TODO turn this into a proper Flex
type Row struct {
	Box
	ManyChildren
	childOffsets []float32
}

// Layout implements Object.
func (row *Row) Layout(r *Renderer) {
	cs := row.constraints
	inCs := cs
	inCs.Min.X = 0
	off := float32(0)
	height := cs.Min.Y
	for i, child := range row.children {
		row.childOffsets[i] = off
		r.Layout(child, inCs)
		sz := child.Size()
		inCs.Max.X -= sz.X
		off += sz.X
		if sz.Y > height {
			height = sz.Y
		}
	}
	row.size = f32.Pt(cs.Max.X, height)
}

func (row *Row) AddChild(child Object) {
	child.SetParent(row)
	row.children = append(row.children, child)
	row.childOffsets = append(row.childOffsets, 0)
}

// Paint implements Object.
func (row *Row) Paint(r *Renderer, ops *op.Ops, offset f32.Point) {
	for i, child := range row.children {
		call := r.Paint(child, offset.Add(f32.Pt(row.childOffsets[i], 0)))
		call.Add(ops)
	}
}
