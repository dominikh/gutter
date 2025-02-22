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
	widget.State[W]
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
		rvtween := reflect.ValueOf(tween)
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

func NewAutomaticAnimatedState[Anims any, W widget.Widget](
	fields map[string]any, // map from widget field to AnimatedField
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

func (m *automaticAnimatedState[W, Anims]) Tweens(yield func(any, any) bool) {
	rvw := reflect.ValueOf(m.Widget).Elem()
	for field, afield := range m.fields {
		tween := reflect.ValueOf(afield).Elem().FieldByName("Tween").Interface()
		if !yield(tween, rvw.FieldByName(field).Interface()) {
			break
		}
	}
}

func (m *automaticAnimatedState[W, Anims]) Transition(t widget.StateTransition[W]) {
	rvanims := reflect.ValueOf(m.animations).Elem()
	if m.animState.Transition(m, t) {
		rvanim := reflect.ValueOf(m.animState.animation)
		for field, afield := range m.fields {
			rvafield := reflect.ValueOf(afield).Elem()
			tween := rvafield.FieldByName("Tween")
			animate := rvafield.FieldByName("Animate")
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

func (m *automaticAnimatedState[W, Anims]) Build(ctx widget.BuildContext) widget.Widget {
	return m.builder(ctx, m, m.animations)
}
