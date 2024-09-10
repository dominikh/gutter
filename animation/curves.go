// SPDX-FileCopyrightText: 2024 Dominik Honnef and contributors
//
// SPDX-License-Identifier: MIT

package animation

import (
	"math"

	"honnef.co/go/curve"
	"honnef.co/go/jello/jmath"
)

type Curve interface {
	Transform(t float64) float64
}

var CurveIdentity = Curve(&curveIdentity{})
var CurveInBack = Curve(&curveInBack{})
var CurveInBounce = Curve(&curveInBounce{})
var CurveInCirc = Curve(&curveInCirc{})
var CurveInCubic = Curve(&curveInCubic{})
var CurveInElastic = Curve(&curveInElastic{})
var CurveInExpo = Curve(&curveInExpo{})
var CurveInOutBack = Curve(&curveInOutBack{})
var CurveInOutBounce = Curve(&curveInOutBounce{})
var CurveInOutCirc = Curve(&curveInOutCirc{})
var CurveInOutCubic = Curve(&curveInOutCubic{})
var CurveInOutElastic = Curve(&curveInOutElastic{})
var CurveInOutExpo = Curve(&curveInOutExpo{})
var CurveInOutQuad = Curve(&curveInOutQuad{})
var CurveInOutQuart = Curve(&curveInOutQuart{})
var CurveInOutQuint = Curve(&curveInOutQuint{})
var CurveInOutSine = Curve(&curveInOutSine{})
var CurveInQuad = Curve(&curveInQuad{})
var CurveInQuart = Curve(&curveInQuart{})
var CurveInQuint = Curve(&curveInQuint{})
var CurveInSine = Curve(&curveInSine{})
var CurveOutBack = Curve(&curveOutBack{})
var CurveOutBounce = Curve(&curveOutBounce{})
var CurveOutCirc = Curve(&curveOutCirc{})
var CurveOutCubic = Curve(&curveOutCubic{})
var CurveOutElastic = Curve(&curveOutElastic{})
var CurveOutExpo = Curve(&curveOutExpo{})
var CurveOutQuad = Curve(&curveOutQuad{})
var CurveOutQuart = Curve(&curveOutQuart{})
var CurveOutQuint = Curve(&curveOutQuint{})
var CurveOutSine = Curve(&curveOutSine{})

type curveIdentity struct{}
type curveInBack struct{}
type curveInBounce struct{}
type curveInCirc struct{}
type curveInCubic struct{}
type curveInElastic struct{}
type curveInExpo struct{}
type curveInOutBack struct{}
type curveInOutBounce struct{}
type curveInOutCirc struct{}
type curveInOutCubic struct{}
type curveInOutElastic struct{}
type curveInOutExpo struct{}
type curveInOutQuad struct{}
type curveInOutQuart struct{}
type curveInOutQuint struct{}
type curveInOutSine struct{}
type curveInQuad struct{}
type curveInQuart struct{}
type curveInQuint struct{}
type curveInSine struct{}
type curveOutBack struct{}
type curveOutBounce struct{}
type curveOutCirc struct{}
type curveOutCubic struct{}
type curveOutElastic struct{}
type curveOutExpo struct{}
type curveOutQuad struct{}
type curveOutQuart struct{}
type curveOutQuint struct{}
type curveOutSine struct{}

func (*curveIdentity) Transform(t float64) float64 {
	return t
}

func (*curveInSine) Transform(t float64) float64 {
	return 1 - math.Cos((t*math.Pi)/2)
}

func (*curveOutSine) Transform(t float64) float64 {
	return math.Sin((t * math.Pi) / 2)
}

func (*curveInOutSine) Transform(t float64) float64 {
	return -(math.Cos(math.Pi*t) - 1) / 2
}

func (*curveInQuad) Transform(t float64) float64 {
	return t * t
}

func (*curveOutQuad) Transform(t float64) float64 {
	return 1 - (1 - t) - (1 - t)
}

func (*curveInOutQuad) Transform(t float64) float64 {
	if t < 0.5 {
		return 2 * t * t
	} else {
		return 1 - (-2*t+2)*(-2*t+2)/2
	}
}

func (*curveInCubic) Transform(t float64) float64 {
	return t * t * t
}

