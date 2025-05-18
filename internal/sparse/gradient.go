// SPDX-FileCopyrightText: 2025 the Piet Authors
// SPDX-FileCopyrightText: 2025 Dominik Honnef and contributors
//
// SPDX-License-Identifier: MIT

package sparse

import (
	"math"
	"slices"

	"honnef.co/go/color"
	"honnef.co/go/curve"
)

//go:generate go tool stringer -type=GradientExtend -trimprefix=GradientExtend
type GradientExtend int

const (
	GradientExtendPad GradientExtend = iota
	GradientExtendRepeat
	GradientExtendReflect
)

type GradientStop struct {
	Offset float32
	Color  color.Color
}

type Gradient interface {
	Paint
}

var _ Gradient = (*LinearGradient)(nil)
var _ Gradient = (*RadialGradient)(nil)
var _ Gradient = (*SweepGradient)(nil)

type LinearGradient struct {
	Stops      []GradientStop
	Transform  curve.Affine
	Extend     GradientExtend
	Start      curve.Point
	End        curve.Point
	ColorSpace *color.Space
}

// encode implements Gradient.
func (l *LinearGradient) encode() encodedPaint {
	// First make sure that the gradient is valid and not degenerate.
	if valid, fallback := l.validate(); !valid {
		return fallback
	}
	hasOpacities := slices.ContainsFunc(l.Stops, func(stop GradientStop) bool {
		return stop.Color.Values[3] != 1.0
	})

	// For each gradient type, before doing anything we first translate it such
	// that one of the points of the gradient lands on the origin (0, 0). We do
	// this because it makes things simpler and allows for some optimizations
	// for certain calculations.
	var xOffset, yOffset float32

	// For linear gradients, we want to interpolate the color along the line
	// that is formed by `start` and `end`.
	p0 := l.Start
	p1 := l.End

	// For simplicity, ensure that the gradient line always goes from left to
	// right.
	stops := l.Stops
	if p0.X >= p1.X {
		p0, p1 = p1, p0
		stops = slices.Clone(stops)
		slices.Reverse(stops)
		for i := range stops {
			stops[i].Offset = 1 - stops[i].Offset
		}
	}

	// Double the length of the iterator, and append stops in reverse order in
	// case we have the extend `Reflect`. Then we can treat it the same as a
	// repeated gradient.
	if l.Extend == GradientExtendReflect {
		p1.X = p1.X + p1.X - p0.X
		p1.Y = p1.Y + p1.Y - p0.Y
		stops = applyReflect(stops)
	}

	// To translate p0 to the origin of the coordinate system, we need to apply
	// the negative.
	xOffset = float32(-p0.X)
	yOffset = float32(-p0.Y)

	dx := float32(p1.X) + xOffset
	dy := float32(p1.Y) + yOffset
	// In order to calculate where a pixel lies along the gradient line (the
	// line made up by the two points of the linear gradient), we need to
	// calculate its position on the gradient line. Remember that our gradient
	// line always start at the origin (0, 0). Therefore, we can simply
	// calculate the normal vector of the line, and then, for each pixel that we
	// render, we calculate the distance to the line. That distance then
	// corresponds to our position on the gradient line, and allows us to
	// resolve which color stops we need to load and how to interpolate them.
	norm := [2]float32{-dy, dx}

	// We precalculate some values so that we can more easily calculate the
	// distance from the position of the pixel to the line of the normal vector.
	// See here for the formula:
	// https://en.wikipedia.org/wiki/Distance_from_a_point_to_a_line#Line_defined_by_two_points

	// The denominator, i.e. sqrt((y_2 - y_1)^2 + (x_2 - x_1)^2). Since x_1 and
	// y_1 are always 0, this shortens to sqrt(y_2^2 + x_2^2).
	distance := sqrt32(norm[1]*norm[1] + norm[0]*norm[0])
	// This corresponds to (y_2 - y_1) in the formula, but because of the above
	// reasons shortens to y_2.
	y2MinusY1 := norm[1]
	// This corresponds to (x_2 - x_1) in the formula, but because of the above
	// reasons shortens to x_2.
	x2MinusX1 := norm[0]
	// Note that we can completely disregard the x_2 * y_1 - y_2 * x_1 factor,
	// since y_1 and x_1 are both 0.

	// The start/end range of the color line. We use this to resolve the extend
	// of the gradient. Currently radial gradients uses normalized values
	// between 0.0 and 1.0, for sweep and linear gradients different values are
	// used (TODO: Would be nice to make this more consistent).
	clampRange := [2]float32{0, sqrt32(dx*dx + dy*dy)}

	return encodeGradient(
		encodedLinearGradient{
			distance:  distance,
			y2MinusY1: y2MinusY1,
			x2MinusX1: x2MinusX1,
		},
		clampRange,
		stops,
		l.Extend,
		xOffset,
		yOffset,
		l.Transform,
		hasOpacities,
		l.ColorSpace,
	)
}

