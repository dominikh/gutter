// SPDX-FileCopyrightText: 2024 Dominik Honnef and contributors
//
// SPDX-License-Identifier: MIT

package harfbuzz

// #include <harfbuzz/hb.h>
// #include "./paint.h"
import "C"
import (
	"bytes"
	"image"
	"image/png"
	"runtime/cgo"
	"sort"
	"unsafe"

	"honnef.co/go/color"
	"honnef.co/go/curve"
	"honnef.co/go/gutter/gfx"
	"honnef.co/go/safeish"
)

var paintFuncs *C.hb_paint_funcs_t

func init() {
	fns := C.hb_paint_funcs_create()
	C.hb_paint_funcs_set_linear_gradient_func(fns, fp(C.linearGradient), nil, nil)
	C.hb_paint_funcs_set_radial_gradient_func(fns, fp(C.radialGradient), nil, nil)
	C.hb_paint_funcs_set_sweep_gradient_func(fns, fp(C.sweepGradient), nil, nil)
	C.hb_paint_funcs_set_push_group_func(fns, fp(C.pushGroup), nil, nil)
	C.hb_paint_funcs_set_pop_group_func(fns, fp(C.popGroup), nil, nil)
	C.hb_paint_funcs_set_push_transform_func(fns, fp(C.pushTransform), nil, nil)
	C.hb_paint_funcs_set_pop_transform_func(fns, fp(C.popTransform), nil, nil)
	C.hb_paint_funcs_set_color_glyph_func(fns, fp(C.colorGlyph), nil, nil)
	C.hb_paint_funcs_set_push_clip_glyph_func(fns, fp(C.pushClipGlyph), nil, nil)
	C.hb_paint_funcs_set_push_clip_rectangle_func(fns, fp(C.pushClipRectangle), nil, nil)
	C.hb_paint_funcs_set_pop_clip_func(fns, fp(C.popClip), nil, nil)
	C.hb_paint_funcs_set_color_func(fns, fp(C.colorFunc), nil, nil)
	C.hb_paint_funcs_set_image_func(fns, fp(C.imageFunc), nil, nil)
	C.hb_paint_funcs_make_immutable(fns)

	paintFuncs = fns
}

var compositeToBlend = [...]gfx.BlendMode{
	C.HB_PAINT_COMPOSITE_MODE_CLEAR:       {Compose: gfx.ComposeClear},
	C.HB_PAINT_COMPOSITE_MODE_SRC:         {Compose: gfx.ComposeCopy},
	C.HB_PAINT_COMPOSITE_MODE_DEST:        {Compose: gfx.ComposeDest},
	C.HB_PAINT_COMPOSITE_MODE_SRC_OVER:    {Compose: gfx.ComposeSrcOver},
	C.HB_PAINT_COMPOSITE_MODE_DEST_OVER:   {Compose: gfx.ComposeDestOver},
	C.HB_PAINT_COMPOSITE_MODE_SRC_IN:      {Compose: gfx.ComposeSrcIn},
	C.HB_PAINT_COMPOSITE_MODE_DEST_IN:     {Compose: gfx.ComposeDestIn},
	C.HB_PAINT_COMPOSITE_MODE_SRC_OUT:     {Compose: gfx.ComposeSrcOut},
	C.HB_PAINT_COMPOSITE_MODE_DEST_OUT:    {Compose: gfx.ComposeDestOut},
	C.HB_PAINT_COMPOSITE_MODE_SRC_ATOP:    {Compose: gfx.ComposeSrcAtop},
	C.HB_PAINT_COMPOSITE_MODE_DEST_ATOP:   {Compose: gfx.ComposeDestAtop},
	C.HB_PAINT_COMPOSITE_MODE_XOR:         {Compose: gfx.ComposeXor},
	C.HB_PAINT_COMPOSITE_MODE_PLUS:        {Compose: gfx.ComposePlus},
	C.HB_PAINT_COMPOSITE_MODE_SCREEN:      {Mix: gfx.MixScreen},
	C.HB_PAINT_COMPOSITE_MODE_OVERLAY:     {Mix: gfx.MixOverlay},
	C.HB_PAINT_COMPOSITE_MODE_DARKEN:      {Mix: gfx.MixDarken},
	C.HB_PAINT_COMPOSITE_MODE_LIGHTEN:     {Mix: gfx.MixLighten},
	C.HB_PAINT_COMPOSITE_MODE_COLOR_DODGE: {Mix: gfx.MixColorDodge},
	C.HB_PAINT_COMPOSITE_MODE_COLOR_BURN:  {Mix: gfx.MixColorBurn},
	C.HB_PAINT_COMPOSITE_MODE_HARD_LIGHT:  {Mix: gfx.MixHardLight},
	C.HB_PAINT_COMPOSITE_MODE_SOFT_LIGHT:  {Mix: gfx.MixSoftLight},
	C.HB_PAINT_COMPOSITE_MODE_DIFFERENCE:  {Mix: gfx.MixDifference},
	C.HB_PAINT_COMPOSITE_MODE_EXCLUSION:   {Mix: gfx.MixExclusion},
	C.HB_PAINT_COMPOSITE_MODE_MULTIPLY:    {Mix: gfx.MixMultiply},
	// XXX support these
	// C.HB_PAINT_COMPOSITE_MODE_HSL_HUE:        {Mix: gfx.MixHue},
	// C.HB_PAINT_COMPOSITE_MODE_HSL_SATURATION: {Mix: gfx.MixSaturation},
	// C.HB_PAINT_COMPOSITE_MODE_HSL_COLOR:      {Mix: gfx.MixColor},
	// C.HB_PAINT_COMPOSITE_MODE_HSL_LUMINOSITY: {Mix: gfx.MixLuminosity},
}

