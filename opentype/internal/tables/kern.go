// SPDX-FileCopyrightText: 2024 Dominik Honnef and contributors
//
// SPDX-License-Identifier: MIT

package tables

import "honnef.co/go/gutter/opentype"

// Apple specifies a backwards-incompatible version 1 of the kern table, which changes the
// size of existing fields. This is contrary to the general rule that versions represented
// as single values are meant to be minor versions, with newer versions being backwards
// compatible. Fonts using the newer format are only compatible with macOS, not Windows
// (but Harfbuzz supports it).
//
// Apple furthermore specifies two new subtable formats, at least one of which has more
// peculiarities, such as having an offset field that is reinterpreted as something else
// whenever its value is not the "expected" offset.
//
// But Apple _also_ introduced the kerx table, which is similar to their incompatible kern
// extension, but cleaned up and with fewer special cases. Because of that, we support the
// kerx table, but only version 0 of the kern table. Well-behaving cross-platform fonts
// won't be using version 1 kern tables, and modern Apple-specific fonts will use kerx
// tables. We do not care about supporting old Apple-specific fonts that use version 1
// kern tables.

type KernTable struct {
	Version   uint16
	NumTables uint16
	subtables []byte `gen:"slice(count=-1)"`
}

type KernSubTable struct {
	Version  uint16
	Length   uint16
	Coverage uint16

	Format0 KernSubtableFormat0 `gen:"union(out.Format(), 0)"`
	Format2 KernSubtableFormat2 `gen:"union(out.Format(), 2)"`
}

type KernSubtableFormat0 struct {
	nPairs         uint16
	_searchRange   uint16
	_entrySelector uint16
	_rnageShift    uint16

	pairs opentype.Slice[KerningPair] `gen:"slice(count=nPairs)"`
}

type KerningPair struct {
	Left  uint16
	Right uint16
	Value int16
}

type KernSubtableFormat2 struct {
	data []byte

	RowWidth           uint16
	LeftClassOffset    opentype.Offset16[KerningClassTable]
	RightClassOffset   opentype.Offset16[KerningClassTable]
	kerningArrayOffset uint16
}

type KerningClassTable struct {
	FirstGlyph uint16
	nGlyphs    uint16
	classes    opentype.Slice[uint16] `gen:"slice(count=nGlyphs,singular=class)"`
}