func encodeGradient(
	kind gradientKind,
	clampRange [2]float32,
	stops []GradientStop,
	extend GradientExtend,
	xOffset float32,
	yOffset float32,
	transform curve.Affine,
	hasOpacities bool,
	space *color.Space,
) encodedPaint {
	if space == nil {
		space = color.LinearSRGB
	}
	pad := extend == GradientExtendPad
	ranges := encodeStops(stops, clampRange[0], clampRange[1], pad, space)

	// This represents the transform that needs to be applied to the starting
	// point of a command before starting with the rendering. First we need to
	// account for a potential offset of the gradient (x_offset/y_offset), then
	// we account for the fact that we sample in the center of a pixel and not
	// in the corner by adding 0.5. Finally, we need to apply the _inverse_
	// transform to the point so that we can account for the transform on the
	// gradient.
	transform = curve.Translate(curve.Vec(float64(xOffset+0.5), float64(yOffset+0.5))).Mul(transform.Invert())

	// One possible approach of calculating the positions would be to apply the
	// above transform to _each_ pixel that we render in the wide tile. However,
	// a much better approach is to apply the transform once for the first
	// pixel, and from then on only apply incremental updates to the current x/y
	// position that we calculated in the beginning.
	//
	// Remember that we render wide tiles in column major order (i.e. we first
	// calculate the values for a specific x for all Tile::HEIGHT y by
	// incrementing y by 1, and then finally we increment the x position by 1
	// and start from the beginning). If we want to implement the above approach
	// of incrementally updating the position, we need to calculate how the x/y
	// unit vectors are affected by the transform, and then use this as the step
	// delta for a step in the x/y direction.

	c := transform.Coefficients()
	scaleSkewTransform := curve.NewAffine([6]float64{c[0], c[1], c[2], c[3], 0, 0})
	xAdvance := curve.Pt(1.0, 0.0).Transform(scaleSkewTransform)
	yAdvance := curve.Pt(0.0, 1.0).Transform(scaleSkewTransform)

	encoded := &encodedGradient{
		kind,
		transform,
		curve.Vec2(xAdvance),
		curve.Vec2(yAdvance),
		ranges,
		pad,
		hasOpacities,
		clampRange,
		space,
	}

	return encoded
}

func (l *LinearGradient) validate() (valid bool, fallback encodedPaint) {
	if valid, fallback := validateStops(l.Stops); !valid {
		return false, fallback
	}
	first := l.Stops[0].Color
	// Start and end points must not be too close together.
	if degeneratePoint(l.Start, l.End) {
		return false, colorToInternal(first)
	}
	return true, nil
}

type RadialGradient struct {
	Stops       []GradientStop
	Transform   curve.Affine
	Extend      GradientExtend
	StartCenter curve.Point
	StartRadius float32
	EndCenter   curve.Point
	EndRadius   float32
	ColorSpace  *color.Space
}

