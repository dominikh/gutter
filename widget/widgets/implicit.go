// SPDX-FileCopyrightText: 2014 The Flutter Authors. All rights reserved.
// SPDX-FileCopyrightText: 2024 Dominik Honnef and contributors
//
// SPDX-License-Identifier: MIT AND BSD-3-Clause

package widgets

import (
	"fmt"
	"reflect"
	"time"

	"honnef.co/go/gutter/animation"
	"honnef.co/go/gutter/widget"
)

type AnimatedState[W widget.Widget] interface {
	// XXX does this have to be exported?

	widget.State[W]
	// Tweens iterates over all tweens, yielding pairs of
	// (tween *animation.Tween[T], target T)
	Tweens(yield func(any, any) bool)
}

type animatedStateHelper[W widget.Widget, S AnimatedState[W]] struct {
	controller     *animation.Controller
	animation      *animation.CurvedAnimation
	statusListener animation.StatusListener
}

func (is *animatedStateHelper[W, S]) RebuildOnAnimation(s S) {
	is.controller.AddListener(func() {
		widget.MarkNeedsBuild(s.GetStateHandle().Element)
	})
}

func (is *animatedStateHelper[W, S]) updateTweens(s S) {
	for tween, targetValue := range s.Tweens {
		// rtype(rvtween) == animation.Tween[T].
		rvtween := reflect.ValueOf(tween)
		// rtype(newBegin) == T
		newBegin := rvtween.MethodByName("Evaluate").
			Call([]reflect.Value{reflect.ValueOf(is.animation.Value())})[0]
		rvtween.Elem().FieldByName("Start").Set(newBegin)
		rvtween.Elem().FieldByName("End").Set(reflect.ValueOf(targetValue))
	}
}

func (is *animatedStateHelper[W, S]) Transition(s S, t widget.StateTransition[W]) (updatedTweens bool) {
	switch t.Kind {
	case widget.StateInitializing:
		is.controller = animation.NewController(s.GetStateHandle().BuildOwner())
		widget := s.GetStateHandle().Widget
		if f := reflect.ValueOf(widget).Elem().FieldByName("Curve"); f.IsValid() {
			ease, _ := f.Interface().(animation.Curve)
			if ease == nil {
				ease = animation.CurveIdentity
			}
			is.animation = animation.NewCurvedAnimation(is.controller, ease, nil)
		} else {
			is.animation = animation.NewCurvedAnimation(is.controller, animation.CurveIdentity, nil)
		}

		is.statusListener = is.controller.AddStatusListener(func(status animation.Status) {
			switch status {
			case animation.StatusCompleted:
				// TODO(dh): call an OnEnd callback stored in the widget
			case animation.StatusDismissed,
				animation.StatusForward,
				animation.StatusReverse:
				// nothing to do
			default:
				panic(fmt.Sprintf("internal error: unhandled status %v", status))
			}
		})

		// Immediately apply widget's starting values
		is.updateTweens(s)
		is.controller.SetValue(1)
		is.controller.Forward()
		return true
	case widget.StateUpdatedWidget:
		widget := s.GetStateHandle().Widget
		rwidget := reflect.ValueOf(widget).Elem()

		if easef := rwidget.FieldByName("Curve"); easef.IsValid() {
			ease, _ := easef.Interface().(animation.Curve)
			if ease == nil {
				ease = animation.CurveIdentity
			}
			oldEase, _ := reflect.ValueOf(t.OldWidget).Elem().FieldByName("Curve").Interface().(animation.Curve)
			if oldEase == nil {
				oldEase = animation.CurveIdentity
			}
			if ease != oldEase {
				is.animation.Dispose()
				is.animation = animation.NewCurvedAnimation(is.controller, ease, nil)
			}
		}

		f := rwidget.FieldByName("Duration")
		if !f.IsValid() {
			panic(fmt.Sprintf("%T does not have a Duration field", widget))
		}
		if d, ok := f.Interface().(time.Duration); ok {
			is.controller.Duration = d
		} else {
			panic(fmt.Sprintf("field %T.Duration has wrong type %T, need time.Duration",
				widget, f.Interface()))
		}
		var startAnim bool
		for tween, targetValue := range s.Tweens {
			if reflect.ValueOf(tween).Elem().FieldByName("End").Interface() != targetValue {
				startAnim = true
				break
			}
		}
		if startAnim {
			is.updateTweens(s)
			is.controller.SetValue(0)
			is.controller.Forward()
			return true
		}
		return false
	case widget.StateChangedDependencies:
	case widget.StateDeactivating:
	case widget.StateActivating:
	case widget.StateDisposing:
		is.animation.Dispose()
		is.controller.Dispose()
	}
	return false
}

func NewAnimatedField[T any](lerp func(start, end T, t float64) T) *animatedField[T] {
	return &animatedField[T]{
		Tween:   &animation.Tween[T]{Compute: lerp},
		Animate: animation.Animate[T],
	}
}

type animatedField[T any] struct {
	Tween   *animation.Tween[T]
	Animate func(
		parent animation.Animation[float64],
		animatable animation.Animatable[T],
	) animation.Animation[T]
}

