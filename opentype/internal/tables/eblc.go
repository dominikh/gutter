// SPDX-FileCopyrightText: 2024 Dominik Honnef and contributors
//
// SPDX-License-Identifier: MIT

package tables

import "honnef.co/go/gutter/opentype"

type EBLCTable struct {
	data []byte

	MajorVersion uint16 `gen:"2"`
	MinorVersion uint16
	numSizes     uint32 `gen:"omit()"`
	bitmapSizes  opentype.Slice[BitmapSizeRecord]
}

type BitmapSizeRecord struct {
	parentData []byte

	indexSubtableListOffset uint32 `gen:"omit()"`
	indexSubtableListSize   uint32 `gen:"omit()"`
	numberOfIndexSubtables  uint32 `gen:"omit()"`
	_colorRef               uint32
	Hori                    SbitLineMetricsRecord
	Vert                    SbitLineMetricsRecord
	StartGlyphIndex         uint16
	EndGlyphIndex           uint16
	PpemX                   uint8
	PpemY                   uint8
	BitDepth                uint8
	Flags                   int8

	// XXX IndexSubtableRecord has offsets relative to indexSubtableListOffset, so we need
	// to set its parentData to this.parentData[indexSubtableListOffset:]
	indexSubtableRecords opentype.Slice[IndexSubtableRecord] `gen:"slice(offset=indexSubtableListOffset,count=numberOfIndexSubtables)"`
}

type SbitLineMetricsRecord struct {
	Ascender              int8
	Descender             int8
	WidthMax              uint8
	CaretSlopeNumerator   int8
	CaretSlopeDenominator int8
	CaretOffset           int8
	MinOriginSB           int8
	MinAdvanceSB          int8
	MaxBeforeBL           int8
	MinAfterBL            int8
	_pad1                 int8
	_pad2                 int8
}

type IndexSubtableRecord struct {
	parentData []byte

	FirstGlyphIndex     uint16
	LastGlyphIndex      uint16
	indexSubtableOffset opentype.Offset32[IndexSubtable]
}

type IndexSubtable struct {
	IndexFormat     uint16
	ImageFormat     uint16
	ImageDataOffset uint32

	Format1 IndexSubtableFormat1 `gen:"union(IndexFormat, 1)"`
	Format2 IndexSubtableFormat2 `gen:"union(IndexFormat, 2)"`
	Format3 IndexSubtableFormat3 `gen:"union(IndexFormat, 3)"`
	Format4 IndexSubtableFormat4 `gen:"union(IndexFormat, 4)"`
	Format5 IndexSubtableFormat5 `gen:"union(IndexFormat, 5)"`
}

type IndexSubtableFormat1 struct {
	sbitOffsets opentype.Slice[uint32] `gen:"slice(count=-1)"`
}

type IndexSubtableFormat2 struct {
	ImageSize  uint32
	BigMetrics BigGlyphMetricsRecord
}

type IndexSubtableFormat3 struct {
	sbitOffsets opentype.Slice[uint16] `gen:"slice(count=-1)"`
}

type IndexSubtableFormat4 struct {
	numGlyphs uint32                                  `gen:"omit()"`
	glyphs    opentype.Slice[GlyphIDOffsetPairRecord] `gen:"slice(count=numGlyphs+1)"`
}

type GlyphIDOffsetPairRecord struct {
	GlyphID    uint16
	SbitOffset uint16
}

type IndexSubtableFormat5 struct {
	ImageSize  uint32
	BigMetrics BigGlyphMetricsRecord
	numGlyphs  uint32 `gen:"omit()"`
	glyphIDs   opentype.Slice[uint16]
}

type BigGlyphMetricsRecord struct {
	Height       uint8
	Width        uint8
	HoribearingX int8
	HoriBearingY int8
	HoriAdvance  uint8
	VertBearingX int8
	VertBearingY int8
	VertAdvance  uint8
}