// encode implements Gradient.
func (r *RadialGradient) encode() encodedPaint {
	// First make sure that the gradient is valid and not degenerate.
	if valid, fallback := r.validate(); !valid {
		return fallback
	}
	hasOpacities := slices.ContainsFunc(r.Stops, func(stop GradientStop) bool {
		return stop.Color.Values[3] != 1.0
	})

	// For each gradient type, before doing anything we first translate it such
	// that one of the points of the gradient lands on the origin (0, 0). We do
	// this because it makes things simpler and allows for some optimizations
	// for certain calculations.
	var xOffset, yOffset float32

	// The start/end range of the color line. We use this to resolve the extend
	// of the gradient. Currently radial gradients uses normalized values
	// between 0.0 and 1.0, for sweep and linear gradients different values are
	// used (TODO: Would be nice to make this more consistent).
	clampRange := [2]float32{0, 1}

	// For radial gradients, we conceptually interpolate a circle from c0 with
	// radius r0 to the circle at c1 with radius r1.
	c0 := r.StartCenter
	c1 := r.EndCenter
	r0 := r.StartRadius
	r1 := r.EndRadius

	// Same story as for linear gradients, mutate stops so that reflect and
	// repeat can be treated the same.
	stops := r.Stops
	if r.Extend == GradientExtendReflect {
		c1 = c1.Translate(c1.Sub(c0))
		r1 = r1 + r1 - r0
		stops = applyReflect(stops)
	}

	// Similarly to linear gradients, ensure that c0 lands on the origin (0, 0).
	xOffset = float32(-c0.X)
	yOffset = float32(-c0.Y)

	endPoint := c1.Sub(c0)

	dist := float32(math.Sqrt(endPoint.X*endPoint.X + endPoint.Y*endPoint.Y))
	conelike := r1 < r0+dist && r0 < r1+dist
	// If the inner circle is not completely contained within the outer circle,
	// the gradient can deform into a cone-like structure where some areas of
	// the shape are not defined. Because of this, we might need opacities and
	// source-over compositing in that case.
	if conelike {
		hasOpacities = true
	}

	return encodeGradient(
		encodedRadialGradient{
			[2]float32{float32(endPoint.X), float32(endPoint.Y)},
			r0,
			r1,
			conelike,
		},
		clampRange,
		stops,
		r.Extend,
		xOffset,
		yOffset,
		r.Transform,
		hasOpacities,
		r.ColorSpace,
	)
}

func (r *RadialGradient) validate() (valid bool, fallback encodedPaint) {
	if valid, fallback := validateStops(r.Stops); !valid {
		return false, fallback
	}
	first := r.Stops[0].Color
	// Radii must not be negative.
	if r.StartRadius < 0.0 || r.EndRadius < 0.0 {
		return false, colorToInternal(first)
	}

	// Radii and center points must not be close to the same.
	if degeneratePoint(r.StartCenter, r.EndCenter) &&
		degenerateVal(r.StartRadius, r.EndRadius) {
		return false, colorToInternal(first)
	}
	return true, nil
}

type SweepGradient struct {
	Stops     []GradientStop
	Transform curve.Affine
	Extend    GradientExtend
	Center    curve.Point

	// The start and end angles, in radian.
	StartAngle float32
	EndAngle   float32

	ColorSpace *color.Space
}

// encode implements Gradient.
func (s *SweepGradient) encode() encodedPaint {
	// First make sure that the gradient is valid and not degenerate.
	if valid, fallback := s.validate(); !valid {
		return fallback
	}

	hasOpacities := slices.ContainsFunc(s.Stops, func(stop GradientStop) bool {
		return stop.Color.Values[3] != 1.0
	})

	// For each gradient type, before doing anything we first translate it such
	// that one of the points of the gradient lands on the origin (0, 0). We do
	// this because it makes things simpler and allows for some optimizations
	// for certain calculations.
	var xOffset, yOffset float32

	// For sweep gradients, the position on the "color line" is defined by the
	// angle towards the gradient center.
	startAngle := s.StartAngle
	endAngle := s.EndAngle

	// Same as before, reduce `Reflect` to `Repeat`.
	stops := s.Stops
	if s.Extend == GradientExtendReflect {
		endAngle = endAngle + endAngle - startAngle
		stops = applyReflect(stops)
	}

	// Make sure the center of the gradient falls on the origin (0, 0).
	xOffset = float32(-s.Center.X)
	yOffset = float32(-s.Center.Y)

	return encodeGradient(
		encodedSweepGradient{},
		[2]float32{startAngle, endAngle},
		stops,
		s.Extend,
		xOffset,
		yOffset,
		s.Transform,
		hasOpacities,
		s.ColorSpace,
	)
}

func (s *SweepGradient) validate() (valid bool, fallback encodedPaint) {
	if valid, fallback := validateStops(s.Stops); !valid {
		return false, fallback
	}
	first := s.Stops[0].Color
	// Angles must be between 0 and 360.
	if s.StartAngle < 0.0 ||
		s.StartAngle > 360.0 ||
		s.EndAngle < 0.0 ||
		s.EndAngle > 360.0 {
		return false, colorToInternal(first)
	}

	// The end angle must be larger than the start angle.
	if degenerateVal(s.StartAngle, s.EndAngle) {
		return false, colorToInternal(first)
	}

	if s.EndAngle <= s.StartAngle {
		return false, colorToInternal(first)
	}
	return true, nil
}

