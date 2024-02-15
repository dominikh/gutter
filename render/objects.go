package render

import (
	"image/color"

	"honnef.co/go/gutter/animation"
	"honnef.co/go/gutter/f32"

	"gioui.org/op"
	"gioui.org/op/paint"
)

var _ Object = (*FillColor)(nil)

var _ ObjectWithChild = (*Clip)(nil)
var _ ObjectWithChild = (*Constrained)(nil)
var _ ObjectWithChild = (*Opacity)(nil)
var _ ObjectWithChild = (*Padding)(nil)

var _ ObjectWithChildren = (*Row)(nil)

type Box struct {
	ObjectHandle
}

// Clip prevents its child from painting outside its bounds.
type Clip struct {
	Box
	SingleChild
}

// PerformLayout implements Object.
func (w *Clip) PerformLayout() f32.Point {
	Layout(w.Child, w.Handle().Constraints(), true)
	return w.Child.Handle().Size()
}

// PerformPaint implements Object.
func (w *Clip) PerformPaint(r *Renderer, ops *op.Ops) {
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
		MarkNeedsPaint(fc)
	}
}

func (fc *FillColor) Color() color.NRGBA {
	return fc.color
}

// PerformLayout implements Object.
func (c *FillColor) PerformLayout() f32.Point {
	return c.Handle().Constraints().Min
}

func (c *FillColor) SizedByParent() {}

// PerformPaint implements Object.
func (c *FillColor) PerformPaint(_ *Renderer, ops *op.Ops) {
	paint.Fill(ops, c.color)
}

type Inset struct {
	Left, Top, Right, Bottom float32
}

func LerpInset(start, end Inset, t float64) Inset {
	return Inset{
		Left:   animation.Lerp(start.Left, end.Left, t),
		Top:    animation.Lerp(start.Top, end.Top, t),
		Right:  animation.Lerp(start.Right, end.Right, t),
		Bottom: animation.Lerp(start.Bottom, end.Bottom, t),
	}
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
		MarkNeedsLayout(p)
	}
}

func (p *Padding) Inset() Inset {
	return p.inset
}

// PerformLayout implements Object.
func (p *Padding) PerformLayout() f32.Point {
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

// PerformPaint implements Object.
func (p *Padding) PerformPaint(r *Renderer, ops *op.Ops) {
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
		MarkNeedsLayout(c)
	}
}

func (c *Constrained) ExtraConstraints() Constraints {
	return c.extraConstraints
}

// PerformLayout implements Object.
func (c *Constrained) PerformLayout() f32.Point {
	cs := c.extraConstraints.Enforce(c.Handle().Constraints())
	Layout(c.Child, cs, true)
	return c.Child.Handle().Size()
}

// PerformPaint implements Object.
func (c *Constrained) PerformPaint(r *Renderer, ops *op.Ops) {
	r.Paint(c.Child).Add(ops)
}

func (c *Constrained) PerformSetChild(child Object) {
	child.Handle().SetParent(c)
	c.Child = child
}

// TODO turn this into a proper Flex
type Row struct {
	Box
	ManyChildren
}

// PerformLayout implements Object.
func (row *Row) PerformLayout() f32.Point {
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

// PerformPaint implements Object.
func (row *Row) PerformPaint(r *Renderer, ops *op.Ops) {
	for _, child := range row.children {
		stack := op.Affine(f32.Affine2D{}.Offset(child.Handle().offset)).Push(ops)
		call := r.Paint(child)
		call.Add(ops)
		stack.Pop()
	}
}

type Opacity struct {
	Box
	SingleChild
	Opacity float32
}

// PerformLayout implements Object.
func (o *Opacity) PerformLayout() (size f32.Point) {
	if o.Child != nil {
		return Layout(o.Child, o.constraints, true)
	} else {
		return o.constraints.Constrain(f32.Pt(0, 0))
	}
}

// PerformPaint implements Object.
func (o *Opacity) PerformPaint(r *Renderer, ops *op.Ops) {
	switch o.Opacity {
	case 0:
		return
	case 1:
		r.Paint(o.Child).Add(ops)
	default:
		defer paint.PushOpacity(ops, o.Opacity).Pop()
		r.Paint(o.Child).Add(ops)
	}
}

func (o *Opacity) SetOpacity(f float32) {
	if o.Opacity != f {
		o.Opacity = f
		MarkNeedsPaint(o)
	}
}
