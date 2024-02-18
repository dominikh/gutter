package widget

import (
	"image/color"
	"reflect"
	"time"

	"honnef.co/go/gutter/animation"
	"honnef.co/go/gutter/f32"
	"honnef.co/go/gutter/io/pointer"
	"honnef.co/go/gutter/render"

	"gioui.org/op"
	"gioui.org/op/paint"
)

var _ RenderObjectWidget = (*ColoredBox)(nil)
var _ RenderObjectWidget = (*Opacity)(nil)
var _ RenderObjectWidget = (*Padding)(nil)
var _ RenderObjectWidget = (*PointerRegion)(nil)
var _ RenderObjectWidget = (*SizedBox)(nil)

var _ Widget = (*AnimatedOpacity)(nil)
var _ Widget = (*AnimatedPadding)(nil)
var _ Widget = (*ColoredBox)(nil)
var _ Widget = (*KeyedSubtree)(nil)
var _ Widget = (*Opacity)(nil)
var _ Widget = (*Padding)(nil)
var _ Widget = (*PointerRegion)(nil)
var _ Widget = (*SizedBox)(nil)

var _ StatefulWidget[*AnimatedOpacity] = (*AnimatedOpacity)(nil)
var _ StatefulWidget[*AnimatedPadding] = (*AnimatedPadding)(nil)

var _ KeyedWidget = (*KeyedSubtree)(nil)

var _ render.Object = (*renderColoredBox)(nil)
var _ render.ObjectWithChildren = (*renderColoredBox)(nil)

type Padding struct {
	Padding render.Inset
	Child   Widget
}

func (p *Padding) CreateRenderObject(ctx BuildContext) render.Object {
	return render.NewPadding(p.Padding)
}

func (p *Padding) UpdateRenderObject(ctx BuildContext, obj render.Object) {
	obj.(*render.Padding).SetInset(p.Padding)
}

func (p *Padding) CreateElement() Element {
	return NewRenderObjectElement(p)
}

type AnimatedPadding struct {
	Padding render.Inset `gutter:"animated"`
	Child   Widget

	Duration time.Duration
	Curve    func(t float64) float64
}

// CreateElement implements StatefulWidget.
func (a *AnimatedPadding) CreateElement() Element {
	return NewInteriorElement(a)
}

// CreateState implements StatefulWidget.
func (a *AnimatedPadding) CreateState() State[*AnimatedPadding] {
	s := &animatedPaddingState{}
	s.AnimatedProperty = makeAnimatedProperty(s, render.LerpInset)
	return s
}

type animatedPaddingState struct {
	StateHandle[*AnimatedPadding]

	AnimatedProperty[render.Inset, *AnimatedPadding, *animatedPaddingState]
}

// Build implements State.
func (a *animatedPaddingState) Build() Widget {
	return &Padding{
		Padding: a.AnimatedProperty.Value,
		Child:   a.Widget.Child,
	}
}

type ColoredBox struct {
	Color color.NRGBA
	Child Widget
}

func (c *ColoredBox) CreateRenderObject(ctx BuildContext) render.Object {
	return &renderColoredBox{color: c.Color}
}

func (c *ColoredBox) UpdateRenderObject(ctx BuildContext, obj render.Object) {
	obj.(*renderColoredBox).setColor(c.Color)
}

func (c *ColoredBox) CreateElement() Element {
	return NewRenderObjectElement(c)
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

func NewRenderObjectElement(w RenderObjectWidget) *SimpleRenderObjectElement {
	el := &SimpleRenderObjectElement{forgottenChildren: make(map[Element]struct{})}
	el.widget = w
	return el
}

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
	return NewRenderObjectElement(box)
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
	return NewRenderObjectElement(w)
}

type Opacity struct {
	Opacity float32
	Child   Widget
}

// CreateRenderObject implements RenderObjectWidget.
func (o *Opacity) CreateRenderObject(ctx BuildContext) render.Object {
	obj := &render.Opacity{}
	obj.SetOpacity(o.Opacity)
	return obj
}