type GlyphPainter interface {
	Foreground() color.Color
	PushGroup()
	PopGroup(mode gfx.BlendMode)
	PushTransform(t curve.Affine)
	PopTransform()
	ColorGlyph(glyph int32) bool
	PushClipGlyph(glyph int32)
	PushClipRect(rect curve.Rect)
	PopClip()
	Fill(b gfx.Paint)
	Image(img image.Image, slant float64, extents GlyphExtents) bool
}

//export linearGradient
func linearGradient(
	pfuncs *C.hb_paint_funcs_t,
	paintData C.uintptr_t,
	line *C.hb_color_line_t,
	x0, y0, x1, y1, x2, y2 C.float,
	userData C.uintptr_t,
) {
	p0 := curve.Vec(float64(x0), float64(-y0))
	p1 := curve.Vec(float64(x1), float64(-y1))
	p2 := curve.Vec(float64(x2), float64(-y2))

	if p0 == p1 || p0 == p2 {
		// Degenerate lines, don't try to render.
		return
	}

	if p1.Sub(p0).Cross(p2.Sub(p0)) == 0 {
		// p0p1 and p0p2 are parallel, don't try to render.
		return
	}

	painter := cgo.Handle(paintData).Value().(*painter)

	// XXX normalize stops to be in the range [0, 1]
	// XXX handle lines with zero and one stops
	stops := colorLineStops(line)
	for i := range stops {
		stop := &stops[i]
		if stop.Color == (color.Color{}) {
			// TODO currently, the user can specify any brush as the foreground
			// paint. This makes perfect sense for normal fonts and when filling
			// glyphs in color fonts, but it doesn't make sense as part of a
			// gradient. For now, we fall back to solid black if the provided
			// brush isn't a solid color, but maybe we should let the user
			// specify a fallback color for that.
			stop.Color = painter.Foreground()
		}
	}

	// COLRv1 specifies linear gradients using two lines, p0p1 and p0p2. p0p2
	// acts as a rotation of p0p1. We can compute an equivalent linear gradient
	// using a single line p0p3, which is the orthogonal projection of the
	// vector from p0 to p1 onto a line that is perpendicular to the line p0p2
	// and that passes through p0. See
	// https://learn.microsoft.com/en-us/typography/opentype/spec/colr#linear-gradients
	// for more details.
	tmp := p2.Sub(p0)
	perpendicularTop0p2 := curve.Vec(tmp.Y, -tmp.X)
	projectOnto := func(v, p curve.Vec2) curve.Vec2 {
		length := p.Hypot()
		if length == 0 {
			return curve.Vec2{}
		}
		return p.Div(length).Mul(v.Dot(p) / length)
	}
	p3 := p0.Add(projectOnto(p1.Sub(p0), perpendicularTop0p2))

	g := &gfx.LinearGradient{
		Start: curve.Point(p0),
		End:   curve.Point(p3),
		Stops: stops,
		// XXX set Extend
	}

	painter.Fill(g)
}

