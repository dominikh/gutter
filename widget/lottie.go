// SPDX-FileCopyrightText: 2024 Dominik Honnef and contributors
//
// SPDX-License-Identifier: MIT

package widget

import (
	"honnef.co/go/gutter/animation"
	"honnef.co/go/gutter/lottie/lottie_model"
	"honnef.co/go/gutter/render"
)

type LottieFrame struct {
	Composition *lottie_model.Composition
	Frame       float64
}

// CreateElement implements RenderObjectWidget.
func (l *LottieFrame) CreateElement() Element {
	return NewRenderObjectElement(l)
}

// CreateRenderObject implements RenderObjectWidget.
func (l *LottieFrame) CreateRenderObject(ctx BuildContext) render.Object {
	var obj render.Lottie
	obj.SetComposition(l.Composition)
	obj.SetFrame(l.Frame)
	return &obj
}

// UpdateRenderObject implements RenderObjectWidget.
func (l *LottieFrame) UpdateRenderObject(ctx BuildContext, obj render.Object) {
	obj_ := obj.(*render.Lottie)
	obj_.SetComposition(l.Composition)
	obj_.SetFrame(l.Frame)
}

type Lottie struct {
	Composition *lottie_model.Composition
	Fit         render.BoxFit
	Width       float64
	Height      float64
	// The animation controller that should drive the animation. If nil,
	// animation will be handled implicitly.
	Controller *animation.Controller
	// TODO(dh): support specifying first and last frame to render
	Animate bool
	Repeat  bool
	Reverse bool
}

// CreateState implements StatefulWidget.
func (l *Lottie) CreateState() State[*Lottie] {
	return &lottieState{}
}

// CreateElement implements Widget.
func (l *Lottie) CreateElement() Element {
	return NewInteriorElement(l)
}

type lottieState struct {
	StateHandle[*Lottie]

	// animation controller to use when Lottie.Controller is nil.
	autoAnimation *animation.Controller
}

// Transition implements State.
func (l *lottieState) Transition(t StateTransition[*Lottie]) {
	switch t.Kind {
	case StateInitializing:
		// XXX guard against malformed frame numbers and frame rates
		l.autoAnimation = animation.NewController(l.GetStateHandle().Element.Handle().BuildOwner)
		l.autoAnimation.Duration = l.Widget.Composition.Duration()
		l.autoAnimation.LowerBound = l.Widget.Composition.FirstFrame
		l.autoAnimation.UpperBound = l.Widget.Composition.LastFrame
		l.updateAutoAnimation()
	case StateUpdatedWidget:
		if l.Widget.Composition != t.OldWidget.Composition ||
			l.Widget.Controller != t.OldWidget.Controller {
			l.autoAnimation.Duration = l.Widget.Composition.Duration()
			l.autoAnimation.LowerBound = l.Widget.Composition.FirstFrame
			l.autoAnimation.UpperBound = l.Widget.Composition.LastFrame
			l.updateAutoAnimation()
		}
		if *l.Widget != *t.OldWidget {
			MarkNeedsBuild(l.Element)
		}
	case StateDisposing:
		l.autoAnimation.Dispose()
	}
}

func (l *lottieState) updateAutoAnimation() {
	l.autoAnimation.Stop()
	if l.Widget.Animate && l.Widget.Controller == nil {
		if l.Widget.Repeat {
			l.autoAnimation.Repeat(l.Widget.Reverse, -1)
		} else {
			l.autoAnimation.Forward()
		}
	}
}

func (l *lottieState) animation() animation.Animation[float64] {
	if l.Widget.Controller != nil {
		return l.Widget.Controller
	} else {
		return l.autoAnimation
	}
}

// Build implements State.
func (l *lottieState) Build(ctx BuildContext) Widget {
	return &ListenableBuilder{
		Listenable: l.animation(),
		Builder: func(ctx BuildContext, child Widget) Widget {
			frame := l.animation().Value()
			w := l.Widget.Width
			h := l.Widget.Height
			ar := float64(l.Widget.Composition.Width) / float64(l.Widget.Composition.Height)
			if w == 0 && h == 0 {
				w = float64(l.Widget.Composition.Width)
				h = float64(l.Widget.Composition.Height)
			} else if h == 0 {
				h = w / ar
			} else if w == 0 {
				w = h * ar
			}
			return &SizedBox{
				Width:  w,
				Height: h,
				Child: &FittedBox{
					Fit:  l.Widget.Fit,
					Clip: true,
					Child: &LottieFrame{
						Composition: l.Widget.Composition,
						Frame:       frame,
					},
				},
			}
		},
	}
}
