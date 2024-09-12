// SPDX-FileCopyrightText: 2024 Dominik Honnef and contributors
//
// SPDX-License-Identifier: MIT

package animation

import (
	"fmt"
	"math"
	"sort"

	"honnef.co/go/color"
	"honnef.co/go/curve"
	"honnef.co/go/gutter/base"
	"honnef.co/go/gutter/maybe"
	"honnef.co/go/jello/gfx"
	"honnef.co/go/jello/jmath"

	"golang.org/x/exp/constraints"
)

type Animation[T any] interface {
	base.Listenable
	StatusListenable

	Animating() bool
	Status() Status
	Value() T
}

type Animatable[T any] interface {
	Evaluate(t float64) T
}

type Lerper[T any] interface {
	Lerp(other T, t float64) T
}

func Animate[T any](parent Animation[float64], animatable Animatable[T]) Animation[T] {
	return &animatedEvaluation[T]{
		parent:     parent,
		animatable: animatable,
	}
}

func Chain[T any](parent Animatable[float64], animatable Animatable[T]) Animatable[T] {
	return &chainedEvaluation[T]{
		parent:     parent,
		animatable: animatable,
	}
}

var _ Animation[float64] = (*CurvedAnimation)(nil)

type CurvedAnimation struct {
	Animation[float64]
	Curve        Curve
	ReverseCurve Curve

	curveDirection maybe.Option[Status]
	statusListener StatusListener
}

func NewCurvedAnimation(parent Animation[float64], curve, reverseCurve Curve) *CurvedAnimation {
	obj := &CurvedAnimation{
		Animation:    parent,
		Curve:        curve,
		ReverseCurve: reverseCurve,
	}
	obj.updateCurveDirection(parent.Status())
	parent.AddStatusListener(obj.updateCurveDirection)
	return obj
}

func (e *CurvedAnimation) effectiveCurve() Curve {
	if e.ReverseCurve == nil {
		return e.Curve
	}
	if e.curveDirection.UnwrapOr(e.Animation.Status()) != StatusReverse {
		return e.Curve
	}
	return e.ReverseCurve
}

func (e *CurvedAnimation) updateCurveDirection(status Status) {
	switch status {
	case StatusDismissed:
	case StatusCompleted:
		e.curveDirection.Clear()
	case StatusForward:
		e.curveDirection = maybe.Some(StatusForward)
	case StatusReverse:
		e.curveDirection = maybe.Some(StatusReverse)
	default:
		panic(fmt.Sprintf("internal error: unhandled status %v", status))
	}
}

func (e *CurvedAnimation) Dispose() {
	e.Animation.RemoveStatusListener(e.statusListener)
}

// Value implements Animation.
func (e *CurvedAnimation) Value() float64 {
	ease := e.effectiveCurve()
	t := e.Animation.Value()
	if ease == nil {
		return t
	}
	if t == 0 || t == 1 {
		return t
	}
	return ease.Transform(t)
}

type chainedEvaluation[T any] struct {
	parent     Animatable[float64]
	animatable Animatable[T]
}

func (c *chainedEvaluation[T]) Evaluate(t float64) T {
	return c.animatable.Evaluate(c.parent.Evaluate(t))
}

type animatedEvaluation[T any] struct {
	parent     Animation[float64]
	animatable Animatable[T]
}

// AddListener implements Animation.
func (a *animatedEvaluation[T]) AddListener(cb func()) base.Listener {
	return a.parent.AddListener(cb)
}

// AddStatusListener implements Animation.
func (a *animatedEvaluation[T]) AddStatusListener(cb func(status Status)) StatusListener {
	return a.parent.AddStatusListener(cb)
}

// Animating implements Animation.
func (a *animatedEvaluation[T]) Animating() bool {
	return a.parent.Animating()
}

// RemoveListener implements Animation.
func (a *animatedEvaluation[T]) RemoveListener(l base.Listener) {
	a.parent.RemoveListener(l)
}

// RemoveStatusListener implements Animation.
func (a *animatedEvaluation[T]) RemoveStatusListener(l StatusListener) {
	a.parent.RemoveStatusListener(l)
}

func (a *animatedEvaluation[T]) ClearListeners() {
	a.parent.ClearListeners()
}

func (a *animatedEvaluation[T]) ClearStatusListeners() {
	a.parent.ClearStatusListeners()
}

// Status implements Animation.
func (a *animatedEvaluation[T]) Status() Status {
	return a.parent.Status()
}

// Value implements Animation.
func (a *animatedEvaluation[T]) Value() T {
	return a.animatable.Evaluate(a.parent.Value())
}

func Lerp[T constraints.Integer | constraints.Float](start, end T, t float64) T {
	switch t {
	case 0:
		return start
	case 1:
		return end
	default:
		return (T(float64(start) + float64(end-start)*t))
	}
}