//export radialGradient
func radialGradient(
	pfuncs *C.hb_paint_funcs_t,
	paintData C.uintptr_t,
	line *C.hb_color_line_t,
	x0, y0, r0, x1, y1, r1 C.float,
	userData C.uintptr_t,
) {
	// XXX verify that the COLRv1 spec matches Vello radial gradients
	g := &gfx.RadialGradient{
		StartCenter: curve.Pt(float64(x0), float64(-y0)),
		StartRadius: float32(r0),
		EndCenter:   curve.Pt(float64(x1), float64(-y1)),
		EndRadius:   float32(r1),
		// XXX handle foreground color in stops
		// XXX normalize stops
		Stops: colorLineStops(line),
		// XXX set Extend,
	}
	painter := cgo.Handle(paintData).Value().(*painter)
	painter.Fill(g)
}

//export sweepGradient
func sweepGradient(
	pfuncs *C.hb_paint_funcs_t,
	paintData C.uintptr_t,
	line *C.hb_color_line_t,
	x0, y0, startAngle, endAngle C.float,
	userData C.uintptr_t,
) {
	// XXX implement

	// cgo.Handle(paintData).Value().(GlyphPainter).SweepGradient(
	// 	ColorLine{line},
	// 	float64(x0),
	// 	float64(y0),
	// 	float64(startAngle),
	// 	float64(endAngle),
	// )
}

//export pushGroup
func pushGroup(
	pfuncs *C.hb_paint_funcs_t,
	paintData C.uintptr_t,
	userData C.uintptr_t,
) {
	cgo.Handle(paintData).Value().(*painter).PushGroup()
}

//export popGroup
func popGroup(
	pfuncs *C.hb_paint_funcs_t,
	paintData C.uintptr_t,
	mode C.hb_paint_composite_mode_t,
	userData C.uintptr_t,
) {
	cgo.Handle(paintData).Value().(*painter).PopGroup(compositeToBlend[mode])
}

//export pushTransform
func pushTransform(
	pfuncs *C.hb_paint_funcs_t,
	paintData C.uintptr_t,
	xx, yx, xy, yy, dx, dy C.float,
	userData C.uintptr_t,
) {
	norm := func(f float64) float64 {
		if f == -0 {
			return 0
		} else {
			return f
		}
	}
	aff := curve.Affine{
		N0: norm(float64(xx)),
		N1: norm(float64(yx)),
		N2: norm(float64(xy)),
		N3: norm(float64(yy)),
		N4: norm(float64(dx)),
		N5: norm(float64(-dy)),
	}
	painter := cgo.Handle(paintData).Value().(*painter)
	painter.PushTransform(aff)
}

//export popTransform
func popTransform(
	pfuncs *C.hb_paint_funcs_t,
	paintData C.uintptr_t,
	userData C.uintptr_t,
) {
	painter := cgo.Handle(paintData).Value().(*painter)
	painter.PopTransform()
}

//export popClip
func popClip(
	pfuncs *C.hb_paint_funcs_t,
	paintData C.uintptr_t,
	userData C.uintptr_t,
) {
	cgo.Handle(paintData).Value().(*painter).PopClip()
}