func validateStops(stops []GradientStop) (valid bool, fallback encodedPaint) {
	black := plainColor{0, 0, 0, 1}

	// Gradients need at least two stops.
	if len(stops) == 0 {
		return false, black
	}

	first := stops[0].Color

	if len(stops) == 1 {
		return false, colorToInternal(first)
	}

	// First stop must be at offset 0.0 and last offset must be at 1.0.
	if stops[0].Offset != 0.0 || stops[len(stops)-1].Offset != 1.0 {
		return false, black
	}

	for i := range len(stops) - 1 {
		f := stops[i]
		n := stops[i+1]

		// Offsets must be between 0 and 1.
		if f.Offset > 1.0 || f.Offset < 0.0 {
			return false, black
		}

		// Stops must be sorted by ascending offset.
		if f.Offset >= n.Offset {
			return false, black
		}
	}

	return true, nil
}

type gradientKind interface {
	curPos(pos curve.Point) float32
	hasUndefined() bool
	isDefined(pos curve.Point) bool
}

type encodedLinearGradient struct {
	distance  float32
	y2MinusY1 float32
	x2MinusX1 float32
}

// curPos implements gradientKind.
func (l encodedLinearGradient) curPos(pos curve.Point) float32 {
	// The position of a point relative to a linear gradient is determined by
	// its distance to the normal vector. See `encode_into` for more
	// information.
	return (float32(pos.X)*l.y2MinusY1 - float32(pos.Y)*l.x2MinusX1) / l.distance
}

// hasUndefined implements gradientKind.
func (l encodedLinearGradient) hasUndefined() bool { return false }

// isDefined implements gradientKind.
func (l encodedLinearGradient) isDefined(pos curve.Point) bool { return true }

type encodedRadialGradient struct {
	c1       [2]float32
	r0       float32
	r1       float32
	coneLike bool
}

// curPos implements gradientKind.
func (r encodedRadialGradient) curPos(pos curve.Point) float32 {
	f, _ := r.posInner(pos)
	return f
}

// hasUndefined implements gradientKind.
func (r encodedRadialGradient) hasUndefined() bool {
	return r.coneLike
}

// isDefined implements gradientKind.
func (r encodedRadialGradient) isDefined(pos curve.Point) bool {
	_, ok := r.posInner(pos)
	return ok
}

func (r encodedRadialGradient) posInner(pos curve.Point) (float32, bool) {
	// The values for a radial gradient can be calculated for any t as follow:
	// Let x(t) = (x_1 - x_0)*t + x_0 (since x_0 is always 0, this shortens to x_1 * t)
	// Let y(t) = (y_1 - y_0)*t + y_0 (since y_0 is always 0, this shortens to y_1 * t)
	// Let r(t) = (r_1 - r_0)*t + r_0
	//
	// Given a pixel at a position (x_2, y_2), we need to find the largest t
	// such that (x_2 - x(t))^2 + (y - y_(t))^2 = r_t()^2, i.e. the circle with
	// the interpolated radius and center position needs to intersect the pixel
	// we are processing.
	//
	// You can reformulate this problem to a quadratic equation (TODO: add
	// derivation. Since I'm not sure if that code will stay the same after
	// performance optimizations I haven't written this down yet), to which we
	// then simply need to find the solutions.

	r0 := r.r0
	dx := r.c1[0]
	dy := r.c1[1]
	dr := r.r1 - r.r0

	px := float32(pos.X)
	py := float32(pos.Y)

	a := dx*dx + dy*dy - dr*dr
	b := -2.0 * (px*dx + py*dy + r0*dr)
	c := px*px + py*py - r0*r0

	discriminant := b*b - 4.0*a*c

	// No solution available.
	if discriminant < 0.0 {
		return 0, false
	}

	sqrtD := sqrt32(discriminant)
	t1 := (-b - sqrtD) / (2.0 * a)
	t2 := (-b + sqrtD) / (2.0 * a)

	max := max(t1, t2)
	min := min(t1, t2)

	// We only want values for `t` where the interpolated radius is actually
	// positive.
	if r.r0+dr*max < 0.0 {
		if r.r0+dr*min < 0.0 {
			return 0, false
		} else {
			return min, true
		}
	} else {
		return max, true
	}
}

