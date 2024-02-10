package render

import (
	"image/color"

	"honnef.co/go/gutter/f32"

	"gioui.org/op"
	"gioui.org/op/paint"
)

var _ ObjectWithChild = (*Clip)(nil)
var _ Object = (*FillColor)(nil)
var _ ObjectWithChild = (*Padding)(nil)
var _ ObjectWithChild = (*Constrained)(nil)
var _ ObjectWithChildren = (*Row)(nil)

func (obj *Clip) MarkNeedsPaint()        { MarkNeedsPaint(obj) }
func (obj *FillColor) MarkNeedsPaint()   { MarkNeedsPaint(obj) }
func (obj *Padding) MarkNeedsPaint()     { MarkNeedsPaint(obj) }
func (obj *Constrained) MarkNeedsPaint() { MarkNeedsPaint(obj) }
func (obj *Row) MarkNeedsPaint()         { MarkNeedsPaint(obj) }

func (obj *Clip) MarkNeedsLayout()        { MarkNeedsLayout(obj) }
func (obj *FillColor) MarkNeedsLayout()   { MarkNeedsLayout(obj) }
func (obj *Padding) MarkNeedsLayout()     { MarkNeedsLayout(obj) }
func (obj *Constrained) MarkNeedsLayout() { MarkNeedsLayout(obj) }
func (obj *Row) MarkNeedsLayout()         { MarkNeedsLayout(obj) }

type Box struct {
	ObjectHandle
}

// Clip prevents its child from painting outside its bounds.
type Clip struct {
	Box
	SingleChild
}

// Layout implements RenderObject.
func (w *Clip) Layout() f32.Point {
	Layout(w.Child, w.Handle().Constraints(), true)
	return w.Child.Handle().Size()
}

// Paint implements RenderObject.
func (w *Clip) Paint(r *Renderer, ops *op.Ops) {
	defer FRect{
		Min: f32.Pt(0, 0),
		Max: w.Handle().Size(),
	}.Op(ops).Push(ops).Pop()
	r.Paint(w.Child).Add(ops)
}

// FillColor fills an infinite plane with the provided color.
//
// In layout, it takes up the least amount of space possible.
type FillColor struct {
	Box
	color color.NRGBA
}

// VisitChildren implements Object.
func (*FillColor) VisitChildren(yield func(Object) bool) {}

func (fc *FillColor) SetColor(c color.NRGBA) {
	if fc.color != c {
		fc.color = c
		fc.MarkNeedsPaint()
	}
}

func (fc *FillColor) Color() color.NRGBA {
	return fc.color
}

// Layout implements RenderObject.
func (c *FillColor) Layout() f32.Point {
	return c.Handle().Constraints().Min
}

func (c *FillColor) SizedByParent() {}

// Paint implements RenderObject.
func (c *FillColor) Paint(_ *Renderer, ops *op.Ops) {
	paint.Fill(ops, c.color)
}

type Inset struct {
	Left, Top, Right, Bottom float32
}

type Padding struct {
	Box
	SingleChild
	inset Inset
}

func NewPadding(padding Inset) *Padding {
	return &Padding{inset: padding}
}

func (p *Padding) SetInset(ins Inset) {
	if p.inset != ins {
		p.inset = ins
		p.MarkNeedsLayout()
	}
}

func (p *Padding) Inset() Inset {
	return p.inset
}

// Layout implements RenderObject.
func (p *Padding) Layout() f32.Point {
	cs := p.Handle().Constraints()
	if p.Child == nil {
		return cs.Constrain(f32.Pt(p.inset.Left+p.inset.Right, p.inset.Top+p.inset.Bottom))
	}
	horiz := p.inset.Left + p.inset.Right
	vert := p.inset.Top + p.inset.Bottom
	newMin := f32.Pt(max(0, cs.Min.X-horiz), max(0, cs.Min.Y-vert))
	innerCs := Constraints{
		Min: newMin,
		Max: f32.Pt(max(newMin.X, cs.Max.X-horiz), max(newMin.Y, cs.Max.Y-vert)),
	}
	childSz := Layout(p.Child, innerCs, true)
	p.Child.Handle().offset = f32.Pt(p.inset.Left, p.inset.Top)
	return cs.Constrain(childSz.Add(f32.Pt(horiz, vert)))
}

// Paint implements RenderObject.
func (p *Padding) Paint(r *Renderer, ops *op.Ops) {
	defer op.Affine(f32.Affine2D{}.Offset(p.Child.Handle().offset)).Push(ops).Pop()
	r.Paint(p.Child).Add(ops)
}

type Constrained struct {
	Box
	SingleChild
	extraConstraints Constraints
}

func (c *Constrained) SetExtraConstraints(cs Constraints) {
	if c.extraConstraints != cs {
		c.extraConstraints = cs
		c.MarkNeedsLayout()
	}
}

func (c *Constrained) ExtraConstraints() Constraints {
	return c.extraConstraints
}

// Layout implements Object.
func (c *Constrained) Layout() f32.Point {
	cs := c.extraConstraints.Enforce(c.Handle().Constraints())
	Layout(c.Child, cs, true)
	return c.Child.Handle().Size()
}

// Paint implements Object.
func (c *Constrained) Paint(r *Renderer, ops *op.Ops) {
	r.Paint(c.Child).Add(ops)
}

func (c *Constrained) SetChild(child Object) {
	child.Handle().SetParent(c)
	c.Child = child
}

// TODO turn this into a proper Flex
type Row struct {
	Box
	ManyChildren
}

// Layout implements Object.
func (row *Row) Layout() f32.Point {
	cs := row.Handle().Constraints()
	inCs := cs
	inCs.Min.X = 0
	off := float32(0)
	height := cs.Min.Y
	for _, child := range row.children {
		child.Handle().offset = f32.Pt(off, 0)
		Layout(child, inCs, true)
		sz := child.Handle().Size()
		inCs.Max.X -= sz.X
		off += sz.X
		if sz.Y > height {
			height = sz.Y
		}
	}
	return f32.Pt(cs.Max.X, height)
}

func (row *Row) AddChild(child Object) {
	child.Handle().SetParent(row)
	row.children = append(row.children, child)
}

// Paint implements Object.
func (row *Row) Paint(r *Renderer, ops *op.Ops) {
	for _, child := range row.children {
		stack := op.Affine(f32.Affine2D{}.Offset(child.Handle().offset)).Push(ops)
		call := r.Paint(child)
		call.Add(ops)
		stack.Pop()
	}
}
