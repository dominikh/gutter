// SPDX-FileCopyrightText: 2024 Dominik Honnef and contributors
//
// SPDX-License-Identifier: MIT

package harfbuzz

// #include <harfbuzz/hb.h>
// #include "./draw.h"
import "C"
import (
	"runtime/cgo"
)

type fp = *[0]byte

type GlyphDrawer interface {
	MoveTo(x, y float64)
	LineTo(x, y float64)
	CubicTo(cx1, cy1, cx2, cy2, x, y float64)
	ClosePath()
}

//export moveTo
func moveTo(
	dfuncs *C.hb_draw_funcs_t,
	drawData C.uintptr_t,
	st *C.hb_draw_state_t,
	x, y C.float,
	userData C.uintptr_t,
) {
	cgo.Handle(drawData).Value().(GlyphDrawer).MoveTo(float64(x), float64(y))
}

//export lineTo
func lineTo(
	dfuncs *C.hb_draw_funcs_t,
	drawData C.uintptr_t,
	st *C.hb_draw_state_t,
	x, y C.float,
	userData C.uintptr_t,
) {
	cgo.Handle(drawData).Value().(GlyphDrawer).LineTo(float64(x), float64(y))
}

//export cubicTo
func cubicTo(
	dfuncs *C.hb_draw_funcs_t,
	drawData C.uintptr_t,
	st *C.hb_draw_state_t,
	c1x, c1y, c2x, c2y, x, y C.float,
	userData C.uintptr_t,
) {
	cgo.Handle(drawData).Value().(GlyphDrawer).CubicTo(
		float64(c1x),
		float64(c1y),
		float64(c2x),
		float64(c2y),
		float64(x),
		float64(y),
	)
}

//export closePath
func closePath(
	dfuncs *C.hb_draw_funcs_t,
	drawData C.uintptr_t,
	st *C.hb_draw_state_t,
	userData C.uintptr_t,
) {
	cgo.Handle(drawData).Value().(GlyphDrawer).ClosePath()
}

var drawFuncs *C.hb_draw_funcs_t

func init() {
	fns := C.hb_draw_funcs_create()
	C.hb_draw_funcs_set_move_to_func(fns, fp(C.moveTo), nil, nil)
	C.hb_draw_funcs_set_line_to_func(fns, fp(C.lineTo), nil, nil)
	C.hb_draw_funcs_set_cubic_to_func(fns, fp(C.cubicTo), nil, nil)
	C.hb_draw_funcs_set_close_path_func(fns, fp(C.closePath), nil, nil)
	C.hb_draw_funcs_make_immutable(fns)

	drawFuncs = fns
}