// UpdateRenderObject implements RenderObjectWidget.
func (o *Opacity) UpdateRenderObject(ctx BuildContext, obj render.Object) {
	obj.(*render.Opacity).SetOpacity(o.Opacity)
}

// CreateElement implements Widget.
func (o *Opacity) CreateElement() Element {
	return NewRenderObjectElement(o)
}

type AnimatedOpacity struct {
	Opacity float32 `gutter:"animated"`
	Child   Widget

	Duration time.Duration
	Curve    func(t float64) float64
}

// CreateState implements StatefulWidget.
func (a *AnimatedOpacity) CreateState() State[*AnimatedOpacity] {
	s := &animatedOpacityState{}
	s.AnimatedProperty = makeAnimatedProperty(s, animation.Lerp[float32])
	return s
}

// CreateElement implements Widget.
func (a *AnimatedOpacity) CreateElement() Element {
	return NewInteriorElement(a)
}

type animatedOpacityState struct {
	StateHandle[*AnimatedOpacity]

	AnimatedProperty[float32, *AnimatedOpacity, *animatedOpacityState]
}

// Build implements State.
func (a *animatedOpacityState) Build() Widget {
	return &Opacity{
		Opacity: a.AnimatedProperty.Value,
		Child:   a.Widget.Child,
	}
}

func makeAnimatedProperty[T any, W Widget, S State[W]](state S, compute func(start, end T, t float64) T) AnimatedProperty[T, W, S] {
	return AnimatedProperty[T, W, S]{
		state:   state,
		compute: compute,
	}
}

type AnimatedProperty[T any, W Widget, S State[W]] struct {
	state   S
	compute func(start, end T, t float64) T

	field     int
	animation animation.Animation[T]
	Value     T

	// This caches the method value. If we didn't cache it, Go would create it anew every time we pass it to
	// AddNextFrameCallback, causing an allocation on every animation frame. The field gets set at state
	// initialization time, i.e. in the Transition method.
	updateAnimationFn func(time.Time)
}

func (p *AnimatedProperty[T, W, S]) Transition(t StateTransition[W]) {
	switch t.Kind {
	case StateInitializing:
		p.updateAnimationFn = p.updateAnimation

		w := reflect.ValueOf(p.state.GetStateHandle().Widget).Elem()
		wt := w.Type()
		for i, n := wt.NumField(), 0; i < n; i++ {
			if wt.Field(i).Tag.Get("gutter") == "animated" {
				p.field = i
				break
			}
		}

		p.Value = w.Field(p.field).Interface().(T)
		p.animation.Curve = curveOrDefault(w.FieldByName("Curve").Interface().(func(float64) float64))
		p.animation.Compute = p.compute
	case StateUpdatedWidget:
		sh := p.state.GetStateHandle()
		w := reflect.ValueOf(sh.Widget).Elem()
		if !w.Field(p.field).Equal(reflect.ValueOf(t.OldWidget).Elem().Field(p.field)) {
			// XXX this should use the event's now, not time.Now
			p.animation.Start(time.Now(), w.FieldByName("Duration").Interface().(time.Duration), p.Value, w.Field(p.field).Interface().(T))
			p.updateAnimation(time.Now())
			MarkNeedsBuild(sh.Element)
		}
	}
}

func (p *AnimatedProperty[T, W, S]) updateAnimation(now time.Time) {
	v, done := p.animation.Evaluate(now)
	p.Value = v

	if !done {
		// XXX this chain of fields is ridiculous
		p.state.GetStateHandle().Element.Handle().BuildOwner.PipelineOwner.AddNextFrameCallback(p.updateAnimationFn)
	}
	MarkNeedsBuild(p.state.GetStateHandle().Element)
}

func curveOrDefault(curve func(float64) float64) func(float64) float64 {
	if curve != nil {
		return curve
	} else {
		return animation.EaseInSine
	}
}

type KeyedSubtree struct {
	Key   any
	Child Widget
}

func (k *KeyedSubtree) GetKey() any {
	return k.Key
}

// CreateElement implements Widget.
func (k *KeyedSubtree) CreateElement() Element {
	return NewProxyElement(k)
}
