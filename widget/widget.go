package widget

import (
	"image/color"
	"time"

	"honnef.co/go/gutter/animation"
	"honnef.co/go/gutter/f32"
	"honnef.co/go/gutter/io/pointer"
	"honnef.co/go/gutter/render"

	"gioui.org/op"
	"gioui.org/op/paint"
)

var _ RenderObjectWidget = (*Padding)(nil)
var _ SingleChildWidget = (*Padding)(nil)
var _ RenderObjectWidget = (*ColoredBox)(nil)
var _ SingleChildWidget = (*ColoredBox)(nil)
var _ RenderObjectWidget = (*PointerRegion)(nil)
var _ SingleChildWidget = (*PointerRegion)(nil)
var _ StatefulWidget[*AnimatedPadding] = (*AnimatedPadding)(nil)
var _ SingleChildWidget = (*AnimatedPadding)(nil)
var _ RenderObjectWidget = (*Opacity)(nil)
var _ SingleChildWidget = (*Opacity)(nil)
var _ StatefulWidget[*AnimatedOpacity] = (*AnimatedOpacity)(nil)
var _ SingleChildWidget = (*AnimatedOpacity)(nil)

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

type AnimatedPadding struct {
	Padding render.Inset
	Child   Widget

	Duration time.Duration
	Curve    func(t float64) float64
}

// GetChild implements SingleChildWidget.
func (a *AnimatedPadding) GetChild() Widget {
	return a.Child
}

// CreateElement implements StatefulWidget.
func (a *AnimatedPadding) CreateElement() Element {
	return NewInteriorElement(a)
}

// CreateState implements StatefulWidget.
func (a *AnimatedPadding) CreateState() State[*AnimatedPadding] {
	return &animatedPaddingState{}
}

// Key implements StatefulWidget.
func (a *AnimatedPadding) Key() any {
	// XXX
	return nil
}

type animatedPaddingState struct {
	StateHandle[*AnimatedPadding]

	animation animation.Animation[render.Inset]
	padding   render.Inset
	// This caches the method value. If we didn't cache it, Go would create it anew every time we pass it to
	// AddNextFrameCallback, causing an allocation on every animation frame. The field gets set at state
	// initialization time, i.e. in the Transition method.
	updateAnimationClosure func(time.Time)
}

// Build implements State.
func (a *animatedPaddingState) Build() Widget {
	return &Padding{
		Padding: a.padding,
		Child:   a.Widget.Child,
	}
}

func (a *animatedPaddingState) updateAnimation(now time.Time) {
	var done bool
	a.padding, done = a.animation.Evaluate(now)
	if !done {
		// XXX this chain of fields is ridiculous
		a.StateHandle.Element.Handle().buildOwner.PipelineOwner.AddNextFrameCallback(a.updateAnimationClosure)
	}
	MarkNeedsBuild(a.Element)
}

// Transition implements State.
func (a *animatedPaddingState) Transition(t StateTransition[*AnimatedPadding]) {
	switch t.Kind {
	case StateInitializing:
		w := a.Widget
		a.updateAnimationClosure = a.updateAnimation
		a.padding = w.Padding
		if w.Curve != nil {
			a.animation.Curve = w.Curve
		} else {
			a.animation.Curve = animation.EaseInSine
		}
		a.animation.Compute = render.LerpInset
	case StateUpdatedWidget:
		w := a.Widget
		if w.Padding != t.OldWidget.Padding {
			// XXX this should use the frame's now, not time.Now
			a.animation.Start(time.Now(), w.Duration, a.padding, w.Padding)
			a.updateAnimation(time.Now())
		}
		MarkNeedsBuild(a.Element)
	}
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
	obj.OnScroll = p.OnScroll
	obj.OnAll = p.OnAll
	return obj
}

// UpdateRenderObject implements RenderObjectWidget.
func (p *PointerRegion) UpdateRenderObject(ctx BuildContext, obj render.Object) {
	obj.(*render.PointerRegion).OnPress = p.OnPress
	obj.(*render.PointerRegion).OnRelease = p.OnRelease
	obj.(*render.PointerRegion).OnMove = p.OnMove
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

type Opacity struct {
	Opacity float32
	Child   Widget
}

// CreateRenderObject implements RenderObjectWidget.
func (o *Opacity) CreateRenderObject(ctx BuildContext) render.Object {
	return &render.Opacity{Opacity: o.Opacity}
}

// UpdateRenderObject implements RenderObjectWidget.
func (o *Opacity) UpdateRenderObject(ctx BuildContext, obj render.Object) {
	obj.(*render.Opacity).SetOpacity(o.Opacity)
}

// CreateElement implements SingleChildWidget.
func (o *Opacity) CreateElement() Element {
	return NewSingleChildRenderObjectElement(o)
}

// GetChild implements SingleChildWidget.
func (o *Opacity) GetChild() Widget {
	return o.Child
}

// Key implements SingleChildWidget.
func (o *Opacity) Key() any {
	// XXX
	return nil
}

type AnimatedOpacity struct {
	Opacity float32
	Child   Widget

	Duration time.Duration
	Curve    func(t float64) float64
}

// CreateState implements StatefulWidget.
func (a *AnimatedOpacity) CreateState() State[*AnimatedOpacity] {
	return &animatedOpacityState{}
}

// CreateElement implements SingleChildWidget.
func (a *AnimatedOpacity) CreateElement() Element {
	return NewInteriorElement(a)
}

// GetChild implements SingleChildWidget.
func (a *AnimatedOpacity) GetChild() Widget {
	return a.Child
}

// Key implements SingleChildWidget.
func (a *AnimatedOpacity) Key() any {
	// XXX
	return nil
}

type animatedOpacityState struct {
	StateHandle[*AnimatedOpacity]

	animation              animation.Animation[float32]
	opacity                float32
	updateAnimationClosure func(time.Time)
}

// Build implements State.
func (a *animatedOpacityState) Build() Widget {
	return &Opacity{
		Opacity: a.opacity,
		Child:   a.Widget.Child,
	}
}

func (a *animatedOpacityState) updateAnimation(now time.Time) {
	var done bool
	a.opacity, done = a.animation.Evaluate(now)
	if !done {
		// XXX this chain of fields is ridiculous
		a.StateHandle.Element.Handle().buildOwner.PipelineOwner.AddNextFrameCallback(a.updateAnimationClosure)
	}
	MarkNeedsBuild(a.Element)
}

// Transition implements State.
func (a *animatedOpacityState) Transition(t StateTransition[*AnimatedOpacity]) {
	switch t.Kind {
	case StateInitializing:
		w := a.Widget
		a.updateAnimationClosure = a.updateAnimation
		a.opacity = w.Opacity
		if w.Curve != nil {
			a.animation.Curve = w.Curve
		} else {
			a.animation.Curve = animation.EaseInSine
		}
		a.animation.Compute = animation.Lerp[float32]
	case StateUpdatedWidget:
		w := a.Widget
		if w.Opacity != t.OldWidget.Opacity {
			// XXX this should use the frame's now, not time.Now
			a.animation.Start(time.Now(), w.Duration, a.opacity, w.Opacity)
			a.updateAnimation(time.Now())
		}
		MarkNeedsBuild(a.Element)
	}
}
