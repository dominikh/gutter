package widget

import (
	"image/color"

	"honnef.co/go/gutter/f32"
	"honnef.co/go/gutter/render"

	"gioui.org/op"
	"gioui.org/op/paint"
)

var _ RenderObjectWidget = (*Padding)(nil)
var _ SingleChildWidget = (*Padding)(nil)
var _ RenderObjectWidget = (*ColoredBox)(nil)
var _ SingleChildWidget = (*ColoredBox)(nil)

var _ render.Object = (*renderColoredBox)(nil)
var _ render.ObjectWithChild = (*renderColoredBox)(nil)

type Padding struct {
	Padding     render.Inset
	ChildWidget Widget
}

// XXX
func (*Padding) Key() any    { return nil }
func (*ColoredBox) Key() any { return nil }

func (p *Padding) Child() Widget {
	return p.ChildWidget
}

func (p *Padding) CreateRenderObject(ctx BuildContext) render.Object {
	println(1)
	return render.NewPadding(p.Padding)
}

func (p *Padding) UpdateRenderObject(ctx BuildContext, obj render.Object) {
	println(2)
	obj.(*render.Padding).SetInset(p.Padding)
}

func (p *Padding) CreateElement() Element {
	println(3)
	return NewSingleChildRenderObjectElement(p)
}

type ColoredBox struct {
	Color       color.NRGBA
	ChildWidget Widget
}

func (c *ColoredBox) Child() Widget {
	return c.ChildWidget
}

func (c *ColoredBox) CreateRenderObject(ctx BuildContext) render.Object {
	return &renderColoredBox{color: c.Color}
}

func (c *ColoredBox) UpdateRenderObject(ctx BuildContext, obj render.Object) {
	obj.(*renderColoredBox).setColor(c.Color)
}

func (c *ColoredBox) CreateElement() Element {
	return NewSingleChildRenderObjectElement(c)
}

type renderColoredBox struct {
	render.Box
	render.SingleChild
	color color.NRGBA
}

// Layout implements render.Object.
func (c *renderColoredBox) Layout() (size f32.Point) {
	if c.Child == nil {
		return c.Constraints().Min
	}
	return render.Layout(c.Child, c.Constraints(), true)
}

// MarkNeedsLayout implements render.Object.
func (c *renderColoredBox) MarkNeedsLayout() {
	render.MarkNeedsLayout(c)
}

// MarkNeedsPaint implements render.Object.
func (c *renderColoredBox) MarkNeedsPaint() {
	render.MarkNeedsPaint(c)
}

func (c *renderColoredBox) SetChild(child render.Object) {
	child.Handle().SetParent(c)
	c.Child = child
}

func (c *renderColoredBox) Paint(r *render.Renderer, ops *op.Ops) {
	sz := c.Size()
	if sz != f32.Pt(0, 0) {
		paint.FillShape(ops, c.color, render.FRect{Max: sz}.Op(ops))
	}
	if c.Child != nil {
		r.Paint(c.Child).Add(ops)
	}
}

func (r *renderColoredBox) setColor(c color.NRGBA) {
	if r.color != c {
		r.color = c
		r.MarkNeedsPaint()
	}
}

func NewSingleChildRenderObjectElement(w RenderObjectWidget) *SingleChildRenderObjectElement {
	el := &SingleChildRenderObjectElement{
		RenderObjectElementHandle: RenderObjectElementHandle{
			ElementHandle: ElementHandle{
				widget: w,
			},
		},
	}
	return el
}

type SingleChildRenderObjectElement struct {
	RenderObjectElementHandle
	ChildElement Element
}

func (el *SingleChildRenderObjectElement) Child() Element {
	return el.ChildElement
}

func (el *SingleChildRenderObjectElement) SetChild(child Element) {
	println("A")
	el.ChildElement = child
}

func (el *SingleChildRenderObjectElement) Update(newWidget Widget) {
	SingleChildRenderObjectElementUpdate(el, newWidget.(RenderObjectWidget))
	el.ChildElement = el.UpdateChild(el.ChildElement, el.widget.(SingleChildWidget).Child(), nil)
}

func (el *SingleChildRenderObjectElement) ForgetChild(child Element) {
	el.ChildElement = nil
}

func (el *SingleChildRenderObjectElement) Activate() {
	ElementActivate(el)
}

func (el *SingleChildRenderObjectElement) RenderObjectAttachingChild() Element {
	return RenderObjectElementRenderObjectAttachingChild(el)
}

func (el *SingleChildRenderObjectElement) UpdateChild(child Element, newWidget Widget, newSlot any) Element {
	return ElementUpdateChild(el, child, newWidget, newSlot)
}

func (el *SingleChildRenderObjectElement) MoveRenderObjectChild(child render.Object, oldSlot, newSlot any) {
	panic("unexpected call")
}

func (el *SingleChildRenderObjectElement) DetachRenderObject() {
	RenderObjectElementDetachRenderObject(el)
}

func (el *SingleChildRenderObjectElement) AttachRenderObject(slot any) {
	RenderObjectElementAttachRenderObject(el, slot)
}

func (el *SingleChildRenderObjectElement) Mount(parent Element, slot any) {
	RenderObjectElementMount(el, parent, slot)
	el.ChildElement = el.UpdateChild(el.ChildElement, el.widget.(SingleChildWidget).Child(), nil)
}

func (el *SingleChildRenderObjectElement) Unmount() {
	RenderObjectElementUnmount(el)
}

func (el *SingleChildRenderObjectElement) InsertRenderObjectChild(child render.Object, slot any) {
	renderObject := el.renderObject.(render.ObjectWithChild)
	renderObject.SetChild(child)
}

func (el *SingleChildRenderObjectElement) RemoveRenderObjectChild(child render.Object, slot any) {
	el.renderObject.(render.ObjectWithChild).SetChild(nil)
}

func (s *SingleChildRenderObjectElement) VisitChildren(yield func(el Element) bool) {
	println("B")
	if s.ChildElement != nil {
		yield(s.ChildElement)
	}
}

var _ RenderObjectWidget = (*SizedBox)(nil)
var _ SingleChildWidget = (*SizedBox)(nil)

type SizedBox struct {
	Width, Height float32
	ChildWidget   Widget
}

// CreateRenderObject implements RenderObjectWidget.
func (box *SizedBox) CreateRenderObject(ctx BuildContext) render.Object {
	obj := &render.Constrained{}
	cs := render.Constraints{Min: f32.Pt(box.Width, box.Height), Max: f32.Pt(box.Width, box.Height)}
	obj.SetExtraConstraints(cs)
	return obj
}

// UpdateRenderObject implements RenderObjectWidget.
func (box *SizedBox) UpdateRenderObject(ctx BuildContext, obj render.Object) {
	cs := render.Constraints{Min: f32.Pt(box.Width, box.Height), Max: f32.Pt(box.Width, box.Height)}
	obj.(*render.Constrained).SetExtraConstraints(cs)
}

// CreateElement implements Widget.
func (box *SizedBox) CreateElement() Element {
	return NewSingleChildRenderObjectElement(box)
}

// Key implements Widget.
func (box *SizedBox) Key() any {
	// XXX
	return nil
}

// Child implements SingleChildWidget.
func (box *SizedBox) Child() Widget {
	return box.ChildWidget
}
