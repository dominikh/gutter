// SPDX-FileCopyrightText: 2024 Dominik Honnef and contributors
//
// SPDX-License-Identifier: MIT

package harfbuzz

// #include <harfbuzz/hb.h>
// #include "./draw.h"
// #include "./paint.h"
// #cgo noescape hb_font_get_glyph_extents
// #cgo nocallback hb_font_get_glyph_extents
// #cgo noescape hb_font_get_h_extents
// #cgo nocallback hb_font_get_h_extents
import "C"
import (
	"runtime/cgo"
	"structs"
	"unsafe"

	"honnef.co/go/gutter/opentype"
	"honnef.co/go/safeish"
)

type Font struct {
	_ structs.HostLayout
	c C.hb_font_t
}

func NewFont(face *Face) *Font {
	return safeish.Cast[*Font](C.hb_font_create(&face.c))
}

func (f *Font) SubFont() *Font {
	return safeish.Cast[*Font](C.hb_font_create_sub_font(&f.c))
}

func (f *Font) Face() *Face {
	return safeish.Cast[*Face](C.hb_font_get_face(&f.c))
}

func (f *Font) Destroy() {
	C.hb_font_destroy(&f.c)
}

func (f *Font) MakeImmutable() { C.hb_font_make_immutable(&f.c) }

func (f *Font) Immutable() bool { return C.hb_font_is_immutable(&f.c) != 0 }

func (f *Font) Scale() (x, y int) {
	var x_, y_ C.int
	C.hb_font_get_scale(&f.c, &x_, &y_)
	return int(x_), int(y_)
}

func (f *Font) SetVariations(values []Variation) {
	args := make([]C.hb_variation_t, len(values))
	for i, v := range values {
		args[i] = C.hb_variation_t{
			tag:   C.hb_tag_t(v.Tag.Uint32()),
			value: C.float(v.Value),
		}
	}
	C.hb_font_set_variations(&f.c, unsafe.SliceData(args), C.uint(len(args)))
}

func (f *Font) SetVariation(tag opentype.Tag, value float64) {
	C.hb_font_set_variation(&f.c, C.hb_tag_t(tag.Uint32()), C.float(value))
}

func (f *Font) SetNamedInstance(instance int) {
	C.hb_font_set_var_named_instance(&f.c, C.uint(instance))
}

func (f *Font) NamedInstance() int {
	return int(C.hb_font_get_var_named_instance(&f.c))
}

func (f *Font) DrawGlyph(glyph int32, d GlyphDrawer) {
	hnd := cgo.NewHandle(d)
	defer hnd.Delete()
	C.my_hb_font_draw_glyph(&f.c, C.hb_codepoint_t(glyph), drawFuncs, C.uintptr_t(hnd))
}

func (f *Font) PaintGlyph(glyph int32, p GlyphPainter) {
	pp := &painter{
		GlyphPainter: p,
	}

	hnd := cgo.NewHandle(pp)
	defer hnd.Delete()
	// XXX support passing palette index
	C.my_hb_font_paint_glyph(&f.c, C.hb_codepoint_t(glyph), paintFuncs, C.uintptr_t(hnd), 0, 0)
}

type GlyphExtents struct {
	structs.HostLayout

	XBearing int32
	YBearing int32
	Width    int32
	Height   int32
}

func (f *Font) GlyphExtents(glyph int32) (GlyphExtents, bool) {
	var extents GlyphExtents
	b := C.hb_font_get_glyph_extents(&f.c, C.hb_codepoint_t(glyph), safeish.Cast[*C.hb_glyph_extents_t](&extents))
	return extents, b != 0
}

type FontExtents struct {
	structs.HostLayout

	Ascender  int32
	Descender int32
	LineGap   int32

	_reserved [9]int32
}

func (f *Font) HorizontalExtents() (FontExtents, bool) {
	var extents FontExtents
	b := C.hb_font_get_h_extents(&f.c, safeish.Cast[*C.hb_font_extents_t](&extents))
	return extents, b != 0
}
