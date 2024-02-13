package animation

import (
	"math"
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

func EaseInSine(t float64) float64 {
	return 1 - math.Cos((t*math.Pi)/2)
}

func EaseOutBounce(t float64) float64 {
	const n1 = 7.5625
	const d1 = 2.75

	if t < 1.0/d1 {
		return n1 * t * t
	} else if t < 2.0/d1 {
		t -= 1.5 / d1
		return n1*t*t + 0.75
	} else if t < 2.5/d1 {
		t -= 2.25 / d1
		return n1*t*t + 0.9375
	} else {
		t -= 2.625 / d1
		return n1*t*t + 0.984375
	}
}

func EaseInBounce(t float64) float64 {
	return 1 - EaseOutBounce(1-t)
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
