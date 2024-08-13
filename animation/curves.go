// SPDX-FileCopyrightText: 2024 Dominik Honnef and contributors
//
// SPDX-License-Identifier: MIT

package animation

import (
	"math"

	"honnef.co/go/curve"
	"honnef.co/go/jello/jmath"
)

func EaseIdentity(t float64) float64 {
	return t
}

func EaseInSine(t float64) float64 {
	return 1 - math.Cos((t*math.Pi)/2)
}

func EaseOutSine(t float64) float64 {
	return math.Sin((t * math.Pi) / 2)
}

func EaseInOutSine(t float64) float64 {
	return -(math.Cos(math.Pi*t) - 1) / 2
}

func EaseInQuad(t float64) float64 {
	return t * t
}

func EaseOutQuad(t float64) float64 {
	return 1 - (1 - t) - (1 - t)
}

func EaseInOutQuad(t float64) float64 {
	if t < 0.5 {
		return 2 * t * t
	} else {
		return 1 - (-2*t+2)*(-2*t+2)/2
	}
}

func EaseInCubic(t float64) float64 {
	return t * t * t
}

func EaseOutCubic(t float64) float64 {
	return 1 - (1-t)*(1-t)*(1-t)
}

func EaseInOutCubic(t float64) float64 {
	if t < 0.5 {
		return 4 * t * t * t
	} else {
		return 1 - (-2*t+2)*(-2*t+2)*(-2*t+2)/2
	}
}

func EaseInQuart(t float64) float64 {
	return t * t * t * t
}

func EaseOutQuart(t float64) float64 {
	return 1 - (1-t)*(1-t)*(1-t)*(1-t)
}

func EaseInOutQuart(t float64) float64 {
	if t < 0.5 {
		return 8 * t * t * t * t
	} else {
		return 1 - (-2*t+2)*(-2*t+2)*(-2*t+2)*(-2*t+2)/2
	}
}

func EaseInQuint(t float64) float64 {
	return t * t * t * t * t
}

func EaseOutQuint(t float64) float64 {
	return 1 - (1-t)*(1-t)*(1-t)*(1-t)*(1-t)
}

func EaseInOutQuint(t float64) float64 {
	if t < 0.5 {
		return 16 * t * t * t * t * t
	} else {
		return 1 - (-2*t+2)*(-2*t+2)*(-2*t+2)*(-2*t+2)*(-2*t+2)/2
	}
}

func EaseInCirc(t float64) float64 {
	return 1 - math.Sqrt(1-t*t)
}

func EaseOutCirc(t float64) float64 {
	return math.Sqrt(1 - (t-1)*(t-1))
}

func EaseInOutCirc(t float64) float64 {
	if t < 0.5 {
		return (1 - math.Sqrt(1-2*t*2*t)) / 2
	} else {
		return (math.Sqrt(1-(-2*t+2)*(-2*t+2)) + 1) / 2
	}
}

func EaseInElastic(t float64) float64 {
	switch t {
	case 0:
		return 0
	case 1:
		return 1
	default:
		const c4 = (2 * math.Pi) / 3
		return -math.Pow(2, 10*t-10) * math.Sin((t*10-10.75)*c4)
	}
}

func EaseOutElastic(t float64) float64 {
	switch t {
	case 0:
		return 0
	case 1:
		return 1
	default:
		const c4 = (2 * math.Pi) / 3
		return math.Pow(2, -10*t)*math.Sin((t*10-0.75)*c4) + 1
	}
}

func EaseInOutElastic(t float64) float64 {
	const c5 = (2 * math.Pi) / 4.5
	if t == 0 {
		return 0
	} else if t == 1 {
		return 1
	} else if t < 0.5 {
		return -(math.Pow(2, 20*t-10) * math.Sin((20*t-11.125)*c5)) / 2
	} else {
		return (math.Pow(2, -20*t+10)*math.Sin((20*t-11.125)*c5))/2 + 1
	}
}

