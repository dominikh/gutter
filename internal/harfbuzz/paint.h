/*
 * SPDX-FileCopyrightText: 2024 Dominik Honnef and contributors
 *
 * SPDX-License-Identifier: MIT
 */

#include <harfbuzz/hb.h>

extern void linearGradient(hb_paint_funcs_t* pfuncs, uintptr_t paintData, hb_color_line_t* line, float x0, float y0, float x1, float y1, float x2, float y2, uintptr_t userData);
extern void radialGradient(hb_paint_funcs_t* pfuncs, uintptr_t paintData, hb_color_line_t* line, float x0, float y0, float r0, float x1, float y1, float r1, uintptr_t userData);
extern void sweepGradient(hb_paint_funcs_t* pfuncs, uintptr_t paintData, hb_color_line_t* line, float x0, float y0, float startAngle, float endAngle, uintptr_t userData);
extern void pushGroup(hb_paint_funcs_t* pfuncs, uintptr_t paintData, uintptr_t userData);
extern void popGroup(hb_paint_funcs_t* pfuncs, uintptr_t paintData, hb_paint_composite_mode_t mode, uintptr_t userData);
extern void pushTransform(hb_paint_funcs_t* pfuncs, uintptr_t paintData, float xx, float yx, float xy, float yy, float dx, float dy, uintptr_t userData);
extern void popTransform(hb_paint_funcs_t* pfuncs, uintptr_t paintData, uintptr_t userData);
extern void popClip(hb_paint_funcs_t* pfuncs, uintptr_t paintData, uintptr_t userData);
extern hb_bool_t imageFunc(hb_paint_funcs_t* pfuncs, uintptr_t paintData, hb_blob_t* img, unsigned int width, unsigned int height, hb_tag_t format, float slant, hb_glyph_extents_t* extents, uintptr_t userData);
extern hb_bool_t customPaletteColor(hb_paint_funcs_t* pfuncs, uintptr_t paintData, unsigned int colorIndex, hb_color_t* c, uintptr_t userData);
extern hb_bool_t colorGlyph(hb_paint_funcs_t* pfuncs, uintptr_t paintData, hb_codepoint_t glyph, hb_font_t* font, uintptr_t userData);
extern void colorFunc(hb_paint_funcs_t* pfuncs, uintptr_t paintData, hb_bool_t isForeground, hb_color_t c, uintptr_t userData);
extern void pushClipGlyph(hb_paint_funcs_t* pfuncs, uintptr_t paintData, hb_codepoint_t glyph, hb_font_t* font, uintptr_t userData);
extern void pushClipRectangle(hb_paint_funcs_t* pfuncs, uintptr_t paintData, float xmin, float ymin, float xmax, float ymax, uintptr_t userData);

void my_hb_font_paint_glyph(
  hb_font_t *font,
  hb_codepoint_t glyph,
  hb_paint_funcs_t *pfuncs,
  uintptr_t paint_data,
  unsigned int palette_index,
  hb_color_t foreground
);