// NewAutomaticAnimatedState can be used to implement implicitly animated
// widgets, by creating [widget.State] that automatically animate select fields
// of widgets.
//
// The type parameter W is the concrete widget we're implementing that we want
// to be implicitly animated. For example, this package uses this function to
// implement [AnimatedAlign].
//
// The type parameter Anims is a struct with one field per animated field in W,
// where each field has the type [animation.Animation], instantiated with the
// corresponding field type in W. A value of type *Anims is passed to the build
// function, which allows type-safe access to the animated fields.
//
// The fields parameter specifies which fields in W to animate and which lerp
// functions to use. Values in the map should all be the return values from
// NewAnimatedField.
//
// The function provided via the build parameter gets called whenever the widget
// needs to be rebuilt and takes the place of the [widget.State.Build] method.
// Commonly, build functions return the equivalent unanimated widget, using the
// current values of the animated fields. For example, [AnimatedAlign] returns
// [Align] whenever the animation advances. In this case, the rebuildOnAnimation
// parameter must be true.
//
// rebuildOnAnimation controls whether the build function gets called every time
// the animation advances. This has to be true for animated widgets that work by
// returning unanimated widgets (as described previously). However, some
// implicitly animated widgets may choose to build to explicitly animated
// widgets instead--for example, [AnimatedOpacity] builds to a [FadeTransition].
// In that case, the animation directly drives a render object and doesn't
// require widgets to be rebuilt.
//
// Implicitly animated widgets are required to have a field called "Duration" of
// type "time.Duration". This field is used to configure the duration of
// animations.
//
// Implicitly animated widgets can optionally have a field called "Curve" of
// type "animation.Curve" to allow configuring the animation's curve (also known
// as the easing function).
//
// Example:
//
//	type AnimatedAlign struct {
//		Alignment    render.Alignment
//		WidthFactor  maybe.Option[float64]
//		HeightFactor maybe.Option[float64]
//		Child        widget.Widget
//
//		Duration time.Duration
//		Curve    animation.Curve
//	}
//
//	type alignAnimations struct {
//		Alignment    animation.Animation[render.Alignment]
//		WidthFactor  animation.Animation[maybe.Option[float64]]
//		HeightFactor animation.Animation[maybe.Option[float64]]
//	}
//
//	func (a *AnimatedAlign) CreateElement() widget.Element {
//		return widget.NewInteriorElement(a)
//	}
//
//	func (a *AnimatedAlign) CreateState() widget.State[*AnimatedAlign] {
//		return NewAutomaticAnimatedState(
//			map[string]any{
//				"Alignment":    NewAnimatedField(render.LerpAlignment),
//				"WidthFactor":  NewAnimatedField(animation.MaybeLerp[float64]),
//				"HeightFactor": NewAnimatedField(animation.MaybeLerp[float64]),
//			},
//			func(ctx widget.BuildContext, s widget.State[*AnimatedAlign], anims *alignAnimations) widget.Widget {
//				return &Align{
//					Alignment:    anims.Alignment.Value(),
//					WidthFactor:  anims.WidthFactor.Value(),
//					HeightFactor: anims.HeightFactor.Value(),
//					Child:        s.GetStateHandle().Widget.Child,
//				}
//			},
//			true,
//		)
//	}
func NewAutomaticAnimatedState[Anims any, W widget.Widget](
	fields map[string]any,
	build func(ctx widget.BuildContext, s widget.State[W], anims *Anims) widget.Widget,
	rebuildOnAnimation bool,
) widget.State[W] {
	return &automaticAnimatedState[W, *Anims]{
		fields:             fields,
		animations:         new(Anims),
		builder:            build,
		rebuildOnAnimation: rebuildOnAnimation,
	}
}

type automaticAnimatedState[W widget.Widget, Anims any] struct {
	widget.StateHandle[W]

	animState animatedStateHelper[W, *automaticAnimatedState[W, Anims]]

	fields             map[string]any // map from widget field to animatedField[T]
	animations         Anims
	builder            func(ctx widget.BuildContext, state widget.State[W], anims Anims) widget.Widget
	rebuildOnAnimation bool
}

// Tweens implements AnimatedState.
func (m *automaticAnimatedState[W, Anims]) Tweens(yield func(any, any) bool) {
	// For every implicitly animated field, we yield the current animation.Tween
	// and the new target value it should be updated to. This is used by
	// animatedStateHelper.updateTweens to evaluate and update the tweens.
	rvwidget := reflect.ValueOf(m.Widget).Elem()
	for field, afield := range m.fields {
		// rtype(tween) == *animatedField[T]
		tween := reflect.ValueOf(afield).Elem().FieldByName("Tween").Interface()
		if !yield(tween, rvwidget.FieldByName(field).Interface()) {
			break
		}
	}
}

// Transition implements widget.State.
func (m *automaticAnimatedState[W, Anims]) Transition(t widget.StateTransition[W]) {
	if m.animState.Transition(m, t) {
		// rtype(rvanims) == Anims
		rvanims := reflect.ValueOf(m.animations).Elem()
		// rtype(rvanim) == *animation.CurvedAnimation
		rvanim := reflect.ValueOf(m.animState.animation)
		for field, afield := range m.fields {
			// rtype(rvafield) == *animatedField[T]
			rvafield := reflect.ValueOf(afield).Elem()
			// rtype(tween) == *animation.Tween[T]
			tween := rvafield.FieldByName("Tween")
			// rtype(animate) ==
			//     func(animation.Animation[float64], animation.Animatable[T]) animation.Animation[T]
			animate := rvafield.FieldByName("Animate")
			// rtype(anim) == animation.Animation[T]
			anim := animate.Call([]reflect.Value{
				rvanim,
				tween,
			})[0]
			rvanims.FieldByName(field).Set(anim)
		}
	}
	if m.rebuildOnAnimation {
		m.animState.RebuildOnAnimation(m)
	}
}

// Transition implements widget.State.
func (m *automaticAnimatedState[W, Anims]) Build(ctx widget.BuildContext) widget.Widget {
	return m.builder(ctx, m, m.animations)
}
