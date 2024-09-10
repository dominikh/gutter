// SPDX-FileCopyrightText: 2024 Dominik Honnef and contributors
//
// SPDX-License-Identifier: MIT

package widget

import (
	"time"

	"honnef.co/go/gutter/animation"
	"honnef.co/go/gutter/lottie/lottie_model"
	"honnef.co/go/gutter/render"
)

var _ Widget = (*Lottie)(nil)
var _ StatefulWidget[*Lottie] = (*Lottie)(nil)

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
	// TODO(dh): support specifying first and last frame to render

	// TODO(dh): allow control over the animation, cf. Flutter's animation controller

	// TODO(dh): we also want to be able to display a certain frame, and to
	// animate to that frame, but also to not animate to that frame. Odds are we
	// want to expose some access to the animation controller. and we probably
	// want a lottie-specific animation controller, so the user can use frames
	// instead of t ∈ [0, 1]
}

// CreateState implements StatefulWidget.
func (l *Lottie) CreateState() State[*Lottie] {
	if l.Composition == nil {
		return &lottieState{}
	}

	// XXX guard against malformed frame numbers and frame rates
	numFrames := float64(l.Composition.LastFrame - l.Composition.FirstFrame)
	fr := l.Composition.Framerate
	duration := time.Duration((numFrames / fr) * float64(time.Second))
	return &lottieState{
		anim: animation.Animation[float64]{
			// XXX instead of time.Now we want a time that is shared between all
			// code running during this frame.
			StartTime:  time.Now(),
			EndTime:    time.Now().Add(duration),
			StartValue: l.Composition.FirstFrame,
			EndValue:   l.Composition.LastFrame - 1,
			Repeat:     true,
			Compute:    animation.Lerp[float64],
			Curve:      animation.CurveIdentity,
		},
	}
}

// CreateElement implements Widget.
func (l *Lottie) CreateElement() Element {
	return NewInteriorElement(l)
}

type lottieState struct {
	StateHandle[*Lottie]

	anim animation.Animation[float64]
}

func (l *lottieState) updateAnimation(now time.Time) {
	// OPT(dh): cache method value
	// XXX this duplicates code with AnimatedProperty
	l.Element.Handle().BuildOwner.AddNextFrameCallback(l.updateAnimation)
	MarkNeedsBuild(l.Element)
}

// Build implements State.
func (l *lottieState) Build(ctx BuildContext) Widget {
	// XXX we don't want time.Now
	frame, _ := l.anim.Evaluate(time.Now())
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
}

// Transition implements State.
func (l *lottieState) Transition(t StateTransition[*Lottie]) {
	switch t.Kind {
	case StateInitializing:
		// XXX we don't want to use time.Now
		l.updateAnimation(time.Now())
	}
}
