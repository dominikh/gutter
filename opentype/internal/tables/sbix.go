// SPDX-FileCopyrightText: 2024 Dominik Honnef and contributors
//
// SPDX-License-Identifier: MIT

package tables

import "honnef.co/go/gutter/opentype"

type SbixTable struct {
	Version       uint16
	Flags         uint16
	numStrikes    uint32 `gen:"omit()"`
	strikeOffsets opentype.Slice[opentype.Offset32[Strike]]
}

type Strike struct {
	Ppem uint16
	PPI  uint16

	// maxp.numGlyphs+1 entries. The opentype.Slice may contain arbitrary trailing data. Offsets
	// are relative to Strike.
	glyphDataOffsets opentype.Slice[opentype.Offset32[GlyphData]] `gen:"slice(count=-1)"`
}

type GlyphData struct {
	OriginalOffsetX int16
	OriginalOffsetY int16
	// One of "jpg ", "png ", "tiff", or "dupe".
	GraphicType opentype.Tag
	// Actual length is determined by delta of offsets in Strike.glyphDataOffsets.
	Data []byte `gen:"slice(count=-1)"`
}