type encodedSweepGradient struct{}

// curPos implements gradientKind.
func (s encodedSweepGradient) curPos(pos curve.Point) float32 {
	// The position in a sweep gradient is simply determined by its angle from
	// the origin.
	angle := float32(math.Atan2(-pos.Y, pos.X))
	if angle >= 0.0 {
		return angle
	} else {
		return angle + 2.0*math.Pi
	}
}

// hasUndefined implements gradientKind.
func (s encodedSweepGradient) hasUndefined() bool { return false }

// isDefined implements gradientKind.
func (s encodedSweepGradient) isDefined(pos curve.Point) bool { return true }

type encodedGradient struct {
	kind gradientKind
	// A transform that needs to be applied to the position of the first
	// processed pixel.
	transform curve.Affine
	// How much to advance along the x and y directions in the gradient for one
	// step in the x direction in the output image.
	xAdvance curve.Vec2
	// How much to advance along the x and y directions in the gradient for one
	// step in the y direction in the output image.
	yAdvance curve.Vec2
	// The color ranges of the gradient.
	ranges []gradientRange
	// Whether the gradient should be padded.
	pad bool
	// Whether the gradient requires `source_over` compositing.
	hasOpacities bool
	// The values that should be used for clamping when applying the extend.
	clampRange [2]float32
	// The color space to use for interpolation.
	space *color.Space
}

// isEncodedPaint implements encodedPaint.
func (e *encodedGradient) isEncodedPaint() {}

type gradientRange struct {
	// The start value of the range.
	x0 float32
	// The end value of the range.
	x1 float32
	// The start color of the range.
	c0 color.Color
	// The interpolation factors of the range.
	factors [4]float32
}

const degenerateThreshold = 1.0e-6

func degeneratePoint(p1 curve.Point, p2 curve.Point) bool {
	return math.Abs(p1.X-p2.X) <= degenerateThreshold &&
		math.Abs(p1.Y-p2.Y) <= degenerateThreshold
}

func degenerateVal(v1 float32, v2 float32) bool {
	return abs32(v2-v1) <= degenerateThreshold
}

// Extend the stops so that we can treat a repeated gradient like a reflected
// gradient.
func applyReflect(stops []GradientStop) []GradientStop {
	// OPT(dh): we could combine the two loops, and also index into out instead
	// of using append.
	out := make([]GradientStop, 0, len(stops)*2)
	for _, stop := range stops {
		out = append(out, GradientStop{stop.Offset / 2, stop.Color})
	}
	for i := len(stops) - 1; i >= 0; i-- {
		stop := stops[i]
		out = append(out, GradientStop{0.5 + (1.0-stop.Offset)/2, stop.Color})
	}

	return out
}

// Encode all stops into a sequence of ranges.
func encodeStops(stops []GradientStop, start, end float32, pad bool, space *color.Space) []gradientRange {
	createRange := func(left_stop, right_stop GradientStop) gradientRange {
		x0 := start + (end-start)*left_stop.Offset
		x1 := start + (end-start)*right_stop.Offset
		c0 := left_stop.Color.Convert(space)
		c1 := right_stop.Color.Convert(space)

		// FIXME(dh): support interpolating correctly in cylindrical color
		// spaces. See
		// https://developer.mozilla.org/en-US/docs/Web/CSS/hue-interpolation-method

		// Given two positions x0 and x1 as well as two corresponding colors c0 and c1,
		// the delta that needs to be applied to c0 to calculate the color of x between x0 and x1
		// is calculated by c0 + ((x - x0) / (x1 - x0)) * (c1 - c0).
		// We can precompute the (c1 - c0)/(x1 - x0) part for each color component.

		// OPT(dh): can we fold color space conversion (i.e. colorToInternal) into the factors?

		// We call this method with two same stops for `left_range` and
		// `right_range`, so make sure we don't actually end up with a 0 here.
		x1MinusX0 := max(x1-x0, 0.0000001)
		factors := [4]float32{
			float32(c1.Values[0]-c0.Values[0]) / x1MinusX0,
			float32(c1.Values[1]-c0.Values[1]) / x1MinusX0,
			float32(c1.Values[2]-c0.Values[2]) / x1MinusX0,
			float32(c1.Values[3]-c0.Values[3]) / x1MinusX0,
		}

		return gradientRange{
			x0,
			x1,
			c0,
			factors,
		}
	}

	if pad {
		// We handle padding by inserting dummy stops in the beginning and end
		// with a very big range.
		stopRanges := make([]gradientRange, len(stops)+1)
		encodedRange := createRange(stops[0], stops[0])
		encodedRange.x0 = -math.MaxFloat32
		stopRanges[0] = encodedRange

		for i := range stops[:len(stops)-1] {
			stopRanges[i+1] = createRange(stops[i], stops[i+1])
		}

		lastStop := stops[len(stops)-1]
		encodedRange = createRange(lastStop, lastStop)
		encodedRange.x1 = math.MaxFloat32
		stopRanges[len(stopRanges)-1] = encodedRange
		return stopRanges
	} else {
		stopRanges := make([]gradientRange, len(stops)-1)
		for i := range stops[:len(stops)-1] {
			stopRanges[i] = createRange(stops[i], stops[i+1])
		}
		return stopRanges
	}
}

