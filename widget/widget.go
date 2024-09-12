// SPDX-FileCopyrightText: 2014 The Flutter Authors. All rights reserved.
// SPDX-FileCopyrightText: 2024 Dominik Honnef and contributors
//
// SPDX-License-Identifier: MIT AND BSD-3-Clause

package widget

import (
	"time"

	"honnef.co/go/color"
	"honnef.co/go/curve"
	"honnef.co/go/gutter/animation"
	"honnef.co/go/gutter/base"
	"honnef.co/go/gutter/debug"
	"honnef.co/go/gutter/io/pointer"
	"honnef.co/go/gutter/maybe"
	"honnef.co/go/gutter/render"
	"honnef.co/go/gutter/wsi"
	"honnef.co/go/jello"
	"honnef.co/go/jello/gfx"
)

var _ KeyedWidget = (*KeyedSubtree)(nil)

var _ ParentDataWidget = (*Flexible)(nil)

var _ RenderObjectWidget = (*Align)(nil)
var _ RenderObjectWidget = (*ColoredBox)(nil)
var _ RenderObjectWidget = (*FadeTransition)(nil)
var _ RenderObjectWidget = (*Flex)(nil)
var _ RenderObjectWidget = (*FittedBox)(nil)
var _ RenderObjectWidget = (*LottieFrame)(nil)
var _ RenderObjectWidget = (*Opacity)(nil)
var _ RenderObjectWidget = (*Padding)(nil)
var _ RenderObjectWidget = (*PointerRegion)(nil)
var _ RenderObjectWidget = (*SizedBox)(nil)

var _ StatefulWidget[*AnimatedOpacity] = (*AnimatedOpacity)(nil)
var _ StatefulWidget[*AnimatedPadding] = (*AnimatedPadding)(nil)
var _ StatefulWidget[*AnimatedAlign] = (*AnimatedAlign)(nil)
var _ StatefulWidget[*ListenableBuilder] = (*ListenableBuilder)(nil)
var _ StatefulWidget[*Lottie] = (*Lottie)(nil)
var _ StatefulWidget[*ValueListenableBuilder[int]] = (*ValueListenableBuilder[int])(nil)

var _ StatelessWidget = (*Builder)(nil)

var _ Widget = (*MediaQuery)(nil)
var _ Widget = (*Row)(nil)
var _ Widget = (*Column)(nil)

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
	Padding render.Inset
	Child   Widget

	Duration time.Duration
	Curve    animation.Curve
}

type paddingAnimations struct {
	Padding animation.Animation[render.Inset]
}

func (a *AnimatedPadding) CreateElement() Element {
	return NewInteriorElement(a)
}

func (a *AnimatedPadding) CreateState() State[*AnimatedPadding] {
	return NewAutomaticAnimatedState[paddingAnimations](
		map[string]any{
			"Padding": NewAnimatedField(render.LerpInset),
		},
		func(ctx BuildContext, s State[*AnimatedPadding], anims *paddingAnimations) Widget {
			return &Padding{
				Padding: anims.Padding.Value(),
				Child:   s.GetStateHandle().Widget.Child,
			}
		},
		true,
	)
}

type ColoredBox struct {
	Color color.Color
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
	color color.Color
}

// PerformLayout implements render.Object.
func (c *renderColoredBox) PerformLayout() (size curve.Size) {
	if c.Child == nil {
		return c.Constraints().Min
	}
	return render.Layout(c.Child, c.Constraints(), true)
}

func (c *renderColoredBox) PerformPaint(p *render.Painter, scene *jello.Scene) {
	sz := c.Size()
	if sz != curve.Sz(0, 0) {
		scene.Fill(
			gfx.NonZero,
			curve.Identity,
			gfx.SolidBrush{Color: c.color},
			curve.Identity,
			curve.NewRectFromOrigin(curve.Pt(0, 0), sz).Path(0.1),
		)
	}
	if c.Child != nil {
		scene.Append(p.Paint(c.Child), curve.Identity)
	}
}

func (r *renderColoredBox) setColor(c color.Color) {
	if r.color != c {
		r.color = c
		render.MarkNeedsPaint(r)
	}
}

func NewRenderObjectElement(w RenderObjectWidget) *SimpleRenderObjectElement {
	el := &SimpleRenderObjectElement{}
	el.widget = w
	return el
}

type SizedBox struct {
	Width, Height float64
	Child         Widget
}

// CreateRenderObject implements RenderObjectWidget.
func (box *SizedBox) CreateRenderObject(ctx BuildContext) render.Object {
	obj := &render.Constrained{}
	cs := render.Constraints{Min: curve.Sz(box.Width, box.Height), Max: curve.Sz(box.Width, box.Height)}
	obj.SetExtraConstraints(cs)
	return obj
}