//export imageFunc
func imageFunc(
	pfuncs *C.hb_paint_funcs_t,
	paintData C.uintptr_t,
	img *C.hb_blob_t,
	width C.uint,
	height C.uint,
	format C.hb_tag_t,
	slant C.float,
	extents *C.hb_glyph_extents_t,
	userData C.uintptr_t,
) C.hb_bool_t {
	// XXX flip Y coords

	var n C.uint
	cdata := C.hb_blob_get_data(img, &n)

	// OPT instead of copying the data, track the image's lifetime and handle
	// the blob's reference count appropriately.
	data := C.GoBytes(unsafe.Pointer(cdata), C.int(n))

	// XXX Do we need to decrease img's reference count?

	switch format {
	case C.HB_PAINT_IMAGE_FORMAT_PNG:
		// OPT somehow avoid decoding the same image over and over
		img, err := png.Decode(bytes.NewReader(data))
		if err != nil {
			return 0
		}
		if (width != 0 && int(width) != img.Bounds().Dx()) || (height != 0 && int(height) != img.Bounds().Dy()) {
			// The image's dimensions don't match the dimensions passed to this
			// function.
			//
			// TODO what should we do in that case?
			return 0
		}
		ret := cgo.Handle(paintData).Value().(*painter).Image(img, float64(slant), *safeish.Cast[*GlyphExtents](extents))
		if ret {
			return 1
		} else {
			return 0
		}
	case C.HB_PAINT_IMAGE_FORMAT_SVG:
		// We don't support SVG.
		return 0
	case C.HB_PAINT_IMAGE_FORMAT_BGRA:
		// TODO support BGRA
		return 0
	default:
		// Not a tag we know, assume it's from a newer version of Harfbuzz.
		return 0
	}
}

//export colorGlyph
func colorGlyph(
	pfuncs *C.hb_paint_funcs_t,
	paintData C.uintptr_t,
	glyph C.hb_codepoint_t,
	font *C.hb_font_t,
	userData C.uintptr_t,
) C.hb_bool_t {
	// XXX implement
	return 0
}

//export colorFunc
func colorFunc(
	pfuncs *C.hb_paint_funcs_t,
	paintData C.uintptr_t,
	isForeground C.hb_bool_t,
	c C.hb_color_t,
	userData C.uintptr_t,
) {
	painter := cgo.Handle(paintData).Value().(*painter)
	if isForeground != 0 {
		painter.Fill(gfx.Solid(painter.Foreground()))
	} else {
		painter.Fill(gfx.Solid(hbColorToColor(c)))
	}
}

func hbColorToColor(c C.hb_color_t) color.Color {
	r := float64((c>>8)&0xFF) / 255
	g := float64((c>>16)&0xFF) / 255
	b := float64((c>>24)&0xFF) / 255
	a := float64(c&0xFF) / 255

	return color.Make(color.SRGB, r, g, b, a)
}

//export pushClipGlyph
func pushClipGlyph(
	pfuncs *C.hb_paint_funcs_t,
	paintData C.uintptr_t,
	glyph C.hb_codepoint_t,
	font *C.hb_font_t,
	userData C.uintptr_t,
) {
	cgo.Handle(paintData).Value().(*painter).PushClipGlyph(int32(glyph))
}

//export pushClipRectangle
func pushClipRectangle(
	pfuncs *C.hb_paint_funcs_t,
	paintData C.uintptr_t,
	xmin, ymin, xmax, ymax C.float,
	userData C.uintptr_t,
) {
	rect := curve.NewRectFromPoints(
		curve.Pt(float64(xmin), float64(-ymin)),
		curve.Pt(float64(xmax), float64(-ymax))).Abs()
	cgo.Handle(paintData).Value().(*painter).PushClipRect(rect)
}

func colorLineStops(l *C.hb_color_line_t) []gfx.GradientStop {
	var n C.uint
	cnt := C.hb_color_line_get_color_stops(l, 0, &n, nil)
	cstops := make([]C.hb_color_stop_t, cnt)
	n = cnt
	C.hb_color_line_get_color_stops(l, 0, &n, &cstops[0])

	stops := make([]gfx.GradientStop, n)
	for i, cstop := range cstops {
		stop := gfx.GradientStop{
			Offset: float32(cstop.offset),
		}
		if cstop.is_foreground == 0 {
			stop.Color = hbColorToColor(cstop.color)
		}
		stops[i] = stop
	}
	sort.Slice(stops, func(i, j int) bool {
		return stops[i].Offset < stops[j].Offset
	})

	return stops
}

type painter struct {
	// Wrapping the GlyphPainter in a painter means that our type assertions on
	// cgo handle values can be for the concrete type *painter, which is cheaper
	// than asserting to the GlyphPainter interface.
	GlyphPainter
}
