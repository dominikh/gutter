package render

import (
	"fmt"
	"image/color"
	"log"
	"strings"

	"honnef.co/go/gutter/f32"

	"gioui.org/op"
	"gioui.org/op/paint"
)

var _ Object = (*Clip)(nil)
var _ Object = (*FillColor)(nil)
var _ Object = (*Padding)(nil)
var _ Object = (*Constrained)(nil)

var _ Parent = (*SingleChild)(nil)
var _ Parent = (*ManyChildren)(nil)

// TODO implement sizedByParent optimization
// TODO implement parentUsesSize optimization
// TODO implement relayout boundary optimization
// TODO implement support for multiple layers
// TODO don't recompute layout of entire tree on every frame
// TODO guard assertions behind debug flag
// TODO support baseline stuff
// TODO hit testing
// TODO accessibility
// TODO RTL support (see https://api.flutter.dev/flutter/dart-ui/TextDirection.html)
// TODO should we handle nil children?
// TODO split repaint and relayout marking, to make use of parentUsesSize
// TODO dry layout/intrinsic dimensions/https://github.com/flutter/flutter/issues/48679

// OPT if we could call op.Ops directly, then we wouldn't have to repaint parents, because their cached ops
//   would still be calling the repainted ops of the child. However, Gio makes us go through macros, and
//   macros record both the start and end PC, and we can't expect those to remain the same.

type Object interface {
	// Layout lays out the object.
	//
	// Don't call Object.Layout directly. Use [Renderer.Layout] instead.
	Layout(r *Renderer)
	// Paint paints the object at the specified offset.
	//
	// Don't call Object.Paint directly. Use [Renderer.Paint] instead.
	Paint(r *Renderer, ops *op.Ops, offset f32.Point)
	Size() f32.Point

	SetParent(parent Object)
	Constraints() Constraints
	// Should only be called by this package.
	SetConstraints(cs Constraints)

	// Mark the object as needing to repaint.
	MarkNeedsPaint()
	NeedRepaint() bool
	ClearRepaint()

	MarkNeedsLayout()
	NeedRelayout() bool
	ClearRelayout()
}

type SizedByParenter interface {
	SizedByParent() bool
}

type Parent interface {
	VisitChildren(yield func(Object) bool)
}

type Constraints struct {
	Min, Max f32.Point
}

func (c Constraints) Enforce(oc Constraints) Constraints {
	return Constraints{
		Min: f32.Point{
			X: f32.Clamp(c.Min.X, oc.Min.X, oc.Max.X),
			Y: f32.Clamp(c.Min.Y, oc.Min.Y, oc.Max.Y),
		},
		Max: f32.Point{
			X: f32.Clamp(c.Max.X, oc.Min.X, oc.Max.X),
			Y: f32.Clamp(c.Max.Y, oc.Min.Y, oc.Max.Y),
		},
	}
}

// Constrain a size so each dimension is in the range [min;max].
func (c Constraints) Constrain(size f32.Point) f32.Point {
	if min := c.Min.X; size.X < min {
		size.X = min
	}
	if min := c.Min.Y; size.Y < min {
		size.Y = min
	}
	if max := c.Max.X; size.X > max {
		size.X = max
	}
	if max := c.Max.Y; size.Y > max {
		size.Y = max
	}
	return size
}

func FormatTree(root Object) string {
	var sb strings.Builder

	seen := map[Object]struct{}{}
	var formatTree func(root Object, depth int)
	formatTree = func(root Object, depth int) {
		if _, ok := seen[root]; ok {
			panic("render object tree is actually circular graph")
		}
		seen[root] = struct{}{}
		fmt.Fprintf(&sb, "%s(%[2]T)(%[2]p) (size: %s)\n", strings.Repeat("\t", depth), root, root.Size())
		if root, ok := root.(Parent); ok {
			root.VisitChildren(func(o Object) bool {
				formatTree(o, depth+1)
				return true
			})
		}
	}
	formatTree(root, 0)

	return sb.String()
}

