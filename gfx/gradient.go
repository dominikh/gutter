// SPDX-FileCopyrightText: 2012 Google Inc.
// SPDX-FileCopyrightText: 2025 the Piet Authors
// SPDX-FileCopyrightText: 2025 the Vello Authors
// SPDX-FileCopyrightText: 2025 Dominik Honnef and contributors
//
// SPDX-License-Identifier: MIT AND BSD-3-Clause

// Parts of the gradient encoding have been copied from Piet and Vello, which
// are licensed under (at your choice) the Apache 2.0 or the MIT license. Part
// of that code has itself been derived from Skia, licensed under the revised
// 3-clause BSD license.

package gfx

import (
	"iter"
	"math"
	"math/bits"
	"slices"

	"honnef.co/go/color"
	"honnef.co/go/curve"
	"honnef.co/go/stuff/math/math32"
)

// TODO(dh): Allow specifying whether gradients should be interpolated with
// straight or premultiplied alpha.

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
	Extend     GradientExtend
	Start      curve.Point
	End        curve.Point
	ColorSpace *color.Space
}

// Encode implements Gradient.
func (l *LinearGradient) Encode(transform curve.Affine) EncodedPaint {
	// First make sure that the gradient is valid and not degenerate.
	if valid, fallback := l.validate(); !valid {
		return fallback
	}
	hasOpacities := slices.ContainsFunc(l.Stops, func(stop GradientStop) bool {
		return stop.Color.Values[3] != 1.0
	})

	p0 := l.Start
	p1 := l.End
	stops := l.Stops
	if l.Extend == GradientExtendReflect {
		p1.X = p1.X + p1.X - p0.X
		p1.Y = p1.Y + p1.Y - p0.Y
		stops = applyReflect(stops)
	}
	baseTransform := mapLineToLine(p0, p1, curve.Point{}, curve.Pt(1.0, 0.0))
	return encodeGradient(
		EncodedLinearGradient{},
		stops,
		baseTransform,
		l.Extend,
		transform,
		hasOpacities,
		l.ColorSpace,
	)
}

func encodeGradient(
	kind GradientKind,
	stops []GradientStop,
	baseTransform curve.Affine,
	extend GradientExtend,
	transform curve.Affine,
	hasOpacities bool,
	space *color.Space,
) EncodedPaint {
	if space == nil {
		space = color.LinearSRGB
	}
	pad := extend == GradientExtendPad

	if firstStop := &stops[0]; firstStop.Offset != 0 {
		firstStop.Offset = 0
	}
	if lastStop := &stops[len(stops)-1]; lastStop.Offset != 1 {
		lastStop.Offset = 1
	}

	ranges := encodeStops(stops, space)

	// This represents the transform that needs to be applied to the starting
	// point of a command before starting with the rendering. First we need to
	// account for a potential offset of the gradient (x_offset/y_offset), then
	// we account for the fact that we sample in the center of a pixel and not
	// in the corner by adding 0.5. Finally, we need to apply the _inverse_
	// transform to the point so that we can account for the transform on the
	// gradient.
	transform = baseTransform.Mul(transform.Invert()).Mul(curve.Translate(curve.Vec(0.5, 0.5)))

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

	encoded := &EncodedGradient{
		kind,
		transform,
		curve.Vec2(xAdvance),
		curve.Vec2(yAdvance),
		ranges,
		pad,
		// Even if the gradient has no stops with transparency, we might have to force
		// alpha-compositing in case the radial gradient is undefined in certain positions,
		// in which case the resulting color will be transparent and thus the gradient overall
		// must be treated as non-opaque.
		hasOpacities || kind.HasUndefined(),
		MakeGradientLUT(ranges),
	}

	return encoded
}

