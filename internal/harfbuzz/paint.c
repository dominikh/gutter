/*
 * SPDX-FileCopyrightText: 2024 Dominik Honnef and contributors
 *
 * SPDX-License-Identifier: MIT
 */

#include <harfbuzz/hb.h>
#include "./paint.h"

void my_hb_font_paint_glyph(
  hb_font_t *font,
  hb_codepoint_t glyph,
  hb_paint_funcs_t *pfuncs,
  uintptr_t paint_data,
  unsigned int palette_index,
  hb_color_t foreground
) {
  hb_font_paint_glyph(font, glyph, pfuncs, (void*) paint_data, palette_index, foreground);
}