type SingleChild struct {
	child Object
}

func (c *SingleChild) VisitChildren(yield func(Object) bool) {
	if c.child != nil {
		yield(c.child)
	}
}

type ManyChildren struct {
	children []Object
}

func (c *ManyChildren) VisitChildren(yield func(Object) bool) {
	for _, child := range c.children {
		if !yield(child) {
			break
		}
	}
}

type Box struct {
	// The size computed by the last call to Layout.
	size        f32.Point
	repaint     bool
	relayout    bool
	parent      Object
	constraints Constraints
}

func (b *Box) Constraints() Constraints {
	return b.constraints
}

func (b *Box) SetConstraints(cs Constraints) {
	b.constraints = cs
}

// SetSize stores the computed size sz.
func (b *Box) SetSize(sz f32.Point) {
	b.size = sz
}

// Size implements RenderObject.
func (b *Box) Size() f32.Point {
	return b.size
}

func (b *Box) MarkNeedsPaint() {
	b.repaint = true
	if b.parent != nil {
		b.parent.MarkNeedsPaint()
	}
}

func (b *Box) NeedRepaint() bool {
	return b.repaint
}

func (b *Box) ClearRepaint() {
	b.repaint = false
}

func (b *Box) MarkNeedsLayout() {
	b.relayout = true
	if b.parent != nil {
		if obj2, ok := b.parent.(SizedByParenter); !ok || !obj2.SizedByParent() {
			b.parent.MarkNeedsLayout()
		}
	}
}

func (b *Box) NeedRelayout() bool {
	return b.relayout
}

func (b *Box) ClearRelayout() {
	b.relayout = false
}

func (b *Box) SetParent(parent Object) {
	b.parent = parent
}

// Clip prevents its child from painting outside its bounds.
type Clip struct {
	Box
	SingleChild
}

func (w *Clip) SetChild(child Object) {
	// TODO make sure the child doesn't already have a parent
	child.SetParent(w)
	w.child = child
}

// Layout implements RenderObject.
func (w *Clip) Layout(r *Renderer) {
	r.Layout(w.child, w.constraints)
	w.SetSize(w.child.Size())
}

// Paint implements RenderObject.
func (w *Clip) Paint(r *Renderer, ops *op.Ops, offset f32.Point) {
	defer FRect{
		Min: offset,
		Max: w.Size().Add(offset),
	}.Op(ops).Push(ops).Pop()
	r.Paint(w.child, offset).Add(ops)
}

// FillColor fills an infinite plane with the provided color.
//
// In layout, it takes up the least amount of space possible.
type FillColor struct {
	Box
	color color.NRGBA
}

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
func (c *FillColor) Layout(_ *Renderer) {
	c.SetSize(c.constraints.Min)
}

func (c *FillColor) SizedByParent() bool {
	return true
}

// Paint implements RenderObject.
func (c *FillColor) Paint(_ *Renderer, ops *op.Ops, offset f32.Point) {
	defer op.Affine(f32.Affine2D{}.Offset(offset)).Push(ops).Pop()
	paint.Fill(ops, c.color)
}

type Inset struct {
	Left, Top, Right, Bottom float32
}