func EaseInBounce(t float64) float64 {
	return 1 - EaseOutBounce(1-t)
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

func EaseInOutBounce(t float64) float64 {
	if t < 0.5 {
		return (1 - EaseOutBounce(1-2*t)) / 2
	} else {
		return (1 + EaseOutBounce(2*t-1)) / 2
	}
}

func EaseInExpo(t float64) float64 {
	if t == 0 {
		return 0
	} else {
		return math.Pow(2, 10*t-10)
	}
}

func EaseOutExpo(t float64) float64 {
	if t == 1 {
		return 1
	} else {
		return 1 - math.Pow(2, -10*t)
	}
}

func EaseInOutExpo(t float64) float64 {
	if t == 0 {
		return 0
	} else if t == 1 {
		return 1
	} else if t < 0.5 {
		return math.Pow(2, 20*t-10) / 2
	} else {
		return (2 - math.Pow(2, -20*t+10)) / 2
	}
}

func EaseInBack(t float64) float64 {
	const c1 = 1.70158
	const c3 = c1 + 1
	return c3*t*t*t - c1*t*t
}

func EaseOutBack(t float64) float64 {
	const c1 = 1.70158
	const c3 = c1 + 1

	return 1 + c3*(t-1)*(t-1)*(t-1) + c1*(t-1)*(t-1)
}

func EaseInOutBack(t float64) float64 {
	const c1 = 1.70158
	const c2 = c1 * 1.525

	if t < 0.5 {
		return (2 * t * 2 * t * ((c2+1)*2*t - c2)) / 2
	} else {
		return ((2*t-2)*(2*t-2)*((c2+1)*(t*2-2)+c2) + 2) / 2
	}
}

// EaseCubicBezier uses a cubic Bézier curve to map the input t to an output t′,
// similar to cubic Bézier easing functions in CSS and many other tools.
//
// The first and last control points are fixed to (0, 0) and (1, 1), and the x
// coordinates of the remaining two control points are constrained to [0, 1).
// This ensures that x(t) is monotonically increasing.
type EaseCubicBezier struct {
	// We'll call the argument p (for progress) instead of the usual t to avoid
	// confusion with the conventional use of t in Bézier curves, which
	// describes the progress along the curve. We'll use p′ for the output
	// value.
	//
	// A Bézier curve is described by two parametric functions x(t) and y(t).
	// The x axis corresponds to values of p and the y axis corresponds to
	// values of p′. To evaluate the curve at p, we're trying to solve x(t) =
	// p for t. This value is plugged into y(t) to get p′. Solving for t boils
	// down to intersecting the curve with a vertical line at x = p, which is
	// equivalent to cubic root finding. This process is only possible because
	// we've ensured monotonically increasing x(t), which means that for any
	// value of p, there is only one possible value of t for which x(t) = p.

	P1, P2 curve.Point
}

func (ecb EaseCubicBezier) Ease(p float64) float64 {
	p0 := curve.Pt(0, 0)
	p1 := curve.Pt(jmath.Clamp(ecb.P1.X, 0, 1), ecb.P1.Y)
	p2 := curve.Pt(jmath.Clamp(ecb.P2.X, 0, 1), ecb.P2.Y)
	p3 := curve.Pt(1, 1)

	if p1 == curve.Pt(0, 0) && p2 == curve.Pt(1, 1) {
		return p
	}

	cb := curve.CubicBez{
		P0: p0,
		P1: p1,
		P2: p2,
		P3: p3,
	}

	l := curve.Line{
		P0: curve.Pt(p, 0),
		P1: curve.Pt(p, 1),
	}
	intersections, n := cb.IntersectLine(l)
	if n == 0 {
		// This should be impossible.
		return p
	} else {
		// There should be exactly one intersection, but we'll silently ignore
		// any additional ones.

		// Clamp to [0, 1) because the value might be slightly outside those
		// bounds at the edges.
		t := jmath.Clamp(intersections[0].SegmentT, 0, 1)
		return cb.Eval(t).Y
	}
}
