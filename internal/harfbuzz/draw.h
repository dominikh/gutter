/*
 * SPDX-FileCopyrightText: 2024 Dominik Honnef and contributors
 *
 * SPDX-License-Identifier: MIT
 */

#include <harfbuzz/hb.h>

void moveTo(
   hb_draw_funcs_t *dfuncs,
   uintptr_t draw_data,
   hb_draw_state_t *st,
   float to_x,
   float to_y,
   uintptr_t user_data
);

void lineTo(
   hb_draw_funcs_t *dfuncs,
   uintptr_t draw_data,
   hb_draw_state_t *st,
   float to_x,
   float to_y,
   uintptr_t user_data
);

void cubicTo(
   hb_draw_funcs_t *dfuncs,
   uintptr_t draw_data,
   hb_draw_state_t *st,
   float c1x,
   float c1y,
   float c2x,
   float c2y,
   float to_x,
   float to_y,
   uintptr_t user_data
);

void closePath(
   hb_draw_funcs_t *dfuncs,
   uintptr_t draw_data,
   hb_draw_state_t *st,
   uintptr_t user_data
);


void my_hb_font_draw_glyph(
  hb_font_t *font,
  hb_codepoint_t glyph,
  hb_draw_funcs_t *dfuncs,
  uintptr_t draw_data
);