func (l *LinearGradient) validate() (valid bool, fallback EncodedPaint) {
	if valid, fallback := validateStops(l.Stops); !valid {
		return false, fallback
	}
	first := l.Stops[0].Color
	// Start and end points must not be too close together.
	if degeneratePoint(l.Start, l.End) {
		return false, ColorToInternal(first)
	}
	return true, nil
}

type RadialGradient struct {
	Stops       []GradientStop
	Extend      GradientExtend
	StartCenter curve.Point
	StartRadius float32
	EndCenter   curve.Point
	EndRadius   float32
	ColorSpace  *color.Space
}

// Encode implements Gradient.
func (r *RadialGradient) Encode(transform curve.Affine) EncodedPaint {
	// The implementation of radial gradients is translated from Skia.
	// See:
	// - <https://skia.org/docs/dev/design/conical/>
	// - <https://github.com/google/skia/blob/main/src/shaders/gradients/SkConicalGradient.h>
	// - <https://github.com/google/skia/blob/main/src/shaders/gradients/SkConicalGradient.cpp>

	// First make sure that the gradient is valid and not degenerate.
	if valid, fallback := r.validate(); !valid {
		return fallback
	}
	hasOpacities := slices.ContainsFunc(r.Stops, func(stop GradientStop) bool {
		return stop.Color.Values[3] != 1.0
	})

	c0 := r.StartCenter
	c1 := r.EndCenter
	r0 := r.StartRadius
	r1 := r.EndRadius
	stops := r.Stops
	var kind GradientKind
	var baseTransform curve.Affine
	if r.Extend == GradientExtendReflect {
		c1 = c1.Translate(c1.Sub(c0))
		r1 = r1 + r1 - r0
		stops = applyReflect(stops)
	}
	dRadius := r1 - r0
	if isNearlyZero(c1.Sub(c0).Hypot()) {
		sf := float64(1.0 / max(r0, r1))
		baseTransform = curve.Translate(curve.Vec(-c1.X, -c1.Y)).ThenScale(sf, sf)
		scale := max(r0, r1) / dRadius
		bias := -r0 / dRadius
		kind = EncodedRadialGradient{bias, scale}
	} else {
		baseTransform = mapLineToLine(c0, c1, curve.Pt(0, 0), curve.Pt(1.0, 0.0))
		if isNearlyZero32(r1 - r0) {
			r0Scaled := r1 / float32(c1.Sub(c0).Hypot())
			kind = EncodedStripGradient{r0ScaledSquared: r0Scaled * r0Scaled}
		} else {
			dCenter := float32(c0.Sub(c1).Hypot())
			var focalData FocalData
			focalData, baseTransform = createFocalData(r0/dCenter, r1/dCenter, baseTransform)
			fp0 := 1.0 / focalData.fr1
			fp1 := focalData.fFocalX
			kind = EncodedFocalGradient{focalData, fp0, fp1}
		}
	}

	return encodeGradient(
		kind,
		stops,
		baseTransform,
		r.Extend,
		transform,
		hasOpacities,
		r.ColorSpace,
	)
}

func (r *RadialGradient) validate() (valid bool, fallback EncodedPaint) {
	if valid, fallback := validateStops(r.Stops); !valid {
		return false, fallback
	}
	first := r.Stops[0].Color
	// Radii must not be negative.
	if r.StartRadius < 0.0 || r.EndRadius < 0.0 {
		return false, ColorToInternal(first)
	}

	// Radii and center points must not be close to the same.
	if degeneratePoint(r.StartCenter, r.EndCenter) &&
		degenerateVal(r.StartRadius, r.EndRadius) {
		return false, ColorToInternal(first)
	}
	return true, nil
}

type SweepGradient struct {
	Stops  []GradientStop
	Extend GradientExtend
	Center curve.Point

	// The start and end angles, in radian.
	StartAngle float32
	EndAngle   float32

	ColorSpace *color.Space
}