func MaybeLerp[T constraints.Integer | constraints.Float](start, end maybe.Option[T], t float64) maybe.Option[T] {
	switch t {
	case 0:
		return start
	case 1:
		return end
	default:
		if start == end {
			return start
		}
		var zero T
		start_ := start.UnwrapOr(zero)
		end_ := start.UnwrapOr(zero)
		return maybe.Some(T(float64(start_) + float64(end_-start_)*t))
	}
}

type Tween[T any] struct {
	Start T
	End   T
	// Easing function to apply to t value passed to [Tween.Evaluate]. Optional.
	Curve   Curve
	Compute func(start, end T, progress float64) T
}

func (tween *Tween[T]) Evaluate(t float64) (v T) {
	if c := tween.Curve; c != nil {
		t = c.Transform(t)
	}
	return tween.Compute(tween.Start, tween.End, t)
}

type Keyframes[T any] struct {
	Frames []float64
	Curves []Curve
	Values []T
	// The function to use for lerping between two values of type T. If it is
	// nil then T must implement [Lerper] or be one of the built-in integer or
	// float types.
	Lerp func(T, T, float64) T
}

func (v *Keyframes[T]) Evaluate(frame float64) T {
	var def T

	v1, v2, t, ok := v.ComputeFramesAndWeight(frame)
	if !ok {
		return def
	}

	if v.Lerp == nil {
		switch v1 := any(v1).(type) {
		case Lerper[T]:
			return v1.Lerp(v2, t)
		case float64:
			return any(Lerp(v1, any(v2).(float64), t)).(T)
		case float32:
			return any(Lerp(v1, any(v2).(float32), t)).(T)
		case int64:
			return any(Lerp(v1, any(v2).(int64), t)).(T)
		case int32:
			return any(Lerp(v1, any(v2).(int32), t)).(T)
		case int16:
			return any(Lerp(v1, any(v2).(int16), t)).(T)
		case int8:
			return any(Lerp(v1, any(v2).(int8), t)).(T)
		case int:
			return any(Lerp(v1, any(v2).(int), t)).(T)
		case uint64:
			return any(Lerp(v1, any(v2).(uint64), t)).(T)
		case uint32:
			return any(Lerp(v1, any(v2).(uint32), t)).(T)
		case uint16:
			return any(Lerp(v1, any(v2).(uint16), t)).(T)
		case uint8:
			return any(Lerp(v1, any(v2).(uint8), t)).(T)
		case uint:
			return any(Lerp(v1, any(v2).(uint), t)).(T)
		case uintptr:
			return any(Lerp(v1, any(v2).(uintptr), t)).(T)
		default:
			panic(fmt.Sprintf("nil Lerp function and unsupported type %T", v1))
		}
	} else {
		return v.Lerp(v1, v2, t)
	}
}

func (kfs Keyframes[T]) ComputeFramesAndWeight(frame float64) (startValue, endValue T, t float64, ok bool) {
	if len(kfs.Frames) == 0 {
		return *new(T), *new(T), 0, false
	} else if len(kfs.Frames) == 1 {
		return kfs.Values[0], kfs.Values[0], kfs.Curves[0].Transform(1), true
	}
	idx := sort.Search(len(kfs.Frames), func(i int) bool {
		return kfs.Frames[i] >= frame
	})
	if idx > 0 && (idx >= len(kfs.Frames) || kfs.Frames[idx] != frame) {
		idx--
	}
	idx0 := min(idx, len(kfs.Frames)-1)
	idx1 := min(idx0+1, len(kfs.Frames)-1)
	t0 := kfs.Frames[idx0]
	t1 := kfs.Frames[idx1]
	easing := kfs.Curves[idx0]
	t = (frame - t0) / (t1 - t0)
	if t1 <= t0 {
		t = 0
	}
	return kfs.Values[idx0], kfs.Values[idx1], easing.Transform(jmath.Clamp(t, 0, 1)), true
}

type Transform struct {
	Anchor   Point
	Position Point
	// Rotation, in radians
	Rotation Keyframes[float64]
	Scale    Vec2
	// Skew, in radians
	Skew Keyframes[float64]
	// Skew angle, in radians
	SkewAngle Keyframes[float64]
}

func (t Transform) Evaluate(frame float64) curve.Affine {
	anchor := t.Anchor.Evaluate(frame)
	position := t.Position.Evaluate(frame)
	rotation := t.Rotation.Evaluate(frame)
	scale := t.Scale.Evaluate(frame)
	skew := t.Skew.Evaluate(frame)
	skewAngle := t.SkewAngle.Evaluate(frame)
	skewMatrix := curve.Identity
	if skew != 0.0 {
		const SKEW_LIMIT = 85.0
		skew := jmath.Clamp(-skew, -SKEW_LIMIT, SKEW_LIMIT)
		// skew = toRadians(skew)
		angle := skewAngle
		// angle := toRadians(skewAngle)
		skewMatrix = curve.Rotate(-angle).Mul(curve.Skew(math.Tan(skew), 0.0)).Mul(curve.Rotate(angle))
	}
	return curve.Translate(curve.Vec(position.X, position.Y)).
		// Mul(curve.Rotate(toRadians(rotation))).
		Mul(curve.Rotate((rotation))).
		Mul(skewMatrix).
		Mul(curve.Scale(scale.X/100.0, scale.Y/100.0)).
		Mul(curve.Translate(curve.Vec(-anchor.X, -anchor.Y)))
}

