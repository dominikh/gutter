// SPDX-FileCopyrightText: 2024 Dominik Honnef and contributors
//
// SPDX-License-Identifier: MIT

package tables

import "honnef.co/go/gutter/opentype"

const (
	GlyphClassBase      = 1
	GlyphClassLigature  = 2
	GlyphClassMark      = 3
	GlyphClassComponent = 4
)

type GDEFTable struct {
	data []byte

	MajorVersion             uint16 `gen:"1"`
	MinorVersion             uint16
	glyphClassDefOffset      opentype.Offset16[ClassDefTable]
	attachListOffset         opentype.Offset16[AttachListTable]
	ligCaretListOffset       opentype.Offset16[LigCaretListTable]
	markAttachClassDefOffset opentype.Offset16[ClassDefTable]

	_                     versionDelimiter `gen:"1.2"`
	markGlyphSetDefOffset opentype.Offset16[MarkGlyphSetsTable]

	_                  versionDelimiter `gen:"1.3"`
	itemVarStoreOffset opentype.Offset32[ItemVariationStoreTable]
}

type AttachListTable struct {
	data []byte

	coverageOffset     opentype.Offset16[CoverageTable]
	glyphCount         uint16 `gen:"omit()"`
	attachPointOffsets opentype.Slice[opentype.Offset16[AttachPointTable]]
}

type AttachPointTable struct {
	pointCount   uint16 `gen:"omit()"`
	pointIndices opentype.Slice[uint16]
}

type LigCaretListTable struct {
	data []byte

	coverageOffset  opentype.Offset16[CoverageTable]
	ligGlyphCount   uint16 `gen:"omit()"`
	ligGlyphOffsets opentype.Slice[opentype.Offset16[LigGlyphTable]]
}

type LigGlyphTable struct {
	data []byte

	caretCount        uint16 `gen:"omit()"`
	caretValueOffsets opentype.Slice[opentype.Offset16[CaretValueTable]]
}

type CaretValueTable struct {
	data []byte

	CaretValueFormat uint16
	Format1          CaretValueTableFormat1 `gen:"union(CaretValueFormat,1)"`
	Format2          CaretValueTableFormat2 `gen:"union(CaretValueFormat,2)"`
	Format3          CaretValueTableFormat3 `gen:"union(CaretValueFormat,3)"`
}

type CaretValueTableFormat1 struct {
	Coordinate int16
}

type CaretValueTableFormat2 struct {
	CaretValuePointIndex uint16
}

type CaretValueTableFormat3 struct {
	parentData []byte

	Coordinate int16
	// Offset to Device table for non-variable fonts or to variation index table for
	// variable fonts.
	DeviceOffset opentype.Offset16[any]
}

type MarkGlyphSetsTable struct {
	data []byte

	Format  uint16
	Format1 MarkGlyphSetsTableFormat1 `gen:"union(Format,1)"`
}

type MarkGlyphSetsTableFormat1 struct {
	parentData []byte

	markGlyphSetCount uint16 `gen:"omit()"`
	coverageOffsets   opentype.Slice[opentype.Offset32[CoverageTable]]
}