// Encode implements Gradient.
func (s *SweepGradient) Encode(transform curve.Affine) EncodedPaint {
	// First make sure that the gradient is valid and not degenerate.
	if valid, fallback := s.validate(); !valid {
		return fallback
	}

	hasOpacities := slices.ContainsFunc(s.Stops, func(stop GradientStop) bool {
		return stop.Color.Values[3] != 1.0
	})

	startAngle := s.StartAngle
	endAngle := s.EndAngle
	stops := s.Stops
	if s.Extend == GradientExtendReflect {
		endAngle = endAngle + endAngle - startAngle
		stops = applyReflect(stops)
	}
	xOffset := -s.Center.X
	yOffset := -s.Center.Y
	baseTransform := curve.Translate(curve.Vec(xOffset, yOffset))

	return encodeGradient(
		EncodedSweepGradient{
			startAngle,
			1.0 / (endAngle - startAngle),
		},
		stops,
		baseTransform,
		s.Extend,
		transform,
		hasOpacities,
		s.ColorSpace,
	)
}

func (s *SweepGradient) validate() (valid bool, fallback EncodedPaint) {
	if valid, fallback := validateStops(s.Stops); !valid {
		return false, fallback
	}
	first := s.Stops[0].Color
	// Angles must be between 0 and 360.
	if s.StartAngle < 0.0 ||
		s.StartAngle > 360.0 ||
		s.EndAngle < 0.0 ||
		s.EndAngle > 360.0 {
		return false, ColorToInternal(first)
	}

	// The end angle must be larger than the start angle.
	if degenerateVal(s.StartAngle, s.EndAngle) {
		return false, ColorToInternal(first)
	}

	if s.EndAngle <= s.StartAngle {
		return false, ColorToInternal(first)
	}
	return true, nil
}

