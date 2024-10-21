// SPDX-FileCopyrightText: 2024 Dominik Honnef and contributors
//
// SPDX-License-Identifier: MIT

package tables

import "honnef.co/go/gutter/opentype"

type SVGTable struct {
	data []byte

	Version            uint16
	documentListOffset opentype.Offset32[SVGDocumentList]
	_reserved          uint32
}

type SVGDocumentList struct {
	data []byte

	numEntries      uint16 `gen:"omit()"`
	documentRecords opentype.Slice[SVGDocumentRecord]
}

type SVGDocumentRecord struct {
	parentData []byte

	StartGlyphID uint16
	EndGlyphID   uint16
	svgDocOffset uint32 `gen:"omit()"`
	svgDocLength uint32 `gen:"omit()"`
	Data         []byte `gen:"slice(offset=svgDocOffset,count=svgDocLength)"`
}