type Padding struct {
	Box
	SingleChild
	inset          Inset
	relChildOffset f32.Point
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

func (p *Padding) SetChild(child Object) {
	child.SetParent(p)
	p.child = child
}

// Layout implements RenderObject.
func (p *Padding) Layout(r *Renderer) {
	cs := p.constraints
	if p.child == nil {
		p.SetSize(cs.Constrain(f32.Pt(p.inset.Left+p.inset.Right, p.inset.Top+p.inset.Bottom)))
		return
	}
	horiz := p.inset.Left + p.inset.Right
	vert := p.inset.Top + p.inset.Bottom
	newMin := f32.Pt(max(0, cs.Min.X-horiz), max(0, cs.Min.Y-vert))
	innerCs := Constraints{
		Min: newMin,
		Max: f32.Pt(max(newMin.X, cs.Max.X-horiz), max(newMin.Y, cs.Max.Y-vert)),
	}
	r.Layout(p.child, innerCs)
	p.relChildOffset = f32.Pt(p.inset.Left, p.inset.Top)
	childSz := p.child.Size()
	p.SetSize(cs.Constrain(childSz.Add(f32.Pt(horiz, vert))))
}

// Paint implements RenderObject.
func (p *Padding) Paint(r *Renderer, ops *op.Ops, offset f32.Point) {
	r.Paint(p.child, offset.Add(p.relChildOffset)).Add(ops)
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
func (c *Constrained) Layout(r *Renderer) {
	cs := c.extraConstraints.Enforce(c.constraints)
	r.Layout(c.child, cs)
	c.size = c.child.Size()
}

// Paint implements Object.
func (c *Constrained) Paint(r *Renderer, ops *op.Ops, offset f32.Point) {
	r.Paint(c.child, offset).Add(ops)
}

func (c *Constrained) SetChild(child Object) {
	child.SetParent(c)
	c.child = child
}

type Renderer struct {
	ops map[Object]cachedOps
}

type cachedOps struct {
	ops  *op.Ops
	call op.CallOp
}

func (r *Renderer) Render(root Object, ops *op.Ops, cs Constraints, offset f32.Point) {
	r.Layout(root, cs)
	fmt.Println("Rendering:")
	fmt.Println(FormatTree(root))
	r.Paint(root, offset).Add(ops)
}

func (r *Renderer) Paint(obj Object, offset f32.Point) op.CallOp {
	var ops *op.Ops
	if obj.NeedRepaint() {
		obj.ClearRepaint()
		if cached, ok := r.ops[obj]; ok {
			ops = cached.ops
			ops.Reset()
		} else {
			ops = new(op.Ops)
		}
	} else if cached, ok := r.ops[obj]; ok {
		log.Printf("Painting cached (%[1]T)(%[1]p)", obj)
		return cached.call
	} else {
		ops = new(op.Ops)
	}

	log.Printf("painting (%[1]T)(%[1]p)", obj)
	m := op.Record(ops)
	obj.Paint(r, ops, offset)
	call := m.Stop()
	r.ops[obj] = cachedOps{ops, call}
	return call
}

func (r *Renderer) Layout(obj Object, cs Constraints) {
	log.Printf("laying out (%[1]T)(%[1]p), cs: %s", obj, cs)
	if cs.Min.X > cs.Max.X || cs.Min.Y > cs.Max.Y || cs.Min.X < 0 || cs.Min.Y < 0 {
		panic(fmt.Sprintf("constraints %v are malformed", cs))
	}
	if obj2, ok := obj.(SizedByParenter); ok && obj2.SizedByParent() {
		if obj.Constraints() == cs {
			log.Printf("\t SizedByParent and able to reuse old layout")
			return
		}
	}
	if !obj.NeedRelayout() && obj.Constraints() == cs {
		log.Printf("\t not marked for relayout and constraints didn't change, reusing old layout")
		return
	}
	obj.ClearRelayout()
	oldSz := obj.Size()
	obj.SetConstraints(cs)
	obj.Layout(r)
	sz := obj.Size()
	if sz.X < cs.Min.X || sz.X > cs.Max.X || sz.Y < cs.Min.Y || sz.Y > cs.Max.Y {
		panic(fmt.Sprintf("(%[1]T)(%[1]p).Layout violated constraints %v by computing size %v", obj, cs, sz))
	}
	if sz != oldSz {
		obj.MarkNeedsPaint()
	}
}

func NewRenderer() *Renderer {
	return &Renderer{
		ops: make(map[Object]cachedOps),
	}
}