// UpdateRenderObject implements RenderObjectWidget.
func (box *SizedBox) UpdateRenderObject(ctx BuildContext, obj render.Object) {
	cs := render.Constraints{Min: curve.Sz(box.Width, box.Height), Max: curve.Sz(box.Width, box.Height)}
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
	Opacity float32
	Child   Widget

	Duration time.Duration
	Curve    animation.Curve
}

type opacityAnimations struct {
	Opacity animation.Animation[float32]
}

// CreateState implements StatefulWidget.
func (a *AnimatedOpacity) CreateState() State[*AnimatedOpacity] {
	return NewAutomaticAnimatedState[opacityAnimations](
		map[string]any{
			"Opacity": NewAnimatedField(animation.Lerp[float32]),
		},
		func(ctx BuildContext, s State[*AnimatedOpacity], anims *opacityAnimations) Widget {
			return &FadeTransition{
				Opacity: anims.Opacity,
				Child:   s.GetStateHandle().Widget.Child,
			}
		},
		false,
	)
}

// CreateElement implements Widget.
func (a *AnimatedOpacity) CreateElement() Element {
	return NewInteriorElement(a)
}

type FadeTransition struct {
	Opacity animation.Animation[float32]
	Child   Widget
}

// CreateElement implements Widget.
func (f *FadeTransition) CreateElement() Element {
	return NewRenderObjectElement(f)
}

func (f *FadeTransition) CreateRenderObject(ctx BuildContext) render.Object {
	obj := &render.AnimatedOpacity{}
	obj.SetOpacity(f.Opacity)
	return obj
}

func (f *FadeTransition) UpdateRenderObject(ctx BuildContext, obj render.Object) {
	obj_ := obj.(*render.AnimatedOpacity)
	obj_.SetOpacity(f.Opacity)
}

