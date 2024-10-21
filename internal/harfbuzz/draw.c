/*
 * SPDX-FileCopyrightText: 2024 Dominik Honnef and contributors
 *
 * SPDX-License-Identifier: MIT
 */

#include <harfbuzz/hb.h>
#include "./draw.h"

void my_hb_font_draw_glyph(
  hb_font_t *font,
  hb_codepoint_t glyph,
  hb_draw_funcs_t *dfuncs,
  uintptr_t draw_data
) {
  hb_font_draw_glyph(font, glyph, dfuncs, (void*) draw_data);
}
