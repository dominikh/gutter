package widget

import (
	"image/color"

	"honnef.co/go/gutter/f32"
	"honnef.co/go/gutter/render"

	"gioui.org/io/pointer"
	"gioui.org/op"
	"gioui.org/op/paint"
)

var _ RenderObjectWidget = (*Padding)(nil)
var _ SingleChildWidget = (*Padding)(nil)
var _ RenderObjectWidget = (*ColoredBox)(nil)
var _ SingleChildWidget = (*ColoredBox)(nil)
var _ RenderObjectWidget = (*PointerRegion)(nil)
var _ SingleChildWidget = (*PointerRegion)(nil)

var _ render.Object = (*renderColoredBox)(nil)
var _ render.ObjectWithChild = (*renderColoredBox)(nil)

type Padding struct {
	Padding render.Inset
	Child   Widget
}

// XXX
func (*Padding) Key() any    { return nil }
func (*ColoredBox) Key() any { return nil }

func (p *Padding) GetChild() Widget {
	return p.Child
}

func (p *Padding) CreateRenderObject(ctx BuildContext) render.Object {
	return render.NewPadding(p.Padding)
}

func (p *Padding) UpdateRenderObject(ctx BuildContext, obj render.Object) {
	obj.(*render.Padding).SetInset(p.Padding)
}

func (p *Padding) CreateElement() Element {
	return NewSingleChildRenderObjectElement(p)
}

type ColoredBox struct {
	Color color.NRGBA
	Child Widget
}

func (c *ColoredBox) GetChild() Widget {
	return c.Child
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

// PerformLayout implements render.Object.
func (c *renderColoredBox) PerformLayout() (size f32.Point) {
	if c.Child == nil {
		return c.Constraints().Min
	}
	return render.Layout(c.Child, c.Constraints(), true)
}

func (c *renderColoredBox) PerformPaint(r *render.Renderer, ops *op.Ops) {
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
		render.MarkNeedsPaint(r)
	}
}

func NewSingleChildRenderObjectElement(w interface {
	RenderObjectWidget
	SingleChildWidget
}) *SimpleSingleChildRenderObjectElement {
	el := &SimpleSingleChildRenderObjectElement{}
	el.widget = w
	return el
}

var _ RenderObjectWidget = (*SizedBox)(nil)
var _ SingleChildWidget = (*SizedBox)(nil)

type SizedBox struct {
	Width, Height float32
	Child         Widget
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

// GetChild implements SingleChildWidget.
func (box *SizedBox) GetChild() Widget {
	return box.Child
}

type PointerRegion struct {
	OnPress   func(hit render.HitTestEntry, ev pointer.Event)
	OnRelease func(hit render.HitTestEntry, ev pointer.Event)
	OnMove    func(hit render.HitTestEntry, ev pointer.Event)
	OnDrag    func(hit render.HitTestEntry, ev pointer.Event)
	OnScroll  func(hit render.HitTestEntry, ev pointer.Event)
	OnAll     func(hit render.HitTestEntry, ev pointer.Event)
	Child     Widget
}

// CreateRenderObject implements RenderObjectWidget.
func (p *PointerRegion) CreateRenderObject(ctx BuildContext) render.Object {
	obj := &render.PointerRegion{}
	obj.HitTestBehavior = render.Opaque
	obj.OnPress = p.OnPress
	obj.OnRelease = p.OnRelease
	obj.OnMove = p.OnMove
	obj.OnDrag = p.OnDrag
	obj.OnScroll = p.OnScroll
	obj.OnAll = p.OnAll
	return obj
}

// UpdateRenderObject implements RenderObjectWidget.
func (p *PointerRegion) UpdateRenderObject(ctx BuildContext, obj render.Object) {
	obj.(*render.PointerRegion).OnPress = p.OnPress
	obj.(*render.PointerRegion).OnRelease = p.OnRelease
	obj.(*render.PointerRegion).OnMove = p.OnMove
	obj.(*render.PointerRegion).OnDrag = p.OnDrag
	obj.(*render.PointerRegion).OnScroll = p.OnScroll
	obj.(*render.PointerRegion).OnAll = p.OnAll
}

// CreateElement implements Widget.
func (w *PointerRegion) CreateElement() Element {
	return NewSingleChildRenderObjectElement(w)
}

func (w *PointerRegion) GetChild() Widget {
	return w.Child
}

// Key implements Widget.
func (*PointerRegion) Key() any {
	// XXX
	return nil
}