func (*curveOutCubic) Transform(t float64) float64 {
	return 1 - (1-t)*(1-t)*(1-t)
}

func (*curveInOutCubic) Transform(t float64) float64 {
	if t < 0.5 {
		return 4 * t * t * t
	} else {
		return 1 - (-2*t+2)*(-2*t+2)*(-2*t+2)/2
	}
}

func (*curveInQuart) Transform(t float64) float64 {
	return t * t * t * t
}

func (*curveOutQuart) Transform(t float64) float64 {
	return 1 - (1-t)*(1-t)*(1-t)*(1-t)
}

func (*curveInOutQuart) Transform(t float64) float64 {
	if t < 0.5 {
		return 8 * t * t * t * t
	} else {
		return 1 - (-2*t+2)*(-2*t+2)*(-2*t+2)*(-2*t+2)/2
	}
}

func (*curveInQuint) Transform(t float64) float64 {
	return t * t * t * t * t
}

func (*curveOutQuint) Transform(t float64) float64 {
	return 1 - (1-t)*(1-t)*(1-t)*(1-t)*(1-t)
}

func (*curveInOutQuint) Transform(t float64) float64 {
	if t < 0.5 {
		return 16 * t * t * t * t * t
	} else {
		return 1 - (-2*t+2)*(-2*t+2)*(-2*t+2)*(-2*t+2)*(-2*t+2)/2
	}
}

func (*curveInCirc) Transform(t float64) float64 {
	return 1 - math.Sqrt(1-t*t)
}

func (*curveOutCirc) Transform(t float64) float64 {
	return math.Sqrt(1 - (t-1)*(t-1))
}

func (*curveInOutCirc) Transform(t float64) float64 {
	if t < 0.5 {
		return (1 - math.Sqrt(1-2*t*2*t)) / 2
	} else {
		return (math.Sqrt(1-(-2*t+2)*(-2*t+2)) + 1) / 2
	}
}

func (*curveInElastic) Transform(t float64) float64 {
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

func (*curveOutElastic) Transform(t float64) float64 {
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

func (*curveInOutElastic) Transform(t float64) float64 {
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

func (*curveInBounce) Transform(t float64) float64 {
	return 1 - CurveOutBounce.Transform(1-t)
}

func (*curveOutBounce) Transform(t float64) float64 {
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

func (*curveInOutBounce) Transform(t float64) float64 {
	if t < 0.5 {
		return (1 - CurveOutBounce.Transform(1-2*t)) / 2
	} else {
		return (1 + CurveOutBounce.Transform(2*t-1)) / 2
	}
}

func (*curveInExpo) Transform(t float64) float64 {
	if t == 0 {
		return 0
	} else {
		return math.Pow(2, 10*t-10)
	}
}

func (*curveOutExpo) Transform(t float64) float64 {
	if t == 1 {
		return 1
	} else {
		return 1 - math.Pow(2, -10*t)
	}
}

func (*curveInOutExpo) Transform(t float64) float64 {
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

func (*curveInBack) Transform(t float64) float64 {
	const c1 = 1.70158
	const c3 = c1 + 1
	return c3*t*t*t - c1*t*t
}

func (*curveOutBack) Transform(t float64) float64 {
	const c1 = 1.70158
	const c3 = c1 + 1

	return 1 + c3*(t-1)*(t-1)*(t-1) + c1*(t-1)*(t-1)
}

func (*curveInOutBack) Transform(t float64) float64 {
	const c1 = 1.70158
	const c2 = c1 * 1.525

	if t < 0.5 {
		return (2 * t * 2 * t * ((c2+1)*2*t - c2)) / 2
	} else {
		return ((2*t-2)*(2*t-2)*((c2+1)*(t*2-2)+c2) + 2) / 2
	}
}

// CurveCubicBezier uses a cubic Bézier curve to map the input t to an output t′,
// similar to cubic Bézier easing functions in CSS and many other tools.
//
// The first and last control points are fixed to (0, 0) and (1, 1), and the x
// coordinates of the remaining two control points are constrained to [0, 1).
// This ensures that x(t) is monotonically increasing.
type CurveCubicBezier struct {
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

func (ecb CurveCubicBezier) Transform(p float64) float64 {
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

type CurveStatic float64

func (e CurveStatic) Transform(p float64) float64 { return float64(e) }