func validateStops(stops []GradientStop) (valid bool, fallback EncodedPaint) {
	black := PlainColor{0, 0, 0, 1}

	// Gradients need at least two stops.
	if len(stops) == 0 {
		return false, black
	}

	first := stops[0].Color

	if len(stops) == 1 {
		return false, ColorToInternal(first)
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

type GradientKind interface {
	CurPos(pos curve.Point) float32
	HasUndefined() bool
	IsDefined(pos curve.Point) bool
}

type EncodedLinearGradient struct{}

// CurPos implements GradientKind.
func (l EncodedLinearGradient) CurPos(pos curve.Point) float32 {
	// The position along a linear gradient is determined by where we are along
	// the gradient line. Since during encoding, we have applied a
	// transformation such that the gradient line always goes from (0, 0) to (1,
	// 0), the position along the gradient line is simply determined by the
	// current x coordinate!
	return float32(pos.X)
}

// HasUndefined implements GradientKind.
func (l EncodedLinearGradient) HasUndefined() bool { return false }

// IsDefined implements GradientKind.
func (l EncodedLinearGradient) IsDefined(pos curve.Point) bool { return true }

type EncodedRadialGradient struct {
	bias  float32
	scale float32
}

// CurPos implements GradientKind.
func (r EncodedRadialGradient) CurPos(pos curve.Point) float32 {
	f, _ := r.posInner(pos)
	return f
}

// HasUndefined implements GradientKind.
func (r EncodedRadialGradient) HasUndefined() bool {
	return false
}

// IsDefined implements GradientKind.
func (r EncodedRadialGradient) IsDefined(pos curve.Point) bool {
	_, ok := r.posInner(pos)
	return ok
}

func (r EncodedRadialGradient) posInner(pos curve.Point) (float32, bool) {
	// We don't use curve.Vec2.Hypot because it uses math.Hypot, which has
	// special handling of Inf and NaN and contains extra logic to avoid
	// unnecessary overflow and underflow. None of that should be necessary for
	// the values we encounter while computing gradients. This simpler
	// computation is quite a bit faster, and posInner is part of the hot loop
	// of rendering radial gradients.
	radius := float32(math.Sqrt(pos.X*pos.X + pos.Y*pos.Y))
	radius = r.bias + radius*r.scale
	return radius, true
}

type EncodedSweepGradient struct {
	startAngle    float32
	invAngleDelta float32
}

// CurPos implements GradientKind.
func (s EncodedSweepGradient) CurPos(pos curve.Point) float32 {
	// The position in a sweep gradient is simply determined by its angle from
	// the origin.
	angle := float32(math.Atan2(-pos.Y, pos.X))
	var adj float32
	if angle >= 0.0 {
		adj = angle
	} else {
		adj = angle + 2.0*math.Pi
	}
	return (adj - s.startAngle) * s.invAngleDelta
}

// HasUndefined implements GradientKind.
func (s EncodedSweepGradient) HasUndefined() bool { return false }

// IsDefined implements GradientKind.
func (s EncodedSweepGradient) IsDefined(pos curve.Point) bool { return true }

type EncodedStripGradient struct {
	r0ScaledSquared float32
}

// CurPos implements GradientKind.
func (g EncodedStripGradient) CurPos(pos curve.Point) float32 {
	f, _ := g.posInner(pos)
	return f
}

// HasUndefined implements GradientKind.
func (g EncodedStripGradient) HasUndefined() bool {
	return true
}

// IsDefined implements GradientKind.
func (g EncodedStripGradient) IsDefined(pos curve.Point) bool {
	_, ok := g.posInner(pos)
	return ok
}

type EncodedFocalGradient struct {
	focalData FocalData
	fp0       float32
	fp1       float32
}

// CurPos implements GradientKind.
func (g EncodedFocalGradient) CurPos(pos curve.Point) float32 {
	f, _ := g.posInner(pos)
	return f
}

// HasUndefined implements GradientKind.
func (g EncodedFocalGradient) HasUndefined() bool {
	return !g.focalData.wellBehaved()
}

// IsDefined implements GradientKind.
func (g EncodedFocalGradient) IsDefined(pos curve.Point) bool {
	_, ok := g.posInner(pos)
	return ok
}

func (g EncodedStripGradient) posInner(pos curve.Point) (float32, bool) {
	p1 := g.r0ScaledSquared - float32(pos.Y)*float32(pos.Y)
	if p1 < 0 {
		return 0, false
	} else {
		return float32(pos.X) + math32.Sqrt(p1), true
	}
}

func (g EncodedFocalGradient) posInner(pos curve.Point) (float32, bool) {
	fp0 := g.fp0
	fp1 := g.fp1

	x := float32(pos.X)
	y := float32(pos.Y)

	var t float32
	if g.focalData.focalOnCircle() {
		// xy_to_2pt_conical_focal_on_circle
		t = x + y*y/x
	} else if g.focalData.wellBehaved() {
		// xy_to_2pt_conical_well_behaved
		t = math32.Sqrt(x*x+y*y) - x*fp0
	} else if g.focalData.swapped() || (1.0-g.focalData.fFocalX < 0.0) {
		// xy_to_2pt_conical_smaller
		t = -math32.Sqrt(x*x-y*y) - x*fp0
	} else {
		// xy_to_2pt_conical_greater
		t = math32.Sqrt(x*x-y*y) - x*fp0
	}

	if !g.focalData.wellBehaved() {
		// mask_2pt_conical_degenerates
		degenerate := t <= 0.0 || math32.IsNaN(t)

		if degenerate {
			return 0, false
		}
	}

	if 1.0-g.focalData.fFocalX < 0.0 {
		// negate_x
		t = -t
	}

	if !g.focalData.nativelyFocal() {
		// alter_2pt_conical_compensate_focal
		t += fp1
	}

	if g.focalData.swapped() {
		// alter_2pt_conical_unswap
		t = 1.0 - t
	}

	return t, true
}

type EncodedGradient struct {
	Kind GradientKind
	// A Transform that needs to be applied to the position of the first
	// processed pixel.
	Transform curve.Affine
	// How much to advance along the x and y directions in the gradient for one
	// step in the x direction in the output image.
	XAdvance curve.Vec2
	// How much to advance along the x and y directions in the gradient for one
	// step in the y direction in the output image.
	YAdvance curve.Vec2
	// The color Ranges of the gradient.
	Ranges []GradientRange
	// Whether the gradient should be padded.
	Pad bool
	// Whether the gradient requires `source_over` compositing.
	HasOpacities bool

	LUT GradientLUT
}

type GradientLUT struct {
	LUT   [][4]float32
	Scale float32
}

func MakeGradientLUT(ranges []GradientRange) GradientLUT {
	// 11 bits of gradient accuracy. Good enough for our GUI rendering purposes.
	const lutSize = 2048
	const invLutSize = 1.0 / (lutSize - 1)
	lut := make([][4]float32, 0, lutSize)
	curIdx := 0
	for idx := range lutSize {
		tVal := float32(idx) * invLutSize
		for ranges[curIdx].X1 < tVal {
			curIdx++
		}
		rng := &ranges[curIdx]
		bias := rng.Bias
		interpolated := [4]float32{
			bias[0] + rng.Scale[0]*tVal,
			bias[1] + rng.Scale[1]*tVal,
			bias[2] + rng.Scale[2]*tVal,
			bias[3] + rng.Scale[3]*tVal,
		}
		lut = append(lut, interpolated)
	}

	return GradientLUT{
		LUT:   lut,
		Scale: lutSize - 1,
	}
}

// isEncodedPaint implements encodedPaint.
func (e *EncodedGradient) isEncodedPaint() {}

// Opaque implements encodedPaint.
func (e *EncodedGradient) Opaque() bool {
	return !e.HasOpacities
}

type GradientRange struct {
	// The end value of the range.
	X1 float32
	// A Bias to apply when interpolating the color (in this case just the
	// values of the start color of the gradient).
	Bias [4]float32
	// The Scale factors of the range. By calculating bias + x * factors (where
	// x is between 0.0 and 1.0), we can interpolate between start and end color
	// of the gradient range.
	Scale [4]float32
}

const degenerateThreshold = 1.0e-6

func degeneratePoint(p1 curve.Point, p2 curve.Point) bool {
	return math.Abs(p1.X-p2.X) <= degenerateThreshold &&
		math.Abs(p1.Y-p2.Y) <= degenerateThreshold
}

func degenerateVal(v1 float32, v2 float32) bool {
	return math32.Abs(v2-v1) <= degenerateThreshold
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
func encodeStops(stops []GradientStop, space *color.Space) []GradientRange {
	createRange := func(left_stop, right_stop encodedGradientStop) GradientRange {
		x0 := left_stop.offset
		x1 := right_stop.offset
		c0 := left_stop.color
		c1 := right_stop.color

		// We calculate a bias and scale factor, such that we can simply
		// calculate bias + x * scale to get the interpolated color, where x is
		// between x0 and x1, to calculate the resulting color.

		// We call this method with two same stops for `left_range` and
		// `right_range`, so make sure we don't actually end up with a 0 here.
		x1MinusX0 := max(x1-x0, 0.0000001)
		scale := [4]float32{
			(c1[0] - c0[0]) / x1MinusX0,
			(c1[1] - c0[1]) / x1MinusX0,
			(c1[2] - c0[2]) / x1MinusX0,
			(c1[3] - c0[3]) / x1MinusX0,
		}
		bias := [4]float32{
			c0[0] - x0*scale[0],
			c0[1] - x0*scale[1],
			c0[2] - x0*scale[2],
			c0[3] - x0*scale[3],
		}

		return GradientRange{x1, bias, scale}
	}

	encodedStops := make([]encodedGradientStop, 0, len(stops)*2)
	for i := range stops[:len(stops)-1] {
		left := stops[i]
		right := stops[i+1]
		for t, c := range approximateGradient(left.Color, right.Color, space, 0.01) {
			stop := encodedGradientStop{
				left.Offset + (right.Offset-left.Offset)*t,
				c,
			}
			encodedStops = append(encodedStops, stop)
		}
	}

	stopRanges := make([]GradientRange, len(encodedStops)-1)
	for i := range encodedStops[:len(encodedStops)-1] {
		stopRanges[i] = createRange(encodedStops[i], encodedStops[i+1])
	}
	return stopRanges
}

func (*EncodedGradient) String() string { return "Gradient" }

type encodedGradientStop struct {
	offset float32
	color  PlainColor
}

type FocalData struct {
	fr1        float32
	fFocalX    float32
	fIsSwapped bool
}

func (fd FocalData) focalOnCircle() bool {
	return isNearlyZero32(1.0 - fd.fr1)
}

func (fd FocalData) swapped() bool {
	return fd.fIsSwapped
}

func (fd FocalData) wellBehaved() bool {
	return !fd.focalOnCircle() && fd.fr1 > 1.0
}

func (fd FocalData) nativelyFocal() bool {
	return isNearlyZero32(fd.fFocalX)
}

// approximateGradient takes two color stops of a gradient, the color space in
// which to interpolate the gradient, and a tolerance. It returns a sequence of
// new color stops that when interpolated in [ColorSpace] approximate the
// original gradient to the specified tolerance at every point. Tolerance is
// specified as the Euclidean distance between original and approximated colors,
// in the Oklab color space.
func approximateGradient(
	start, end color.Color,
	cs *color.Space,
	tol float32,
) iter.Seq2[float32, PlainColor] {
	// TODO(dh): support cylindrical color spaces

	return func(yield func(float32, PlainColor) bool) {
		interpolator := Interpolate(start, end, cs)
		target0 := ColorToInternal(start)
		target1 := ColorToInternal(end)
		endColor := target1
		var t0 uint32
		var dt float32

	yieldLoop:
		for {
			if dt == 0 {
				dt = 1
				if !yield(0, target0) {
					return
				}
				continue yieldLoop
			}
			_t0 := float32(t0) * dt
			if _t0 == 1 {
				return
			}
			for {
				// compute midpoint color

				// OPT(dh): there's a stupid amount of going between straight
				// and premultiplied here. can we avoid that?
				midpoint := interpolator.Evaluate(float64(_t0 + 0.5*dt))
				midpointOklab := midpoint.Convert(color.Oklab)
				midpointOklabPm := [4]float64{
					midpointOklab.Values[0] * midpointOklab.Values[3],
					midpointOklab.Values[1] * midpointOklab.Values[3],
					midpointOklab.Values[2] * midpointOklab.Values[3],
					midpointOklab.Values[3],
				}
				approxPm := PlainColor{
					target0[0] + 0.5*(target1[0]-target0[0]),
					target0[1] + 0.5*(target1[1]-target0[1]),
					target0[2] + 0.5*(target1[2]-target0[2]),
					target0[3] + 0.5*(target1[3]-target0[3]),
				}
				var approxStraight [4]float32
				if approxPm[3] == 0 || approxPm[3] == 1 {
					approxStraight = approxPm
				} else {
					approxStraight = [4]float32{
						approxPm[0] / approxPm[3],
						approxPm[1] / approxPm[3],
						approxPm[2] / approxPm[3],
						approxPm[3],
					}
				}
				approxOklab := color.Color{
					Values: [4]float64{
						float64(approxStraight[0]),
						float64(approxStraight[1]),
						float64(approxStraight[2]),
						float64(approxStraight[3]),
					},
					Space: ColorSpace,
				}.Convert(color.Oklab)
				approxOklabPm := [4]float64{
					approxOklab.Values[0] * approxOklab.Values[3],
					approxOklab.Values[1] * approxOklab.Values[3],
					approxOklab.Values[2] * approxOklab.Values[3],
					approxOklab.Values[3],
				}
				d := [4]float64{
					midpointOklabPm[0] - approxOklabPm[0],
					midpointOklabPm[1] - approxOklabPm[1],
					midpointOklabPm[2] - approxOklabPm[2],
					midpointOklabPm[3] - approxOklabPm[3],
				}
				error := float32(math.Sqrt(d[0]*d[0] + d[1]*d[1] + d[2]*d[2] + d[3]*d[3]))
				if error <= tol {
					t1 := _t0 + dt
					t0++
					shift := bits.TrailingZeros32(t0)
					t0 >>= shift
					dt *= float32(uint32(1) << shift)
					target0 = target1
					newT1 := t1 + dt
					if newT1 < 1 {
						target1 = ColorToInternal(interpolator.Evaluate(float64(newT1)))
					} else {
						target1 = endColor
					}
					if !yield(t1, target0) {
						return
					}
					continue yieldLoop
				}
				t0 *= 2
				dt *= 0.5
				target1 = ColorToInternal(midpoint)
			}
		}
	}
}

// mapLineToLine computes the transform necessary to map the line spanned by
// points src1 and src2 to the line spanned by dst1 and dst2.
//
// This creates a transformation that maps any line segment to any other line
// segment. For gradients, we use this to transform the gradient line to a
// standard form (0,0) → (1,0).
func mapLineToLine(src1, src2, dst1, dst2 curve.Point) curve.Affine {
	unitToLine1 := mapUnitToLine(src1, src2)
	// Calculate the transform necessary to map line1 to the unit vector.
	line1ToUnit := unitToLine1.Invert()
	// Then map the unit vector to line2.
	unitToLine2 := mapUnitToLine(dst1, dst2)

	return unitToLine2.Mul(line1ToUnit)
}

// Calculate the transform necessary to map the unit vector to the line spanned
// by the points p1 and p2.
func mapUnitToLine(p0, p1 curve.Point) curve.Affine {
	return curve.Affine{
		N0: p1.Y - p0.Y,
		N1: p0.X - p1.X,
		N2: p1.X - p0.X,
		N3: p1.Y - p0.Y,
		N4: p0.X,
		N5: p0.Y,
	}
}

func createFocalData(r0, r1 float32, matrix curve.Affine) (FocalData, curve.Affine) {
	swapped := false
	fFocalX := r0 / (r0 - r1)

	if isNearlyZero32(fFocalX - 1.0) {
		matrix = matrix.ThenTranslate(curve.Vec(-1.0, 0.0))
		matrix = matrix.ThenScale(-1.0, 1.0)
		r1 = r0
		fFocalX = 0.0
		swapped = true
	}

	focalMatrix := mapLineToLine(
		curve.Pt(float64(fFocalX), 0.0),
		curve.Pt(1.0, 0.0),
		curve.Pt(0.0, 0.0),
		curve.Pt(1.0, 0.0),
	)
	matrix = focalMatrix.Mul(matrix)

	fr1 := r1 / math32.Abs(1.0-fFocalX)

	data := FocalData{
		fr1,
		fFocalX,
		swapped,
	}

	if data.focalOnCircle() {
		matrix = matrix.ThenScale(0.5, 0.5)
	} else {
		matrix = matrix.ThenScale(
			float64(fr1/(fr1*fr1-1.0)),
			1.0/math.Sqrt(math.Abs(float64(fr1*fr1-1.0))),
		)
	}

	f := math.Abs(float64(1.0 - fFocalX))
	matrix = matrix.ThenScale(f, f)

	return data, matrix
}

func isNearlyZero(f float64) bool {
	return math.Abs(f) <= 1.0/(1<<12)
}

func isNearlyZero32(f float32) bool {
	return math32.Abs(f) <= 1.0/(1<<12)
}
