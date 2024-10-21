// SPDX-FileCopyrightText: 2024 Dominik Honnef and contributors
//
// SPDX-License-Identifier: MIT

package tables

import "honnef.co/go/gutter/opentype"

type CmapTable struct {
	data []byte

	Version         uint16
	numTables       uint16 `gen:"omit()"`
	encodingRecords opentype.Slice[EncodingRecord]
}

type EncodingRecord struct {
	parentData []byte

	PlatformID     opentype.PlatformID
	EncodingID     opentype.EncodingID
	subtableOffset opentype.Offset32[CmapSubtable]
}

type CmapSubtable struct {
	Format  uint16
	Format0 CmapSubtableFormat0 `gen:"union(Format,0)"`
	// TODO(dh): Format 2 is silly and I don't care enough to support it at this point. It
	// is, however, required for Big5 and ShiftJIS.
	Format4 CmapSubtableFormat4 `gen:"union(Format,4)"`
	Format6 CmapSubtableFormat6 `gen:"union(Format,6)"`
	// TODO(dh): Format 8 allows for mixing 16-bit and 32-bit but isn't commonly used or
	// supported. Harfbuzz doesn't support it, for example.
	Format10 CmapSubtableFormat10 `gen:"union(Format,10)"`
	Format12 CmapSubtableFormat12 `gen:"union(Format,12)"`
	Format13 CmapSubtableFormat13 `gen:"union(Format,13)"`
	Format14 CmapSubtableFormat14 `gen:"union(Format,14)"`
}

// CmapSubtableFormat0 maps 8-bit character codes to glyph indices.
type CmapSubtableFormat0 struct {
	Length   uint16
	Language uint16
	// XXX it is not clear if GlyphIDs is guaranteed to be 256 bytes long, or if Length
	// can cap it
	GlyphIDs *[256]byte
}

// CmapSubtableFormat4 maps 16-bit characters to glyph indices, supporting multiple
// segments. Used for Unicode BMP.
type CmapSubtableFormat4 struct {
	Length         uint16
	Language       uint16
	segCountX2     uint16 `gen:"omit()"`
	_searchRange   uint16
	_entrySelector uint16
	_rangeShift    uint16
	endCodes       opentype.Slice[uint16] `gen:"slice(count=segCountX2/2)"`
	_reserved      uint16
	startCodes     opentype.Slice[uint16] `gen:"slice(count=segCountX2/2)"`
	idDeltas       opentype.Slice[int16]  `gen:"slice(count=segCountX2/2,singular=IDDelta)"`
	idRangeOffsets opentype.Slice[uint16] `gen:"slice(count=segCountX2/2,singular=IDRangeOffset)"`
	glyphIDs       opentype.Slice[uint16] `gen:"slice(count=-1)"`
}

// CmapSubtableFormat6 maps 16-bit characters to glyph indices, supporting a single
// segment.
type CmapSubtableFormat6 struct {
	Length     uint16
	Language   uint16
	FirstCode  uint16
	entryCount uint16 `gen:"omit()"`
	glyphIDs   opentype.Slice[uint16]
}

type SequentialMapGroupRecord struct {
	StartCharCode uint32
	EndCharCode   uint32
	StartGlyphID  uint32
}

// CmapSubtableFormat10 is the 32-bit equivalent of CmapSubtableFormat6.
type CmapSubtableFormat10 struct {
	_reserved     uint16
	Length        uint32
	Language      uint32
	StartCharCode uint32
	numChars      uint32 `gen:"omit()"`
	glyphIDs      opentype.Slice[uint16]
}

// CmapSubtableFormat12 maps 32-bit characters to glyph indices, supporting multiple
// segments. Used for the full range of Unicode.
type CmapSubtableFormat12 struct {
	_reserved uint16
	Length    uint32
	Language  uint32
	numGroups uint32 `gen:"omit()"`
	groups    opentype.Slice[SequentialMapGroupRecord]
}

// CmapSubtableFormat13 is similar to CmapSubtableFormat12, but is used for mapping many
// characters to the same glyphs.
type CmapSubtableFormat13 struct {
	_reserved uint16
	Length    uint32
	Language  uint32
	numGroups uint32 `gen:"omit()"`
	groups    opentype.Slice[ConstantMapGroupRecord]
}

type ConstantMapGroupRecord struct {
	StartCharCode uint32
	EndCharCode   uint32
	GlyphID       uint32
}

// CmapSubtableFormat14 maps Unicode variation selector sequences to glyphs.
type CmapSubtableFormat14 struct {
	Length                uint32
	numVarSelectorRecords uint32 `gen:"omit()"`
	varSelectors          opentype.Slice[VariationSelectorRecord]
}

type VariationSelectorRecord struct {
	VarSelector         opentype.Uint24
	DefaultUVSOffset    opentype.Offset32[DefaultUVSTable]
	NonDefaultUVSOffset opentype.Offset32[NonDefaultUVSTable]
}

type DefaultUVSTable struct {
	numUnicodeValueRanges uint32 `gen:"omit()"`
	ranges                opentype.Slice[UnicodeRangeRecord]
}

type UnicodeRangeRecord struct {
	StartUnicodeValue opentype.Uint24
	AdditionalCount   uint8
}

type NonDefaultUVSTable struct {
	numUVSMappings uint32 `gen:"omit()"`
	uvsMappings    opentype.Slice[UVSMappingRecord]
}

type UVSMappingRecord struct {
	UnicodeValue opentype.Uint24
	GlyphID      uint16
}