func curveOrDefault(curve animation.Curve) animation.Curve {
	if curve != nil {
		return curve
	} else {
		return animation.CurveInSine
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

type Builder struct {
	Child   Widget
	Builder func(ctx BuildContext, child Widget) Widget
}

// Build implements StatelessWidget.
func (b *Builder) Build(ctx BuildContext) Widget {
	return b.Builder(ctx, b.Child)
}

// CreateElement implements StatelessWidget.
func (b *Builder) CreateElement() Element {
	return NewInteriorElement(b)
}

type CallbackEvent func()

func (CallbackEvent) ImplementsEvent() {}

type FittedBox struct {
	Fit render.BoxFit
	// TODO(dh): add alignment option
	Clip  bool
	Child Widget
}

// CreateElement implements RenderObjectWidget.
func (f *FittedBox) CreateElement() Element {
	return NewRenderObjectElement(f)
}

// CreateRenderObject implements RenderObjectWidget.
func (f *FittedBox) CreateRenderObject(ctx BuildContext) render.Object {
	obj := new(render.FittedBox)
	f.UpdateRenderObject(ctx, obj)
	return obj
}

// UpdateRenderObject implements RenderObjectWidget.
func (f *FittedBox) UpdateRenderObject(ctx BuildContext, obj render.Object) {
	obj_ := obj.(*render.FittedBox)
	obj_.SetFit(f.Fit)
	obj_.SetClip(f.Clip)
}

type ListenableBuilder struct {
	Listenable base.Listenable
	Builder    func(ctx BuildContext, child Widget) Widget
	Child      Widget
}

// CreateElement implements Widget.
func (b *ListenableBuilder) CreateElement() Element {
	return NewInteriorElement(b)
}

func (b *ListenableBuilder) CreateState() State[*ListenableBuilder] {
	return &listenableBuilderState{}
}

type listenableBuilderState struct {
	StateHandle[*ListenableBuilder]

	listener base.Listener
}

// Transition implements State.
func (a *listenableBuilderState) Transition(t StateTransition[*ListenableBuilder]) {
	switch t.Kind {
	case StateInitializing:
		a.listener = a.Widget.Listenable.AddListener(a.handleChange)
	case StateUpdatedWidget:
		if a.Widget.Listenable != t.OldWidget.Listenable {
			t.OldWidget.Listenable.RemoveListener(a.listener)
			a.listener = a.Widget.Listenable.AddListener(a.handleChange)
		}
	case StateDisposing:
		a.Widget.Listenable.RemoveListener(a.listener)
	}
}

// Build implements State.
func (a *listenableBuilderState) Build(ctx BuildContext) Widget {
	return a.Widget.Builder(ctx, a.Widget.Child)
}

func (a *listenableBuilderState) handleChange() {
	MarkNeedsBuild(a.Element)
}

type ValueListenableBuilder[T any] struct {
	ValueListenable base.ValueListenable[T]
	Builder         func(ctx BuildContext, v maybe.Option[T], child Widget) Widget
	Child           Widget
}

// CreateState implements StatefulWidget.
func (v *ValueListenableBuilder[T]) CreateState() State[*ValueListenableBuilder[T]] {
	return &valueListenableBuilderState[T]{}
}

// CreateElement implements Widget.
func (v *ValueListenableBuilder[T]) CreateElement() Element {
	return NewInteriorElement(v)
}

type valueListenableBuilderState[T any] struct {
	StateHandle[*ValueListenableBuilder[T]]

	listener base.Listener
	value    maybe.Option[T]
}

// Build implements State.
func (v *valueListenableBuilderState[T]) Build(ctx BuildContext) Widget {
	return v.Widget.Builder(ctx, v.value, v.Widget.Child)
}

// Transition implements State.
func (v *valueListenableBuilderState[T]) Transition(t StateTransition[*ValueListenableBuilder[T]]) {
	switch t.Kind {
	case StateInitializing:
		v.value = v.Widget.ValueListenable.Value()
		v.listener = v.Widget.ValueListenable.AddListener(v.valueChanged)
	case StateUpdatedWidget:
		if t.OldWidget.ValueListenable != v.Widget.ValueListenable {
			t.OldWidget.ValueListenable.RemoveListener(v.listener)
			v.value = v.Widget.ValueListenable.Value()
			v.listener = v.Widget.ValueListenable.AddListener(v.valueChanged)
		}
	case StateDisposing:
		v.Widget.ValueListenable.RemoveListener(v.listener)
	}
}

func (v *valueListenableBuilderState[T]) valueChanged() {
	v.value = v.Widget.ValueListenable.Value()
	MarkNeedsBuild(v.Element)
}

var _ base.ValueListenable[int] = (*ChannelListener[int])(nil)

type ChannelListener[T any] struct {
	base.PlainListenable
	ch        <-chan T
	emitEvent func(ev wsi.Event)
	value     maybe.Option[T]
	g         chan struct{}
}

func NewChannelListener[T any](ch <-chan T, emitEvent func(ev wsi.Event)) *ChannelListener[T] {
	l := &ChannelListener[T]{
		ch:        ch,
		emitEvent: emitEvent,
	}
	l.startGoroutine()
	return l
}

func (l *ChannelListener[T]) Value() maybe.Option[T] { return l.value }
func (l *ChannelListener[T]) Dispose()               { close(l.g) }

func (l *ChannelListener[T]) startGoroutine() {
	debug.Assert(l.g == nil)
	l.g = make(chan struct{})
	debug.Assert(l.ch != nil)
	go func() {
		for {
			select {
			case <-l.g:
				return
			case v, ok := <-l.ch:
				if !ok {
					return
				}
				l.emitEvent(CallbackEvent(func() {
					l.value = maybe.Some(v)
					l.NotifyListeners()
				}))
			}
		}
	}()
}

type Align struct {
	Alignment    render.Alignment
	WidthFactor  maybe.Option[float64]
	HeightFactor maybe.Option[float64]
	Child        Widget
}

// CreateElement implements RenderObjectWidget.
func (a *Align) CreateElement() Element {
	return NewRenderObjectElement(a)
}

// CreateRenderObject implements RenderObjectWidget.
func (a *Align) CreateRenderObject(ctx BuildContext) render.Object {
	obj := &render.PositionedBox{}
	a.UpdateRenderObject(ctx, obj)
	return obj
}

// UpdateRenderObject implements RenderObjectWidget.
func (a *Align) UpdateRenderObject(ctx BuildContext, obj render.Object) {
	obj_ := obj.(*render.PositionedBox)
	obj_.SetAlignment(a.Alignment)
	obj_.SetWidthFactor(a.WidthFactor)
	obj_.SetHeightFactor(a.HeightFactor)
}

type AnimatedAlign struct {
	Alignment    render.Alignment
	WidthFactor  maybe.Option[float64]
	HeightFactor maybe.Option[float64]
	Child        Widget

	Duration time.Duration
	Curve    animation.Curve
}

type alignAnimations struct {
	Alignment    animation.Animation[render.Alignment]
	WidthFactor  animation.Animation[maybe.Option[float64]]
	HeightFactor animation.Animation[maybe.Option[float64]]
}

func (a *AnimatedAlign) CreateElement() Element {
	return NewInteriorElement(a)
}

func (a *AnimatedAlign) CreateState() State[*AnimatedAlign] {
	return NewAutomaticAnimatedState[alignAnimations](
		map[string]any{
			"Alignment":    NewAnimatedField(render.LerpAlignment),
			"WidthFactor":  NewAnimatedField(animation.MaybeLerp[float64]),
			"HeightFactor": NewAnimatedField(animation.MaybeLerp[float64]),
		},
		func(ctx BuildContext, s State[*AnimatedAlign], anims *alignAnimations) Widget {
			return &Align{
				Alignment:    anims.Alignment.Value(),
				WidthFactor:  anims.WidthFactor.Value(),
				HeightFactor: anims.HeightFactor.Value(),
				Child:        s.GetStateHandle().Widget.Child,
			}
		},
		true,
	)
}