type gradientFiller struct {
	curPos   curve.Point
	rangeIdx int
	gradient *encodedGradient
}

func newGradientFiller(
	e *encodedGradient,
	startX uint16,
	startY uint16,
) *gradientFiller {
	return &gradientFiller{
		curPos:   curve.Pt(float64(startX), float64(startY)).Transform(e.transform),
		gradient: e,
	}
}

func (gf *gradientFiller) curRange() *gradientRange {
	// OPT(dh): cache the current range to avoid repeated bounds checks
	return &gf.gradient.ranges[gf.rangeIdx]
}

func (gf *gradientFiller) advanceTo(targetPos float32) {
	for targetPos > gf.curRange().x1 || targetPos < gf.curRange().x0 {
		if gf.rangeIdx == 0 {
			gf.rangeIdx = len(gf.gradient.ranges) - 1
		} else {
			gf.rangeIdx--
		}
	}
}

func (gf *gradientFiller) run(dst [][stripHeight]plainColor) {
	oldPos := gf.curPos

	for x := range dst {
		col := &dst[x]
		gf.runColumn(col)
		gf.curPos = gf.curPos.Translate(gf.gradient.xAdvance)
	}

	// Radial gradients can have positions that are undefined and thus shouldn't be
	// painted at all. Checking for this inside of the main filling logic would be
	// an unnecessary overhead for the general case, while this is really just an edge
	// case. Because of this, in the first run we will fill it using a dummy color, and
	// in case the gradient might have undefined locations, we do another run over
	// the buffer and override the positions with a transparent fill. This way, we need
	// 2x as long to handle such gradients, but for the common case we don't incur any
	// additional overhead.
	if gf.gradient.kind.hasUndefined() {
		gf.curPos = oldPos
		gf.runUndefined(dst)
	}
}

func (gf *gradientFiller) runColumn(col *[stripHeight]plainColor) {
	pos := gf.curPos
	extend := func(val float32) float32 {
		return extend(val, gf.gradient.pad, gf.gradient.clampRange)
	}
	for y := range col {
		px := &col[y]
		extendedPos := extend(gf.gradient.kind.curPos(pos))
		gf.advanceTo(extendedPos)
		rng := gf.curRange()

		c := rng.c0

		for compIdx := range px {
			factor := (rng.factors[compIdx] * (extendedPos - rng.x0))
			c.Values[compIdx] += float64(factor)
		}
		*px = colorToInternal(c)
		pos = pos.Translate(gf.gradient.yAdvance)
	}
}

func (gf *gradientFiller) runUndefined(dst [][stripHeight]plainColor) {
	for i := range dst {
		col := &dst[i]
		pos := gf.curPos
		for i := range col {
			px := &col[i]
			if !gf.gradient.kind.isDefined(pos) {
				*px = plainColor{}
			}
			pos = pos.Translate(gf.gradient.yAdvance)
		}
		gf.curPos = gf.curPos.Translate(gf.gradient.xAdvance)
	}
}

func extend(val float32, pad bool, clampRange [2]float32) float32 {
	if pad {
		return val
	}

	start := clampRange[0]
	end := clampRange[1]

	for val < start {
		val += end - start
	}
	for val > end {
		val -= end - start
	}
	return val
}