type Vec2 struct {
	X, Y Keyframes[float64]
}

func (p Vec2) Evaluate(frame float64) curve.Vec2 {
	return curve.Vec2{
		X: p.X.Evaluate(frame),
		Y: p.Y.Evaluate(frame),
	}
}

type Point struct {
	X, Y Keyframes[float64]
}

func (p Point) Evaluate(frame float64) curve.Point {
	return curve.Point{
		X: p.X.Evaluate(frame),
		Y: p.Y.Evaluate(frame),
	}
}

type Size struct {
	Width, Height Keyframes[float64]
}

func (sz Size) Evaluate(frame float64) curve.Size {
	return curve.Size{
		Width:  sz.Width.Evaluate(frame),
		Height: sz.Height.Evaluate(frame),
	}
}

type Stroke struct {
	Width      Keyframes[float64]
	Join       curve.Join
	MiterLimit maybe.Option[float64]
	Cap        curve.Cap
}

func (s Stroke) Evaluate(frame float64) curve.Stroke {
	width := s.Width.Evaluate(frame)
	stroke := curve.DefaultStroke.WithWidth(width).WithCaps(s.Cap).WithJoin(s.Join)
	if l, ok := s.MiterLimit.Get(); ok {
		stroke.MiterLimit = l
	}
	return stroke
}

type Ellipse struct {
	Position Point
	Size     Size
}

func (e Ellipse) Evaluate(frame float64) curve.Ellipse {
	pos := e.Position.Evaluate(frame)
	size := e.Size.Evaluate(frame)
	radii := curve.Vec(size.Width*0.5, size.Height*0.5)
	return curve.NewEllipse(pos, radii, 0)
}

type RoundedRect struct {
	Position     Point
	Size         Size
	CornerRadius Keyframes[float64]
}

func (r RoundedRect) Evaluate(frame float64) curve.RoundedRect {
	pos := r.Position.Evaluate(frame)
	size := r.Size.Evaluate(frame)
	radius := r.CornerRadius.Evaluate(frame)
	return curve.NewRectFromCenter(pos, size).RoundedRect(curve.RoundedRectRadii{
		TopLeft:     radius,
		TopRight:    radius,
		BottomRight: radius,
		BottomLeft:  radius,
	})
}

type ColorStop struct {
	Offset float64
	Color  color.Color
}

type ColorStops struct {
	Keyframes[[]ColorStop]
	// XXX do we need Count
	Count int
}

func (s ColorStops) Evaluate(frame float64) []gfx.ColorStop {
	v0, v1, t, ok := s.ComputeFramesAndWeight(frame)
	if !ok {
		return nil
	}

	var stops []gfx.ColorStop
	for i := range s.Count {
		j := i
		if j >= len(v0) || j >= len(v1) {
			return nil
		}
		offset := Lerp(v0[j].Offset, v1[j].Offset, t)
		r := Lerp(v0[j].Color.Values[0], v1[j].Color.Values[0], t)
		g := Lerp(v0[j].Color.Values[1], v1[j].Color.Values[1], t)
		b := Lerp(v0[j].Color.Values[2], v1[j].Color.Values[2], t)
		a := Lerp(v0[j].Color.Alpha, v1[j].Color.Alpha, t)
		stops = append(stops, gfx.ColorStop{
			Offset: float32(offset),
			Color:  color.Make(color.SRGB, r, g, b, a),
		})
	}
	return stops
}

type Gradient struct {
	IsRadial   bool
	StartPoint Point
	EndPoint   Point
	Stops      ColorStops
}

func (g Gradient) Evaluate(frame float64) gfx.Brush {
	start := g.StartPoint.Evaluate(frame)
	end := g.EndPoint.Evaluate(frame)
	stops := g.Stops.Evaluate(frame)
	if g.IsRadial {
		radius := curve.Vec2(end).Sub(curve.Vec2(start)).Hypot()
		gg := gfx.RadialGradient{
			StartCenter: start,
			EndCenter:   start,
			EndRadius:   float32(radius),
			Stops:       stops,
		}
		return gfx.GradientBrush{
			Gradient: gg,
		}
	} else {
		gg := gfx.LinearGradient{
			Start: start,
			End:   end,
			Stops: stops,
		}
		return gfx.GradientBrush{
			Gradient: gg,
		}
	}
}
