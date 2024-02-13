package animation

import (
	"time"

	"golang.org/x/exp/constraints"
)

func AnimationProgress(start, end, now time.Time) float64 {
	if now.Before(start) {
		return 0
	} else if now.After(end) {
		return 1
	} else {
		return float64(now.Sub(start)) / float64(end.Sub(start))
	}
}

type Tween[T any] func(start, end T, progress float64) T
type Ease func(t float64) float64

var _ Tween[int] = Lerp[int]

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

type Animation[T any] struct {
	StartTime  time.Time
	EndTime    time.Time
	StartValue T
	EndValue   T
	Compute    func(start, end T, t float64) T
	Curve      func(t float64) float64
}

func (anim *Animation[T]) Start(now time.Time, d time.Duration, start, end T) {
	*anim = Animation[T]{
		StartTime:  now,
		EndTime:    now.Add(d),
		StartValue: start,
		EndValue:   end,
		Compute:    anim.Compute,
		Curve:      anim.Curve,
	}
}

func (anim *Animation[T]) Evaluate(now time.Time) (v T, done bool) {
	t := AnimationProgress(anim.StartTime, anim.EndTime, now)
	switch t {
	case 0:
		return anim.StartValue, false
	case 1:
		return anim.EndValue, true
	default:
		t = anim.Curve(t)
		return anim.Compute(anim.StartValue, anim.EndValue, t), false
	}
}
