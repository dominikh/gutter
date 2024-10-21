// SPDX-FileCopyrightText: 2024 Dominik Honnef and contributors
//
// SPDX-License-Identifier: MIT

package tables

import "honnef.co/go/gutter/opentype"

type ShortLocaTable struct {
	// Offsets divided by 2 to locations of glyphs, relative to the beginning of the glyf
	// table.
	offsets opentype.Slice[uint16] `gen:"slice(count=-1)"`
}

type LongLocaTable struct {
	// Offsets to locations of glyphs, relative to the beginning of the glyf table.
	offsets opentype.Slice[uint32] `gen:"slice(count=-1)"`
}
