// SPDX-FileCopyrightText: 2024 Dominik Honnef and contributors
//
// SPDX-License-Identifier: MIT

package opentype

type GlyphTableKind int

const (
	SimpleGlyphTableKind GlyphTableKind = iota
	CompositeGlyphTableKind
)

func (tbl *GlyfTable) Glyph(offset uint32) GlyphTable {
	// XXX check bounds
	data := tbl.Data[offset:]

	var num int16
	parseInt16(data, &num)
	if num >= 0 {
		out := GlyphTable{
			Kind: SimpleGlyphTableKind,
		}
		ParseSimpleGlyphTable(data, &out.Simple)
		return out
	} else {
		out := GlyphTable{
			Kind: SimpleGlyphTableKind,
		}
		ParseCompositeGlyphTable(data, &out.Composite)
		return out
	}
}

type GlyphTable struct {
	Kind GlyphTableKind

	Simple    SimpleGlyphTable
	Composite CompositeGlyphTable
}
